// Package config is the single place the process reads its environment. It
// loads the optional .env file once, resolves every setting into a typed
// Config, and hands sub-configs to each component's constructor. No other
// package calls os.Getenv or godotenv.Load — this keeps configuration
// discoverable and testable, and avoids the previous scattered .env loads.
package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config is the fully-resolved process configuration. Each field is a
// sub-config handed to the component that owns it, so constructors never
// reach for the environment themselves.
type Config struct {
	HTTP     HTTPConfig
	GRPCAddr string // gRPC server listen address, e.g. ":50051"
	DB       DBConfig
	Redis    RedisConfig
	CORS     CORSConfig
}

// HTTPConfig configures the public HTTP transport.
type HTTPConfig struct {
	Addr         string // public HTTP listen address, e.g. ":8080"
	GRPCEndpoint string // in-process gRPC dial target the gateway/client use
	SwaggerHost  string // Swagger "Try it out" host; "" = relative (swagger-ui uses window.location). Set to pin per-env, e.g. localhost:80 behind Nginx.
}

// DBConfig configures the PostgreSQL repository.
type DBConfig struct {
	DSN string // Postgres connection string (DB env var)
}

// RedisConfig configures the Redis cache and counter.
type RedisConfig struct {
	Addr string // Redis host:port (REDIS_ADDR env var)
}

// CORSConfig configures cross-origin access. AllowedOrigins is nil when
// CORS_ALLOWED_ORIGINS is unset — the HTTP layer then falls back to localhost
// dev origins.
type CORSConfig struct {
	AllowedOrigins []string
}

// Load reads the environment (after a best-effort .env load) and returns the
// resolved Config. It never fails: absent variables surface as empty values
// and the component constructors decide whether that is fatal. The listen /
// dial addresses are fixed for now and can be promoted to env vars later
// without touching call sites.
func Load() *Config {
	// Best-effort: a missing .env is normal in containers where the
	// variables are injected directly into the environment.
	_ = godotenv.Load()

	return &Config{
		HTTP: HTTPConfig{
			Addr:         ":8080",
			GRPCEndpoint: "localhost:50051",
			// Empty when SWAGGER_HOST is unset → relative host (works in every
			// environment). Set the env var to pin an absolute host.
			SwaggerHost: os.Getenv("SWAGGER_HOST"),
		},
		GRPCAddr: ":50051",
		DB:       DBConfig{DSN: os.Getenv("DB")},
		Redis:    RedisConfig{Addr: os.Getenv("REDIS_ADDR")},
		CORS:     CORSConfig{AllowedOrigins: parseOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))},
	}
}

// parseOrigins splits a comma-separated origin list, trimming whitespace and
// dropping empties and duplicates. Returns nil for an empty input so callers
// can detect "unset" and apply their own fallback.
func parseOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := make(map[string]struct{})
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if _, dup := seen[o]; dup {
			continue
		}
		seen[o] = struct{}{}
		origins = append(origins, o)
	}
	return origins
}
