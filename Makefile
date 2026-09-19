.PHONY: bench bench-500k bench-1m bench-5m bench-small test tidy db-up db-down db-logs migrate

DATABASE_URL ?= postgres://contextcompiler:contextcompiler@localhost:5432/contextcompiler?sslmode=disable

bench:
	go run ./cmd/bench -tokens 100000 -budget 2000

bench-500k:
	go run ./cmd/bench -tokens 500000 -budget 2000

bench-1m:
	go run ./cmd/bench -tokens 1000000 -budget 2000

bench-5m:
	go run ./cmd/bench -tokens 5000000 -budget 2000

bench-small:
	go run ./cmd/bench -small

test:
	go test ./...

tidy:
	go mod tidy

db-up:
	docker compose up -d
	@echo "Waiting for Postgres..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		docker compose exec -T db pg_isready -U contextcompiler && break; \
		sleep 1; \
	done

db-down:
	docker compose down

db-logs:
	docker compose logs -f db

# Schema is also auto-migrated by the Go store on connect; this applies migrations/ SQL explicitly.
migrate:
	docker compose exec -T db psql -U contextcompiler -d contextcompiler < migrations/001_init.sql
