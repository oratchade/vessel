# Changelog

All notable changes to fabric are documented in this file. This project adheres to [Semantic Versioning](https://semver.org/) and follows the [Keep a Changelog](https://keepachangelog.com/) format.

For a high-level overview of releases, see [RELEASES.md](./RELEASES.md).

## [Unreleased]

### Planned for v1.1.0

#### Added

- Extended type support (time.Time, UUID, custom JSON types)
- Batch upsert operations
- Query result caching layer
- Performance benchmarks suite

#### Changed

- Improved error messages with additional context
- Enhanced connection pool statistics
- **QueryOptions.OrderBy** - Changed from `[]string` to `[]OrderBy` struct type for better type safety and clarity

#### Fixed

- Minor performance optimizations in row scanning

---

## [1.0.0] - 2026-03-15

### Added

#### Core Features

- **Multi-Database Support** - Full support for MySQL, PostgreSQL, SQLite, and MSSQL with unified API
- **Query Builder** - Complete query builder with support for:
  - SELECT, INSERT, UPDATE, DELETE operations
  - WHERE conditions with full operator support
  - INNER, LEFT, RIGHT, FULL OUTER JOINs
  - GROUP BY and HAVING clauses
  - ORDER BY with ASC/DESC
  - LIMIT and OFFSET pagination
  - Subqueries

- **Type-Safe Row Scanning** - `ScanRowsTo()` method for mapping SQL rows to Go structs with zero-copy efficiency

- **Transaction Support**
  - Begin/Commit/Rollback with full ACID compliance
  - `WithTransaction()` helper with automatic rollback on panic
  - Savepoint support (MySQL, PostgreSQL)

- **Connection Pooling**
  - Per-dialect optimizations (pgxpool for PostgreSQL)
  - Connection pool statistics via `PoolStats()`
  - Configurable MaxOpenConns, MaxIdleConns, ConnMaxLifetime

- **Error Handling**
  - Sentinel errors: `ErrDuplicateKey`, `ErrForeignKeyConstraint`, `ErrConnectionFailed`, `ErrNoRows`, `ErrNotSupported`
  - Dialect-specific error mapping for MySQL, PostgreSQL, SQLite, MSSQL
  - Error wrapping with full context via `fmt.Errorf`

- **Type Support**
  - Basic types: `string`, `int`, `uint`, `float64`, `bool`, `[]byte`
  - SQL Null types: `sql.NullString`, `sql.NullInt64`, `sql.NullFloat64`, `sql.NullBool`, `sql.NullTime`
  - Automatic type coercion in row scanning

- **DBManager** - Multi-database management system with:
  - Health-first routing with intelligent fallback
  - Separate read-only and read-write database pools
  - Priority-based database selection
  - Worker pool pattern for concurrent query handling
  - Async query operations via channel-based API
  - Automatic health checking with configurable intervals
  - Load balancing via round-robin distribution
  - Configuration via JSON, YAML, or TOML files
  - Support for application-level sharding and geographic distribution

- **OpenTelemetry Integration** - Distributed tracing and observability
  - Automatic instrumentation of all database operations
  - Span tracking for queries, transactions, and row scanning
  - Configurable via `OTEL_ENABLED` environment variable
  - Zero overhead when disabled (no-op tracer provider)

- Query builder chaining enhancements
- **OrderBy Struct Redesign** - Type-safe ordering with structured `OrderBy` type
  - New `OrderBy` struct with `Column` and `Direction` fields
  - Improved type safety and discoverability
  - Automatic direction normalization (lowercase → uppercase)
  - Default direction handling (empty → "ASC")
  - Backward compatible with FluentDB API
  - Comprehensive documentation in `ORDERBY_CHANGES.md`

#### Documentation

- **README.md** - Comprehensive feature overview with 10+ code examples covering basic connections, queries, inserts, bulk operations, and transactions
- **ERROR_HANDLING.md** - Complete error handling guide with patterns and recovery strategies for all dialects
- **CONTRIBUTING.md** - Developer guidelines with setup instructions, code style, testing guidelines, and PR process
- **DBManager.md** - Comprehensive guide to multi-database management with architecture, configuration, and best practices
- **RELEASES.md** - Version history, roadmap, and release management
- **OPERATORS_COMPATIBILITY.md** - Detailed operator support matrix per database dialect
- **SQL_NULL_TYPES.md** - Guide to handling nullable SQL types in Go structs

#### Testing

- 260+ comprehensive test cases (241 unit tests + 20 integration tests)
- Unit tests for all core functionality with 100% pass rate
- Integration tests across all supported databases (MySQL, PostgreSQL, SQLite, MSSQL)
- Mock generation via `go:generate` for testability
- Error mapping tests for all dialects
- Test build tags for proper test isolation
- Zero test failures

#### Quality Assurance

- 40+ enabled linters (golangci-lint configuration)
- 100% pass rate on all tests
- Zero linting issues
- Gofumpt formatting compliance
- Full test coverage for critical paths

#### Infrastructure

- `.github/pull_request_template.md` - Automated PR template for GitHub
- Semantic versioning with proper version tags
- Release automation ready

### Changed

- API stabilized at `db/v1` package (breaking off from v0.9.x)
- Error types reorganized under `db/db/v1error`
- `RowsAdapter` replaces generic `Row` interface from v0.9
- `QueryOptions` structure formalized with comprehensive options

### Breaking Changes (from v0.9.0)

- Moved public API from `db` to `db/v1`
- `Row` interface replaced with `RowsAdapter`
- Error types moved from root to `dberror` subpackage
- Some error codes changed (see [MIGRATION.md](./MIGRATION.md) for mapping)
- `QueryOptions` struct signature changed

### Deprecated

- `COMMENT_UPDATE.md` - Replaced by comprehensive code comments
- Old error type names - Use sentinel errors in `dberror` instead
- `Row` interface - Use `RowsAdapter` instead

### Security

- All queries parameterized to prevent SQL injection
- Error messages sanitized to avoid information leakage
- No credentials stored in error messages
- Proper context timeout propagation

### Performance

- Zero-copy row scanning with pointer-based field mapping
- Efficient error wrapping without cascading allocations
- Connection pooling optimizations per dialect
- Minimal memory allocations in hot paths

### Database Support Matrix

| Feature            | MySQL | PostgreSQL | SQLite | MSSQL |
| ------------------ | ----- | ---------- | ------ | ----- |
| CRUD Operations    | ✅    | ✅         | ✅     | ✅    |
| Transactions       | ✅    | ✅         | ✅     | ✅    |
| JOINs              | ✅    | ✅         | ✅     | ✅    |
| Subqueries         | ✅    | ✅         | ✅     | ✅    |
| GROUP BY/HAVING    | ✅    | ✅         | ✅     | ✅    |
| Window Functions   | ✅    | ✅         | ⚠️     | ⚠️    |
| CTEs               | ✅    | ✅         | ✅     | ✅    |
| Connection Pooling | ✅    | ✅         | ✅     | ✅    |
| Error Mapping      | ✅    | ✅         | ✅     | ✅    |

### Known Issues

- Integration tests require Docker for full database testing
- Window functions have limited support on SQLite and MSSQL
- No built-in migration framework
- Planned: Batch insert/update operations (planned for v1.1.0)
- Planned: Extended type support for time.Time and UUID (planned for v1.1.0)

### Tested Against

- Go 1.26.0
- MySQL 8.0.35
- PostgreSQL 15.2
- SQLite 3.44.0
- MSSQL 2022 (21.0)

---

## Unreleased Changes

### Proposed for v1.1.0 (Q2 2026)

The following changes are proposed for the next release:

#### Features Under Discussion

- Extended type support (time.Time, UUID, custom JSON types)
- Batch insert/upsert helpers
- Query result caching layer
- OpenTelemetry integration (opt-in)
- Performance benchmarks

#### Improvements Under Discussion

- Enhanced error messages
- Connection retry policies
- Graceful shutdown helpers
- Query plan analysis
- Migration integration hints

---

## Guidelines for Maintainers

### Changelog Entry Rules

1. **Date Format** - Use ISO 8601 (YYYY-MM-DD)
2. **Version Format** - Follow semantic versioning (MAJOR.MINOR.PATCH)
3. **Categories** - Use: Added, Changed, Deprecated, Removed, Fixed, Security, Performance
4. **Description** - Use clear, user-facing language
5. **References** - Link to issues and commits

### Example Entry

```markdown
### Added

- New `BatchInsert()` method for bulk operations (fixes #123)
- Support for `time.Time` in `ScanRowsTo()`

### Fixed

- Connection pool leak when calling `Close()` (fixes #456)

### Deprecated

- `QueryOptions.Raw` field (use `Raw()` builder method instead)
```

### Releasing New Versions

1. Update CHANGELOG.md with all changes
2. Update version in code and go.mod
3. Update RELEASES.md with new release entry
4. Tag with semver format: `v1.0.0`
5. Create GitHub release with changelog excerpt

---

## External References

- [GitHub Releases](https://github.com/oratchade/fabric/releases)
- [Commit History](https://github.com/oratchade/fabric/commits/main)
- [Pull Requests](https://github.com/oratchade/fabric/pulls)
- [Issues](https://github.com/oratchade/fabric/issues)

---

## Changelog Format

This changelog is maintained according to the [Keep a Changelog](https://keepachangelog.com/) specification with the following categories:

- **Added** - New features and functionality
- **Changed** - Changes in existing functionality
- **Deprecated** - Soon-to-be removed features
- **Removed** - Removed features
- **Fixed** - Bug fixes
- **Security** - Security-related changes
- **Performance** - Performance improvements and optimizations

---

**Last Updated:** March 3, 2026  
**Maintained by:** [@oratchade](https://github.com/oratchade)

For detailed migration information, see [MIGRATION.md](./MIGRATION.md).  
For release information, see [RELEASES.md](./RELEASES.md).
