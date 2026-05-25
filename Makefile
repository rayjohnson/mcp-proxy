BIN          := bin/mcp-proxy
VERSION      := $(shell cat VERSION)
_BRANCH      := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo main)
_BRANCH_SLUG := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null \
                  | tr '[:upper:]' '[:lower:]' \
                  | tr -cs 'a-z0-9' '-' \
                  | cut -c1-20 \
                  | sed 's/-*$$//')

ifeq ($(_BRANCH),main)
BUILD_VERSION := v$(VERSION)
else
BUILD_VERSION := v$(VERSION)-$(_BRANCH_SLUG)
endif

# Generate a stable local KMS key once and write it to .env.local.
# 32 random bytes as hex = 64 chars.
.env.local:
	@printf 'LOCAL_KMS_KEY=%s\nDB_DSN=postgres://mcpproxy:devpassword@localhost:5432/mcpproxy\nKMS_KEY_NAME=local\nBASE_URL=http://localhost:8080\nPORT=8080\nLOCAL_MODE=false\n' \
	  "$$(openssl rand -hex 32)" > .env.local
	@echo "Created .env.local with a random LOCAL_KMS_KEY"
	@echo "Note: for local mode use 'make run-local' (port 9753) instead"

.PHONY: build
build:
	go build -ldflags "-X main.version=$(BUILD_VERSION)" -o $(BIN) ./cmd/mcp-proxy

COVER_THRESHOLD := 20

.PHONY: test
test:
	go test ./...

.PHONY: cover
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@pct=$$(go tool cover -func=coverage.out | awk '/^total:/ { gsub(/%/, "", $$3); print $$3 }'); \
	  if awk "BEGIN { exit !($${pct} < $(COVER_THRESHOLD)) }"; then \
	    echo "FAIL: coverage $${pct}% is below threshold $(COVER_THRESHOLD)%"; exit 1; \
	  else \
	    echo "OK: coverage $${pct}% meets threshold $(COVER_THRESHOLD)%"; \
	  fi

.PHONY: test-integration
test-integration: check-docker db-up
	TEST_DATABASE_URL=postgres://mcpproxy:devpassword@localhost:5432/mcpproxy \
	  go test -tags integration ./tests/integration/... -v

.PHONY: check-docker
check-docker:
	@docker info > /dev/null 2>&1 || \
	  (echo "Error: Docker is not running. Start OrbStack (or Docker Desktop) first." && exit 1)

.PHONY: db-up
db-up: check-docker
	docker compose up -d db
	@echo "Waiting for Postgres to be ready..."
	@until docker compose exec db pg_isready -U mcpproxy -q; do sleep 1; done
	@echo "Postgres is ready."

.PHONY: db-down
db-down: check-docker
	docker compose down

.PHONY: db-reset
db-reset: check-docker
	docker compose down -v
	$(MAKE) db-up

.PHONY: run
run: build .env.local db-up
	@set -a && . ./.env.local && set +a && ./$(BIN)

.PHONY: run-local
run-local: build
	@LOCAL_MODE=true KMS_KEY_NAME=local LOCAL_KMS_KEY=$$(openssl rand -hex 32) \
	  BASE_URL=http://localhost:9753 PORT=9753 ./$(BIN)

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix ./...

.PHONY: vuln
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

.PHONY: clean
clean:
	rm -f $(BIN)
