package urlservice

import (
	"errors"
	"testing"
	"time"
)

// fixedNow is a stable reference time used across the table tests so
// expected results do not depend on the machine clock.
var fixedNow = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

func TestResolveExpiry(t *testing.T) {
	future := fixedNow.Add(48 * time.Hour)
	past := fixedNow.Add(-1 * time.Hour)

	tests := []struct {
		name        string
		noExpire    bool
		explicit    *time.Time
		wantNil     bool
		wantTime    time.Time
		wantErr     error // nil means no error expected
	}{
		{
			name:     "default 30 days when nothing provided",
			wantTime: fixedNow.Add(DefaultURLTTL),
		},
		{
			name:     "no_expire true → nil (never expires)",
			noExpire: true,
			wantNil:  true,
		},
		{
			name:     "explicit future expires_at is honoured",
			explicit: &future,
			wantTime: future,
		},
		{
			name:     "expires_at in the past → ErrExpiresAtPast",
			explicit: &past,
			wantErr:  ErrExpiresAtPast,
		},
		{
			name:     "expires_at equal to now → ErrExpiresAtPast (must be strictly after)",
			explicit: &fixedNow,
			wantErr:  ErrExpiresAtPast,
		},
		{
			name:     "no_expire and explicit together → ErrNoExpireConflict",
			noExpire: true,
			explicit: &future,
			wantErr:  ErrNoExpireConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveExpiry(tt.noExpire, tt.explicit, fixedNow)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error wrapping %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected errors.Is(err, %v), got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil expiry, got %v", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil expiry, got nil")
			}
			if !got.Equal(tt.wantTime) {
				t.Fatalf("expected %v, got %v", tt.wantTime, *got)
			}
		})
	}
}
