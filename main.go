package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/ducanng/URLShortener/client"
	_ "github.com/ducanng/URLShortener/docs"
	"github.com/ducanng/URLShortener/logger"
	"github.com/ducanng/URLShortener/metrics"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"
	"github.com/ducanng/URLShortener/server"
	"github.com/ducanng/URLShortener/storage"
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
func newHTTPServer(ctx context.Context, log *logger.Logger, grpcClient *client.Client, redisStore *storage.Redis, sqlStore *storage.SQLStore) *http.Server {
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
		if err := sqlStore.Client.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "component": "postgres", "error": err.Error()})
			return
		}
		if _, err := redisStore.Client.Ping().Result(); err != nil {
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

	router.POST("/shorted", handleCreateURL(gwMux))
	router.GET("/info/:path", handleGetURL(gwMux))

	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/index.html")
	})
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/:path", handleRedirect(grpcClient))

	return &http.Server{
		Addr:    ":8080",
		Handler: router.Handler(),
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
	redisStore, err := storage.NewRedis(log)
	if err != nil {
		log.Fatalf("init redis: %v", err)
	}
	sqlStore, err := storage.NewSQLStore(log)
	if err != nil {
		log.Fatalf("init sql: %v", err)
	}

	// Seed the Redis global counter from PG MAX(id) — uses SET NX so it is a
	// no-op when the counter already exists (normal restart). Fail fast when
	// Redis is unreachable because ID generation depends on it.
	maxID, err := sqlStore.MaxID(ctx)
	if err != nil {
		log.Fatalf("read MAX(id) from DB: %v", err)
	}
	if err := redisStore.InitCounter(ctx, maxID+1); err != nil {
		log.Fatalf("init Redis counter: %v", err)
	}

	// gRPC server
	grpcServer, err := server.NewGRPCServer(log, redisStore, sqlStore)
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
	httpSrv := newHTTPServer(ctx, log, grpcClient, redisStore, sqlStore)

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

func main() {
	RunServer()
}
