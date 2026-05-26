# Changelog

All notable changes to Fabric are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-05-23

### Added

- Core `db/v1.DB` interface for context-aware database access:
  - `Get`, `GetRaw`, `GetByID`, `GetByIDRaw`
  - `Insert`, `Inserts`, `Update`, `Delete`, `Exec`
  - `Upsert`
  - query preview methods
  - `Begin`, `WithTransaction`, health checks, pool stats, and close support
- Core `db/v1.Tx` interface for transaction-scoped operations:
  - read, write, upsert, and query preview operations
  - `Savepoint`, `RollbackToSavepoint`, and `ReleaseSavepoint`
  - `Commit` and `Rollback`
- Built-in database support:
  - MySQL
  - PostgreSQL
  - SQLite through the pure-Go `modernc.org/sqlite` driver
  - Microsoft SQL Server
- Fluent query builders:
  - `SelectBuilder`
  - `InsertBuilder`
  - `UpdateBuilder`
  - `DeleteBuilder`
- SQL preview for all fluent builders.
- Parameterized condition DSL with:
  - equality and comparison operators
  - grouped `AND` / `OR` / `NOT`
  - `IN` / `NOT IN`
  - `IS NULL` / `IS NOT NULL`
  - portable case-insensitive search through `ILike`
  - `BETWEEN`
- Projection helpers:
  - `Column`
  - `ColumnAs`
  - `ColumnRaw`
  - `ColumnRawAs`
- Grouped select helpers:
  - `GroupBy`
  - parameterized `Having`
  - trusted raw `HavingRaw`
  - `Count`
  - `CountRaw`
  - `CountQuery`
- Join support with aliases, equality predicates, and additional `JoinOn`
  conditions.
- Dialect-specific SQL generation for:
  - joined `SELECT`
  - joined `UPDATE`
  - joined `DELETE`
  - select pagination
  - mutation `ORDER BY` / `LIMIT` where supported
  - mutation returning/output query preview
- Upsert support:
  - MySQL `ON DUPLICATE KEY UPDATE`
  - PostgreSQL `ON CONFLICT`
  - SQLite `ON CONFLICT`
  - explicit unsupported errors for MSSQL
- Portable create-and-fetch helper through `InsertAndFetch`.
- Typed row scanning:
  - `ScanRowsTo`
  - `ScanAll`
  - `ScanOne`
  - `db` and `json` tag mapping
  - `sql.Null*` support
  - custom scanner support
- Streaming row access through `RowsAdapter`.
- RowsAdapter pooling and managed adapter helpers.
- Transaction options:
  - isolation level
  - read-only transactions
  - variadic options on `Begin` and `WithTransaction`
- Transaction panic handling:
  - callback errors roll back
  - callback panics roll back and return non-nil errors with stack details
  - rollback and commit failures are surfaced
- Transaction savepoints on `Tx`.
- Driver error mapping to Fabric sentinel errors for duplicate key, foreign key,
  syntax, timeout, connection, and cancellation scenarios.
- Explicit mutation returning/output execution rejection to avoid silently
  discarding returned rows.
- OpenTelemetry tracing integration.
- Logger adapters for standard and structured loggers.
- Plugin registry for custom database drivers.
- Plugin conformance helper for validating driver factories.
- Manager package for operational database routing:
  - lifecycle management
  - read/write workers
  - health checks
  - priority selection
  - async APIs
  - opt-in insert batching
  - batching-aware compatible insert routing
- Retry package with fixed, exponential, linear, jitter, and random strategies.
- Docker-backed integration test harness for MySQL, PostgreSQL, SQLite, and
  MSSQL.
- Documentation:
  - README
  - integration setup guide
  - error handling guide
  - manager guide
  - resource pooling guide
  - SQL null type guide
  - portability matrix
  - architecture and specification references
- Benchmarks for query building and typed scanning entry points.

### Dialect Notes

- MySQL supports non-joined mutation `ORDER BY` / `LIMIT`.
- PostgreSQL supports mutation `RETURNING` in query preview.
- MSSQL supports mutation `OUTPUT` in query preview and requires `ORDER BY` for
  offset/fetch pagination.
- SQLite uses the driver name `sqlite` and the pure-Go `modernc.org/sqlite`
  implementation.
- Mutation returning/output execution is intentionally unsupported because
  mutation execution returns `ExecResult`, not rows.
- Raw SQL helpers are explicit caller-owned escape hatches. Values should stay
  parameterized through conditions or raw query arguments.

### Security

- Values are parameterized by default.
- Identifiers are quoted per dialect.
- Identifier quote delimiters are escaped.
- Unsupported SQL operators and unsupported dialect options return explicit
  errors.
- Raw projections, raw HAVING clauses, raw queries, and raw exec statements must
  use trusted or allowlisted SQL.

### Testing

- Unit tests cover builders, dialects, conditions, query options, fluent APIs,
  transactions, row scanning, manager lifecycle, batching, retry behavior, and
  plugin conformance.
- Race test targets cover manager, DB, query, internal builder/dialect, and retry
  packages.
- Integration tests cover regular DB and FluentDB flows against SQLite and the
  Docker-backed MySQL/PostgreSQL/MSSQL matrix.
