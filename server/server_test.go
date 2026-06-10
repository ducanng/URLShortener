package server

import (
	"testing"
	"time"

	"github.com/ducanng/URLShortener/proto/urlshortenerpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fixedNow is a stable reference time used across the table tests so the
// expected results don't depend on the machine clock.
var fixedNow = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

func TestResolveExpiry(t *testing.T) {
	future := fixedNow.Add(48 * time.Hour)
	past := fixedNow.Add(-1 * time.Hour)

	tests := []struct {
		name         string
		req          *urlshortenerpb.CreateURLRequest
		wantNil      bool
		wantTime     time.Time // only checked when wantNil == false
		wantErrCode  codes.Code
		wantHasError bool
	}{
		{
			name:     "default 30 days when nothing provided",
			req:      &urlshortenerpb.CreateURLRequest{Url: "x"},
			wantTime: fixedNow.Add(defaultURLTTL),
		},
		{
			name:    "no_expire true → nil (vĩnh viễn)",
			req:     &urlshortenerpb.CreateURLRequest{Url: "x", NoExpire: true},
			wantNil: true,
		},
		{
			name:     "explicit future expires_at is honoured",
			req:      &urlshortenerpb.CreateURLRequest{Url: "x", ExpiresAt: timestamppb.New(future)},
			wantTime: future,
		},
		{
			name:         "expires_at in the past → InvalidArgument",
			req:          &urlshortenerpb.CreateURLRequest{Url: "x", ExpiresAt: timestamppb.New(past)},
			wantHasError: true,
			wantErrCode:  codes.InvalidArgument,
		},
		{
			name:         "expires_at equal to now → InvalidArgument (must be strictly after)",
			req:          &urlshortenerpb.CreateURLRequest{Url: "x", ExpiresAt: timestamppb.New(fixedNow)},
			wantHasError: true,
			wantErrCode:  codes.InvalidArgument,
		},
		{
			name:         "no_expire and expires_at together → InvalidArgument",
			req:          &urlshortenerpb.CreateURLRequest{Url: "x", NoExpire: true, ExpiresAt: timestamppb.New(future)},
			wantHasError: true,
			wantErrCode:  codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveExpiry(tt.req, fixedNow)

			if tt.wantHasError {
				if err == nil {
					t.Fatalf("expected error code=%s, got nil", tt.wantErrCode)
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %v", err)
				}
				if st.Code() != tt.wantErrCode {
					t.Fatalf("expected code=%s, got %s", tt.wantErrCode, st.Code())
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
