# Changelog

All notable changes to Fabric are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0] - 2026-04-03 (INITIAL RELEASE)

### 🎉 Overview

Fabric v1.0.0 is the first stable release of a production-grade SQL
abstraction library for Go. It provides a type-safe, multi-database
query builder and abstraction layer that unifies MySQL, PostgreSQL,
SQLite, and MSSQL behind a single fluent API.

**Key Milestone**: 829 comprehensive tests (100% pass rate) across
all modules and databases.

---

## Added

### Core Features

- **🗄️ Multi-Database Support**
  - MySQL 5.7+ (`mysql.go`)
  - PostgreSQL 9.6+ (`postgres.go`)
  - SQLite 3.x (`sqlite.go`)
  - MSSQL 2016+ (`mssql.go`)
  - Consistent API across all databases

- **🔄 Fluent Query DSL** (`db/v1/fluentDB.go`)
  - SelectBuilder with WHERE, ORDER BY, LIMIT, OFFSET, GROUP BY,
    HAVING
  - InsertBuilder with single and bulk inserts
  - UpdateBuilder with WHERE filtering and UpdateAll() for unfiltered updates
  - DeleteBuilder with WHERE filtering and DeleteAll() for unfiltered deletes
  - Method chaining for ergonomic query construction

- **🛡️ Type-Safe SQL Generation**
  - Parameterized queries throughout (zero SQL injection vectors)
  - Identifier quoting per database dialect
  - SQL function support (SUM, COUNT, AVG, MAX, MIN, CONCAT, UPPER, LOWER, etc.)
  - Aliasing support (AS keyword)

- **🔌 Pluggable Logger Adapters** (`db/v1/logger_adapters.go`)
  - **SlogAdapter** - Go stdlib `log/slog.Logger` integration
  - **LogrusAdapter** - `github.com/sirupsen/logrus` integration with
    dual type support
  - **ZapAdapter** - `go.uber.org/zap` integration with efficient field handling
  - **ApexAdapter** - `github.com/apex/log` integration
  - All adapters implement Logger interface with context
    chaining via `With()`

- **📊 Query Capabilities**
  - CRUD operations (Get, Insert, Update, Delete, Query, Exec)
  - JOINs (INNER, LEFT, RIGHT, FULL OUTER)
  - JOIN support in UPDATE and DELETE queries
  - Subqueries and expressions
  - Aggregate functions with GROUP BY
  - HAVING clauses for aggregate filtering
  - ORDER BY with ASC/DESC direction control
  - DISTINCT support
  - LIMIT and OFFSET pagination
  - RETURNING clauses (PostgreSQL, SQLite support)
  - Bulk inserts for all 4 databases
  - Query plan capabilities (EXPLAIN format)

- **🔐 Transaction Support** (`db.Tx` interface)
  - Atomicity guarantees
  - Nested transaction awareness
  - Context propagation with timeouts
  - Automatic rollback on errors

- **🌎 Observability**
  - OpenTelemetry tracing integration (`internal/pkg/otel/`)
  - Distributed tracing support for database operations
  - Transaction context tracking

- **🔌 Plugin System** (`db/v1/plugin/`)
  - Custom driver registration
  - Extensible without core modifications
  - Registry-based driver discovery

- **📦 Manager API** (`manager/v1/`)
  - Multi-database connection management
  - Database routing by entity type
  - Configuration from YAML/TOML/JSON
  - Health check support
  - Request dispatch with query routing (feature)

- **✅ Row Scanning Abstraction** (`db/v1/row_adapter.go`)
  - Universal Row interface for all databases
  - Type-safe field access
  - Flexible scanning strategies

---

## Fixed

### Bug Fixes & Stability

- **Transaction ID Uniqueness** (`9b75bf1`)
  - Improved transactionID generation to ensure uniqueness in high-concurrency scenarios
  - Prevents transaction lookup failures

