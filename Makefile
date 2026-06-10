module:
	go mod init github.com/ducanng/URLShortener
	go mod tidy
gen-urlshortener:
	protoc -I . -I third_party \
		--go_out=. --go_opt=module=github.com/ducanng/URLShortener \
		--go-grpc_out=. --go-grpc_opt=module=github.com/ducanng/URLShortener \
		--grpc-gateway_out=. --grpc-gateway_opt=module=github.com/ducanng/URLShortener \
		proto/urlshortener.proto
go-build:
	go build -o bin/main.exe main.go
swag:
	swag init
network:
	docker network create urlshortener-net || true

app-up: network
	docker compose up -d --build

app-down:
	docker compose down

monitoring-up: network
	docker compose -f docker-compose.monitoring.yml up -d

monitoring-down:
	docker compose -f docker-compose.monitoring.yml down

# ---- Database migrations (golang-migrate, embedded in storage/migrations) ----
# Run manually before/after schema changes. The app does NOT auto-migrate.

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down $(if $(STEPS),$(STEPS),1)

migrate-version:
	go run ./cmd/migrate version

migrate-force:
	@if [ -z "$(V)" ]; then echo "usage: make migrate-force V=<version>"; exit 1; fi
	go run ./cmd/migrate force $(V)

# Create a new migration pair via golang-migrate CLI.
# Usage: make migrate-create NAME=add_users_table
# -seq  : sequential numbering (000002, 000003, ...) instead of timestamp
# -ext  : file extension for the generated up/down files
# -dir  : output directory (must match the embed path in storage/migrate.go)
migrate-create:
	@if [ -z "$(NAME)" ]; then echo "usage: make migrate-create NAME=<name>"; exit 1; fi
	@mkdir -p storage/migrations
	go run github.com/golang-migrate/migrate/v4/cmd/migrate create \
		-ext sql -dir storage/migrations -seq $(NAME)