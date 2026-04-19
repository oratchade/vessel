# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-04-19

### Added

- Multi-database support: MySQL 5.7+, PostgreSQL 9.6+, SQLite 3.x, MSSQL 2016+
  with consistent API
- Configuration with variable expansion and environment file loading
- Fluent query DSL with SelectBuilder, InsertBuilder, UpdateBuilder, DeleteBuilder
- Method chaining for ergonomic query construction
- Type-safe SQL generation with parameterized queries
- Identifier quoting per database dialect
- SQL function support (SUM, COUNT, AVG, MAX, MIN, CONCAT, UPPER, LOWER)
- Aliasing support (AS keyword)
- Pluggable logger adapters: SlogAdapter, LogrusAdapter, ZapAdapter, ApexAdapter
- CRUD operations (Get, Insert, Update, Delete, Query, Exec)
- JOIN support (INNER, LEFT, RIGHT, FULL OUTER) in SELECT, UPDATE and DELETE queries
- Subqueries and expressions
- Aggregate functions with GROUP BY and HAVING clauses
- ORDER BY with ASC/DESC direction control
- DISTINCT support
- LIMIT and OFFSET pagination
- RETURNING clauses (PostgreSQL, SQLite)
- Bulk inserts for all 4 databases
- Query plan capabilities (EXPLAIN format)
- Transaction support with atomicity guarantees, nested transaction awareness
- Context propagation with timeouts and automatic rollback
- OpenTelemetry tracing integration for distributed tracing
- Plugin system with custom driver registration
- Manager API for multi-database connection management
- Database routing by entity type
- Configuration loading from YAML/TOML/JSON
- Health check support
- Row scanning abstraction with universal Row interface
- Type-safe field access and flexible scanning strategies
- 829 comprehensive tests with 100% pass rate

### Changed

- Fluent API redesigned for ergonomics with clear separation of concerns
- OrderBy implementation restructured to support ASC/DESC direction control
- SQL function support enhanced with better AS aliases
- Replaced `interface{}` with `any` throughout codebase
- Changed from async-first to sync-first API design
- Project restructured with `db/v1` package naming
- Manager query routing improved for better performance
- Synchronous API as default, matching Go's standard library patterns
- Interface modernization using modern Go idioms
- Gitignore updated with complete coverage of build artifacts

### Fixed

- Transaction ID uniqueness improved for high-concurrency scenarios
- PostgreSQL support fixed: removed `lastInsertID` dependency, uses RETURNING clause
- MSSQL integration tests fixed: improved connection pool timeout handling
- PostgreSQL hanging connection tests resolved
- Error handling improved with `ErrQueryTimeout` and comprehensive error wrapping
- Linting infrastructure added with markdown linting and CI/CD integration

## Documentation

Comprehensive documentation created:

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - 5-layer architecture breakdown,
  design patterns
- [docs/CODE_REVIEW.md](docs/CODE_REVIEW.md) - Architecture review,
  testing improvements, quality standards
- [docs/ENVIRONMENT_VARIABLES.md](docs/ENVIRONMENT_VARIABLES.md) - Configuration
  guide, test setup
- [docs/ERROR_HANDLING.md](docs/ERROR_HANDLING.md) - Error types,
  database error codes, NULL mapping
- [docs/DB_MANAGER.md](docs/DB_MANAGER.md) - Multi-database management, load balancing
- [docs/LINTING.md](docs/LINTING.md) - Code style standards, 40+ linters configuration
- [README.md](README.md) - Quick start guide, feature overview, OpenTelemetry integration
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines, development setup
- [Agents.md](Agents.md) - AI agent customization patterns

## Testing & Quality Assurance

- 829 comprehensive tests with 100% pass rate
- Builder tests: 30+ new test functions for UpdateAll/DeleteAll operations
- Dialect testing: MySQL, PostgreSQL, SQLite, MSSQL with operator coverage
- Query options testing: OrderBy, Limit/Offset, GroupBy, HAVING, RETURNING
- Complex conditions testing: nested AND/OR, wildcards, Unicode handling
- Integration tests: 19+ tests across MySQL, PostgreSQL, SQLite with Docker containers
- Retry integration examples with 4 comprehensive patterns and 27 test cases
- Test coverage improved across all modules (55%+ → 80%+)
- Full CI/CD integration with 100% pass rate maintained

## Dependencies

### Core Dependencies

- `database/sql` - Standard library database interface
- `github.com/mitchellh/go-wordwrap` - CLI text wrapping
- `gopkg.in/yaml.v3` - YAML configuration parsing
- `github.com/stretchr/testify` - Testing assertions

### Database Drivers

- `github.com/go-sql-driver/mysql` - MySQL driver
- `github.com/jackc/pgx` - PostgreSQL driver
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/denisenkom/go-mssqldb` - MSSQL driver

### Logger Adapters (Optional)

- `log/slog` - Go stdlib logging
- `github.com/sirupsen/logrus` - Structured logger
- `go.uber.org/zap` - High-performance logger
- `github.com/apex/log` - Minimal logging

### Observability

- `go.opentelemetry.io/otel` - OpenTelemetry tracing

## Security

- Parameterized queries throughout entire codebase (zero SQL injection vectors)
- Identifier quoting per database dialect
- No string concatenation in SQL generation
- Type-safe value binding
- No hardcoded secrets or credentials
- Environment variable configuration
- No known CVEs in dependencies
- Regular updates from maintainers

## Known Limitations

1. **Default Row API Returns Maps** - `Get()` materializes all rows as `map[string]any`
   (high GC pressure on massive datasets)
   - Mitigation: Use `GetRaw() + ScanRowsTo[T]()` for zero-copy streaming
     into typed structs

1. **No ORM Features** - Fabric is query-focused, not entity-focused
   - By design: Gives full SQL control; ORM can be built on top if needed

1. **Manager API Maturity** - Less documented than core db/v1
   - Roadmap: Dedicated tutorial and examples in v1.1

1. **No Property-Based Testing** - Fuzzing not yet integrated
   - Roadmap: Add fuzzing for SQL injection validation in v1.1

## Roadmap

### v1.1 (Planned)

- Performance benchmarks
- Manager API tutorial and examples
- Property-based testing with fuzzing
- Performance tuning guide

### v1.2 (Future)

- Query result caching layer
- Connection pool auto-tuning
- Metrics export (Prometheus)
- Schema migration helpers

## Credits

**Fabric** is maintained by the **oratchade** team with contributions
from the Go community.

Contributors: Touni Atchadé (@oratchade), GitHub Copilot

## License

Fabric is licensed under the [MIT License](LICENSE.md).

---

[1.0.0]: https://github.com/oratchade/fabric/releases/tag/v1.0.0
