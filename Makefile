.PHONY: test coverage cobertura cover-html cover-func tools fmt fmt-check lint deps-install deps-validate check all mocks integration-test integration-test-sqlite integration-test-mysql integration-test-postgres integration-test-mssql integration-test-all

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
GOTESTSUM_VERSION ?=
GOCOVER_VERSION ?=

GOFUMPT_VERSION ?=
GOLANGCI_VERSION ?=
MOCKGEN_VERSION ?=

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
GOLANGCI_INSTALL = github.com/golangci/golangci-lint/cmd/golangci-lint@latest
else
GOLANGCI_INSTALL = github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
endif

ifeq ($(MOCKGEN_VERSION),)
MOCKGEN_INSTALL = github.com/golang/mock/mockgen@latest
else
MOCKGEN_INSTALL = github.com/golang/mock/mockgen@$(MOCKGEN_VERSION)
endif

# Build/test flags
GOFLAGS ?= -tags=test
CGO_ENABLED ?= 1
TEST_FLAGS ?= -covermode=atomic -coverpkg=./... -coverprofile=$(COVER_FILE)

tools:
	@echo "Installing: $(GOTESTSUM_INSTALL) $(GOCOVER_INSTALL) $(GOFUMPT_INSTALL) $(GOLANGCI_INSTALL) $(MOCKGEN_INSTALL)"
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(GOTESTSUM_INSTALL)
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(GOCOVER_INSTALL)
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(GOFUMPT_INSTALL)
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(GOLANGCI_INSTALL)
	@GOBIN=$(GOBIN) CGO_ENABLED=$(CGO_ENABLED) go install $(MOCKGEN_INSTALL)

test:
	@mkdir -p $(OUT_JUNIT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(JUNIT_FILE) -- $(GOFLAGS) $(TEST_PKGS)

test-integration: GOFLAGS = -tags=integration
test-integration:
	@mkdir -p $(OUT_JUNIT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(JUNIT_FILE) -- $(GOFLAGS) $(TEST_PKGS)

coverage:
	@mkdir -p $(OUT_COVER_DIR) $(OUT_JUNIT_DIR)
	CGO_ENABLED=$(CGO_ENABLED) gotestsum --format=short-verbose --junitfile $(JUNIT_FILE) -- $(GOFLAGS) $(TEST_FLAGS) $(TEST_PKGS)

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
	@chmod +x scripts/run-integration-tests.sh
	@./scripts/run-integration-tests.sh sqlite

integration-test-mysql:
	@echo "Running MySQL integration tests..."
	@chmod +x scripts/run-integration-tests.sh
	@docker-compose -f docker-compose.test.yml up -d mysql
	@sleep 10
	@DB_TYPE=mysql go test -timeout 300s -v ./tests -run "TestIntegration"
	@docker-compose -f docker-compose.test.yml down mysql

integration-test-postgres:
	@echo "Running PostgreSQL integration tests..."
	@chmod +x scripts/run-integration-tests.sh
	@docker-compose -f docker-compose.test.yml up -d postgres
	@sleep 10
	@DB_TYPE=postgres go test -timeout 300s -v ./tests -run "TestIntegration"
	@docker-compose -f docker-compose.test.yml down postgres

integration-test-mssql:
	@echo "Running MSSQL integration tests..."
	@chmod +x scripts/run-integration-tests.sh
	@docker-compose -f docker-compose.test.yml up -d mssql
	@sleep 15
	@DB_TYPE=sqlserver go test -timeout 300s -v ./tests -run "TestIntegration"
	@docker-compose -f docker-compose.test.yml down mssql

integration-test-all:
	@echo "Running all integration tests..."
	@chmod +x scripts/run-integration-tests.sh
	@docker-compose -f docker-compose.test.yml up -d
	@sleep 15
	@$(MAKE) integration-test-sqlite
	@$(MAKE) integration-test-mysql
	@$(MAKE) integration-test-postgres
	@$(MAKE) integration-test-mssql
	@docker-compose -f docker-compose.test.yml down


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

# CI check: formatting, deps validation, lint, mocks generation, and tests
check: fmt-check deps-validate lint mocks coverage
	@echo "All checks passed."

all: tools fmt lint deps-install deps-validate mocks check test coverage cobertura cover-html cover-func
	@echo "All tasks completed."