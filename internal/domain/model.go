// Package domain contains the core business entities of the URL shortener.
// Entities are pure Go types with no dependencies on transport, storage,
// or third-party frameworks.
package domain

import "time"

// URLEntry is the canonical record for a shortened URL. ExpiresAt is a
// pointer so a nil value cleanly represents "never expires" both in JSON
// (omitted) and in PostgreSQL (NULL).
type URLEntry struct {
	Id          int64      `json:"id"`
	OriginalURL string     `json:"original_url"`
	ShortedURL  string     `json:"short_url"`
	Clicks      int32      `json:"clicks"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}
