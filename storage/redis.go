package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/domain"

	base62 "github.com/alextanhongpin/base62"
	"github.com/joho/godotenv"

	"github.com/go-redis/redis"
)

// Redis wraps a go-redis client and the process logger. The embedded *Logger
// gives every method the typed/printf/structured zap helpers without the
// caller juggling a separate dependency.
//
// NOTE: go-redis v6 does not accept context.Context on its API; we still
// thread ctx through every method to enrich logs with trace_id and to be
// drop-in ready when we upgrade to v9 (which is context-aware).
type Redis struct {
	*logger.Logger
	Client *redis.Client
}

// NewRedis dials Redis using REDIS_ADDR, verifies connectivity with PING,
// and returns a ready-to-use store. Loading .env is best-effort: missing
// file is fine when env vars are already set (e.g. inside Docker).
func NewRedis(log *logger.Logger) (*Redis, error) {
	if err := godotenv.Load(); err != nil {
		log.Warnf(".env not found, using environment variables: %v", err)
	}
	client := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "",
		DB:       0,
	})
	pong, err := client.Ping().Result()
	if err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	log.Infof("redis ping: %s", pong)
	return &Redis{Logger: log, Client: client}, nil
}

// Set caches the entry as JSON keyed by its numeric ID. Failure to write to
// Redis is non-fatal at the call site (treated as a warning) — callers
// decide how to react via the returned error.
//
// TTL policy: when entry.ExpiresAt is non-nil, the cache TTL is aligned
// with the remaining lifetime so Redis evicts the key at (or before) the
// real expiry. When ExpiresAt is nil ("never expires") TTL = 0 which means
// "no expiry" in Redis. If ExpiresAt is already in the past we skip the
// write — caching a known-expired value would only mislead Get callers.
func (s *Redis) Set(ctx context.Context, entry domain.URLEntry) error {
	log := s.WithContext(ctx)
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

	if _, err := s.Client.Set(strconv.FormatUint(uint64(entry.Id), 10), marshal, ttl).Result(); err != nil {
		log.Warnf("failed to set key-value to redis: %v", err)
		return err
	}
	return nil
}

// Get returns the cached entry for a base62-encoded short key. redis.Nil
// (cache miss) is returned to the caller without a log line, since the
// cache-aside pattern treats it as the normal fallback path.
func (s *Redis) Get(ctx context.Context, key string) (domain.URLEntry, error) {
	log := s.WithContext(ctx)
	id := base62.Decode(key)
	val, err := s.Client.Get(strconv.FormatUint(id, 10)).Result()
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
// expressive (Set vs Update).
func (s *Redis) Update(ctx context.Context, entry domain.URLEntry) error {
	return s.Set(ctx, entry)
}

// NextID atomically increments the global url counter and returns the new
// value. This is a critical path — callers must treat any error as a hard
// failure (return Unavailable to upstream) since IDs cannot be reissued.
func (s *Redis) NextID(ctx context.Context) (int64, error) {
	_ = ctx // reserved for go-redis v9 upgrade
	val, err := s.Client.Incr("url:counter").Result()
	if err != nil {
		return 0, fmt.Errorf("redis NextID: %w", err)
	}
	return val, nil
}

// InitCounter seeds url:counter to start only if the key does not yet exist
// (SET NX). Safe to call on every startup — it will not overwrite a live
// counter, only repopulate one lost to a Redis flush.
func (s *Redis) InitCounter(ctx context.Context, start int64) error {
	log := s.WithContext(ctx)
	ok, err := s.Client.SetNX("url:counter", start, 0).Result()
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
