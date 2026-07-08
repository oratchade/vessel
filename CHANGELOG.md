# Changelog

All notable changes to Vessel are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-07-08

### Removed

- Removed the unused `RetryMetricsCollector` and `QueryRetryMetrics` types
  from `manager/v1`. They were never wired into any retry path
  (`RecordAttempt` was never called) and had no callers, tests, or docs.
- **Breaking:** removed `DBManager.MultiEntryQuery`,
  `DBManager.QueryWithCustomRetry`, and the `MultiEntryQueryFunc` type.
  Both were near-duplicates of `QueryWithRetry`, which now covers their
  use cases: its callback receives the 1-indexed attempt number
  (`RetryableQueryFunc`), and a custom strategy is passed via
  `QueryWithRetryConfig{Strategy: ...}`. This also removes a bug where
  `MultiEntryQuery` returned a nil error on failure when no logger was
  configured.

### Changed

- Updated indirect dependencies (`golang.org/x/*`, `modernc.org/libc`).
- Hoisted per-call regular-expression compilation out of hot paths: the
  projection alias pattern in the query builder was compiled once per
  column per query, and the table-name pattern in `SanitizeTableName`
  once per logged query. Both are now compiled once at package init.
- **Breaking:** `DBManager.QueryWithRetry` callbacks now receive the
  1-indexed attempt number: the parameter type changed from
  `func(context.Context) ([]map[string]any, error)` to
  `RetryableQueryFunc`. Existing callbacks compile after adding an
  ignored `_ int` parameter.
- `NewFluentDB` now declares its parameter as the named `dbActions`
  interface instead of an equivalent anonymous interface literal, removing
  a type assertion that could never fail. Call sites are unaffected.
- The MySQL DSN builder formats `parseTime` with `strconv.FormatBool`
  instead of `fmt.Sprintf("%t", ...)`. Identical output, idiomatic form.
- Dialect detection in the SQL builder now uses type identity (type
  switches on the concrete dialect types) instead of matching the printed
  type name with `strings.Contains(fmt.Sprintf("%T", d), "MySQL")`. The
  duplicated name-sniffing capability fallback in `CapabilitiesFor` was
  deleted; all built-in dialects implement `CapabilityProvider`, and
  custom dialects that don't now report no capabilities instead of
  capabilities guessed from their type name.
- `GenerateTransactionID` now discards the `crypto/rand.Read` error
  explicitly (`_, _ =`). The call cannot fail on supported Go versions;
  the explicit discard documents that and satisfies errcheck.
- Simplified `SafeLogger.Debug` from `Debug(msg ...string)` to
  `Debug(msg string)`. It no longer logs a fixed `"database debug"`
  message with the caller's text buried in a field; the caller's message
  is now the log message. All callers already passed a single string.

### Fixed

- `Postgres.Commit` and `Postgres.Rollback` now pass the span-derived
  context to the pgx commit/rollback call and to the safe logger, matching
  every other traced method on the driver. Previously they discarded the
  context returned by the tracer, so those operations and their log events
  were not correlated with the span.
- Fixed `QueryWithRetry` and `ExecWithRetry` returning a non-nil error on
  success: the success path wrapped a nil error with `%w`, so every
  successful call reported `query with retry failed: %!w(<nil>)`. Both
  helpers now return a nil error when the operation succeeds.
- Fixed typed scanning (`ScanRowsTo`, `ScanAll`, `ScanOne`) silently
  corrupting numeric columns scanned into `string` struct fields: the
  reflect conversion fast path turned integers into their Unicode code
  point (`int64(65)` became `"A"`). Numeric values are now formatted as
  decimal text (`"65"`).
- Fixed embedded-struct column mapping so a field declared on the outer
  struct shadows a same-named field promoted from an embedded struct,
  matching `encoding/json` depth semantics. Previously the embedded field
  silently won, scanning the column into the wrong field. Same-depth
  collisions between embedded structs are now dropped as ambiguous
  instead of resolving by declaration order.
- Fixed the database manager silencing all driver-level logging: each
  `DBEntry` constructed its `db.DB` with a `nil` logger, so slow-query
  warnings, query errors, and transaction lifecycle events were dropped
  for databases accessed through the manager. The manager's logger is now
  passed through to the driver.
- `ORDER BY` directions are now validated at the SQL-generation layer for
  every entry point: anything other than `ASC`/`DESC` (case-insensitive,
  empty defaults to `ASC`) returns an error instead of being concatenated
  raw into the generated SQL. Previously only the fluent API validated
  directions; direct `DB`/manager calls with hand-built query options
  bypassed the check.
- Manager configuration is now validated fail-fast: entries must have a
  name, a `type` of `readonly` or `readwrite`, and names must be unique.
  Previously an entry with a missing or misspelled type was silently
  dropped at startup, and duplicate names silently overwrote earlier
  entries. Configs that omitted `type` must now set it explicitly.
- `ClassifyError` now prefers typed detection (the `dberror` sentinels
  attached by the drivers' error mappers, and `context.Canceled`) over
  message substring matching, and the string fallbacks were narrowed:
  a bare `near` no longer classifies as a syntax error, a bare `invalid`
  no longer classifies as a validation error, MySQL's `SQL syntax`
  message is now recognized, and a duplicated `connection refused`
  needle was removed.
- `SelectBuilder.Count` now accepts every `COUNT(*)` return type drivers
  produce — including `string`/`[]byte` from text protocols and unsigned
  integers — by reusing the shared numeric coercion helper. Previously
  only `int64`/`int`/`int32`/`float64` were handled and other types
  errored.

## [0.1.3] - 2026-06-18

### Added

- Added manager-level `Upsert` and `UpsertAsync` APIs, including write-worker
  dispatch and coverage for direct execution and insert-batching interaction.
- Added bulk upsert support through `Upserts`, `UpsertsQuery`, fluent
  `InsertBuilder.Upserts`, and manager `Upserts`/`UpsertsAsync`.

### Changed

- Simplified repeated manager write enqueue and synchronous mutation response
  handling while preserving existing public method signatures and behavior.
- Documented manager upsert usage and clarified that `PingAsync` returns an
  immediate error rather than a response channel.

### Fixed

- Removed a stale manager test double with outdated `db.DB` method signatures.
- Corrected manager `Update` and `Delete` examples to match the current
  argument order.
- Preserved pgxpool defaults for omitted PostgreSQL pool settings and made the
  PostgreSQL integration readiness check deterministic.

## [0.1.2] - 2026-06-13

### Fixed

- Scan SQL array columns (e.g. `uuid[]`, `text[]`, `int[]`) into Go slice fields.
  pgx/v5 delivers arrays as `[]interface{}` carrying the driver's native per-element
  type (UUIDs arrive as `[16]byte`); the previous JSON fallback could not map those
  elements onto the destination's element type and failed with errors such as
  `cannot unmarshal array into Go value of type string`. Slice fields are now
  populated element-by-element, reusing the existing scalar/pointer/Scanner rules
  (including the `[16]byte`→UUID-string reformat). Byte slices (`[]byte`) and
  JSON-array strings continue to use the JSON path.

## [0.1.1] - 2026-05-27

### Changed

- Renamed the module and all public documentation to Vessel.
- Updated public import paths to `tounilab.com/vessel`.
- Renamed integration strict-mode environment variable to
  `VESSEL_INTEGRATION_STRICT`.
- Refreshed package documentation, examples, and release automation metadata for
  the Vessel module path.

### Removed

- Removed remaining previous-name references from the Vessel repository with no backward
  compatibility aliases.

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
- Driver error mapping to Vessel sentinel errors for duplicate key, foreign key,
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
