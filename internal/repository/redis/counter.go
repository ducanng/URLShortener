package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis"

	"github.com/ducanng/URLShortener/internal/config"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/repository"
)

// Compile-time proof that *Counter satisfies the repository.IDGenerator contract.
var _ repository.IDGenerator = (*Counter)(nil)

// Counter is the Redis-backed global monotonic ID generator used to feed
// the base62 short-code encoder. It intentionally lives in its own struct
// (separate from Cache) so the two responsibilities can be split later —
// e.g. moving Counter to a dedicated snowflake service — without dragging
// the URL entry cache along.
type Counter struct {
	*logger.Logger
	Client *redis.Client
}

// NewCounter dials Redis using cfg.Addr and returns a ready-to-use counter.
// The connection is independent from the Cache client so the two can be
// sharded onto different Redis nodes when scale requires it.
func NewCounter(cfg config.RedisConfig, log *logger.Logger) (*Counter, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: "",
		DB:       0,
	})
	if _, err := client.Ping().Result(); err != nil {
		return nil, fmt.Errorf("redis counter ping: %w", err)
	}
	return &Counter{Logger: log, Client: client}, nil
}

// NextID atomically increments the global url counter and returns the new
// value. This is a critical path — callers must treat any error as a hard
// failure (return Unavailable to upstream) since IDs cannot be reissued.
func (c *Counter) NextID(ctx context.Context) (int64, error) {
	_ = ctx // reserved for go-redis v9 upgrade
	val, err := c.Client.Incr("url:counter").Result()
	if err != nil {
		return 0, fmt.Errorf("redis NextID: %w", err)
	}
	return val, nil
}

// InitCounter seeds url:counter to start only if the key does not yet
// exist (SET NX). Safe to call on every startup — it will not overwrite a
// live counter, only repopulate one lost to a Redis flush.
func (c *Counter) InitCounter(ctx context.Context, start int64) error {
	log := c.WithContext(ctx)
	ok, err := c.Client.SetNX("url:counter", start, 0).Result()
	if err != nil {
		return fmt.Errorf("redis InitCounter: %w", err)
	}
	if ok {
		log.Infof("redis: url:counter initialised to %d", start)
	} else {
		log.Info("redis: url:counter already exists, skipping seed")
	}
	return nil
}
