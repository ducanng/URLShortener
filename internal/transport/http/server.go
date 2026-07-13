package http

import (
	"context"
	"net/http"
	"time"

	"github.com/ducanng/URLShortener/internal/config"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/repository/postgres"
	"github.com/ducanng/URLShortener/internal/repository/redis"
	grpctransport "github.com/ducanng/URLShortener/internal/transport/grpc"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// NewServer sets up the Gin router with grpc-gateway, Swagger, Prometheus
// instrumentation, the trace-id middleware, health probes, and the redirect
// handler, and wraps it in an *http.Server with production timeouts. The
// /metrics endpoint is NOT mounted here — it lives on the dedicated metrics
// server (port 7070) for security and load isolation.
func NewServer(ctx context.Context, cfg config.HTTPConfig, corsCfg config.CORSConfig, grpcClient *grpctransport.Client, cache *redis.Cache, pgRepo *postgres.Repo, log *logger.Logger) *http.Server {
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
		ctx, gwMux, cfg.GRPCEndpoint, dialOpts,
	); err != nil {
		log.Fatalf("Failed to register grpc-gateway: %v", err)
	}

	router := gin.Default()

	// Health endpoints — registered before the Prometheus middleware so they
	// don't inflate request metrics.
	registerHealth(router, pgPinger{repo: pgRepo}, redisPinger{cache: cache})

	// Trace-ID first so every downstream middleware/handler logs with it.
	router.Use(logger.TraceIDMiddleware())

	// Prometheus instrumentation middleware only — the /metrics endpoint is
	// served on a separate port by the metrics package.
	router.Use(prometheusMiddleware())

	// CORS middleware — production-hardened, origins from CORS_ALLOWED_ORIGINS.
	router.Use(corsMiddleware(corsCfg, log))

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

	router.POST("/shorted", HandleCreateURL(gwMux))
	router.GET("/info/:path", HandleGetURL(gwMux))

	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/index.html")
	})
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/:path", HandleRedirect(grpcClient))

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
		Addr:              cfg.Addr,
		Handler:           router.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       75 * time.Second,
	}
}
