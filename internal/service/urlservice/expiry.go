package urlservice

import (
	"errors"
	"fmt"
	"time"
)

// DefaultURLTTL is applied to new entries when the caller provides neither
// expires_at nor no_expire=true. 30 days is a safe default that keeps
// Redis memory bounded while covering the typical link-sharing lifecycle.
const DefaultURLTTL = 30 * 24 * time.Hour

// Sentinel errors returned by resolveExpiry. Transport adapters (gRPC,
// HTTP) map these to the appropriate status codes — they must not assume
// any gRPC or HTTP vocabulary at this layer.
var (
	// ErrNoExpireConflict is returned when both no_expire=true and a
	// concrete expires_at are supplied together (mutually exclusive).
	ErrNoExpireConflict = errors.New("no_expire and expires_at are mutually exclusive")

	// ErrExpiresAtPast is returned when the supplied expires_at timestamp
	// is not strictly in the future relative to the request time.
	ErrExpiresAtPast = errors.New("expires_at must be in the future")
)

// resolveExpiry derives the effective expiry from the decoded request
// fields. All arguments are pure Go types — no proto or gRPC types.
//
//	(noExpire=true,  explicit=nil) → nil          (永久 / never expires)
//	(noExpire=false, explicit=nil) → now + DefaultURLTTL
//	(noExpire=false, explicit=t)   → t  (must be strictly after now)
//	(noExpire=true,  explicit=t)   → ErrNoExpireConflict
func resolveExpiry(noExpire bool, explicit *time.Time, now time.Time) (*time.Time, error) {
	if noExpire && explicit != nil {
		return nil, ErrNoExpireConflict
	}
	if noExpire {
		return nil, nil
	}
	if explicit != nil {
		if !explicit.After(now) {
			return nil, fmt.Errorf("%w: got %s, now %s", ErrExpiresAtPast, explicit.Format(time.RFC3339), now.Format(time.RFC3339))
		}
		return explicit, nil
	}
	t := now.Add(DefaultURLTTL)
	return &t, nil
}
