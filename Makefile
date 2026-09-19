.PHONY: bench bench-small bench-multitask test tidy db-up db-down db-logs migrate api

API_ADDR ?= :8080

DATABASE_URL ?= postgres://contextcompiler:contextcompiler@localhost:5432/contextcompiler?sslmode=disable
TOKENS ?= 100000
BUDGET ?= 2000
# GNU make treats `make bench -mock` as its own flags (not passed through). Use MOCK=1.
MOCK ?= 0
ifeq ($(MOCK),1)
MOCK_FLAG := -mock
endif

bench:
	go run ./cmd/bench -tokens $(TOKENS) -budget $(BUDGET) $(MOCK_FLAG)

bench-small:
	go run ./cmd/bench -small

bench-multitask:
	go run ./cmd/bench -suite -small -mock -budget $(BUDGET)

test:
	go test ./...

api:
	go run ./cmd/api -addr $(API_ADDR)

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
