// Package http implements the public HTTP transport: the Gin router,
// grpc-gateway REST↔gRPC bridge, Swagger UI, Prometheus instrumentation,
// health probes, CORS, per-request timeouts, and the redirect handler. It is
// a thin transport layer — all business logic lives in the service package,
// reached via the gRPC client / gateway. The exported DTOs below carry the
// Swagger example tags consumed by `swag init`.
package http

// ShortenRequest represents the JSON body for creating a short URL.
// expires_at and no_expire are optional. Defaults to a 30-day TTL when
// neither is provided.
type ShortenRequest struct {
	URL       string `json:"url" example:"https://example.com"`
	ExpiresAt string `json:"expires_at,omitempty" example:"2026-12-31T00:00:00Z"`
	NoExpire  bool   `json:"no_expire,omitempty" example:"false"`
}

// URLResponse represents the JSON response from grpc-gateway.
type URLResponse struct {
	Message string        `json:"message" example:"Create short url"`
	Status  string        `json:"status" example:"Success"`
	URL     *ShortenedURL `json:"url,omitempty"`
}

// ShortenedURL represents URL details in the response. ExpiresAt is
// omitted when the short URL never expires.
type ShortenedURL struct {
	OriginalURL  string `json:"originalURL" example:"https://example.com"`
	ShortenedURL string `json:"shortenedURL" example:"http://localhost:8080/abc123"`
	Clicks       int32  `json:"clicks" example:"0"`
	ExpiresAt    string `json:"expires_at,omitempty" example:"2026-12-31T00:00:00Z"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Message string `json:"message" example:"URL not found"`
}
