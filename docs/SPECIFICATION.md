# Fabric - Go SQL Builder and Multi-Database Abstraction Specification

## Executive Summary

### Problem Statement

Go services often need dynamic SQL and operational database behavior without
committing to a full ORM. Common tradeoffs are:

- **Raw SQL Strings**: Flexible, but repetitive and easy to assemble
  incorrectly.
- **Driver Lock-in**: Some dialect details leak into application data access
  code.
- **Dialect Differences**: MySQL, PostgreSQL, SQLite, and MSSQL have subtly
  different SQL syntax
- **Scanning Boilerplate**: `database/sql` requires repeated error handling and
  type conversion code.
- **Operational Wrappers**: Tracing, retries, health checks, pool stats, and
  transaction helpers are often built separately in each service.

### Target Users

- Go services that need dynamic query construction and explicit SQL behavior.
- Projects that need portable core CRUD/query flows across MySQL, PostgreSQL,
  SQLite, and MSSQL.
- Teams that want tracing, retries, pool stats, transaction helpers, and
  optional manager routing close to the data layer.
- Applications that do not want ORM-managed relationships or generated query
  methods as the primary abstraction.

### Competitive Analysis

#### Adjacent Tools

| Tool         | Strongest fit                                       |
| ------------ | --------------------------------------------------- |
| **sqlc**     | Compile-time validation for hand-written SQL        |
| **sqlx**     | Thin scanning wrapper over explicit SQL             |
| **pgx**      | PostgreSQL-first access with a strong native driver |
| **Squirrel** | SQL composition without an execution layer          |
| **GORM**     | Full ORM workflows and application CRUD speed       |
| **Ent**      | Schema-first graph/entity modeling                  |

#### Fabric Fit

Fabric's fit is narrower: dynamic SQL builders, typed scanning, dialect-aware
core operations, transaction helpers, observability, retry utilities, and an
optional operational manager in one package.

Fabric is a poor fit when the main requirement is compile-time SQL validation,
ORM-managed relationships, migration ownership, or database-specific SQL as the
primary API.

### Success Criteria

- Values are parameterized by default.
- Identifiers are quoted per dialect by builder-owned paths.
- Raw SQL escape hatches are explicit and caller-owned.
- The same public API covers core flows for MySQL, PostgreSQL, SQLite, and
  MSSQL.
- Unsupported dialect features return clear errors.
- OpenTelemetry tracing, retry helpers, transaction helpers, and pool stats are
  available without extra service-local wrappers.
- Unit and integration tests cover supported dialect behavior.

---

## Functional Requirements

### FR1: Multi-Database SQL Abstraction

**Requirement**: Fabric shall support MySQL 5.7+, PostgreSQL 9.6+, SQLite 3.x,
and MSSQL 2016+ for core query and execution flows through one API.

**Scope**:

- Query builders (SELECT, INSERT, UPDATE, DELETE) generate correct SQL per dialect
- Connection pooling per database type
- Query execution and result scanning
- Transaction support with rollback semantics
- Type-safe parameter binding (no string concat)

**Out of Scope**:

