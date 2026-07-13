package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ducanng/URLShortener/client"
	_ "github.com/ducanng/URLShortener/docs"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/metrics"
	"github.com/ducanng/URLShortener/internal/repository/postgres"
	"github.com/ducanng/URLShortener/internal/repository/redis"
	"github.com/ducanng/URLShortener/internal/service/urlservice"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"
	"github.com/ducanng/URLShortener/server"
	"github.com/gin-contrib/cors"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ShortenRequest represents the JSON body for creating a short URL.
// expires_at and no_expire are optional. Defaults to a 30-day TTL when
// neither is provided.
type ShortenRequest struct {
	URL       string `json:"url" example:"https://example.com"`
	ExpiresAt string `json:"expires_at,omitempty" example:"2026-12-31T00:00:00Z"`
	NoExpire  bool   `json:"no_expire,omitempty" example:"false"`
}

// URLResponse represents the JSON response from grpc-gateway.
type URLResponse struct {
	Message string        `json:"message" example:"Create short url"`
	Status  string        `json:"status" example:"Success"`
	URL     *ShortenedURL `json:"url,omitempty"`
}

// ShortenedURL represents URL details in the response. ExpiresAt is
// omitted when the short URL never expires.
type ShortenedURL struct {
	OriginalURL  string `json:"originalURL" example:"https://example.com"`
	ShortenedURL string `json:"shortenedURL" example:"http://localhost:8080/abc123"`
	Clicks       int32  `json:"clicks" example:"0"`
	ExpiresAt    string `json:"expires_at,omitempty" example:"2026-12-31T00:00:00Z"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Message string `json:"message" example:"URL not found"`
}

// newHTTPServer sets up the Gin router with grpc-gateway, Swagger, Prometheus
// instrumentation, the trace-id middleware, and the redirect handler. The
// /metrics endpoint is NOT mounted here — it lives on the dedicated metrics
// server (port 7070) for security and load isolation.
func newHTTPServer(ctx context.Context, log *logger.Logger, grpcClient *client.Client, cache *redis.Cache, pgRepo *postgres.Repo) *http.Server {
	// grpc-gateway: REST → gRPC reverse proxy, generated from proto annotations.
	// runtime.WithMetadata forwards the X-Trace-Id HTTP header into gRPC
	// metadata so the server-side trace interceptor can pick it up and keep
	// the trace_id stable across the gateway boundary.
	gwMux := runtime.NewServeMux(
		runtime.WithMetadata(func(_ context.Context, r *http.Request) metadata.MD {
			if id := r.Header.Get(logger.TraceHeader); id != "" {
				return metadata.Pairs("x-trace-id", id)
			}
			return nil
		}),
	)
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := urlshortenerpb.RegisterURLShortenerServiceHandlerFromEndpoint(
		ctx, gwMux, "localhost:50051", dialOpts,
	); err != nil {
		log.Fatalf("Failed to register grpc-gateway: %v", err)
	}

	router := gin.Default()

	// Health endpoints — registered BEFORE Prometheus middleware so they
	// don't inflate request metrics. /healthz is a pure liveness probe
	// (always 200 while the process is up); /readyz is a readiness probe
	// that verifies the app can actually serve traffic by pinging Redis
	// and Postgres. Used by Nginx passive HC, Docker healthcheck, and
	// rolling-deploy gates.
	router.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pgRepo.Client.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "component": "postgres", "error": err.Error()})
			return
		}
		if _, err := cache.Client.Ping().Result(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "component": "redis", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Trace-ID first so every downstream middleware/handler logs with it.
	router.Use(logger.TraceIDMiddleware())

	// Prometheus instrumentation middleware only — the /metrics endpoint is
	// served on a separate port by the metrics package. Calling p.Use(router)
	// would also mount /metrics on :8080, which we explicitly avoid.
	p := ginprometheus.NewPrometheus("gin")
	p.ReqCntURLLabelMappingFn = func(c *gin.Context) string {
		// Use Gin's registered route pattern (e.g. "/:path", "/info/:path")
		// instead of the raw URL path to prevent cardinality explosion when
		// thousands of unique short-URLs each create a distinct time series.
		if fp := c.FullPath(); fp != "" {
			return fp
		}
		return c.Request.URL.Path
	}
	router.Use(p.HandlerFunc())

	// CORS middleware — production-hardened.
	//
	// Origins are read from CORS_ALLOWED_ORIGINS (comma-separated list).
	// When the env var is absent the server falls back to localhost dev
	// origins so local development keeps working without extra config.
	// Never use "*" in production: it disables credentials and prevents
	// cookie / Authorization header forwarding on cross-origin requests.
	allowedOrigins := corsAllowedOrigins(log)
	router.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Requested-With",
			"X-Trace-Id",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"X-Trace-Id",
		},
		// AllowCredentials lets browsers send cookies / Authorization
		// headers on cross-origin requests. Requires explicit origins
		// (not "*") — gin-contrib/cors enforces this automatically.
		AllowCredentials: true,
		// Cache preflight response for 24 h to cut OPTIONS round-trips.
		MaxAge: 24 * time.Hour,
	}))

	// Per-request deadline — the PRIMARY work-cancellation mechanism. It
	// propagates through grpc-gateway / the gRPC client (as a grpc-timeout
	// header) into the server ctx and on to Postgres (ExecContext /
	// QueryRowContext honour it), so a stuck backend call is cancelled and
	// the goroutine freed instead of leaking under load. The http.Server
	// Read/Write timeouts below are only a coarse socket-level backstop.
	// Sized to 5s: > the worst write-path latency observed under peak load
	// (~3.2s during a Postgres burst) plus margin, so legitimate traffic is
	// not cut. NOTE: go-redis v6 ignores ctx, so Redis ops are not
	// cancelled — acceptable as they are sub-millisecond (revisit on v9).
	router.Use(requestTimeout(5 * time.Second))

	router.POST("/shorted", handleCreateURL(gwMux))
	router.GET("/info/:path", handleGetURL(gwMux))

	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/index.html")
	})
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/:path", handleRedirect(grpcClient))

	// These socket-level timeouts are a COARSE BACKSTOP only. The real
	// work-cancellation mechanism is the per-request context deadline
	// (requestTimeout middleware, 5s) which actually aborts backend calls
	// and frees goroutines. WriteTimeout cannot do that — it only fails
	// the I/O write; the handler goroutine keeps running until it next
	// touches the socket. So these are sized generously, above the
	// client-facing Nginx timeouts, to catch only slow-reading clients and
	// stuck writes that the context deadline can't.
	//
	// Ordering invariant: context 5s < Nginx proxy 10s < Go Write 15s.
	// Context fires first (stops work), Nginx is the client-facing bound,
	// Go socket timeouts are the last-resort net.
	//
	//   Axis     Nginx                      Go              Note
	//   ─────────────────────────────────────────────────────────────
	//   Idle     keepalive_timeout 60s   →  IdleTimeout 75s   Go > Nginx
	//   Read     proxy_send_timeout 10s  →  ReadTimeout 10s   backstop
	//   Write    proxy_read_timeout 10s  →  WriteTimeout 15s  backstop
	//
	// Idle axis: Nginx closes idle keepalive first → sends FIN to Go →
	// Go cleans up passively. Go's 75s > Nginx 60s keeps Nginx the closer
	// and avoids the half-open-socket race.
	//
	// ReadHeaderTimeout (5s) caps slow-loris attacks where a client
	// dribbles headers one byte per second; cheaper than ReadTimeout
	// because it only arms a deadline during the header phase. (The edge
	// equivalent, Nginx client_header_timeout, is the real front-door
	// defence — see nginx/nginx.conf.)
	return &http.Server{
		Addr:              ":8080",
		Handler:           router.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       75 * time.Second,
	}
}

// handleCreateURL godoc
// @Summary      Create shortened URL
// @Description  Create a shortened URL from the original URL via grpc-gateway
// @Tags         shorten
// @Accept       json
// @Produce      json
// @Param        body body ShortenRequest true "Original URL"
// @Success      200 {object} URLResponse
// @Failure      500 {object} ErrorResponse
// @Router       /shorted [post]
func handleCreateURL(gwMux *runtime.ServeMux) gin.HandlerFunc {
	return gin.WrapH(gwMux)
}

// handleGetURL godoc
// @Summary      Get URL info
// @Description  Get info of a shortened URL (original URL, clicks count)
// @Tags         info
// @Produce      json
// @Param        path path string true "Short URL path" example("abc123")
// @Success      200 {object} URLResponse
// @Failure      404 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /info/{path} [get]
func handleGetURL(gwMux *runtime.ServeMux) gin.HandlerFunc {
	return gin.WrapH(gwMux)
}

// handleRedirect godoc
// @Summary      Redirect to original URL
// @Description  Redirect (HTTP 301) to the original URL using the short path
// @Tags         redirect
// @Param        path path string true "Short URL path" example("abc123")
// @Success      301 "Moved Permanently"
// @Failure      404 {object} ErrorResponse
// @Failure      410 {object} ErrorResponse
// @Router       /{path} [get]
func handleRedirect(grpcClient *client.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		res, err := grpcClient.CC.GetURL(c.Request.Context(), &urlshortenerpb.GetURLRequest{URL: path})
		if err != nil {
			// Distinguish expired (410 Gone) from real not-found (404).
			// FailedPrecondition is the gRPC code chosen for "exists but
			// expired" — see server.GetURL.
			if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
				c.JSON(http.StatusGone, gin.H{"message": "URL expired"})
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"message": "URL not found"})
			return
		}
		if res.GetStatus() == "Failed" {
			c.JSON(http.StatusNotFound, gin.H{"message": "URL not found"})
			return
		}
		c.Redirect(http.StatusMovedPermanently, res.GetUrl().GetOriginalURL())
	}
}

// requestTimeout returns a Gin middleware that attaches a deadline to every
// request's context. The deadline propagates downstream (grpc-gateway → gRPC
// client → server ctx → Postgres) so slow backend calls are cancelled and
// their goroutines freed, rather than piling up under load. cancel() is
// deferred so the timer is always released when the handler chain returns.
func requestTimeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// @title URL Shortener API
// @description This is a server for URL Shortener API.
// @version 1.5.0
// @BasePath /
// @schemes http https
// @host localhost:8080
// @securityDefinitions.basic  BasicAuth
func RunServer() {
	// Initialise the structured logger first; everything else depends on it
	// for error reporting. Failure here is fatal — no point continuing
	// without observability.
	log, err := logger.New(logger.Config{Level: zapcore.InfoLevel})
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// Redirect the stdlib log package so any third-party library that uses
	// `log.Printf` also emits structured JSON.
	zap.RedirectStdLog(log.Logger)

	// Context that cancels on SIGINT / SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Storage. Constructors return errors so main owns the fatal decision.
	cache, err := redis.NewCache(log)
	if err != nil {
		log.Fatalf("init redis cache: %v", err)
	}
	counter, err := redis.NewCounter(log)
	if err != nil {
		log.Fatalf("init redis counter: %v", err)
	}
	pgRepo, err := postgres.New(log)
	if err != nil {
		log.Fatalf("init postgres: %v", err)
	}

	// Seed the Redis global counter from PG MAX(id) — uses SET NX so it is a
	// no-op when the counter already exists (normal restart). Fail fast when
	// Redis is unreachable because ID generation depends on it.
	maxID, err := pgRepo.MaxID(ctx)
	if err != nil {
		log.Fatalf("read MAX(id) from DB: %v", err)
	}
	if err := counter.InitCounter(ctx, maxID+1); err != nil {
		log.Fatalf("init Redis counter: %v", err)
	}

	// Service layer — business logic is isolated here; transport adapters
	// (gRPC, HTTP) call into it via the URLService methods.
	svc := urlservice.New(log, pgRepo, cache, counter)

	// gRPC server
	grpcServer, err := server.NewGRPCServer(log, svc)
	if err != nil {
		log.Fatalf("init grpc server: %v", err)
	}

	// gRPC client (used by HTTP gateway + redirect handler)
	grpcClient, cleanup, err := client.NewClient("localhost:50051", log)
	if err != nil {
		log.Fatalf("init grpc client: %v", err)
	}
	defer cleanup()

	// HTTP server (Gin + grpc-gateway + Swagger + Prometheus middleware + trace_id)
	httpSrv := newHTTPServer(ctx, log, grpcClient, cache, pgRepo)

	// Dedicated metrics server on :7070 — isolated from public :8080.
	metricsSrv := metrics.NewServer(metrics.DefaultAddr)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error { return grpcServer.ListenAndServe() })

	g.Go(func() error {
		log.Info("HTTP gateway starting on :8080")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		log.Infof("Metrics server starting on %s", metrics.DefaultAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	// Graceful shutdown
	g.Go(func() error {
		<-gCtx.Done()
		log.Info("Shutting down servers...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Errorf("HTTP shutdown error: %v", err)
		}
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			log.Errorf("Metrics shutdown error: %v", err)
		}

		grpcShutdownCtx, grpcCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer grpcCancel()
		grpcServer.Shutdown(grpcShutdownCtx)

		log.Info("All servers stopped gracefully")
		return nil
	})

	if err := g.Wait(); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}

// corsAllowedOrigins returns the CORS origin whitelist for the server.
//
// It reads the CORS_ALLOWED_ORIGINS environment variable, which must be a
// comma-separated list of fully-qualified origins, e.g.:
//
//	CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
//
// When the variable is absent or empty the server falls back to a set of
// localhost dev origins so that local development works out of the box
// without any extra configuration.
//
// "AllowOrigins: *" is intentionally never used: the wildcard disables
// AllowCredentials support (browsers reject it), leaks the API to arbitrary
// origins, and prevents cookie / Authorization header forwarding.
func corsAllowedOrigins(log *logger.Logger) []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		devOrigins := []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
		log.Warn("CORS_ALLOWED_ORIGINS is not set — falling back to localhost dev origins. Set this variable in production.",
			zap.Strings("origins", devOrigins),
		)
		return devOrigins
	}

	seen := make(map[string]struct{})
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if _, dup := seen[o]; dup {
			continue
		}
		seen[o] = struct{}{}
		origins = append(origins, o)
	}

	log.Info("CORS allowed origins configured", zap.Strings("origins", origins))
	return origins
}

func main() {
	RunServer()
}
