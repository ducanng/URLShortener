// Package redis holds all Redis-backed adapters. Two independent structs
// live here: Cache (URL entry look-ups, aligned TTLs) and Counter (global
// monotonic ID generator). They both use go-redis v6 but are decoupled at
// the interface level so future replacement (e.g. move Counter to a
// dedicated snowflake service) does not touch Cache.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	base62 "github.com/alextanhongpin/base62"
	"github.com/go-redis/redis"

	"github.com/ducanng/URLShortener/internal/config"
	"github.com/ducanng/URLShortener/internal/domain"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/repository"
)

// Compile-time proof that *Cache satisfies the repository.Cache contract.
var _ repository.Cache = (*Cache)(nil)

// Cache is the JSON-serialising URL entry cache backed by Redis.
//
// go-redis v6 does not accept context.Context on its API; we still thread
// ctx through every method to enrich logs with trace_id and to be
// drop-in ready when we upgrade to v9 (which is context-aware).
type Cache struct {
	*logger.Logger
	Client *redis.Client
}

// NewCache dials Redis using cfg.Addr, verifies connectivity with PING, and
// returns a ready-to-use cache.
func NewCache(cfg config.RedisConfig, log *logger.Logger) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: "",
		DB:       0,
	})
	pong, err := client.Ping().Result()
	if err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	log.Infof("redis ping: %s", pong)
	return &Cache{Logger: log, Client: client}, nil
}

// Set caches the entry as JSON keyed by its numeric ID. Failure to write
// to Redis is non-fatal at the call site (treated as a warning) — callers
// decide how to react via the returned error.
//
// TTL policy: when entry.ExpiresAt is non-nil, the cache TTL is aligned
// with the remaining lifetime so Redis evicts the key at (or before) the
// real expiry. When ExpiresAt is nil ("never expires") TTL = 0 which means
// "no expiry" in Redis. If ExpiresAt is already in the past we skip the
// write — caching a known-expired value would only mislead Get callers.
func (c *Cache) Set(ctx context.Context, entry domain.URLEntry) error {
	log := c.WithContext(ctx)
	marshal, err := json.Marshal(entry)
	if err != nil {
		log.Errorf("marshaling entry for redis: %v", err)
		return err
	}

	var ttl time.Duration
	if entry.ExpiresAt != nil {
		ttl = time.Until(*entry.ExpiresAt)
		if ttl <= 0 {
			return nil
		}
	}

	if _, err := c.Client.Set(strconv.FormatUint(uint64(entry.Id), 10), marshal, ttl).Result(); err != nil {
		log.Warnf("failed to set key-value to redis: %v", err)
		return err
	}
	return nil
}

// Get returns the cached entry for a base62-encoded short key. redis.Nil
// (cache miss) is returned to the caller without a log line, since the
// cache-aside pattern treats it as the normal fallback path.
func (c *Cache) Get(ctx context.Context, key string) (domain.URLEntry, error) {
	log := c.WithContext(ctx)
	id := base62.Decode(key)
	val, err := c.Client.Get(strconv.FormatUint(id, 10)).Result()
	if err != nil {
		if err != redis.Nil {
			log.Warnf("failed to get key-value from redis: %v", err)
		}
		return domain.URLEntry{}, err
	}
	var entry domain.URLEntry
	_ = json.Unmarshal([]byte(val), &entry)
	return entry, nil
}

// Update refreshes a cached entry. Delegates to Set so the TTL policy
// stays in one place; kept as a distinct method to keep call sites
// expressive (Set vs Update). Not part of the repository.Cache interface —
// callers that must update can call Set directly.
func (c *Cache) Update(ctx context.Context, entry domain.URLEntry) error {
	return c.Set(ctx, entry)
}
