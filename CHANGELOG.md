# Changelog

All notable changes to Vessel are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.1] - 2026-10-18

### Added

- **`MSSQLConfig.Validate`** refuses a host the SQL Server driver cannot reach or would
  read as another address, naming the host only: a named instance or a link-local zone
  at an IPv6 address, or any host the driver's parser does not give back as configured.
  Opening a manager and `migrate.New` call it, so `migrate.New` refuses such a host
  where it used to return a migrator that failed at `Up`.

### Fixed

- **An IPv6 host may be written bare or in brackets.** The PostgreSQL and MySQL DSNs
  joined host and port with a bare colon, so `fd00::5` and its port read as one invalid
  address, and the SQL Server DSN, which bracketed the host, turned `[fd00::5]` into
  `[[fd00::5]]`. Each now brackets an IPv6 address, given either way, beside its port.
  Any other host is passed as written, as before for PostgreSQL and MySQL. For SQL
  Server, a host its driver would not read back as written, such as `[::1`,
  `0:0:0:0:0:0:0` or `srv host`, is refused naming only the host: it connected to
  another address (on Go 1.27, `0:0:0:0:0:0:0` reached `[::1]:1433`), or failed on a
  driver parse error that quoted the whole DSN, password included, outside the manager.
- **An SQL Server named instance at an IPv6 address is refused, saying why.** The driver
  cannot find one there in either spelling, and failed on its own parse, Browser or
  lookup error; give the instance's port instead. A link-local IPv6 address with its zone
  (`fe80::1%eth0`) is refused for SQL Server too, since its driver dials without the
  zone, which only a link-local address needs; PostgreSQL and MySQL keep it.

## [0.5.0] - 2026-10-17

### Fixed

