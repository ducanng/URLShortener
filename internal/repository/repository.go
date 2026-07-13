// Package repository declares the data-access contracts consumed by the
// service layer. Implementations live in child packages (postgres/, redis/)
// and are wired in main; the service layer only ever sees these interfaces.
//
// Splitting the store into three narrow interfaces keeps future scale-out
// options open: a read-only replica can implement URLRepository without
// touching the write path, a Redis cluster can implement Cache without
// owning the ID counter, and a KV counter can implement IDGenerator
// without knowing anything about the URL entity.
package repository

import (
	"context"

	"github.com/ducanng/URLShortener/internal/domain"
)

// URLRepository is the primary source of truth for URL entries. Save is
// the write path; Load/MaxID/UpdateClicks are read/maintenance paths.
// Implementations return sql.ErrNoRows (or a wrapping error) on cache miss
// so the service layer can distinguish "not found" from "internal error".
type URLRepository interface {
	Save(ctx context.Context, entry domain.URLEntry) error
	Load(ctx context.Context, key string) (domain.URLEntry, error)
	MaxID(ctx context.Context) (int64, error)
	UpdateClicks(ctx context.Context, path string, click int32)
}

// Cache is the ephemeral fast-path lookup. Get returns a miss-signalling
// error (implementation-specific, e.g. redis.Nil) that callers treat as a
// normal cache-miss without alarming logs.
type Cache interface {
	Get(ctx context.Context, key string) (domain.URLEntry, error)
	Set(ctx context.Context, entry domain.URLEntry) error
}

// IDGenerator issues monotonically increasing IDs used by the base62
// short-code generator. InitCounter is only invoked at startup to reseed
// the counter after a data-loss event; NextID is on the hot write path
// and must be treated as critical (failure = 503 to upstream).
type IDGenerator interface {
	NextID(ctx context.Context) (int64, error)
	InitCounter(ctx context.Context, start int64) error
}
