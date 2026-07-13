// Package urlservice implements the core URL shortener business logic.
// It depends only on domain types and repository interfaces — no gRPC,
// HTTP, SQL, or Redis packages are imported here. This isolation means
// business logic can be unit-tested with simple in-memory stubs, and
// switching transports or storage backends does not require touching this
// layer.
package urlservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ducanng/URLShortener/internal/domain"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/repository"
	"github.com/ducanng/URLShortener/internal/shortcode"
)

// CreateParams holds the decoded request fields for CreateURL. All types
// are pure Go — proto/HTTP decoding happens in the transport adapter
// before constructing this struct.
type CreateParams struct {
	OriginalURL    string
	NoExpire       bool
	ExplicitExpiry *time.Time // nil = "not provided by caller"
}

// URLService orchestrates URL shortening operations. It owns the
// business rules (expiry resolution, ID generation, cache-aside) while
// delegating persistence and caching to injected repository interfaces.
type URLService struct {
	log     *logger.Logger
	repo    repository.URLRepository
	cache   repository.Cache
	counter repository.IDGenerator
}

// New constructs a URLService. All arguments are required; passing nil
// will cause a nil-pointer panic on first use.
func New(
	log *logger.Logger,
	repo repository.URLRepository,
	cache repository.Cache,
	counter repository.IDGenerator,
) *URLService {
	return &URLService{log: log, repo: repo, cache: cache, counter: counter}
}

// CreateURL shortens originalURL and persists the entry.
//
// Error classification (for transport layer mapping):
//   - ErrNoExpireConflict / ErrExpiresAtPast  → InvalidArgument (caller's fault)
//   - counter.NextID failure                  → wrapped, transport maps to Unavailable
//   - repo.Save failure                       → wrapped, transport maps to Internal
//
// On success the returned domain.URLEntry is fully populated including
// the generated ShortedURL path. Cache failures are logged but do not
// fail the write (best-effort warm cache).
func (s *URLService) CreateURL(ctx context.Context, p CreateParams, now time.Time) (domain.URLEntry, error) {
	expiresAt, err := resolveExpiry(p.NoExpire, p.ExplicitExpiry, now)
	if err != nil {
		return domain.URLEntry{}, err
	}

	id, err := s.counter.NextID(ctx)
	if err != nil {
		return domain.URLEntry{}, fmt.Errorf("next ID: %w", err)
	}

	entry := domain.URLEntry{
		Id:          id,
		OriginalURL: p.OriginalURL,
		ShortedURL:  shortcode.ShortPathFromID(id),
		Clicks:      0,
		ExpiresAt:   expiresAt,
	}

	if err := s.repo.Save(ctx, entry); err != nil {
		return domain.URLEntry{}, fmt.Errorf("save: %w", err)
	}
	if err := s.cache.Set(ctx, entry); err != nil {
		s.log.WithContext(ctx).Warnf("cache set after create: %v", err)
	}
	return entry, nil
}

// GetURL retrieves the live (non-expired) entry for a base62 short key.
//
// Error classification:
//   - domain.ErrNotFound  → key does not exist (transport maps to NotFound / 404)
//   - domain.ErrExpired   → key exists but past expiry (transport maps to FailedPrecondition / 410)
//   - other errors        → storage fault (transport maps to Internal / 500)
//
// Cache-aside: Redis is consulted first; on miss, PostgreSQL is loaded
// and the result is re-cached best-effort.
func (s *URLService) GetURL(ctx context.Context, key string) (domain.URLEntry, error) {
	entry, err := s.cache.Get(ctx, key)
	if err != nil {
		entry, err = s.repo.Load(ctx, key)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return domain.URLEntry{}, domain.ErrNotFound
			}
			return domain.URLEntry{}, fmt.Errorf("load: %w", err)
		}
		if cacheErr := s.cache.Set(ctx, entry); cacheErr != nil {
			s.log.WithContext(ctx).Warnf("cache re-set after load: %v", cacheErr)
		}
	}

	if entry.ExpiresAt != nil && !time.Now().Before(*entry.ExpiresAt) {
		return domain.URLEntry{}, domain.ErrExpired
	}
	return entry, nil
}