- **A substituted config value can no longer change how the file parses (#85).**
  `${VAR}` was expanded in the file's raw text before parsing, so a value was pasted in
  as syntax: a password beginning with `*` was read as a YAML alias, one with `"` broke
  a JSON or TOML string, and the manager failed to start with a valid credential. Each
  placeholder now stands as a number while the file parses, and values go into the
  parsed data, where the format's own library escapes them:
  - in any string, a value is text, exactly as it is;
  - unquoted in YAML, or as a whole bare value in JSON and TOML, it is typed as if
    written there, so ports and durations still decode;
  - in a key, it is one key.

  **Load and validation errors never carry a substituted value.** A decoder error at a
  YAML line or a JSON or TOML key holding one names the placeholder instead
  (`entries.postgres.port: the value of ${DB_PORT} is not valid here`): decoders quote
  part of a value they reject, YAML up to its first seven characters. Other errors have
  each value replaced with `[REDACTED]`. **From setup on, the passwords are redacted**,
  wherever each came from and with or without an EnvOption: from setup errors and from
  every line the manager, its entries and their drivers log for as long as it runs,
  while the user, host and database stay readable. A redacted error is new, not a wrapper, since a TOML decode
  error kept the expanded source and the PostgreSQL driver's parse error keeps the
  connection string.

  Behaviour changes:

  - **A value escaped or quoted for the file now arrives with its escapes or quotes.**
    Expanded as text, a value was read by the file's syntax, so a `\"` in a
    double-quoted string, a doubled quote in a YAML single-quoted one, or a value
    JSON-encoded to survive the file lost its escapes. A value now goes in as it is: drop
    such workarounds when upgrading.
  - **A default is a value too**, redacted from load errors like one from the
    environment. Escapes in it are no longer read (`"${X:a\tb}"` gives a
    backslash and a `t`), and it cannot hold syntax, such as a quote that opens a string
    for the next placeholder.
  - **In JSON and TOML, placeholders glued to other text outside quotes fail the load**
    (`54${P}`, `${A}${B}`): quote them. In YAML that text is a plain scalar, typed as
    a whole.
  - **A value can no longer inject structure.** A value expanding to `[a, b]` is that
    text, a string; outside quotes, a value that is not a JSON number, boolean or null,
    or a TOML integer or boolean, is a string, which a numeric field rejects; a key from the
    environment is one key, so a dot in it no longer reaches into another table.
  - **A placeholder the file's syntax reads part of fails the load**, naming it: inside
    an escape (`"\u${X}"`, `"\x${X}"`), or in a number written around it (`${X}.5` in
    TOML). Expanded as text, the value built the escape or the number.
  - **A value or a default that is not valid UTF-8 fails the load**, naming its
    variable, instead of reaching the database with its bytes replaced.
  - A YAML alias must repeat its anchor's placeholder as written (`&${A}` … `*${A}`), and
    a placeholder in a YAML tag fails the load.
  - In JSON and TOML, two keys equal but for case, which the decoder matches to one
    field, now fail the load once any EnvOption is set, rather than the last one
    written winning. So does a JSON key written twice in one object, where two objects
    under one key were merged and the last of two values won.
  - A non-empty value that YAML would read as null, such as `null` or `~`, now arrives
    as that string rather than empty.
  - A placeholder in a comment is no longer expanded.
  - The validation error for an unknown entry type no longer quotes the type it got.
  - A failure to create or ping an entry's database is logged once per level by the
    setup that called it, redacted, instead of also by the entry itself.
  - `errors.Is` and `errors.As` on a setup error, and once anything was substituted on a
    load or validation error, reach only an errno or a context error, not the driver's or
    decoder's own error, which can hold a password the driver read itself (`PGPASSWORD`).
    **This applies without an EnvOption too**, as does the redaction of passwords from
    setup errors and logs.

- **An MSSQL named instance written as the host (`srv\INSTANCE`) reaches the driver
  (#89).** The backslash went into the URL's host, escaped, which the driver rejects; the
  instance now goes in the path, where the driver reads it. The driver then finds the
  instance's port through SQL Server Browser (UDP 1434), so the configured `port` is not
  sent with an instance: to connect on a fixed port, write the host without it.
- **An MSSQL host given as an IPv6 address (`fd00::5`) connects.** It went beside its port
  unbracketed, so the driver could not tell the host from the port.
- **An MSSQL entry without `encrypt` connects, encrypted (#88).** The connection string
  wrote it empty, which the driver rejects. Unset now means `true`: the whole session is
  encrypted, a server without encryption is refused, and its certificate is verified
  unless `trust_server_cert` is set, as a self-signed one needs. The driver's own
  default would encrypt at most the login, and none of it when the server answers that
  it has no encryption; write `false` or `disable` to choose less.
- **An MSSQL user, password or database holding URL syntax reaches the server as
  given.** The connection string pasted them in unescaped: `%41` in a password arrived
  as `A`, and `@`, `/` or `%` could make it fail to parse with an error quoting part of
  the password.

## [0.4.0] - 2026-10-12

### Changed

- **Behaviour change: a caller's context now governs the query, not only the wait (#82).**
  Queued queries ran with the worker's own context, so a cancelled or expired caller only
  stopped waiting: the statement ran on, a write the caller gave up on could still commit,
  and against a stalled database every abandoned query held a worker until the TCP
  connection died. A query now runs in a context that ends when its caller's ends, when a
  synchronous caller stops waiting (the 30 s default included), or when the manager shuts
  down; a query whose caller gave up while it was queued is answered with an error and not
  run. Raw rows (`GetRaw`, `GetByIDRaw`, `QueryRaw`, `ExecReturning`) keep that context
  alive until they are closed, through the new `db.RowsAdapter.OnClose`. Batched inserts
  keep the worker's context, since one batch carries several callers' rows.
  Transactions already used the caller's context and are unchanged. **To keep work running
  after its caller's context ends, detach it with `context.WithoutCancel`.** Async callers'
  contexts now bound execution too.

### Added

- `db.RowsAdapter.OnClose(f)` registers a function to run once, after the rows are closed.
  Every function registered runs, last registered first, so a caller's hook never replaces
  the manager's own cleanup.

### Fixed

- A database that stops answering without closing its connection now turns its entry
  unhealthy (#81). The background health check pinged with no deadline, so a stalled
  server blocked the ping, no failure was ever counted, and `Ping` and `HealthStatus`
  reported the entry healthy for as long as the stall lasted. Each ping is now bounded
  by the entry's health interval: a slower ping counts as a failure, and the usual five
  in a row mark the entry unhealthy. The wait is bounded by the health check itself, so a
  driver that blocks past cancellation (go-mssqldb waits for the server to acknowledge it)
  cannot hold the check either, and a stuck ping is never joined by another.

## [0.3.0] - 2026-09-30

### Added

- `migrate/v1` applies versioned SQL migrations with goose (`github.com/pressly/goose/v3`
  v3.28.0). `New(cfg, fsys, opts...)` takes the same `DBConfig` as `db.NewDB` and
  opens a short-lived connection pool of its own, released by `Close`; no `*sql.DB`
  is exposed. `Up`, `Status` and `Version` return Vessel's own `Result` and `Status`
  types, so goose stays out of the public API. `WithVersionTable` renames the
  version table; `WithSlog` sends progress to a `*slog.Logger` with goose's structured
  fields, executed statements at Debug. Only the supplied file system runs:
  goose's process-wide registry of Go migrations is disabled, so a migration another
  component registers for a different database is never applied. Concurrent runs against one database are serialized on PostgreSQL
  (advisory lock) and MySQL (lock table); SQLite and MSSQL leave that to the
  caller. A MySQL configuration without `ParseTime` is refused with
  `ErrMySQLParseTime`, a plugin driver with `ErrUnsupportedDriver`. See
  `docs/MIGRATIONS.md`. (#72)

### Changed

- The README and the specification no longer list schema migrations among what
  Vessel does not do: `migrate/v1` runs migration files the caller writes. Schema
  generated from models remains out of scope.
- `golang.org/x/crypto` and `golang.org/x/text` are raised to the minimum
  versions goose requires.

## [0.2.1] - 2026-09-13

### Added

- `ExecReturning(ctx)` on `InsertBuilder`, `UpdateBuilder`, and
  `DeleteBuilder` executes a mutation configured with `Returning` as a single
  statement and returns the rows as `*RowsAdapter` (PostgreSQL `RETURNING`,
  MSSQL `OUTPUT`). It covers upserts and honors `WithTx`, removing the
  insert-then-select race of fetching by key. MySQL and SQLite return an
  explicit unsupported error without executing the statement. `Exec` still
  rejects `Returning`. Errors reported while reading the returned rows, such
  as a duplicate key, map to `dberror` sentinels like the rest of the driver
  API. (#26)
- `ReturningExecutor`, an optional extension of `DBActions` implemented by the
  built-in drivers and by `DBManager`. Custom `DBActions` implementations opt in
  by adding `ExecReturning(ctx, query, args...)`; the builders return a clear
  error when it is missing. Existing interfaces are unchanged.
- `DBManager.ExecReturning` and `DBManager.ExecReturningAsync` run a
  row-returning mutation on a read-write entry (never a read-only replica) and
  return `*db.RowsAdapter`, via the new `ReqExecReturning` request type. These
  requests are never coalesced by insert batching. If a synchronous
  `ExecReturning` caller cancels or times out, rows that arrive afterwards are
  closed so their connection returns to the pool.
- `DBManager.WithTransaction(ctx, entryName, fn, opts...)` runs a callback
  inside a transaction on a named `readwrite` entry. Every statement issued
  through the callback's `db.Tx`, including `SELECT ... FOR UPDATE`, runs on
  the transaction's connection and bypasses worker queues and read/write
  routing, so multi-statement atomicity and row-lock serialisation are
  available to DBManager users. New sentinel errors `ErrEntryNotFound`,
  `ErrEntryReadOnly`, and `ErrEntryUnhealthy` report entries that cannot host
  a transaction.
- `RawExpr` (`db/v1.RawExpr`, an alias of `pkg/query/condition.RawExpr`)
  marks a trusted SQL expression such as `NOW()` or `counter + 1` in a value
  position. It renders inline instead of binding a parameter in `Set`,
  `SetMap`, `Values`, `ValuesBulk`, `DoUpdateSet`, and `condition.Expr`
  values, with placeholder numbering kept contiguous around it. Same trust
  contract as `ColumnRaw`: trusted or allowlisted SQL only, never user input.
  Empty raw expressions return an error, and `InsertAndFetch` rejects a
  `RawExpr` key value because it cannot fetch the row back by it (#29).
- Upsert conflict predicates: `InsertBuilder.TargetWhere(cond)` renders
  `ON CONFLICT (cols) WHERE <cond>` so upserts can target partial unique
  indexes (the soft-delete uniqueness pattern), and
  `InsertBuilder.UpdateWhere(cond)` renders `DO UPDATE SET ... WHERE <cond>`
  for conditional updates such as expired-claim takeover. Both map to the new
  `options.UpsertOptions.TargetWhere` and `UpdateWhere` fields, work for
  single and bulk upserts on PostgreSQL and SQLite, and return an explicit
  error on MySQL (`ON DUPLICATE KEY UPDATE` supports neither) and when
  `UpdateWhere` is combined with `DoNothing`. On SQLite, a `TargetWhere`
  that binds values returns an error, because SQLite can't match a bound
  value against a partial index predicate. Upserts without predicates render
  unchanged SQL.

### Fixed

- PostgreSQL `UpsertQuery`/`UpsertsQuery` with `Returning` rendered
  `RETURNING` before `ON CONFLICT`, which is invalid SQL. `RETURNING` is now
  rendered after the conflict clause.
- `DBManager.GetRaw`, `GetByIDRaw`, and `QueryRaw` now close rows that arrive
  after a synchronous caller cancels or times out, so their connection returns
  to the pool. Previously each such call left a connection checked out.

## [0.2.0] - 2026-07-08

### Removed

- Removed the scratch working documents `CODE_QUALITY_IMPROVEMENTS.md` and
  `INTEGRATION_TESTS_SETUP.md` from the repository root. Integration test
  setup is covered by `CONTRIBUTING.md` and
  `docs/ENVIRONMENT_VARIABLES.md`.
- Removed the stale developer document `RELEASES.md` (a parallel release
  record that stopped at 0.1.1; release contents live in `CHANGELOG.md`,
  the process in `CONTRIBUTING.md` and the release workflow).
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
- Rewrote `Agents.md` from scratch: accurate project structure and test
  commands (including the `test` build-tag gotcha), current v0.2.0 API
  facts, task recipes, the release process, and a pitfalls section
  distilled from the bugs fixed in this release. The previous version was
  last updated 2026-03-15 and had drifted from the code.
- Corrected stale documentation: `NewFluentDB` doc examples showed a
  removed two-argument form, `GetByID`/`Query` doc comments described
  wrong return types, `CONTRIBUTING.md` now notes that unit tests require
  the `test` build tag, and the manager guide documents the fail-fast
  config validation.
- Hoisted per-call regular-expression compilation out of hot paths: the
  projection alias pattern in the query builder was compiled once per
  column per query, and the table-name pattern in `SanitizeTableName`
  once per logged query. Both are now compiled once at package init.
- **Breaking:** `DBManager.QueryWithRetry` callbacks now receive the
  1-indexed attempt number: the parameter type changed from
  `func(context.Context) ([]map[string]any, error)` to
  `RetryableQueryFunc`. Existing callbacks compile after adding an
  ignored `_ int` parameter.
- `NewFluentDB` now declares its parameter as the exported `DBActions`
  interface (the query-capable subset shared by `DB` and `Tx`) instead of
  an equivalent anonymous interface literal, removing a type assertion
  that could never fail and giving the public API a documented named
  type. Call sites are unaffected.
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