- **PostgreSQL Support**
  - Removed `lastInsertID` dependency (PostgreSQL doesn't support it)
  - Uses RETURNING clause instead for last insert ID retrieval
  - Full PostgreSQL dialect compliance

- **MSSQL Integration Tests** (`2056065`)
  - Fixed hanging MSSQL integration tests
  - Improved connection pool timeout handling
  - Reliable CI/CD execution on MSSQL

- **PostgreSQL Hanging Connection Tests** (`48f6557`)
  - Fixed hanging postgres integration test
  - Better connection cleanup and timeout handling

- **Error Handling**
  - Added `ErrQueryTimeout` for timeout scenarios
  - Renamed `ErrGrammarError` to be more idiomatic
  - Comprehensive error wrapping with context

---

## Improved

### Code Quality & Architecture

- **Fluent API Redesign** (`29099e1`)
  - Restructured for maximum ergonomics
  - Chainable builder methods
  - Clear separation of concerns (improved)

- **OrderBy Implementation** (`d07bace`)
  - Restructured to support ASC/DESC direction control
  - Flexible column expression handling
  - Proper dialect-specific SQL generation

- **SQL Function Support** (`4badb9e`)
  - Better support for AS aliases
  - SQL function integration in column expressions
  - Proper parenthesis handling (improved)

- **Interface Modernization** (`a998f29`)
  - Replaced `interface{}` with `any` throughout
  - More readable, modern Go idioms
  - No functional changes, style improvement

- **Synchronous API as Default** (`d896240`)
  - Changed from async-first to sync-first design
  - Better performance for typical use cases
  - Matches Go's standard library patterns

- **Project Restructuring** (`1902c35`, `da85ff6`)
  - Renamed from `db` to `v1` package structure
  - More idiomatic Go versioning approach (`db/v1` instead of `v1/db`)

- **Manager Query Routing** (`fe2fc27`)
  - Improved routing logic for multi-database queries
  - Better performance characteristics
  - Enhanced query dispatch

- **Linting Infrastructure** (`1f1ddda`)
  - Added markdown linting for documentation
  - CI/CD integration for code quality
  - 40+ linters enabled

- **Gitignore Updates** (`a33ed93`)
  - Complete coverage of build artifacts
  - IDE and tooling files ignored
  - Cleaner repository state

---

## Documentation

### Comprehensive Guides Created

- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**
  - 5-layer architecture breakdown
  - Component responsibilities
  - Design patterns and rationales
  - Extension points via plugin system

- **[docs/CODE_REVIEW.md](docs/CODE_REVIEW.md)**
  - Complete architecture review
  - Phase 5 testing improvements
  - Future recommendation roadmap
  - Code quality standards and testing requirements

- **[docs/ENVIRONMENT_VARIABLES.md](docs/ENVIRONMENT_VARIABLES.md)**
  - Complete credential configuration guide
  - Test environment setup with environment variables
  - CI/CD integration examples
  - Database connection string templates

- **[docs/ERROR_HANDLING.md](docs/ERROR_HANDLING.md)**
  - Error types and handling patterns
  - Database-specific error codes reference
  - NULL type mapping and best practices
  - Wrapped error guidance

- **[docs/DBMANAGER.md](docs/DBMANAGER.md)**
  - Multi-database management and routing
  - Configuration loading from YAML/TOML/JSON
  - Health check API usage
  - Load balancing across databases

- **[docs/LINTING.md](docs/LINTING.md)**
  - Code style and linting standards
  - 40+ enabled linters configuration
  - Format and quality requirements

- **[Agents.md](Agents.md)**
  - AI agent customization for fabric project
  - Claude Code integration patterns

- **[README.md](README.md)**
  - Quick start guide with code examples
  - Feature overview and installation
  - OpenTelemetry integration documentation
  - Integration test instructions

- **[CONTRIBUTING.md](CONTRIBUTING.md)**
  - Contribution guidelines
  - Development setup instructions
  - Code style standards and conventions

---

## Testing

### Comprehensive Test Suite: 802 Tests (100% Pass Rate)

#### Phase 5 Testing Improvements

- **Builder Tests** - 30+ new test functions across critical modules
  - UpdateAll() operations: 5 test cases
  - DeleteAll() operations: 4 test cases
  - Edge cases and error paths: 21+ additional tests

- **Dialect Testing** (`internal/pkg/sqldialect/operators_test.go`)
  - MySQL dialect: Backtick identifier quoting
  - PostgreSQL dialect: Double-quote identifier quoting
  - SQLite dialect: No-quote mode
  - MSSQL dialect: Square bracket identifier quoting
  - Operator coverage: =, >, <, >=, <=, !=, LIKE, IN, BETWEEN, AND, OR
  - Complex comparisons and NULL handling
  - Coverage improved: 55.8% → 75%+

- **Query Options Testing** (`pkg/query/options/options_test.go`)
  - OrderBy behavior: Single and multiple columns
  - Limit and Offset: Pagination correctness
  - GroupBy and Having: Aggregate query validation
  - Returning clause: Database-specific RETURNING support
  - Coverage improved: 0% → 85%+

- **Complex Conditions Testing** (`pkg/query/condition/complex_conditions_test.go`)
  - Nested AND/OR expressions
  - Pattern matching for wildcards
  - Unicode identifier handling
  - NULL comparisons and edge cases
  - Coverage improved: 59.4% → 80%+

#### Integration Tests

- **Multi-Database Coverage**
  - 19+ integration tests across MySQL, PostgreSQL, SQLite
  - Docker-based test containers
  - Real database validation
  - MSSQL-specific tests

#### CI/CD Integration

- Full test suite runs on every commit
- 100% pass rate maintained
- Coverage reporting enabled
- Test artifacts preserved

#### Phase 6 Retry Integration Examples (April 3, 2026)

- **Retry Example Package** (`examples/manager-example/retry/`)
  - 4 comprehensive example patterns with retry integration
  - `exampleBasicRetry()`: Default exponential, linear, and fixed
    backoff patterns
  - `exampleContextTimeout()`: Context deadline handling with
    2-second timeout
  - `exampleBackoffStrategies()`: Reference guide for 5 practical
    scenarios
  - `exampleRetryPatterns()`: Real-world patterns (read fallback,
    write guarantee, batch ops)

- **Documentation & Configuration**
  - `examples/manager-example/retry/README.md`: Comprehensive strategy recommendations
  - `examples/manager-example/retry/config.yaml`: Multi-entry database setup
  - Updated `examples/manager-example/README.md`: Added section 4 with retry examples
  - Added to "Running All Examples" workflow

- **Test Integration**
  - 27 new test cases integrated from retry examples
  - Total tests: 802 → 829 tests (all passing, 100% pass rate)
  - Example patterns demonstrate backoff strategies (exponential, linear, fixed)
  - Production-ready example code for developers

---

## Dependencies

### Core Dependencies

- `database/sql` - Standard library database interface
- `github.com/mitchellh/go-wordwrap` - CLI text wrapping
- `gopkg.in/yaml.v3` - YAML configuration parsing
- `github.com/stretchr/testify` - Testing assertions

### Database Drivers

- `github.com/go-sql-driver/mysql` - MySQL driver
- `github.com/jackc/pgx` - PostgreSQL driver (high-performance)
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/denisenkom/go-mssqldb` - MSSQL driver

### Logger Adapters (Optional)

- `log/slog` - Go 1.21+ stdlib JSON logging
- `github.com/sirupsen/logrus` - Popular structured logger
- `go.uber.org/zap` - High-performance typed logger
- `github.com/apex/log` - Minimal, clean logging

### Observability

- `go.opentelemetry.io/otel` - OpenTelemetry tracing

### Test Assertions

- `github.com/stretchr/testify` - Assertions and mocking

---

## Security

### SQL Injection Prevention

- ✅ Parameterized queries throughout entire codebase
- ✅ Identifier quoting per database dialect
- ✅ No string concatenation in SQL generation
- ✅ Type-safe value binding

### Credential Management

- ✅ No hardcoded secrets in codebase
- ✅ Environment variable configuration
- ✅ `.gitignore` prevents credential leaks
- ✅ Error messages never leak passwords/keys

### Dependency Security

- ✅ No known CVEs in dependencies
- ✅ Regular updates from maintainers
- ✅ Secure versions of all database drivers

---

## Known Limitations

1. **Row Materialization**: All rows returned as maps (high GC pressure on
   massive datasets)
   - **Mitigation**: `GetRaw()` for streaming; `RowsAdapter` for custom scanning

2. **No ORM Features**: Fabric is query-focused, not entity-focused
   - **By Design**: Gives full SQL control; ORM can be built on top if needed

3. **Manager API Maturity**: Less documented than core db/v1
   - **Roadmap**: Dedicated tutorial and examples in v1.1

4. **No Property-Based Testing**: Fuzzing not yet integrated
   - **Roadmap**: Add fuzzing for SQL injection validation in v1.1

---

## Roadmap

### v1.1 (Planned)

- [ ] Performance benchmarks (`*_bench_test.go`)
- [ ] Manager API tutorial and examples
- [ ] Property-based testing with fuzzing
- [ ] CHANGELOG semantic versioning automation
- [ ] Performance tuning guide (`docs/PERFORMANCE.md`)

### v1.2 (Future)

- [ ] Query result caching layer
- [ ] Connection pool auto-tuning
- [ ] Metrics export (Prometheus)
- [ ] Schema migration helpers

---

## Credits

**Fabric** is maintained by the **oratchade** team with contributions
from the Go community.

### Key Contributors

- Touni Atchadé (@oratchade) - Creator and architect
- GitHub Copilot - Assisted with Phase 5 testing improvements

---

## License

Fabric is licensed under the [MIT License](LICENSE.md).

---

## Getting Started

### Installation

```bash
go get tounilab.com/fabric
```

### Quick Example

```go
import "tounilab.com/fabric/db/v1"

cfg := v1.PostgresConfig{
    Host:     "localhost",
    Port:     5432,
    User:     "postgres",
    Password: os.Getenv("DB_PASSWORD"),
    DBName:   "myapp",
}

logger := v1.NewSlogAdapter(slog.Default())
db, err := v1.NewDB(cfg, logger)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Simple query
rows, err := v1.NewFluentDB(db, context.Background()).
    Select("users", "id", "name", "email").
    Where(
        condition.NewExpr().Column("age").Op(">").Value(18),
    ).
    OrderBy("name ASC").
    Limit(10).
    Execute()
```

See [README.md](README.md) and [examples/\*\*](examples/) for more.

---

## Version History

| Version | Date       | Status | Notes            |
| ------- | ---------- | ------ | ---------------- |
| 1.0.0   | 2026-04-03 | Stable | v1.0 - 802 tests |

---

**Last Updated**: April 3, 2026  
**Maintainer**: oratchade  
**Status**: ✅ Stable Production Release
