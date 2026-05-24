.PHONY: test coverage cobertura cover-html cover-func tools fmt fmt-check lint lint-markdown lint-docs check all mocks integration-test integration-test-sqlite integration-test-mysql integration-test-postgres integration-test-mssql integration-test-all wait-mssql deps-install deps-validate clean

TEST_PKGS ?= ./...

# Output layout
OUT_DIR ?= out
OUT_COVER_DIR ?= $(OUT_DIR)/coverage
OUT_JUNIT_DIR ?= $(OUT_DIR)/junit
OUT_COBERTURA_DIR ?= $(OUT_DIR)/cobertura
OUT_HTML_DIR ?= $(OUT_DIR)/html

COVER_FILE ?= $(OUT_COVER_DIR)/coverage.out
JUNIT_FILE ?= $(OUT_JUNIT_DIR)/junit.xml
COBERTURA_FILE ?= $(OUT_COBERTURA_DIR)/coverage.xml
HTML_FILE ?= $(OUT_HTML_DIR)/coverage.html

# Pin tool versions by setting these variables when invoking make, e.g.
# `make GOTESTSUM_VERSION=v1.9.0 GOCOVER_VERSION=v0.0.0-20220101 tools`
GOTESTSUM_VERSION ?= v1.13.0
GOCOVER_VERSION ?=

GOFUMPT_VERSION ?= v0.9.2
GOLANGCI_VERSION ?= v2.11.4
MOCKGEN_VERSION ?= v1.6.0

ifeq ($(GOTESTSUM_VERSION),)
GOTESTSUM_INSTALL = gotest.tools/gotestsum@latest
else
GOTESTSUM_INSTALL = gotest.tools/gotestsum@$(GOTESTSUM_VERSION)
endif

ifeq ($(GOCOVER_VERSION),)
GOCOVER_INSTALL = github.com/t-yuki/gocover-cobertura@latest
else
GOCOVER_INSTALL = github.com/t-yuki/gocover-cobertura@$(GOCOVER_VERSION)
endif

ifeq ($(GOFUMPT_VERSION),)
GOFUMPT_INSTALL = mvdan.cc/gofumpt@latest
else
GOFUMPT_INSTALL = mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
endif

ifeq ($(GOLANGCI_VERSION),)
GOLANGCI_INSTALL = github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
else
GOLANGCI_INSTALL = github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
endif

ifeq ($(MOCKGEN_VERSION),)
MOCKGEN_INSTALL = github.com/golang/mock/mockgen@latest
else
MOCKGEN_INSTALL = github.com/golang/mock/mockgen@$(MOCKGEN_VERSION)
endif

# Markdown linting tools
MARKDOWNLINT_VERSION ?=
VALE_VERSION ?=

ifeq ($(MARKDOWNLINT_VERSION),)
MARKDOWNLINT_INSTALL = markdownlint-cli2@latest
else
MARKDOWNLINT_INSTALL = markdownlint-cli2@$(MARKDOWNLINT_VERSION)
endif

ifeq ($(VALE_VERSION),)
VALE_INSTALL = github.com/errata-ai/vale/v3/cmd/vale@latest
else
VALE_INSTALL = github.com/errata-ai/vale/v3/cmd/vale@$(VALE_VERSION)
endif
GOFLAGS ?= -tags=test
CGO_ENABLED ?= 0
TEST_FLAGS ?= -covermode=atomic -coverpkg=./... -coverprofile=$(COVER_FILE)

tools:
	@echo "Installing: $(GOTESTSUM_INSTALL) $(GOCOVER_INSTALL) $(GOFUMPT_INSTALL) $(GOLANGCI_INSTALL) $(MOCKGEN_INSTALL) $(MARKDOWNLINT_INSTALL) $(VALE_INSTALL)"
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(GOTESTSUM_INSTALL)
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(GOCOVER_INSTALL)
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(GOFUMPT_INSTALL)
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(GOLANGCI_INSTALL)
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(MOCKGEN_INSTALL)
	@echo "Installing markdownlint-cli2..."
	@npm install -g markdownlint-cli2 2>/dev/null || echo "Note: markdownlint-cli2 requires Node.js/npm"
	@GOBIN=$(GOBIN) go install $(VALE_INSTALL)