- Schema migrations (distinct tool)
- ORM-style relationships (use raw query results or service layer)
- Cache layer (app's responsibility)

**Acceptance Criteria**:

- Identical Go code produces correct SQL for supported cross-dialect features
- Same test suite passes on all 4 databases
- Database-specific features are either rendered intentionally or rejected with
  explicit unsupported errors

### FR2: Fluent Query Builders with Method Chaining

**Requirement**: Users shall construct queries using a DSL
with readable method chaining.

**Examples**:

```go
// SELECT (context passed at query time)
v1.NewFluentDB(db).
    Select("users", "id", "name", "email").
    Where(cdt.NewExpr().Column("status").Op("=").Value("active")).
    OrderBy("created_at DESC").
    Limit(10).
    Get(ctx)

// INSERT
v1.NewFluentDB(db).
    Insert().
    Into("users").
    Set("name", "Alice").
    Set("age", 30).
    Exec(ctx)

// UPDATE
v1.NewFluentDB(db).
    Update("users").
    Set("last_login", time.Now()).
    Where(cdt.NewExpr().Column("id").Op("=").Value(userID)).
    Exec(ctx)

// DELETE
v1.NewFluentDB(db).
    Delete().
    From("users").
    Where(cdt.NewExpr().Column("id").Op("=").Value(userID)).
    Exec(ctx)
```

**Acceptance Criteria**:

- Method chaining syntax is intuitive (no parentheses hell)
- Each builder (Select, Insert, Update, Delete) is usable independently
- Builders are composable (reuse WHERE clause, options)

### FR3: Type-Safe Parameter Binding

**Requirement**: All values passed through the condition and mutation APIs shall
be parameterized.

**Mechanism**:

- Parameters never appear in raw SQL strings
- All user values passed to `Value()` method become placeholders (`?`, `$1`, etc.)
- Database driver handles sanitization

```go
// ✅ SAFE: Parameterized
cdt.NewExpr().Column("email").Op("=").Value(userEmail)  // → email = $1

// Raw SQL helpers exist for trusted SQL fragments only.
```

**Acceptance Criteria**:

- 0% SQL injection vulnerability risk
- All existing SQLi test vectors pass
- Integration tests include SQLi attempted on all 4 databases

### FR4: Logger Abstraction with Pluggable Adapters

**Requirement**: Fabric shall support popular Go loggers (slog, logrus, zap, apex)
without forcing a choice.

**Adapter Pattern**:

- Define minimal `Logger` interface (5 methods)
- Provide adapters for each logger library
- Users can wrap any logger via `NewSlogAdapter()`, `NewLogrusAdapter()`, etc.

**Logger Interface**:

```go
type Logger interface {
    Debug(msg string, keyvals ...any)
    Info(msg string, keyvals ...any)
    Warn(msg string, keyvals ...any)
    Error(msg string, keyvals ...any)
    With(key string, value any) Logger  // Contextual logging
}
```

**Adapters**:

- `NewSlogAdapter(*slog.Logger) Logger`
- `NewLogrusAdapter(*logrus.Logger) Logger`
- `NewZapAdapter(*zap.Logger) Logger`
- `NewApexAdapter(*log.Logger) Logger`

**Acceptance Criteria**:

- Developers can swap loggers at init time
- No Fabric code locks to one logger
- Context chaining via `With()` works across all adapters
- Each adapter tested independently (9-10 tests per adapter)

### FR5: OpenTelemetry Integration

**Requirement**: All database queries shall emit OpenTelemetry spans
for distributed tracing.

**Span Attributes**:

- `db.system`: MySQL, PostgreSQL, SQLite, MSSQL
- `db.statement`: Actual SQL executed
- `db.rows_affected`: Count of rows inserted/updated/deleted
- `span.kind`: CLIENT
- `duration`: Query execution time

**Acceptance Criteria**:

- Every `Execute()` call emits a span
- Spans are valid per OpenTelemetry spec
- Span attributes include SQL, row counts, duration
- Tests validate span emission

### FR6: Transaction Support with ACID Guarantees

**Requirement**: Users shall be able to group multiple queries in atomic
transactions with explicit rollback.

**API**:

```go
db.WithTransaction(ctx, func(tx Tx) error {
    // All queries use tx, not db
    _, err := tx.Insert(ctx, "accounts", map[string]any{...})
    if err != nil {
        return err  // Automatic rollback
    }
    _, err = tx.Update(ctx, "accounts", ...)
    if err != nil {
        return err  // Automatic rollback
    }
    return nil  // Automatic commit
})
```

**Acceptance Criteria**:

- Transactions properly commit on success
- Automatic rollback on error within callback
- Nested transactions not supported (panic with clear message)
- Tests verify ACID semantics via real databases

### FR7: Connection Pooling and Lifecycle

**Requirement**: Database connections shall be pooled and managed efficiently.

**Features**:

- Auto-managed per driver (pgxpool for PostgreSQL, custom for MySQL/SQLite/MSSQL)
- `Ping()` for health checks
- `PoolStats()` for observability (open, idle connections)
- `close()` (private) for graceful shutdown - called automatically on cleanup

**Acceptance Criteria**:

- Connections reused across queries
- Pool size configurable per driver setup
- Health checks work
- Graceful shutdown drains pool
- Resources properly cleaned up via `defer` patterns

### FR8: Plugin Registry for Custom Drivers

**Requirement**: Power users shall be able to register custom SQL dialects
without forking Fabric.

**Mechanism**:

- Global plugin registry
- Factory function accepting config, returning driver
- NewDB() checks registry before using built-in drivers

```go
type DriverFactory interface {
    Name() string
    Create(ctx context.Context, cfg any) (any, error)
}

// User code
fabric.RegisterDriver(CustomSQLDriver{})
db, _ := v1.NewDB(myCustomConfig, logger)  // Uses custom driver
```

**Acceptance Criteria**:

- Custom driver can be registered at init time
- Registry checked before built-in drivers
- No code changes to Fabric core needed

### FR9: Error Handling and Type Safety

**Requirement**: Fabric shall provide clear, actionable error types.

**Error Types**:

```go
type DBError struct {
    Op  string  // "select", "insert", "update", "delete"
    Err error   // Wrapped error from driver
}

// Specific errors
ErrNoRows       // No rows returned
ErrInvalidQuery // SQL generation failed
ErrConnFailed   // Failed to connect to database
```

**Acceptance Criteria**:

- All errors wrapped with op context
- Integration tests verify error handling on all databases
- No bare errors returned

### FR10: Row Scanning and Data Mapping

**Requirement**: Query results shall be scanned into Go types with zero boilerplate.

**Row Interface**:

```go
type Row interface {
    Scan(dest ...any) error
    Columns() ([]string, error)
}

// Usage
rows, _ := db.Query(ctx, "SELECT id, name, age FROM users")
for _, row := range rows {
    var id int
    var name string
    var age int
    row.Scan(&id, &name, &age)
}
```

**Acceptance Criteria**:

- Rows scannable via `row.Scan()`
- Column names accessible via `row.Columns()`
- No runtime panics on type mismatch

---

## Non-Functional Requirements

### NFR1: Performance

**Requirement**: Query execution shall have minimal overhead.

**Targets**:

- Small query (<10 rows): <10ms
- Medium query (100-1000 rows): <50ms
- Large query (10K+ rows): Scales linearly with row count
- Builder construction: <1ms per query
- No memory leaks over 1M queries

**Acceptance Criteria**:

- Benchmark suite measures overhead vs raw database/sql
- Query performance regressions caught in CI

### NFR2: Scalability

**Requirement**: Fabric shall scale horizontally with multiple service instances.

**Assumptions**:

- Stateless query building (no global state)
- Connection pooling per instance
- User responsible for database scaling (sharding, replication)

**Acceptance Criteria**:

- Multiple instances can safely share a database
- No race conditions on concurrent queries
- Thread-safe connection pooling

### NFR3: Maintainability

**Requirement**: Code shall be clean, modular, and easy to understand.

**Standards**:

- <800 lines per file
- <50 lines per function
- Layered architecture (public API → builder → dialect → driver)
- High cohesion, low coupling

**Acceptance Criteria**:

- Code passes gofumpt, golangci-lint
- No circular dependencies
- Clear separation between public and private APIs

### NFR4: Security

**Requirement**: No SQL injection vulnerabilities; secrets never logged.

**Standards**:

- All parameters parameterized
- No secrets in logs or error messages
- Least privilege for database credentials
- Input validation at boundaries

**Acceptance Criteria**:

- SQLi test suite passes on all dialects
- Secrets (passwords) never logged
- Audit trail via OpenTelemetry spans

### NFR5: Reliability

**Requirement**: Database operations shall recover gracefully from transient failures.

**Features**:

- Explicit error handling (no silent failures)
- Configurable retry not built-in (app's responsibility)
- Health checks via `Ping()`
- Graceful shutdown

**Acceptance Criteria**:

- All errors returned (never swallowed)
- Health checks detect connection issues
- Integration tests validate error scenarios

### NFR6: Observability

**Requirement**: Operators shall have visibility into database activity via
distributed tracing.

**Integration Points**:

- OpenTelemetry spans for all queries
- Logger integration for stack traces
- Connection pool stats via `PoolStats()`

**Acceptance Criteria**:

- Every query emits a span
- Spans include SQL, duration, row count
- Logs don't contain sensitive data

### NFR7: Testability

**Requirement**: Fabric shall be fully testable with both mocks and real databases.

**Mechanisms**:

- All public types are interfaces (mockable)
- Auto-generated mocks via mockgen
- Integration tests on real 4 databases
- Table-driven tests for builder logic

**Acceptance Criteria**:

- Unit tests use mocks (no DB needed)
- Integration tests use Docker or SQLite
- Coverage ≥80% on all code
- Build tag `test` isolates test code

---

## Architecture Overview

### Layered Architecture Design

```text
┌────────────────────────────────────────────────┐
│  Application Code                              │
│  (User implements business logic)              │
└──────────────┬─────────────────────────────────┘
               │
┌──────────────▼─────────────────────────────────┐
│  PUBLIC API LAYER (db/v1/)                     │
│  • DB interface (Get, Insert, Update, Delete)  │
│  • Tx interface (transactions)                 │
│  • FluentDB builders (Select, Insert, etc.)    │
│  • Logger adapters                             │
│  • Error types                                 │
└──────────────┬─────────────────────────────────┘
               │
┌──────────────▼─────────────────────────────────┐
│  BUILDER LAYER (internal/pkg/builder/)         │
│  • QueryBuilder interface                      │
│  • MySQLBuilder implementation                 │
│  • PostgresBuilder implementation              │
│  • SQLiteBuilder implementation                │
│  • MSSQLBuilder implementation                 │
│  • Generates SQL per dialect                   │
└──────────────┬─────────────────────────────────┘
               │
┌──────────────▼─────────────────────────────────┐
│  DIALECT LAYER (internal/pkg/sqldialect/)      │
│  • Dialect interface (rendering, operators)    │
│  • MySQLDialect (backticks, InnoDB)            │
│  • PostgresDialect (double quotes, RETURNING)  │
│  • SQLiteDialect (simple quoting)              │
│  • MSSQLDialect (square brackets)              │
└──────────────┬─────────────────────────────────┘
               │
┌──────────────▼─────────────────────────────────┐
│  DRIVER LAYER (database/sql + vendors)         │
│  • go-sql-driver/mysql                         │
│  • jackc/pgx (with pgxpool)                    │
│  • modernc.org/sqlite                          │
│  • denisenkom/go-mssqldb                       │
└────────────────────────────────────────────────┘
```

**Design Rationale**:

- Each layer has one responsibility
- Changes to SQL generation don't affect public API
- New dialects added without touching builders
- Builders isolated at bottom (easy to swap)
- Tests mock at each layer boundary

### Core Components

#### 1. Public API (db/v1/)

**Core Interfaces**:

```go
// DB is the main entry point
type DB interface {
    reader
    writer
    upserter
    introspector
    transactional
    healthCheck
    closer
}

type TransactionOptions struct {
    Isolation sql.IsolationLevel
    ReadOnly  bool
}

type transactional interface {
    Begin(ctx context.Context, opts ...TransactionOptions) (Tx, error)
    WithTransaction(ctx context.Context, fn func(Tx) error, opts ...TransactionOptions) error
}

type Tx interface {
    reader
    writer
    upserter
    introspector
    savepointer
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}

type savepointer interface {
    Savepoint(ctx context.Context, name string) error
    RollbackToSavepoint(ctx context.Context, name string) error
    ReleaseSavepoint(ctx context.Context, name string) error
}
```

Primary method groups include:

```go
type reader interface {
    Get(
        ctx context.Context,
        table string,
        columns []string,
        joins []condition.Join,
        conditions condition.Condition,
        opts *options.QueryOptions,
    ) ([]map[string]any, error)
    GetRaw(
        ctx context.Context,
        table string,
        columns []string,
        joins []condition.Join,
        conditions condition.Condition,
        opts *options.QueryOptions,
    ) (*RowsAdapter, error)
    Query(ctx context.Context, query string, args ...any) ([]map[string]any, error)
    QueryRaw(ctx context.Context, query string, args ...any) (*RowsAdapter, error)
}

type upserter interface {
    Upsert(
        ctx context.Context,
        table string,
        data map[string]any,
        upsertOpts *options.UpsertOptions,
        opts *options.QueryOptions,
    ) (*ExecResult, error)
    UpsertQuery(
        table string,
        data map[string]any,
        upsertOpts *options.UpsertOptions,
        opts *options.QueryOptions,
    ) (string, []any, error)
}
```

Additional lifecycle methods:

```go
type healthCheck interface {
    Ping(ctx context.Context) error
    PoolStats() (*PoolStatistics, error)
}

type closer interface {
    Close() error
}
```

```go
// Logger is the abstraction
type Logger interface {
    Debug(msg string, keyvals ...any)
    Info(msg string, keyvals ...any)
    Warn(msg string, keyvals ...any)
    Error(msg string, keyvals ...any)
    With(key string, value any) Logger
}

// Row represents a single query result
type Row interface {
    Scan(dest ...any) error
    Columns() ([]string, error)
}

// Result wraps execution outcome
type Result interface {
    RowsAffected() (int64, error)
}
```

**Factory Functions**:

```go
// NewDB creates a DB instance
func NewDB(cfg DBConfig, logger Logger) (DB, error)

// Config types
type MySQLConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    DBName   string
}

type PostgresConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    DBName   string
}

type SQLiteConfig struct {
    Path string
}

type MSSQLConfig struct {
    Server   string
    Port     int
    User     string
    Password string
    Database string
}
```

#### 2. Query Builders (db/v1/fluentDB.go)

**SelectBuilder**:

```go
type SelectBuilder interface {
    // Column selection
    Select(table string, columns ...string) SelectBuilder

    // Filtering
    Where(conditions ...Condition) SelectBuilder

    // Ordering
    OrderBy(orderBy ...string) SelectBuilder

    // Pagination
    Limit(limit int) SelectBuilder
    Offset(offset int) SelectBuilder

    // Grouping
    GroupBy(columns ...string) SelectBuilder
    Having(conditions ...Condition) SelectBuilder

    // Execution
    Execute(ctx context.Context) ([]Row, error)
}

type InsertBuilder interface {
    Insert(table string, data map[string]any) InsertBuilder
    Execute(ctx context.Context) (Row, error)
}

type UpdateBuilder interface {
    Update(table string) UpdateBuilder
    Set(updates map[string]any) UpdateBuilder
    Where(conditions ...Condition) UpdateBuilder
    Execute(ctx context.Context) error
}

type DeleteBuilder interface {
    Delete(table string) DeleteBuilder
    Where(conditions ...Condition) DeleteBuilder
    Execute(ctx context.Context) error
}
```

#### 3. Condition DSL (pkg/query/condition/)

**Purpose**: Composable WHERE clause construction

```go
type Condition interface {
    // Combines conditions
}

// Usage examples
cdt.NewExpr().Column("age").Op(">").Value(18)
cdt.NewExpr().Column("status").Op("=").Value("active")

cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("age").Op(">").Value(18),
    cdt.NewExpr().Column("status").Op("=").Value("active"),
)

cdt.NewOr().Conditions(
    cdt.NewExpr().Column("role").Op("=").Value("admin"),
    cdt.NewExpr().Column("role").Op("=").Value("moderator"),
)

cdt.NewExpr().Column("name").Op("IN").Value([]string{"alice", "bob"})

cdt.NewExpr().Column("created_at").Op("BETWEEN").Value(start).Value(end)
```

**Supported Operators**:

- `=`, `!=`, `<>`, `<`, `>`, `<=`, `>=`
- `LIKE`, `NOT LIKE`
- `IN`, `NOT IN`
- `BETWEEN`, `NOT BETWEEN`
- `IS NULL`, `IS NOT NULL`
- Database-specific: `GLOB` (SQLite), `~` (PostgreSQL regex), etc.

#### 4. Query Options (pkg/query/options/)

**Purpose**: Pagination, ordering, grouping

```go
type Options interface {
    OrderBy(clauses ...string) Options
    Limit(limit int) Options
    Offset(offset int) Options
    GroupBy(columns ...string) Options
    Having(conditions ...Condition) Options
}
```

#### 5. Builder Layer (internal/pkg/builder/)

**Interface**:

```go
type QueryBuilder interface {
    BuildSelect(table string, columns []string, conditions []Condition, opts Options)
     (string, []any, error)
    BuildInsert(table string, data map[string]any) (string, []any, error)
    BuildUpdate(table string, updates map[string]any, conditions []Condition)
     (string, []any, error)
    BuildDelete(table string, conditions []Condition) (string, []any, error)
}
```

**Implementations**: Separate MySQL, PostgreSQL, SQLite, MSSQL builders

#### 6. Dialect Layer (internal/pkg/sqldialect/)

**Interface**:

```go
type Dialect interface {
    QuoteIdentifier(name string) string
    RenderOperator(op string) string
    SupportsFeature(feature string) bool
}
```

**Implementations**:

- MySQL: backticks for identifiers
- PostgreSQL: double quotes, supports RETURNING
- SQLite: no quoting needed
- MSSQL: square brackets

---

## Design Patterns Used

### 1. Adapter Pattern (Logger Adapters)

**Problem**: Applications use different loggers (slog, logrus, zap, apex)
**Solution**: Define minimal `Logger` interface, provide adapters
**Files**: `db/v1/logger_adapters.go`
**Benefits**: Users can swap loggers without Fabric changes

### 2. Builder Pattern (Query Construction)

**Problem**: Complex SQL construction via positional args
**Solution**: Fluent builders with method chaining
**Files**: `db/v1/fluentDB.go`
**Benefits**: Readable, composable, type-safe

### 3. Layer Pattern (Architecture)

**Problem**: SQL dialects mixed with query logic mixed with drivers
**Solution**: Separate layers (public → builder → dialect → driver)
**Benefits**: Independent testing, easy to extend

### 4. Plugin Pattern (Custom Drivers)

**Problem**: Users might need custom databases
**Solution**: Registry-based driver discovery
**Files**: `internal/pkg/plugin/registry.go`
**Benefits**: Extensibility without forks

### 5. Mock Pattern (Testing)

**Problem**: Tests shouldn't depend on real databases
**Solution**: All public types are interfaces, mockgen for auto-generation
**Files**: `db/v1/db_mocks.go` (auto-generated)
**Benefits**: Fast unit tests, isolation

### 6. Factory Pattern (Configuration)

**Problem**: Different databases need different config
**Solution**: `NewDB(config, logger)` with database-specific configs
**Files**: `db/v1/mysql.go`, `postgres.go`, `sqlite.go`, `mssql.go`
**Benefits**: Clear separation, validation at init time

---

## Database Support Details

### MySQL (MySQL 5.7+)

**Connection String Format**:

```ini
user:password@tcp(host:port)/dbname?config=options
```

**Dialect Features**:

- Identifier quoting: backticks
- Parameter placeholder: `?`
- RETURNING clause: not supported (no last_insert_id tracking)
- BigQuery: not supported

**Configuration**:

```go
cfg := v1.MySQLConfig{
    Host:     "localhost",
    Port:     3306,
    User:     "root",
    Password: "secret",
    DBName:   "myapp",
}
db, _ := v1.NewDB(cfg, logger)
```

**Test Database**: MySQL 5.7 in Docker

### PostgreSQL (PostgreSQL 9.6+)

**Connection String Format**:

```ini
postgres://user:password@host:port/dbname?sslmode=disable
```

**Dialect Features**:

- Identifier quoting: double quotes
- Parameter placeholder: `$1`, `$2`, ...
- RETURNING clause: fully supported
- ArrayType: handled via parameterization
- JSON/JSONB: raw string values

**Configuration**:

```go
cfg := v1.PostgresConfig{
    Host:     "localhost",
    Port:     5432,
    User:     "postgres",
    Password: "secret",
    DBName:   "myapp",
}
db, _ := v1.NewDB(cfg, logger)  // Uses pgxpool
```

**Test Database**: PostgreSQL 14 in Docker
**Driver**: jackc/pgx with pgxpool

### SQLite (SQLite 3.x)

**Connection String Format**:

```ini
/path/to/file.db    (file mode)
:memory:            (in-memory)
```

**Dialect Features**:

- Identifier quoting: not needed (single identifier)
- Parameter placeholder: `?`
- RETURNING clause: not supported
- Concurrency: limited (single writer)

**Configuration**:

```go
cfg := v1.SQLiteConfig{
    Path: "/tmp/myapp.db",
}
db, _ := v1.NewDB(cfg, logger)
```

**Test Database**: SQLite in-memory (no Docker)
**Driver**: modernc.org/sqlite

### MSSQL (SQL Server 2016+)

**Connection String Format**:

```ini
server=host;user id=user;password=secret;database=dbname
```

**Dialect Features**:

- Identifier quoting: square brackets
- Parameter placeholder: `@p1`, `@p2`, ...
- RETURNING clause: not supported (use OUTPUT)
- IDENTITY: handled via parameterization

**Configuration**:

```go
cfg := v1.MSSQLConfig{
    Server:   "localhost",
    Port:     1433,
    User:     "sa",
    Password: "secret",
    Database: "myapp",
}
db, _ := v1.NewDB(cfg, logger)
```

**Test Database**: SQL Server 2019 in Docker
**Driver**: denisenkom/go-mssqldb

---

## Test Strategy

### Test Organization

**Unit Tests**: Verify behavior in isolation (no database)

- Location: `*_test.go` files alongside code
- Scope: Builders, dialects, adapters, error handling
- Tool: testify asserts, table-driven tests
- Mocks: mockgen-generated mocks for interfaces

**Integration Tests**: Real databases

- Location: `tests/integration_test.go`
- Scope: Full query lifecycle (SELECT, INSERT, UPDATE, DELETE, transactions)
- Databases: SQLite (fast), MySQL, PostgreSQL, MSSQL (Docker)
- Parallelization: Run 4 databases in parallel

**Test Build Tag**: `//go:build test` isolates test code

### Coverage Targets

| Component           | Target | Rationale                              |
| ------------------- | ------ | -------------------------------------- |
| Public API (db/v1/) | 100%   | Users depend on correctness            |
| Builders            | 90%    | SQL generation is critical             |
| Dialects            | 85%    | Database-specific edge cases           |
| Adapters            | 90%    | Logger integration must work           |
| Operators           | 85%    | Operator rendering is dialect-specific |
| Helpers             | 80%    | Lower risk utilities                   |

**Minimum Overall**: 80%

### Test Patterns

#### Pattern 1: Table-Driven Builder Tests

```go
func TestMySQLBuilder_Select(t *testing.T) {
    testCases := []struct {
        name       string
        table      string
        columns    []string
        expected   string
        wantErr    bool
    }{
        {
            name:       "select all columns",
            table:      "users",
            columns:    []string{},
            expected:   "SELECT * FROM `users`",
            wantErr:    false,
        },
        {
            name:       "select specific columns",
            table:      "users",
            columns:    []string{"id", "name"},
            expected:   "SELECT `id`, `name` FROM `users`",
            wantErr:    false,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            builder := mysql_builder.NewMySQLBuilder()
            sql, _, err := builder.BuildSelect(tc.table, tc.columns, nil, nil)

            if tc.wantErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tc.expected, sql)
        })
    }
}
```

#### Pattern 2: Adapter Tests with Real Outputs

```go
func TestSlogAdapter_StructuredLogging(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, nil)
    logger := slog.New(handler)
    adapter := v1.NewSlogAdapter(logger)

    adapter.Info("user created", "user_id", 123, "email", "alice@example.com")

    var logEntry map[string]interface{}
    err := json.Unmarshal(buf.Bytes(), &logEntry)
    require.NoError(t, err)
    assert.Equal(t, "user created", logEntry["msg"])
    assert.Equal(t, float64(123), logEntry["user_id"])
    assert.Equal(t, "alice@example.com", logEntry["email"])
}
```

#### Pattern 3: Integration Tests Across Dialects

```go
func TestIntegration_InsertSelect(t *testing.T) {
    for _, dbName := range []string{"sqlite", "mysql", "postgres", "mssql"} {
        t.Run(dbName, func(t *testing.T) {
            db, cleanup := setupTestDB(t, dbName)
            defer cleanup()

            // INSERT
            row, err := v1.NewFluentDB(db, context.Background()).
                Insert("users", map[string]any{"name": "Bob", "age": 25}).
                Execute()
            require.NoError(t, err)

            // SELECT
            rows, err := v1.NewFluentDB(db, context.Background()).
                Select("users", "name", "age").
                Where(cdt.NewExpr().Column("id").Op("=").Value(userID)).
                Execute()
            require.NoError(t, err)
            assert.Len(t, rows, 1)
        })
    }
}
```

### Continuous Integration

**CI Pipeline**:

1. Format check (gofumpt)
2. Linting (golangci-lint)
3. Unit tests (fast, no Docker)
4. Integration tests (SQLite only, fast)
5. Integration tests (all databases, Docker)
6. Coverage report (must be ≥80%)
7. Security scanning (no hardcoded secrets)

---

## API Surface Design

### Public Interfaces

#### Entry Point

```go
db, err := v1.NewDB(dbConfig, logger)
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

#### Query Builders

```go
// All return errors (no silent failures)
fluentDB := v1.NewFluentDB(db)

// SELECT
rows, err := fluentDB.Select("users", "id", "name").
    Where(someCondition).
    OrderBy("name ASC").
    Limit(10).
    Get(ctx)

// INSERT
row, err := fluentDB.Insert("users", map[string]any{
    "name": "Alice",
    "age":  30,
}).Execute()

// UPDATE
err := fluentDB.Update("users").
    Set(map[string]any{"status": "active"}).
    Where(idCondition).
    Execute()

// DELETE
err := fluentDB.Delete("users").
    Where(idCondition).
    Execute()
```

#### Transactions

```go
err := db.WithTransaction(ctx, func(tx v1.Tx) error {
    // Both queries in same transaction
    row, err := tx.Insert(ctx, "accounts", ...)
    if err != nil {
        return err  // Auto-rollback
    }
    err = tx.Update(ctx, "transactions", ...)
    if err != nil {
        return err  // Auto-rollback
    }
    return nil  // Auto-commit
})
```

#### Conditions

```go
import "tounilab.com/fabric/pkg/query/condition"

cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("age").Op(">").Value(18),
    cdt.NewExpr().Column("status").Op("=").Value("active"),
)

cdt.NewOr().Conditions(
    cdt.NewExpr().Column("role").Op("=").Value("admin"),
    cdt.NewExpr().Column("role").Op("=").Value("moderator"),
)

cdt.NewExpr().Column("id").Op("IN").Value([]int{1, 2, 3})
```

### Private (Internal) APIs

**Not exported**:

- Builder implementations (`MySQLBuilder`, `PostgresBuilder`)
- Dialect implementations (`MySQLDialect`, `PostgresDialect`)
- Operator internals
- Plugin registry details

**Rationale**: Prevent users from depending on implementation details

---

## Performance & Security Considerations

### Performance

- **Query Building**: <1ms per query (no blocking)
- **Parameterization**: Zero overhead (native driver feature)
- **Connection Pooling**: Per-driver management (pgxpool for PostgreSQL)
- **Benchmarks**: Compare overhead vs raw database/sql

### Security

- **SQL Injection**: Values are parameterized by default; raw SQL helpers are
  caller-owned and must use trusted SQL
- **Secrets in Logs**: Never logged (password fields excluded)
- **Error Messages**: Don't leak schema details
- **Input Validation**: Delegated to database (drivers enforce constraints)

---

## Deployment & Observability

### OpenTelemetry Integration

- Every query emits a span
- Attributes: `db.system`, `db.statement`, `db.rows_affected`
- Fits standard distributed tracing (Jaeger, Datadog, etc.)

### Logging

- Structured logs via pluggable adapters
- Context propagation (`With()` method)
- No magic logging (user controls delegation)

### Health Checks

```go
err := db.Ping(ctx)  // Verify connectivity
stats := db.PoolStats()  // Monitor connections
```

---

## Acceptance & Rollout

### v1 Stable

- Core flows covered for MySQL, PostgreSQL, SQLite, and MSSQL
- Unit and integration coverage for supported dialect behavior
- Public API frozen (no breaking changes)
- Documentation complete

### Documentation Artifacts

- CLAUDE.md (this file)
- Architecture diagrams (ASCII in docs/)
- API godoc (in-code comments)
- Examples directory (working code samples)

### Success Metrics

- Stable public `db/v1` interfaces
- Layered architecture with dialect-specific rendering isolated internally
- Pluggable loggers, drivers, and dialects
- OpenTelemetry integration
- Parameterized values by default
- Explicit unsupported errors for dialect gaps
