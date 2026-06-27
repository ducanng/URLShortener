# URL Shortener

A URL shortening service written in Go. It exposes both a
gRPC API and a REST gateway, persists data in PostgreSQL, caches reads in
Redis, and ships with structured logging, Prometheus metrics, and database
schema migrations baked into the binary.

## Features

- Create short URLs from any long URL.
- Optional expiry per URL — defaults to 30 days, supports an explicit
  `expires_at`, or `no_expire: true` for permanent links.
- Redirect endpoint with proper HTTP status codes (404 for not found, 410 for expired) and count click.
- Get short URL info by code.
- No authentication.
- No security short URL.

## Tech Stack

- **Language**: Go
- **API**: gRPC + grpc-gateway (REST), Swagger UI
- **Storage**: PostgreSQL 14
- **Cache**: Redis
- **Observability**: Zap (logs), Prometheus (metrics), Grafana + Loki
- **Containerization**: Docker, Docker Compose

## Getting Started

### Prerequisites

- Docker
- Go 1.26+ (only required for local development)
- Prepare environment variables (optional, defaults are provided in `.env.example`)

### Run with Docker Compose

```bash
make app-up
```

This builds the image, starts PostgreSQL and Redis, runs the `migrate`
service to bring the schema up to date, then starts the app. The
`migrate` container exits 0 on success and the app waits on
`service_completed_successfully` before starting — no manual migration
step is needed.

To stop the stack:

```bash
make app-down
```

Optional monitoring stack (Prometheus, Grafana, Loki, exporters):

```bash
make monitoring-up      # start
make monitoring-down    # stop
```

### Endpoints

| URL | Purpose |
| --- | --- |
| `http://localhost:8080`                    | REST gateway |
| `http://localhost:8080/docs/index.html`    | Swagger UI |
| `http://localhost:7070/metrics`            | Prometheus metrics |
| `localhost:50051`                          | gRPC server |
| `http://localhost:3000` *(monitoring)*     | Grafana |

## Usage

### Create a short URL

Default 30-day TTL:

```bash
curl -X POST http://localhost:8080/shorted \
  -H 'Content-Type: application/json' \
  -d '{"url": "https://example.com"}'
```

Explicit expiry (RFC 3339):

```bash
curl -X POST http://localhost:8080/shorted \
  -H 'Content-Type: application/json' \
  -d '{"url": "https://example.com", "expires_at": "2026-12-31T00:00:00Z"}'
```

Permanent (never expires):

```bash
curl -X POST http://localhost:8080/shorted \
  -H 'Content-Type: application/json' \
  -d '{"url": "https://example.com", "no_expire": true}'
```

Successful response:

```json
{
  "message": "Create short url",
  "status":  "Success",
  "url": {
    "originalURL":  "https://example.com",
    "shortenedURL": "http://localhost:8080/abc",
    "clicks":       0,
    "expiresAt":    "2026-07-10T14:30:41Z"
  }
}
```

### Resolve a short URL

```bash
curl -i http://localhost:8080/abc
```

Possible responses:

| Status | Meaning |
| --- | --- |
| `301 Moved Permanently` | Redirected to the original URL |
| `404 Not Found`         | Short path does not exist |
| `410 Gone`              | Short path existed but has expired |

### Inspect a short URL (no redirect)

```bash
curl http://localhost:8080/info/abc
```

## Development

### Local run without Docker

```bash
cp .env.example .env       # adjust DB / REDIS_ADDR if needed
make migrate-up            # apply pending migrations
go run .
```

### Regenerate code

```bash
make gen-urlshortener      # protobuf + grpc-gateway stubs
make swag                  # Swagger docs
```

### Run tests

```bash
go test ./...
```

## Database Migrations

Migrations live in `storage/migrations/` as `*.up.sql` / `*.down.sql`
pairs. They are embedded into the `migrate` binary at build time, so
production deployments do not need any SQL files on disk.

When running through Docker Compose the `migrate` service applies pending
migrations automatically before the app starts. The commands below are
for local development or operational recovery.

| Command | Purpose |
| --- | --- |
| `make migrate-up`                         | Apply all pending migrations |
| `make migrate-down`                       | Roll back one step |
| `make migrate-down STEPS=2`               | Roll back N steps |
| `make migrate-version`                    | Show current version + dirty flag |
| `make migrate-force V=<version>`          | Force a version (recover from a dirty state) |
| `make migrate-create NAME=<name>`         | Generate a new migration pair |

Filename convention: `{6-digit version}_{name}.up.sql` /
`.down.sql` — e.g. `000003_add_users_table.up.sql`. Always commit both
`up` and `down` files together.