test:
	@mkdir -p $(OUT_JUNIT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(JUNIT_FILE) -- -count=1 -timeout 300s $(GOFLAGS) $(TEST_PKGS)

test-integration: GOFLAGS = -tags=integration
test-integration:
	@echo "Running all integration tests..."
	@mkdir -p $(OUT_JUNIT_DIR)
	@docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for all services to be healthy..."
	@sleep 10
	@docker compose -f docker-compose.test.yml exec -T mysql mysqladmin --wait=60 --count=10 ping -h localhost -u root -proot_password || true
	@docker compose -f docker-compose.test.yml exec -T postgres pg_isready -U test_user -d test_db -t 60 || true
	@$(MAKE) wait-mssql
	@echo "Services are ready, running tests..."
	CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(JUNIT_FILE) -- -count=1 -timeout 300s $(GOFLAGS) $(TEST_PKGS)
	@docker compose -f docker-compose.test.yml down -v

coverage:
	@mkdir -p $(OUT_COVER_DIR) $(OUT_JUNIT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(JUNIT_FILE) -- -count=1 -timeout 300s $(GOFLAGS) $(TEST_FLAGS) $(TEST_PKGS)

cobertura: coverage
	@mkdir -p $(OUT_COBERTURA_DIR)
	gocover-cobertura < $(COVER_FILE) > $(COBERTURA_FILE)

cover-html: coverage
	@mkdir -p $(OUT_HTML_DIR)
	go tool cover -html=$(COVER_FILE) -o $(HTML_FILE)

cover-func: coverage
	go tool cover -func=$(COVER_FILE) | awk '/total/ {print $$3}'

# Integration tests
.PHONY: integration-test integration-test-sqlite integration-test-mysql integration-test-postgres integration-test-mssql integration-test-all

integration-test: integration-test-sqlite
	@echo "Integration tests completed"

integration-test-sqlite:
	@echo "Running SQLite integration tests..."
	@mkdir -p $(OUT_JUNIT_DIR)
	@DB_TYPE=sqlite CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(OUT_JUNIT_DIR)/integration-sqlite.xml -- -count=1 -timeout 300s -tags=integration ./tests -run "TestIntegration"

integration-test-mysql:
	@echo "Running MySQL integration tests..."
	@docker compose -f docker-compose.test.yml up -d mysql
	@echo "Waiting for MySQL to be healthy..."
	@sleep 5
	@docker compose -f docker-compose.test.yml exec -T mysql mysqladmin --wait=60 --count=10 ping -h localhost -u root -proot_password || true
	@sleep 5
	@mkdir -p $(OUT_JUNIT_DIR)
	@DB_TYPE=mysql \
		DB_MYSQL_USER=root \
		DB_MYSQL_PASSWORD=root_password \
		DB_MYSQL_HOST=localhost \
		DB_MYSQL_DATABASE=test_db \
		CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(OUT_JUNIT_DIR)/integration-mysql.xml -- -count=1 -timeout 300s -tags=integration ./tests -run "TestIntegration"
	@docker compose -f docker-compose.test.yml down mysql

integration-test-postgres:
	@echo "Running PostgreSQL integration tests..."
	@docker compose -f docker-compose.test.yml up -d postgres
	@echo "Waiting for PostgreSQL to be healthy..."
	@sleep 5
	@docker compose -f docker-compose.test.yml exec -T postgres pg_isready -U test_user -d test_db -t 60 || true
	@sleep 5
	@mkdir -p $(OUT_JUNIT_DIR)
	@DB_TYPE=postgres \
		DB_POSTGRES_USER=test_user \
		DB_POSTGRES_PASSWORD=test_password \
		DB_POSTGRES_HOST=localhost \
		DB_POSTGRES_DATABASE=test_db \
		CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(OUT_JUNIT_DIR)/integration-postgres.xml -- -count=1 -timeout 300s -tags=integration ./tests -run "TestIntegration"
	@docker compose -f docker-compose.test.yml down postgres

integration-test-mssql:
	@echo "Running MSSQL integration tests..."
	@mkdir -p $(OUT_JUNIT_DIR)
	@docker compose -f docker-compose.test.yml up -d mssql
	@$(MAKE) wait-mssql
	@DB_TYPE=sqlserver \
		DB_MSSQL_USER=sa \
		DB_MSSQL_PASSWORD=TestPassword123! \
		DB_MSSQL_HOST=127.0.0.1 \
		DB_MSSQL_DATABASE=test_db \
		CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(OUT_JUNIT_DIR)/integration-sqlserver.xml -- -count=1 -timeout 300s -tags=integration ./tests -run "TestIntegration"
	@docker compose -f docker-compose.test.yml down -v

integration-test-all:
	@echo "Running all integration tests..."
	@mkdir -p $(OUT_JUNIT_DIR)
	@docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for all services to be healthy..."
	@sleep 10
	@docker compose -f docker-compose.test.yml exec -T mysql mysqladmin --wait=60 --count=10 ping -h localhost -u root -proot_password || true
	@docker compose -f docker-compose.test.yml exec -T postgres pg_isready -U test_user -d test_db -t 60 || true
	@$(MAKE) wait-mssql
	@echo "Services are ready, running tests..."
	@DB_TYPE=sqlite CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(OUT_JUNIT_DIR)/integration-sqlite.xml -- -count=1 -timeout 300s -tags=integration ./tests -run "TestIntegration"
	@DB_TYPE=mysql \
		DB_MYSQL_USER=root \
		DB_MYSQL_PASSWORD=root_password \
		DB_MYSQL_HOST=localhost \
		DB_MYSQL_DATABASE=test_db \
		CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(OUT_JUNIT_DIR)/integration-mysql.xml -- -count=1 -timeout 300s -tags=integration ./tests -run "TestIntegration"
	@DB_TYPE=postgres \
		DB_POSTGRES_USER=test_user \
		DB_POSTGRES_PASSWORD=test_password \
		DB_POSTGRES_HOST=localhost \
		DB_POSTGRES_DATABASE=test_db \
		CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(OUT_JUNIT_DIR)/integration-postgres.xml -- -count=1 -timeout 300s -tags=integration ./tests -run "TestIntegration"
	@DB_TYPE=sqlserver \
		DB_MSSQL_USER=sa \
		DB_MSSQL_PASSWORD=TestPassword123! \
		DB_MSSQL_HOST=127.0.0.1 \
		DB_MSSQL_DATABASE=test_db \
		CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(OUT_JUNIT_DIR)/integration-sqlserver.xml -- -count=1 -timeout 300s -tags=integration ./tests -run "TestIntegration"
	@docker compose -f docker-compose.test.yml down -v

wait-mssql:
	@echo "Waiting for MSSQL to be ready (this can take several minutes)..."
	@for i in $$(seq 1 180); do \
		if docker compose -f docker-compose.test.yml exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P TestPassword123! -C -Q "SELECT 1" >/dev/null 2>&1; then \
			echo "MSSQL is ready"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "MSSQL did not become ready in time. Container logs:"; \
	docker compose -f docker-compose.test.yml logs mssql; \
	exit 1


# Formatting
fmt:
	@echo "Formatting Go files with gofumpt..."
	@gofumpt -w .

fmt-check:
	@echo "Checking formatting with gofumpt..."
	@if [ -n "$(shell go list ./... 2>/dev/null | xargs -I{} sh -c 'gofumpt -l {} || true' )" ]; then \
		echo 'Files need formatting. Run `make fmt`.'; exit 1; \
	fi

# Linting
lint:
	@echo "Running golangci-lint using .golangci.yml"
	@golangci-lint run --config .golangci.yml ./...

# Markdown linting with markdownlint
lint-markdown:
	@echo "Running markdownlint-cli2"
	@if command -v markdownlint-cli2 >/dev/null 2>&1; then \
		markdownlint-cli2; \
	else \
		echo "markdownlint-cli2 not found. Install with: npm install -g markdownlint-cli2"; exit 1; \
	fi

# Documentation linting with Vale
lint-docs:
	@echo "Running Vale for documentation quality check"
	@if command -v vale >/dev/null 2>&1; then \
		vale --config .vale.yaml ./docs ./README.md *.md 2>/dev/null || true; \
	else \
		echo "Vale not found. Install with: go install github.com/errata-ai/vale/v3@latest"; exit 1; \
	fi

# Mock generation
mocks:
	@echo "Generating mocks using go generate..."
	@go generate ./...
	@echo "Mocks generated successfully!"

# Clean cache to ensure clean test runs, especially important for coverage and integration tests
clean:
	@echo "Cleaning cache..."
	@go clean -cache
	@go clean -testcache
	@echo "Cache cleaned successfully!"

# Dependencies
deps-install:
	@echo "Downloading module dependencies..."
	@go mod download

deps-validate:
	@echo "Validating module checksums and tidy state..."
	@go mod verify
	@if [ -n "$(shell git status --porcelain go.mod go.sum)" ]; then \
		echo 'go.mod or go.sum changed; run `go mod tidy` and commit'; exit 1; \
	fi

# CI check: formatting, deps validation, lint, markdown linting, mocks generation, and tests
check: fmt-check deps-validate lint lint-markdown lint-docs mocks coverage
	@echo "All checks passed."

all: tools fmt lint deps-install deps-validate mocks check test coverage cobertura cover-html cover-func
	@echo "All tasks completed."
