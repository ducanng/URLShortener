// Package postgres implements repository.URLRepository against PostgreSQL.
// The package owns the *sql.DB lifecycle plus the connection-pool sizing
// tuned for the current 8-vCPU host. It never calls Fatal — errors are
// returned to main which decides whether to abort.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	base62 "github.com/alextanhongpin/base62"
	_ "github.com/lib/pq"

	"github.com/ducanng/URLShortener/internal/config"
	"github.com/ducanng/URLShortener/internal/domain"
	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/repository"
)

// Compile-time proof that *Repo satisfies the URLRepository contract.
// Break-the-build check: if a method drifts, the compiler complains here
// instead of failing at runtime when the service tries to use it.
var _ repository.URLRepository = (*Repo)(nil)

// Repo is the PostgreSQL-backed URLRepository implementation. The embedded
// *logger.Logger keeps method call sites terse (r.Errorf vs r.log.Errorf).
type Repo struct {
	*logger.Logger
	Client *sql.DB
}

// New opens a *sql.DB against cfg.DSN, sizes the connection pool for an
// 8-vCPU SSD host, and returns the wrapper. Schema is NOT created here — run
// `make migrate-up` before starting the app for the first time.
//
// Connection pool: (CPU×2)+spindle = 17 for an 8-vCPU SSD machine. Tune
// SetMaxOpenConns/SetMaxIdleConns when running on smaller / larger hosts.
func New(cfg config.DBConfig, log *logger.Logger) (*Repo, error) {
	client, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	client.SetMaxOpenConns(17)
	client.SetMaxIdleConns(17)
	client.SetConnMaxLifetime(5 * time.Minute)
	client.SetConnMaxIdleTime(1 * time.Minute)

	return &Repo{Logger: log, Client: client}, nil
}

// Save inserts a new URLEntry. Uses ExecContext so the query honours the
// caller's deadline / cancellation. ExpiresAt is a pointer so passing nil
// stores SQL NULL — meaning the entry never expires.
func (r *Repo) Save(ctx context.Context, entry domain.URLEntry) error {
	log := r.WithContext(ctx)
	_, err := r.Client.ExecContext(
		ctx,
		"INSERT INTO url_list (id, original_url, shorted_url, clicks, expires_at) VALUES ($1, $2, $3, $4, $5)",
		entry.Id,
		entry.OriginalURL,
		entry.ShortedURL,
		entry.Clicks,
		entry.ExpiresAt,
	)
	if err != nil {
		log.Errorf("inserting to db: %v", err)
	}
	return err
}

// Load reads the row identified by a base62-encoded short key. Returns
// sql.ErrNoRows when the key does not exist — callers map that to a
// gRPC NotFound. Expiry filtering is intentionally NOT done here; the
// service layer must distinguish "not found" from "expired" to return
// different status codes (404 vs 410 / NotFound vs FailedPrecondition).
func (r *Repo) Load(ctx context.Context, key string) (domain.URLEntry, error) {
	log := r.WithContext(ctx)
	id := int64(base62.Decode(key))
	var value domain.URLEntry
	err := r.Client.QueryRowContext(
		ctx,
		"SELECT id, original_url, shorted_url, clicks, expires_at FROM url_list WHERE id = $1",
		id,
	).Scan(&value.Id, &value.OriginalURL, &value.ShortedURL, &value.Clicks, &value.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.URLEntry{}, fmt.Errorf("%w", domain.ErrNotFound)
		}
		log.Errorf("loading from db: %v", err)
	}
	return value, err
}

// MaxID returns the largest id currently stored, or 0 if the table is
// empty. Used at startup to seed the Redis counter safely after a Redis
// data loss.
func (r *Repo) MaxID(ctx context.Context) (int64, error) {
	var maxID int64
	err := r.Client.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM url_list").Scan(&maxID)
	return maxID, err
}

// UpdateClicks bumps the click counter for a given short path. Failures
// are logged at error level but do not propagate — click counting is
// best-effort and must not break the redirect path.
func (r *Repo) UpdateClicks(ctx context.Context, path string, click int32) {
	log := r.WithContext(ctx)
	id := int64(base62.Decode(path))
	if _, err := r.Client.ExecContext(
		ctx,
		"UPDATE url_list SET clicks = $1 WHERE id = $2",
		click,
		id,
	); err != nil {
		log.Errorf("updating clicks in db: %v", err)
	}
}
