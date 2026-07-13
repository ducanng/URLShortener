package http

import (
	"context"
	"time"

	"github.com/ducanng/URLShortener/internal/config"
	"github.com/ducanng/URLShortener/internal/logger"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.uber.org/zap"
)

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

// prometheusMiddleware returns the Prometheus instrumentation middleware only
// — the /metrics endpoint is served on a separate port by the metrics
// package. Calling p.Use(router) would also mount /metrics on :8080, which we
// explicitly avoid.
func prometheusMiddleware() gin.HandlerFunc {
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
	return p.HandlerFunc()
}

// corsMiddleware builds the production-hardened CORS middleware.
//
// Origins come pre-resolved from config (CORS_ALLOWED_ORIGINS). When the list
// is empty the server falls back to localhost dev origins so local
// development keeps working without extra config. Never use "*" in
// production: it disables credentials and prevents cookie / Authorization
// header forwarding on cross-origin requests.
func corsMiddleware(cfg config.CORSConfig, log *logger.Logger) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: corsAllowedOrigins(cfg, log),
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
	})
}

// corsAllowedOrigins returns the CORS origin whitelist for the server.
//
// Origins are resolved by config from the CORS_ALLOWED_ORIGINS environment
// variable (comma-separated, e.g.
// CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com).
// When that list is empty the server falls back to a set of localhost dev
// origins so that local development works out of the box.
//
// "AllowOrigins: *" is intentionally never used: the wildcard disables
// AllowCredentials support (browsers reject it), leaks the API to arbitrary
// origins, and prevents cookie / Authorization header forwarding.
func corsAllowedOrigins(cfg config.CORSConfig, log *logger.Logger) []string {
	if len(cfg.AllowedOrigins) == 0 {
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

	log.Info("CORS allowed origins configured", zap.Strings("origins", cfg.AllowedOrigins))
	return cfg.AllowedOrigins
}
