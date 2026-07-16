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

# ---- Database migrations (golang-migrate, embedded in internal/repository/postgres/migrations) ----
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
# -dir  : output directory (must match the embed path in internal/repository/postgres/migrate.go)
migrate-create:
	@if [ -z "$(NAME)" ]; then echo "usage: make migrate-create NAME=<name>"; exit 1; fi
	@mkdir -p internal/repository/postgres/migrations
	go run github.com/golang-migrate/migrate/v4/cmd/migrate create \
		-ext sql -dir internal/repository/postgres/migrations -seq $(NAME)

# ---- k6 load testing (docker; see docs/k6-load-testing.md) ----
# Requires the app + monitoring stacks up (make app-up monitoring-up).
# BASE_URL picks the target:
#   http://nginx-lb            end-to-end through Nginx (default)
#   http://urlshortener-1:8080 straight to one app instance (bypass Nginx)
# Extra k6 flags via K6ARGS, e.g. `make k6-load K6ARGS="-e RATE=500 -e DURATION=3m"`.
BASE_URL ?= http://nginx-lb
K6_UID   ?= $(shell id -u)
K6_GID   ?= $(shell id -g)
# STAMP makes each run's testid unique so the "k6 Prometheus" Grafana dashboard
# (monitoring/grafana/dashboards/k6.json) can select this run in its $testid picker.
STAMP := $(shell date +%Y%m%d-%H%M%S)
K6_RUN = BASE_URL=$(BASE_URL) K6_UID=$(K6_UID) K6_GID=$(K6_GID) \
	docker compose -f docker-compose.k6.yml run --rm k6 run -o experimental-prometheus-rw

k6-smoke:
	$(K6_RUN) --tag testid=smoke-$(STAMP) $(K6ARGS) /scripts/smoke.js
k6-load:
	$(K6_RUN) --tag testid=load-$(STAMP) $(K6ARGS) /scripts/load.js
k6-stress:
	$(K6_RUN) --tag testid=stress-$(STAMP) $(K6ARGS) /scripts/stress.js
k6-spike:
	$(K6_RUN) --tag testid=spike-$(STAMP) $(K6ARGS) /scripts/spike.js
k6-soak:
	$(K6_RUN) --tag testid=soak-$(STAMP) $(K6ARGS) /scripts/soak.js
k6-breakpoint:
	$(K6_RUN) --tag testid=breakpoint-$(STAMP) $(K6ARGS) /scripts/breakpoint.js