package http

import (
	"context"
	"net/http"
	"time"

	"github.com/ducanng/URLShortener/internal/repository/postgres"
	"github.com/ducanng/URLShortener/internal/repository/redis"

	"github.com/gin-gonic/gin"
)

// Pinger reports whether a backing store is reachable. /readyz probes the
// Postgres and Redis dependencies through this interface so the health
// handler stays decoupled from their concrete client types.
type Pinger interface {
	Ping(ctx context.Context) error
}

// pgPinger adapts a Postgres repository to Pinger. PingContext honours the
// ctx deadline, so a stuck DB is bounded by the /readyz timeout.
type pgPinger struct{ repo *postgres.Repo }

func (p pgPinger) Ping(ctx context.Context) error { return p.repo.Client.PingContext(ctx) }

// redisPinger adapts a Redis cache to Pinger. NOTE: go-redis v6 ignores ctx,
// so the ping is not deadline-bound — acceptable as it is sub-millisecond.
type redisPinger struct{ cache *redis.Cache }

func (p redisPinger) Ping(_ context.Context) error {
	_, err := p.cache.Client.Ping().Result()
	return err
}

// registerHealth wires the health endpoints onto the router.
//
// They are registered BEFORE the Prometheus middleware so they don't inflate
// request metrics. /healthz is a pure liveness probe (always 200 while the
// process is up); /readyz is a readiness probe that verifies the app can
// actually serve traffic by pinging Redis and Postgres. Used by Nginx passive
// HC, Docker healthcheck, and rolling-deploy gates.
func registerHealth(router *gin.Engine, db, cache Pinger) {
	router.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "component": "postgres", "error": err.Error()})
			return
		}
		if err := cache.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "component": "redis", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
