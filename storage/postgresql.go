package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/ducanng/URLShortener/internal/logger"
	"github.com/ducanng/URLShortener/internal/domain"

	base62 "github.com/alextanhongpin/base62"

	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

// SQLStore is the PostgreSQL-backed primary store. The embedded *Logger
// gives every method structured logging; the URLEntry field is preserved
// from the previous shape — callers that relied on it continue to work.
type SQLStore struct {
	*logger.Logger
	Client   *sql.DB
	URLEntry domain.URLEntry
}

// NewSQLStore opens a *sql.DB to the database referenced by the DB env var
// and applies the connection-pool sizing for an 8-vCPU SSD host. Returns
// an error rather than calling Fatal so main can decide how to fail.
//
// Schema is NOT created here — run `make migrate-up` (or
// `go run ./cmd/migrate up`) before starting the app for the first time.
//
// Connection pool: (CPU×2)+spindle = 17 for an 8-vCPU SSD machine. Tune
// SetMaxOpenConns/SetMaxIdleConns when running on smaller / larger hosts.
func NewSQLStore(log *logger.Logger) (*SQLStore, error) {
	if err := godotenv.Load(); err != nil {
		log.Warnf(".env not found, using environment variables: %v", err)
	}

	client, err := sql.Open("postgres", os.Getenv("DB"))
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	client.SetMaxOpenConns(17)
	client.SetMaxIdleConns(17)
	client.SetConnMaxLifetime(5 * time.Minute)
	client.SetConnMaxIdleTime(1 * time.Minute)

	return &SQLStore{Logger: log, Client: client}, nil
}

// Save inserts a new URLEntry. Uses ExecContext so the query honours the
// caller's deadline / cancellation. ExpiresAt is a pointer so passing nil
// stores SQL NULL — meaning the entry never expires.
func (s *SQLStore) Save(ctx context.Context, entry domain.URLEntry) error {
	log := s.WithContext(ctx)
	_, err := s.Client.ExecContext(
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
func (s *SQLStore) Load(ctx context.Context, key string) (domain.URLEntry, error) {
	log := s.WithContext(ctx)
	id := int64(base62.Decode(key))
	var value domain.URLEntry
	err := s.Client.QueryRowContext(
		ctx,
		"SELECT id, original_url, shorted_url, clicks, expires_at FROM url_list WHERE id = $1",
		id,
	).Scan(&value.Id, &value.OriginalURL, &value.ShortedURL, &value.Clicks, &value.ExpiresAt)
	if err != nil {
		log.Errorf("loading from db: %v", err)
	}
	return value, err
}

// MaxID returns the largest id currently stored, or 0 if the table is
// empty. Used at startup to seed the Redis counter safely after a Redis
// data loss.
func (s *SQLStore) MaxID(ctx context.Context) (int64, error) {
	var maxID int64
	err := s.Client.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM url_list").Scan(&maxID)
	return maxID, err
}

// UpdateClicks bumps the click counter for a given short path. Failures
// are logged at error level but do not propagate — click counting is
// best-effort and must not break the redirect path.
func (s *SQLStore) UpdateClicks(ctx context.Context, path string, click int32) {
	log := s.WithContext(ctx)
	id := int64(base62.Decode(path))
	if _, err := s.Client.ExecContext(
		ctx,
		"UPDATE url_list SET clicks = $1 WHERE id = $2",
		click,
		id,
	); err != nil {
		log.Errorf("updating clicks in db: %v", err)
	}
}
