BIN := bin/server

# Generate a stable local KMS key once and write it to .env.local.
# 32 random bytes as hex = 64 chars.
.env.local:
	@printf 'LOCAL_KMS_KEY=%s\nDB_DSN=postgres://mcpproxy:devpassword@localhost:5432/mcpproxy\nKMS_KEY_NAME=local\nBASE_URL=http://localhost:8080\nPORT=8080\n' \
	  "$$(openssl rand -hex 32)" > .env.local
	@echo "Created .env.local with a random LOCAL_KMS_KEY"

.PHONY: build
build:
	go build -o $(BIN) ./cmd/server

.PHONY: test
test:
	go test ./...

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

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: clean
clean:
	rm -f $(BIN)
