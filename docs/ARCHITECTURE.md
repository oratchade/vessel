# Fabric Architecture

**Fabric** is a production-grade, multi-database SQL abstraction layer for Go.
This document provides the definitive architectural reference for understanding
the entire system design, enabling rapid onboarding and informed extension decisions.

**Version**: 1.0+ (Post-Phase 5)  
**Last Updated**: April 18, 2026  
**Maintainer**: oratchade  
**Status**: Production Ready (Grade A+, 802 tests, 100% pass rate)  
**Target Audience**: AI agents, new contributors, maintainers

## Recent API Changes (April 15-19, 2026)

- **Comprehensive Interface Encapsulation**: Made all 6 internal composition
  interfaces private (lowercase names): reader, writer, introspector, transactional,
  healthCheck, closer. This reduces public API surface from 9 types to 3 (DB, Tx,
  FluentDB) while maintaining full backward compatibility and improving implementation
  flexibility.
- **FluentDB Constructor Simplified**: Changed from three separate parameters
  `(reader Reader, writer Writer, introspector Introspector)`
  to single composed interface `(db interface {reader; writer; introspector})`.
  This reduces parameter passing complexity
  and improves API ergonomics while maintaining backward compatibility.
- **Error Wrapping Standardized**: All errors follow `function: operation: %w`
  pattern for consistency.
- **Builder Cleanup**: `Close()` method made private; resources auto-managed
  via defer patterns.

---

## Quick Navigation

### For Understanding the System

- [Executive Summary](#executive-summary)
- [Architectural Philosophy](#architectural-philosophy)
- [Layered Architecture](#layered-architecture)
- [System Data Flow](#system-data-flow)

### For Making Changes

- [Common Workflows](#common-workflows)
- [Decision Trees](#decision-trees-for-modifications)
- [Extension Patterns](#extension-architecture)

### For Deep Dives

- [Builder Implementation Details](#builder-implementation-deep-dive)
- [Dialect System Architecture](#dialect-system-deep-dive)
- [Connection Pooling Strategy](#connection-pooling-strategy-detailed)
- [Transaction Lifecycle](#transaction-handling-and-lifecycle)
- [Error Handling](#error-handling)

### For Troubleshooting

- [Common Issues](#troubleshooting-guide)
- [Performance Optimization](#performance-optimization-points)
- [Security Safeguards](#security-model-deep-dive)

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Architectural Philosophy](#architectural-philosophy)
3. [Layered Architecture](#layered-architecture)
4. [System Data Flow](#system-data-flow)
5. [Directory Structure & Rationale](#directory-structure--rationale)
6. [Core Components](#core-components)
7. [Builder Implementation Deep Dive](#builder-implementation-deep-dive)
8. [Dialect System Deep Dive](#dialect-system-deep-dive)
9. [Connection Pooling Strategy Detailed](#connection-pooling-strategy-detailed)
10. [Query Construction Flow](#query-construction-flow-with-examples)
11. [Transaction Handling & Lifecycle](#transaction-handling-and-lifecycle)
12. [Error Handling](#error-handling)
13. [Testing Architecture](#testing-architecture)
14. [Performance Optimization Points](#performance-optimization-points)
15. [Security Model Deep Dive](#security-model-deep-dive)
16. [OpenTelemetry Integration](#opentelemetry-integration)
17. [Design Patterns](#design-patterns)
18. [Common Workflows](#common-workflows)
19. [Decision Trees for Modifications](#decision-trees-for-modifications)
20. [Dependency Landscape](#dependency-landscape)
21. [Troubleshooting Guide](#troubleshooting-guide)
22. [Future Extensibility Roadmap](#future-extensibility-roadmap)
23. [API Surface Reference](#api-surface-reference)

---

## Executive Summary

### What is Fabric

Fabric is a **type-safe SQL query abstraction library** that unifies
MySQL, PostgreSQL, SQLite, and MSSQL behind a single, ergonomic Go API.
It eliminates the need for manual SQL string construction while
maintaining complete control over query generation.

Unlike traditional ORMs (Gorm, sqlc, Ent), Fabric occupies
the sweet spot between manual SQL writing and full ORM abstraction:

| Aspect               | Manual SQL        | Fabric         | Full ORM |
| -------------------- | ----------------- | -------------- | -------- |
| **Control**          | 100%              | 100%           | Limited  |
| **Type Safety**      | None              | 100%           | 100%     |
| **Flexibility**      | 100%              | 100%           | Limited  |
| **Boilerplate**      | High              | Low            | Very Low |
| **Performance**      | Excellent         | Excellent      | Good     |
| **Database Support** | Database-specific | Multi-database | Limited  |

### Why Fabric Exists

**Problem**: Go developers face a false choice:

- Write database code with raw SQL strings (error-prone, dialect-specific, repeatable)
- Use a heavy ORM (opinionated, limited flexibility, significant overhead)

**Solution**: Fabric provides:

- **Fluent Query Builders**: Method chaining for readable SQL construction
- **Zero SQL Injection**: All parameters automatically parameterized
- **Multi-Database**: Single codebase works across MySQL, PostgreSQL, SQLite, MSSQL
- **Minimal Runtime Overhead**: No reflection-based magic, compiled type-safety
- **Pluggable Everything**: Loggers, drivers, dialects—no lock-in

### Key Metrics

| Metric     | Value          |
| ---------- | -------------- |
| Test Suite | 802 tests pass |
| Code Grade | A+ (94/100)    |
| Coverage   | 85%+           |
| Linting    | 0 issues       |
| Security   | 0 advisories   |
| Status     | Production     |

---

## Architectural Philosophy

Fabric is built on **clean architecture principles** with these core values:

### 1. Separation of Concerns

Each layer has **one job**:

- **Public API layer**: User-facing interfaces, configuration
- **Builder layer**: SQL generation logic per dialect
- **Dialect layer**: Database-specific SQL rendering rules
- **Driver layer**: Database connection management

**Why this matters**: Changes to MySQL SQL generation don't require testing
PostgreSQL builders. New dialects can be added without touching builder logic.

### 2. Interface-First Design

All public types are **interfaces**, not concrete implementations:

```go
// Users depend on interfaces
type DB interface {
    Get(ctx context.Context, query string, args ...any) ([]Row, error)
    Insert(ctx context.Context, table string, data map[string]any) (Row, error)
    Begin(ctx context.Context) (Tx, error)
}

// Implementations are hidden
type pgDB struct { /* internal */ }
type mysqlDB struct { /* internal */ }
```

**Benefit**: Tests mock interfaces; multiple implementations can coexist.

### 3. Pluggable Extensibility

No feature is hard-coded:

- **Logger**: slog, logrus, zap, apex, or custom
- **Driver**: Add custom databases via plugin registry
- **Dialect**: Register custom SQL dialects

### 4. Type Safety Without Boilerplate

Builders are **fluent and type-safe**:

```go
// Type-safe, readable, zero risk of SQL injection
rows, err := v1.NewFluentDB(db).
    Select("users", "id", "name", "email").
    Where(cdt.NewExpr().Column("age").Op(">").Value(18)).
    OrderBy("name ASC").
    Limit(10).
    Get(ctx)
```

### 5. Minimal Runtime Overhead

No reflection-based magic. Queries are **compiled to SQL** at builder time,
not runtime. Performance is predictable and consistent.

---

## Layered Architecture

### Complete System Diagram

```text
┌────────────────────────────────────────────────────────────────┐
│  User Application Code                                         │
│  (depends on db/v1 interfaces + query/condition builders)      │
└────────────────────┬───────────────────────────────────────────┘
                     │
┌────────────────────▼──────────────────────────────────────────┐
│  PUBLIC API LAYER (db/v1/)                                    │
│  ───────────────────────────────────────────────────────────  │
│  Interfaces: DB, Tx, Logger, Row, FluentDB                    │
│  Configs: MySQLConfig, PostgresConfig, SQLiteConfig, etc      │
│  Adapters: SlogAdapter, LogrusAdapter, ZapAdapter, etc        │
│  Entry points: NewDB(), NewFluentDB() (context-free)          │
│                                                               │
│  • All types are interfaces (testable, mockable)              │
│  • Factories for each database configuration                  │
│  • Logger adapters for pluggable logging                      │
│  • Error types and handling utilities                         │
└────────────────────┬──────────────────────────────────────────┘
                     │
┌────────────────────▼──────────────────────────────────────────┐
│  QUERY BUILDER LAYER (pkg/query/ + internal/pkg/builder/)     │
│  ───────────────────────────────────────────────────────────  │
│  Query DSL: SelectBuilder, InsertBuilder, UpdateBuilder, etc  │
│  Conditions: Expr, And, Or, In, Between, IsNull               │
│  Options: OrderBy, GroupBy, Limit, Offset, Returning          │
│                                                               │
│  Per-Dialect Builders:                                        │
│  • MySQLBuilder      → SELECT ... WHERE ... (backticks)       │
│  • PostgresBuilder   → SELECT ... RETURNING ... (quotes)      │
│  • SQLiteBuilder     → SELECT ... LIMIT (no quoting)          │
│  • MSSQLBuilder      → SELECT TOP ... OFFSET...FETCH          │
│                                                               │
│  • Accumulates query components (non-mutating)                │
│  • Delegates SQL rendering to Dialect                         │
│  • Returns parameterized SQL + args                           │
└────────────────────┬──────────────────────────────────────────┘
                     │
┌────────────────────▼──────────────────────────────────────────┐
│  DIALECT LAYER (internal/pkg/sqldialect/)                     │
│  ───────────────────────────────────────────────────────────  │
│  Abstract Dialect Interface:                                  │
│  • QuoteIdentifier(name) → quoted identifier                  │
│  • RenderOperator(op) → dialect-specific operator             │
│  • SupportedOptions() → feature matrix                        │
│  • SupportsReturning() → boolean for RETURNING/OUTPUT         │
│                                                               │
│  Concrete Dialects:                                           │
│  • MySQLDialect      (backticks: `table`.`column`)            │
│  • PostgresDialect   (double quotes: "table"."column")        │
│  • SQLiteDialect     (no quoting, simplified)                 │
│  • MSSQLDialect      (square brackets: [table].[column])      │
│                                                               │
│  Operator Registry:                                           │
│  • =, !=, <>, <, >, <=, >=, IN, NOT IN                        │
│  • LIKE, NOT LIKE, BETWEEN, IS NULL, IS NOT NULL              │
│  • AND, OR, NOT                                               │
│  • Database-specific operators (e.g., JSONPath for Postgres)  │
└────────────────────┬──────────────────────────────────────────┘
                     │
┌────────────────────▼─────────────────────────────────────────┐
│  DRIVER & CONNECTION LAYER (database/sql + vendor drivers)   │
│  ──────────────────────────────────────────────────────────  │
│  • MySQL:      github.com/go-sql-driver/mysql                │
│  • PostgreSQL: github.com/jackc/pgx (with pgxpool)           │
│  • SQLite:     github.com/mattn/go-sqlite3 (CGO)             │
│  • MSSQL:      github.com/denisenkom/go-mssqldb              │
│                                                              │
│  Connection Pool (per driver):                               │
│  • TCP/Unix socket connections                               │
│  • Prepared statement caching                                │
│  • Configurable pool size and timeouts                       │
│  • Health checks and auto-reconnect                          │
└──────────────────────────────────────────────────────────────┘

                        ▲
                        │
                    Database Server
                  (MySQL, PG, SQLite, MSSQL)
```

---

## System Data Flow

### End-to-End Query Execution Flow

```text
┌──────────────────────────────────────────────────────────────────┐
│ (1) USER CODE: Build Query                                       │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│ builder := v1.NewFluentDB(db)              ← Create builder      │
│ result := builder.                                               │
│     Select("users", "id", "name").        ← Accumulate: table    │
│     Where(cdt.Expr()...Op(">")...Value()). ← Accumulate: clause  │
│     OrderBy("name ASC").                   ← Accumulate: order   │
│     Limit(10).                             ← Accumulate: limit   │
│     Get(ctx)                                ← Execute with ctx    │
│                                                                  │
└──────────────────────────────────────┬───────────────────────────┘
                                       │
┌──────────────────────────────────────▼───────────────────────────┐
│ (2) BUILDER LAYER: Accumulate Query Components                   │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│ SelectBuilder {                                                  │
│     table:      "users"                                          │
│     columns:    ["id", "name"]                                   │
│     conditions: [Expr{Column: "age", Op: ">", Value: 18}]        │
│     orderBy:    ["name ASC"]                                     │
│     limit:      10                                               │
│     offset:     0                                                │
│ }                                                                │
│                                                                  │
│ At Execute() time, call builder.Build()                          │
│                                                                  │
└──────────────────────────────────────┬───────────────────────────┘
                                       │
┌──────────────────────────────────────▼───────────────────────────┐
│ (3) SQL GENERATION: Builder → Dialect → SQL String               │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│ Builder.Build() calls:                                           │
│  ├ dialect.QuoteIdentifier("users") → "users" (no change)        │
│  ├ dialect.QuoteIdentifier("id") → "id"                          │
│  ├ dialect.QuoteIdentifier("name") → "name"                      │
│  └ Render conditions with dialect operators                      │
│                                                                  │
│ Result SQL string:                                               │
│ "SELECT id, name FROM users WHERE age > ? ORDER BY name ASC      │
│  LIMIT ?"                                                        │
│                                                                  │
│ Args collected:                                                  │
│ [18, 10]                                                         │
│                                                                  │
│ CRITICAL: Never string concatenation, always parameterized ✓     │
│                                                                  │
└──────────────────────────────────────┬───────────────────────────┘
                                       │
┌──────────────────────────────────────▼───────────────────────────┐
│ (4) EXECUTE: Send to Driver                                      │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│ db.Query(sqlString, args...)                                     │
│  └ database/sql.DB.Query(ctx, sqlString, 18, 10)                 │
│      └ Vendor Driver (pgx, mysql, sqlite3, mssqldb)              │
│          └ Network/IPC to database server                        │
│                                                                  │
└──────────────────────────────────────┬───────────────────────────┘
                                       │
┌──────────────────────────────────────▼───────────────────────────┐
│ (5) RESULT SCANNING: rows.Scan() → Row objects                   │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│ For each row from database:                                      │
│  1. Create Row object (key-value map)                            │
│  2. Scan column values into Row                                  │
│  3. Append to results slice                                      │
│                                                                  │
│ []Row{                                                           │
│     {id: "1", name: "Alice"},                                    │
│     {id: "2", name: "Bob"},                                      │
│     ...                                                          │
│ }                                                                │
│                                                                  │
└──────────────────────────────────────┬───────────────────────────┘
                                       │
┌──────────────────────────────────────▼───────────────────────────┐
│ (6) RETURN TO USER                                               │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│ rows, err := builder.Execute()                                   │
│                                                                  │
│ User can now iterate: for i, row := range rows { ... }           │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## Directory Structure & Rationale

### Complete Project Layout

```text
fabric/
│
├── .github/
│   ├── workflows/                    # CI/CD pipelines (testing, linting)
│   └── pull_request_template.md      # PR submission guidelines
│
├── .claude/
│   └── skills/                       # Claude Code skills for the project
│       └── iterative-retrieval/      # Custom skill for agent context
│
├── db/
│   └── v1/                           # PUBLIC API LAYER
│       ├── db.go                     # Core DB, Tx, Logger interfaces
│       ├── fluentDB.go               # Query builder implementations
│       ├── logger.go                 # Logger interface definition
│       ├── logger_adapters.go        # slog, logrus, zap, apex adapters
│       ├── mysql.go                  # MySQL config factory
│       ├── postgres.go               # PostgreSQL config factory
│       ├── sqlite.go                 # SQLite config factory
│       ├── mssql.go                  # MSSQL config factory
│       ├── row_adapter.go            # Row interface implementation
│       ├── config_test.go            # Config initialization tests
│       ├── db_test.go                # DB interface tests
│       ├── fluentDB_test.go          # Builder tests
│       ├── logger_adapters_test.go   # Logger adapter tests (77+ tests)
│       ├── mysql_test.go, postgres_test.go, etc  # Driver-specific tests
│       ├── db_mocks.go               # Auto-generated mocks
│       └── logger_mocks.go           # Auto-generated logger mocks
│
├── internal/
│   └── pkg/
│       ├── builder/                  # BUILDER LAYER
│       │   ├── builder.go            # QueryBuilder interface definition
│       │   ├── select.go             # SelectBuilder implementation
│       │   ├── insert.go             # InsertBuilder implementation
│       │   ├── update.go             # UpdateBuilder implementation
│       │   ├── delete.go             # DeleteBuilder implementation
│       │   ├── mysql_builder.go      # MySQL-specific builders
│       │   ├── postgres_builder.go   # PostgreSQL-specific builders
│       │   ├── sqlite_builder.go     # SQLite-specific builders
│       │   ├── mssql_builder.go      # MSSQL-specific builders
│       │   ├── builders_test.go      # Builder tests
│       │   ├── mysql_builder_test.go # MySQL builder tests
│       │   └── ... (similar for other dialects)
│       │
│       ├── sqldialect/               # DIALECT LAYER
│       │   ├── sql_dialect.go        # Base Dialect interface
│       │   ├── mysql_dialect.go      # MySQL dialect (backticks)
│       │   ├── postgres_dialect.go   # PostgreSQL dialect (double quotes)
│       │   ├── sqlite_dialect.go     # SQLite dialect (no quoting)
│       │   ├── mssql_dialect.go      # MSSQL dialect (square brackets)
│       │   ├── dialect_test.go       # Shared dialect tests
│       │   └── *_dialect_test.go     # Per-dialect tests
│       │
│       ├── operator/                 # OPERATOR DEFINITIONS
│       │   ├── operators.go          # Operator constants and registry
│       │   └── operators_test.go     # Operator tests
│       │
│       ├── helpers/                  # UTILITY FUNCTIONS
│       │   ├── strings.go            # String building helpers
│       │   └── validation.go         # Input validation helpers
│       │
│       └── otel/                     # OPENTELEMETRY INTEGRATION
│           ├── tracing.go            # Trace instrumentation
│           ├── metrics.go            # Metrics collection
│           └── otel_test.go          # Instrumentation tests
│
├── pkg/
│   ├── query/                        # QUERY DSL (USER-FACING)
│   │   ├── condition/                # WHERE clause expressions
│   │   │   ├── condition.go          # Expr, And, Or, In, Between, IsNull
│   │   │   └── condition_test.go     # Condition tests
│   │   │
│   │   ├── options/                  # Query modifiers
│   │   │   ├── options.go            # OrderBy, GroupBy, Limit, etc
│   │   │   └── options_test.go       # Options tests
│   │   │
│   │   ├── definition/               # Constants and enums
│   │   │   ├── drivers.go            # Driver names, query types
│   │   │   └── definition_test.go    # Definition tests
│   │   │
│   │   └── result/                   # Result processing
│   │       ├── mapper.go             # Row to struct mapping
│   │       └── mapper_test.go        # Mapper tests
│   │
│   ├── retry/                        # RETRY LOGIC (Utility)
│   │   ├── backoff.go                # Backoff strategies
│   │   ├── retry.go                  # Retry with jitter
│   │   └── retry_test.go             # Retry tests
│   │
│   └── dberror/                      # ERROR TYPES
│       ├── errors.go                 # Error wrapping and types
│       └── errors_test.go            # Error tests
│
├── tests/                            # INTEGRATION TESTS
│   ├── integration_test.go           # End-to-end tests across all databases
│   ├── fixtures/                     # Test data and fixtures
│   │   ├── schema.sql                # Database schema for tests
│   │   └── seeds.go                  # Test data populations
│   │
│   └── mocks.go                      # Test utilities and helpers
│
├── examples/                         # USAGE EXAMPLES
│   ├── basic-crud/                   # Basic CRUD examples
│   ├── transactions/                 # Transaction examples
│   ├── custom-logger/                # Custom logger adapter
│   ├── custom-driver/                # Custom database driver
│   ├── multi-db/                     # Using multiple databases
│   └── plugin-example/               # Plugin registration
│
├── docs/                             # DOCUMENTATION
│   ├── architecture.md               # This file (system architecture)
│   ├── ARCHITECTURE.md               # (alias to this file)
│   ├── ERROR_HANDLING.md             # Error handling deep dive
│   ├── ENVIRONMENT_VARIABLES.md      # Configuration via env vars
│   ├── DB_MANAGER.md                 # Database manager patterns
│   ├── CODE_REVIEW.md                # Code review checklist
│   ├── LINTING.md                    # Linting and formatting
│   └── CHANGELOG.md                  # Version history
│
├── .github/workflows/                # CI/CD automation
│   ├── test.yml                      # Run all tests
│   ├── lint.yml                      # Run linters
│   └── coverage.yml                  # Check coverage
│
├── Makefile                          # Build automation
├── go.mod                            # Go module definition
├── go.sum                            # Dependency checksums
├── docker-compose.test.yml           # Multi-database test environment
├── .golangci.yml                     # Linter configuration
├── README.md                         # Project overview
├── CONTRIBUTING.md                   # Contributor guide
├── LICENSE.md                        # License information
├── RELEASES.md                       # Release notes
└── CODE_QUALITY_IMPROVEMENTS.md      # Quality tracking
```

### Design Rationale

#### Public vs Internal Separation

- **`db/v1/`**: Only types and functions users interact with directly
  - Stable API surface
  - All types exported (capitalized)
  - Concrete implementations hidden
- **`internal/pkg/`**: Implementation details, never imported by users
  - Multiple builder implementations per dialect
  - Dialect-specific rendering logic
  - Plugin registry and extension hooks

#### File Organization Principles

**One file, one concept**:

- `builder.go` → Interface definition
- `select.go` → SelectBuilder implementation
- `mysql_builder.go` → MySQL-specific logic

**Tests co-located with source**:

- `db.go` ↔ `db_test.go` (same package)
- Integration tests separate: `tests/integration_test.go`

**Per-dialect implementations**:

- `mysql_builder.go`, `postgres_builder.go`, etc.
- Each file is self-contained, minimal shared code
- Differences highlighted by file structure

---

## Core Components

### 1. Public API Layer (`db/v1/`)

**Responsibility**: Define contracts that users depend on.

#### Primary Interfaces

| Interface    | Purpose                  | Key Methods                         |
| ------------ | ------------------------ | ----------------------------------- |
| **DB**       | Connection pool + CRUD   | Get, Insert, Update, Delete, Begin  |
| **Tx**       | Transaction scope        | Get, Insert, Update, Delete, Commit |
| **Logger**   | Pluggable logging        | Debug, Info, Warn, Error, With      |
| **Row**      | Result row abstraction   | Scan(columns), ScanOne(column)      |
| **FluentDB** | Query builder entrypoint | Select, Insert, Update, Delete      |

#### Configuration Factories

Each database has a config struct:

```go
// MySQL
type MySQLConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    DBName   string
    // Optional: SSL, Charset, MaxConnections, etc
}

// PostgreSQL
type PostgresConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    DBName   string
    SSLMode  string  // disable, require, verify-ca, verify-full
}

// SQLite
type SQLiteConfig struct {
    Path    string  // File path or ":memory:"
    Mode    string  // rw, ro, rwc
}

// MSSQL
type MSSQLConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    Database string
    // Optional: WindowsAuth, NamedPipe, etc
}
```

### 2. Query Builder Layer

**Files**: `internal/pkg/builder/*.go`

#### How Builders Work

1. **Accumulation Phase** (non-mutating)

   ```go
   b := NewSelectBuilder(dialect)
   b.WithTable("users")        // Returns new builder
   b = b.WithColumns("id", "name")  // Builder pattern
   b = b.WithWhere(condition)
   ```

2. **Build Phase** (SQL generation)

   ```go
   sqlString, args := b.Build()
   // sqlString: "SELECT id, name FROM users WHERE age > ?"
   // args: [18]
   ```

3. **Execution Phase** (driver invocation)

   ```go
   rows, err := db.Query(ctx, sqlString, args...)
   ```

#### SelectBuilder Anatomy

```go
type SelectBuilder struct {
    table      string
    columns    []string
    conditions []Condition
    orderBy    []string
    groupBy    []string
    having     Condition
    limit      int
    offset     int
    returning  []string
    distinct   bool
    dialect    Dialect
}

func (b *SelectBuilder) Build() (string, []any) {
    // 1. Validate inputs
    // 2. Build SELECT clause
    // 3. Build FROM clause
    // 4. Build WHERE clause
    // 5. Build GROUP BY / HAVING
    // 6. Build ORDER BY
    // 7. Build LIMIT / OFFSET
    // 8. Build RETURNING (if supported)
    // Return parameterized SQL + args
}
```

### 3. Dialect Layer

**Files**: `internal/pkg/sqldialect/*.go`

Each dialect implements:

```go
type Dialect interface {
    // Identifier quoting
    QuoteIdentifier(name string) string

    // Feature support matrix
    SupportsReturning() bool
    SupportsUpsert() bool
    SupportsJSON() bool

    // Operator rendering
    RenderOperator(op string) string

    // Keyword rendering
    RenderLimit(limit int) string
    RenderOffset(offset int) string
}
```

#### Dialect-Specific Examples

**MySQL**:

- Quote: backticks `` `table` ``
- No RETURNING
- UPSERT: `INSERT INTO ... ON DUPLICATE KEY UPDATE`

**PostgreSQL**:

- Quote: double quotes `"table"`
- RETURNING supported
- JSON/JSONB operators
- Array operators

**SQLite**:

- No quoting needed
- UPSERT: `INSERT INTO ... ON CONFLICT DO UPDATE`
- Limited JSON operators

**MSSQL**:

- Quote: square brackets `[table]`
- No RETURNING (uses OUTPUT clause)
- OFFSET...FETCH syntax

---

## Builder Implementation Deep Dive

### SelectBuilder Details

```go
// Complete SelectBuilder.Build() flow

func (b *SelectBuilder) Build() (string, []any) {
    var sb strings.Builder
    args := []any{}

    // 1. SELECT clause
    sb.WriteString("SELECT ")
    if b.distinct {
        sb.WriteString("DISTINCT ")
    }
    for i, col := range b.columns {
        if i > 0 { sb.WriteString(", ") }
        sb.WriteString(b.dialect.QuoteIdentifier(col))
    }

    // 2. FROM clause
    sb.WriteString(" FROM ")
    sb.WriteString(b.dialect.QuoteIdentifier(b.table))

    // 3. WHERE clause
    if b.conditions != nil {
        sb.WriteString(" WHERE ")
        condSQL, condArgs := b.buildConditions(b.conditions)
        sb.WriteString(condSQL)
        args = append(args, condArgs...)
    }

    // 4. GROUP BY
    if len(b.groupBy) > 0 {
        sb.WriteString(" GROUP BY ")
        for i, col := range b.groupBy {
            if i > 0 { sb.WriteString(", ") }
            sb.WriteString(b.dialect.QuoteIdentifier(col))
        }
    }

    // 5. HAVING
    if b.having != nil {
        sb.WriteString(" HAVING ")
        havingSQL, havingArgs := b.having.ToSQL(b.dialect)
        sb.WriteString(havingSQL)
        args = append(args, havingArgs...)
    }

    // 6. ORDER BY
    if len(b.orderBy) > 0 {
        sb.WriteString(" ORDER BY ")
        sb.WriteString(strings.Join(b.orderBy, ", "))
    }

    // 7. LIMIT / OFFSET (dialect-specific)
    if b.limit > 0 {
        sb.WriteString(" ")
        sb.WriteString(b.dialect.RenderLimit(b.limit))
    }
    if b.offset > 0 && !b.dialect.UsesOffsetFetch() {
        sb.WriteString(" ")
        sb.WriteString(b.dialect.RenderOffset(b.offset))
    }
    if b.offset > 0 && b.dialect.UsesOffsetFetch() {
        // MSSQL: OFFSET x ROWS FETCH NEXT y ROWS ONLY
        sb.WriteString(fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY",
            b.offset, b.limit))
    }

    // 8. RETURNING (PostgreSQL only)
    if b.dialect.SupportsReturning() && len(b.returning) > 0 {
        sb.WriteString(" RETURNING ")
        for i, col := range b.returning {
            if i > 0 { sb.WriteString(", ") }
            sb.WriteString(b.dialect.QuoteIdentifier(col))
        }
    }

    return sb.String(), args
}
```

### InsertBuilder Details

```go
// InsertBuilder focuses on bulk insert optimization

type InsertBuilder struct {
    table   string
    columns []string
    values  [][]any          // Multiple rows
    dialect Dialect
}

func (b *InsertBuilder) Build() (string, []any) {
    var sb strings.Builder
    args := []any{}

    sb.WriteString("INSERT INTO ")
    sb.WriteString(b.dialect.QuoteIdentifier(b.table))
    sb.WriteString(" (")

    // Columns
    for i, col := range b.columns {
        if i > 0 { sb.WriteString(", ") }
        sb.WriteString(b.dialect.QuoteIdentifier(col))
    }
    sb.WriteString(") VALUES ")

    // Values (one row or multiple)
    for rowIdx, row := range b.values {
        if rowIdx > 0 { sb.WriteString(", ") }
        sb.WriteString("(")
        for colIdx := range b.columns {
            if colIdx > 0 { sb.WriteString(", ") }
            sb.WriteString("?")
            args = append(args, row[colIdx])
        }
        sb.WriteString(")")
    }

    return sb.String(), args
}

// Example output:
// INSERT INTO users (id, name, email) VALUES (?, ?, ?), (?, ?, ?)
// Args: [1, "Alice", "alice@...", 2, "Bob", "bob@..."]
```

### UpdateBuilder Details

```go
type UpdateBuilder struct {
    table      string
    columns    map[string]any  // Column → new value
    conditions []Condition
    dialect    Dialect
}

func (b *UpdateBuilder) Build() (string, []any) {
    var sb strings.Builder
    args := []any{}

    sb.WriteString("UPDATE ")
    sb.WriteString(b.dialect.QuoteIdentifier(b.table))
    sb.WriteString(" SET ")

    // SET clause
    i := 0
    for col, val := range b.columns {
        if i > 0 { sb.WriteString(", ") }
        sb.WriteString(b.dialect.QuoteIdentifier(col))
        sb.WriteString(" = ?")
        args = append(args, val)
        i++
    }

    // WHERE clause
    if b.conditions != nil {
        sb.WriteString(" WHERE ")
        condSQL, condArgs := b.buildConditions(b.conditions)
        sb.WriteString(condSQL)
        args = append(args, condArgs...)
    }

    return sb.String(), args
}
```

### DeleteBuilder Details

```go
type DeleteBuilder struct {
    table      string
    conditions []Condition
    dialect    Dialect
}

func (b *DeleteBuilder) Build() (string, []any) {
    var sb strings.Builder
    args := []any{}

    sb.WriteString("DELETE FROM ")
    sb.WriteString(b.dialect.QuoteIdentifier(b.table))

    // WHERE clause (REQUIRED for deleteBuilder safety)
    if b.conditions == nil || len(b.conditions) == 0 {
        return "", nil  // Refuse unqualified DELETE
    }

    sb.WriteString(" WHERE ")
    condSQL, condArgs := b.buildConditions(b.conditions)
    sb.WriteString(condSQL)
    args = append(args, condArgs...)

    return sb.String(), args
}
```

---

## Dialect System Deep Dive

### Dialect Interface Specification

```go
type Dialect interface {
    // Identifier quoting for table/column names
    QuoteIdentifier(name string) string

    // Feature matrix: what does this dialect support?
    SupportsReturning() bool      // RETURNING / OUTPUT clause
    SupportsUpsert() bool         // INSERT ... ON CONFLICT / DUPLICATE KEY
    SupportsJSON() bool           // JSON/JSONB operators
    SupportsArray() bool          // Array types
    SupportsRanges() bool         // Range types

    // Operator rendering
    RenderOperator(op string) string

    // Keyword rendering
    RenderLimit(limit int) string
    RenderOffset(offset int) string
    UsesOffsetFetch() bool        // MSSQL: OFFSET...FETCH vs LIMIT...OFFSET

    // Type mapping (schema generation)
    GoTypeToSQLType(goType string) string
}
```

### MySQL Dialect Implementation

```go
type MySQLDialect struct {}

func (d *MySQLDialect) QuoteIdentifier(name string) string {
    return "`" + name + "`"  // MySQL uses backticks
}

func (d *MySQLDialect) SupportsReturning() bool {
    return false  // MySQL doesn't have RETURNING
}

func (d *MySQLDialect) RenderOperator(op string) string {
    // MySQL-specific operator handling
    switch op {
    case "LIKE", "NOT LIKE", ">", "<", "=":
        return op  // Standard operators
    case "JSON_CONTAINS":
        return "JSON_CONTAINS"
    default:
        return op
    }
}

func (d *MySQLDialect) RenderLimit(limit int) string {
    return fmt.Sprintf("LIMIT %d", limit)
}
```

### PostgreSQL Dialect Implementation

```go
type PostgresDialect struct {}

func (d *PostgresDialect) QuoteIdentifier(name string) string {
    return `"` + name + `"`  // PostgreSQL uses double quotes
}

func (d *PostgresDialect) SupportsReturning() bool {
    return true  // PostgreSQL supports RETURNING
}

func (d *PostgresDialect) RenderOperator(op string) string {
    switch op {
    case "@>", "<@":  // JSON operators
        return op
    case "&&":        // Array overlap
        return op
    default:
        return op
    }
}

func (d *PostgresDialect) RenderLimit(limit int) string {
    return fmt.Sprintf("LIMIT %d", limit)
}
```

### SQLite Dialect Implementation

```go
type SQLiteDialect struct {}

func (d *SQLiteDialect) QuoteIdentifier(name string) string {
    return name  // SQLite doesn't require quoting
}

func (d *SQLiteDialect) SupportsReturning() bool {
    return false  // SQLite 3.35+ has RETURNING, but older versions don't
}

func (d *SQLiteDialect) RenderLimit(limit int) string {
    return fmt.Sprintf("LIMIT %d", limit)
}
```

### MSSQL Dialect Implementation

```go
type MSSQLDialect struct {}

func (d *MSSQLDialect) QuoteIdentifier(name string) string {
    return "[" + name + "]"  // MSSQL uses square brackets
}

func (d *MSSQLDialect) SupportsReturning() bool {
    return false  // MSSQL uses OUTPUT instead
}

func (d *MSSQLDialect) UsesOffsetFetch() bool {
    return true  // MSSQL: OFFSET...FETCH instead of LIMIT
}

func (d *MSSQLDialect) RenderLimit(limit int) string {
    // MSSQL uses TOP clause instead of LIMIT
    return fmt.Sprintf("TOP (%d)", limit)
}
```

---

## Connection Pooling Strategy Detailed

### Per-Database Pool Implementation

#### PostgreSQL (pgxpool)

```go
// Optimized pgxpool from jackc

config, _ := pgxpool.ParseConfig("postgresql://...")

// Configure pool
config.MaxConns = 25                      // Max connections
config.MinConns = 5                       // Min idle connections
config.MaxConnLifetime = 15 * time.Minute // Connection expiration
config.MaxConnIdleTime = 5 * time.Minute  // Close idle after 5 min
config.HealthCheckInterval = 1 * time.Minute

pool, _ := pgxpool.NewWithConfig(ctx, config)

// Pool manages: connection creation, idle management, health checks
```

**Features**:

- Automatic connection management
- Prepared statement caching per connection
- Connection health checks
- Built-in metrics via `pool.Stat()`

#### MySQL (database/sql)

```go
// Standard library database/sql

db, _ := sql.Open("mysql", "user:password@/dbname")

// Configure pool
db.SetMaxOpenConns(25)           // Max concurrent connections
db.SetMaxIdleConns(5)            // Max idle connections
db.SetConnMaxLifetime(15 * time.Minute)     // Auto-close after 15 min
db.SetConnMaxIdleTime(5 * time.Minute)      // Close idle after 5 min

// Pool manages: connection creation, idle timeout, reaping
```

**Trade-offs**:

- Simpler than pgxpool
- No automatic prepared statement caching
- Requires manual cache if needed

#### SQLite (in-memory or file)

```go
// SQLite uses single connection (file-based concurrency)

db, _ := sql.Open("sqlite3", "file:test.db?cache=shared&mode=rwc")

// Set max open connections to 1 for single-writer concurrency
db.SetMaxOpenConns(1)
db.SetMaxIdleConns(1)

// SQLite handles: file locking, WAL mode for concurrency
```

**Characteristics**:

- Single writer (FIFO queue)
- Multiple readers (with WAL mode enabled)
- No network overhead
- File-based persistence

#### MSSQL (denisenkom/go-mssqldb)

```go
// Standard database/sql with MSSQL driver

db, _ := sql.Open("sqlserver", "server=localhost;user id=sa;password=...")

db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(15 * time.Minute)

// Pool behavior: similar to MySQL driver
```

### Pool Health & Monitoring

```go
type PoolStats struct {
    OpenConnections  int       // Currently open
    InUseConnections int       // Currently executing queries
    IdleConnections  int       // Waiting for work
}

// PostgreSQL (pgxpool)
stat := pool.Stat()
fmt.Printf("Open: %d, InUse: %d, Idle: %d\n",
    stat.AcquiredConns(),
    stat.BusyConns(),
    stat.IdleConns(),
)

// MySQL (database/sql)
dbStats := db.Stats()
fmt.Printf("Open: %d, InUse: %d, Idle: %d, MaxOpen: %d\n",
    dbStats.OpenConnections,
    dbStats.InUse,
    dbStats.Idle,
    dbStats.MaxOpenConnections,
)
```

### Tuning Connection Pools

**For High-Traffic APIs** (100+ RPS):

```ini
MaxOpenConns: 25 (per CPU core)
MinConns: 5
MaxConnLifetime: 15 minutes
MaxConnIdleTime: 5 minutes
```

**For Batch Processing**:

```ini
MaxOpenConns: 10 (fewer long-lived connections)
MinConns: 1
MaxConnLifetime: 1 hour
MaxConnIdleTime: 30 minutes
```

**For Development/Testing**:

```ini
MaxOpenConns: 5
MinConns: 1
MaxConnLifetime: 5 minutes
MaxConnIdleTime: 1 minute
```

---

## Query Construction Flow with Examples

### Complete User Example

```go
package main

import (
    "context"
    "log"
    v1 "tounilab.com/fabric/db/v1"
    "tounilab.com/fabric/pkg/query/condition"
)

func main() {
    // 1. Setup database
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

    ctx := context.Background()

    // 2. Query: Find active users over 18
    rows, err := v1.NewFluentDB(db).
        Select("users", "id", "name", "email", "age").
        Where(
            condition.And(
                condition.NewExpr().Column("status").Op("=").Value("active"),
                condition.NewExpr().Column("age").Op(">").Value(18),
            ),
        ).
        OrderBy("name ASC", "created_at DESC").
        Limit(100).
        Offset(0).
        Get(ctx)

    if err != nil {
        log.Printf("Query failed: %v", err)
        return
    }

    // 3. Process results
    for _, row := range rows {
        id := row["id"].(string)
        name := row["name"].(string)
        email := row["email"].(string)
        age := row["age"].(int64)

        log.Printf("User %d: %s (%s), age %d", id, name, email, age)
    }
}
```

### SQL Generated (PostgreSQL)

```sql
SELECT "id", "name", "email", "age"
FROM "users"
WHERE ("status" = $1 AND "age" > $2)
ORDER BY "name" ASC, "created_at" DESC
LIMIT $3 OFFSET $4

Args: ["active", 18, 100, 0]
```

### Step-by-Step Internal Flow

```text
1. NewFluentDB(db)
   ↓ Returns FluentDB{db}

2. FluentDB.Select("users", "id", "name", "email", "age")
   ↓ Creates SelectBuilder{
       table: "users",
       columns: ["id", "name", "email", "age"],
       dialect: postgresDialect,
     }

3. SelectBuilder.Where(condition)
   ↓ Appends condition to SelectBuilder.conditions[]

4. SelectBuilder.OrderBy("name ASC", "created_at DESC")
   ↓ Appends to SelectBuilder.orderBy[]

5. SelectBuilder.Limit(100)
   ↓ Sets SelectBuilder.limit = 100

6. SelectBuilder.Offset(0)
   ↓ Sets SelectBuilder.offset = 0

7. SelectBuilder.Execute()
   ↓ Calls SelectBuilder.Build()
      - Returns SQL string and args
   ↓ Calls db.Query(sqlString, args...)
      - Sends to database/sql
      - Driver converts to PostgreSQL protocol
      - Network transmission to server
   ↓ Scans results into []Row
   ↓ Returns to user
```

---

## Transaction Handling and Lifecycle

### Transaction Interface

```go
type Tx interface {
    // Same CRUD methods as DB
    Get(ctx context.Context, query string, args ...any) ([]Row, error)
    Insert(ctx context.Context, table string, data map[string]any) (Row, error)
    Update(ctx context.Context, table string, data map[string]any, cond Condition)
     (int64, error)
    Delete(ctx context.Context, table string, cond Condition) (int64, error)

    // Transaction control
    Commit() error
    Rollback() error
}
```

### Transaction Lifecycle

```text
┌─────────────────────────────────────────────────────────────┐
│ 1. BEGIN: Client initiates transaction                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ tx, err := db.Begin(ctx)                                    │
│                                                             │
│ Database state: Transaction opened, isolation level set     │
│ Locks released: None (unless statements acquire locks)      │
│ Rollback point: Created                                     │
│                                                             │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│ 2. STATEMENTS: Execute operations within transaction        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ row1, err := tx.Insert(ctx, "users", userData1)             │
│ row2, err := tx.Insert(ctx, "users", userData2)             │
│ _, err := tx.Update(ctx, "accounts", balanceUpdate, cond)   │
│ _, err := tx.Delete(ctx, "sessions", sessionCond)           │
│                                                             │
│ Each statement: Executes within transaction scope           │
│ Isolation: Per transaction isolation level (READ COMMITTED) │
│ Visibility: Changes not visible outside transaction         │
│                                                             │
│ If any error:                                               │
│   - Continue (manual rollback later)                        │
│   - Or defer tx.Rollback() (automatic on error)             │
│                                                             │
└──────────────────────┬──────────────────────────────────────┘
                       │
        ┌──────────────┴────────────────┐
        │                               │
        ▼ (No errors)                   ▼ (Error encountered)
┌──────────────────────┐        ┌──────────────────────┐
│ 3a. COMMIT           │        │ 3b. ROLLBACK         │
├──────────────────────┤        ├──────────────────────┤
│                      │        │                      │
│ err := tx.Commit()   │        │ err := tx.Rollback() │
│                      │        │                      │
│ Database state:      │        │ Database state:      │
│ Transaction closes   │        │ All changes undone   │
│ Changes durable      │        │ Transactionally      │
│ Locks released       │        │ consistent           │
│ All-or-nothing       │        │ Locks released       │
│                      │        │                      │
└──────────────────────┘        └──────────────────────┘
        │                               │
        │ Write to WAL/log              │ Undo log
        │                               │
        ▼                               ▼
    ✓ Success                       ✓ Back to consistent
```

### Usage Patterns

#### Manual Commit/Rollback

```go
tx, err := db.Begin(ctx)
if err != nil {
    return err
}

// Execute operations
result, err := tx.Insert(ctx, "orders", orderData)
if err != nil {
    tx.Rollback()
    return err
}

result2, err := tx.Insert(ctx, "order_items", itemData)
if err != nil {
    tx.Rollback()  // Undo the first insert
    return err
}

return tx.Commit()
```

#### Helper: WithTransaction

```go
// Signature (if available):
func (db DB) WithTransaction(ctx context.Context,
    fn func(tx Tx) error) error {

    tx, err := db.Begin(ctx)
    if err != nil {
        return err
    }

    if err := fn(tx); err != nil {
        tx.Rollback()
        return err
    }

    return tx.Commit()
}

// Usage:
err := db.WithTransaction(ctx, func(tx v1.Tx) error {
    if _, err := tx.Insert(ctx, "orders", order); err != nil {
        return err  // Automatic rollback
    }
    if _, err := tx.Insert(ctx, "order_items", item); err != nil {
        return err  // Automatic rollback
    }
    return nil  // Automatic commit
})
```

#### Nested Transactions (SavePoints)

```go
// Start outer transaction
tx, _ := db.Begin(ctx)
defer tx.Rollback()

// Batch operation 1
_, _ = tx.Insert(ctx, "users", user1)
_, _ = tx.Insert(ctx, "users", user2)

// Batch operation 2 (might fail)
if err := riskyOperation(ctx, tx); err != nil {
    // Rollback only operation 2, keep operation 1
    // (Requires save point support)
    tx.Rollback()
    return
}

// All good
tx.Commit()
```

---

## Error Handling

### Error Types

```go
package dberror

type DatabaseError interface {
    error
    Code() string           // Error code (e.g., "UNIQUE_VIOLATION")
    IsRetryable() bool      // Can operation be retried?
    Severity() string       // "ERROR", "FATAL", "WARNING"
}

type Error struct {
    Err        error         // Underlying error
    Code       string        // Standardized code
    Message    string        // User-friendly message
    Retryable  bool          // Safe to retry?
    Context    map[string]any // Additional context (no secrets!)
}

// Example codes
const (
    ErrConnectionFailed = "CONNECTION_FAILED"
    ErrUniqueViolation  = "UNIQUE_VIOLATION"
    ErrForeignKeyViolation = "FOREIGN_KEY_VIOLATION"
    ErrQueryTimeout     = "QUERY_TIMEOUT"
    ErrConnectionPoolExhausted = "POOL_EXHAUSTED"
)
```

### Common Errors & Handling

```go
// Connection errors (usually retryable)
if err := db.Get(ctx, "SELECT ...", args); err != nil {
    if isDatabaseError(err, dberror.ErrConnectionFailed) {
        // Retry with exponential backoff
        return retryWithBackoff(ctx, fn, 3, 100*time.Millisecond)
    }
}

// Unique constraint violation (NOT retryable)
if err := db.Insert(ctx, "users", userData); err != nil {
    if isDatabaseError(err, dberror.ErrUniqueViolation) {
        return fmt.Errorf("user email already exists: %w", err)
    }
}

// Query timeout (retryable)
if err := db.Get(ctx, "SELECT ...", args); err != nil {
    if isDatabaseError(err, dberror.ErrQueryTimeout) {
        // Retry with longer timeout
        ctx2, _ := context.WithTimeout(ctx, 30*time.Second)
        return db.Get(ctx2, "SELECT ...", args)
    }
}

// Connection pool exhausted (retryable with backoff)
if err := db.Insert(ctx, "events", data); err != nil {
    if isDatabaseError(err, dberror.ErrConnectionPoolExhausted) {
        return retryWithBackoff(ctx, fn, 5, 500*time.Millisecond)
    }
}
```

### Error Sanitization

```go
// ✓ GOOD: Don't leak sensitive data
if err != nil {
    logger.Error("database operation failed",
        "operation", "insert_user",
        "table", "users",
        "error", err.Error(),  // Only error message
    )
    // Logs: operation failed, error: duplicate key
    // Does NOT log: password, connection string, etc
}

// ✗ BAD: Logging sensitive data
logger.Error("database error", "config", cfg.String())
// Leaks: host, port, password, credentials
```

### Retry Logic

```go
func retryWithBackoff(ctx context.Context, fn func() error,
    maxRetries int, initialBackoff time.Duration) error {

    var lastErr error
    backoff := initialBackoff

    for attempt := 0; attempt < maxRetries; attempt++ {
        if err := fn(); err == nil {
            return nil  // Success
        } else {
            lastErr = err
        }

        if attempt < maxRetries-1 {
            // Exponential backoff with jitter
            jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
            sleep := backoff + jitter

            select {
            case <-time.After(sleep):
                backoff *= 2
            case <-ctx.Done():
                return ctx.Err()  // Context cancelled
            }
        }
    }

    return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
```

---

## Testing Architecture

### Test Organization & Strategy

#### Unit Tests (db/v1/)

Test each public method in isolation:

```go
// File: db/v1/db_test.go
func TestDB_Get_Success(t *testing.T) {
    // Arrange
    mockDriver := &MockDriver{}
    db := NewDB(mockDriver, nullLogger())

    // Act
    rows, err := db.Get(ctx, "SELECT ...", args)

    // Assert
    require.NoError(t, err)
    assert.Len(t, rows, 1)
}

func TestDB_Get_ConnectionError(t *testing.T) {
    // Arrange
    mockDriver := &MockDriver{
        QueryError: errors.New("connection refused"),
    }
    db := NewDB(mockDriver, nullLogger())

    // Act
    _, err := db.Get(ctx, "SELECT ...", args)

    // Assert
    require.Error(t, err)
    assert.True(t, isDatabaseError(err, dberror.ErrConnectionFailed))
}
```

#### Builder Tests (internal/pkg/builder/)

Test SQL generation per dialect:

```go
// File: internal/pkg/builder/mysql_builder_test.go
func TestMySQLBuilder_SelectWithWhere(t *testing.T) {
    builder := NewSelectBuilder(mysqlDialect)
    builder.WithTable("users")
    builder.WithColumns("id", "name")
    builder.WithWhere(condition.NewExpr().Column("age").Op(">").Value(18))

    sql, args := builder.Build()

    assert.Equal(t, "SELECT `id`, `name` FROM `users` WHERE `age` > ?", sql)
    assert.Equal(t, []any{18}, args)
}
```

#### Dialect Tests (internal/pkg/sqldialect/)

Test dialect-specific rendering:

```go
// File: internal/pkg/sqldialect/mysql_dialect_test.go
func TestMySQLDialect_QuoteIdentifier(t *testing.T) {
    dialect := MySQLDialect{}

    assert.Equal(t, "`users`", dialect.QuoteIdentifier("users"))
    assert.Equal(t, "`user_id`", dialect.QuoteIdentifier("user_id"))
}

func TestMySQLDialect_SupportsReturning(t *testing.T) {
    dialect := MySQLDialect{}
    assert.False(t, dialect.SupportsReturning())
}
```

#### Integration Tests (tests/integration_test.go)

Test against real databases:

```go
// File: tests/integration_test.go
func TestIntegration_SelectWithWhere(t *testing.T) {
    databases := []struct {
        name string
        db   v1.DB
    }{
        {"MySQL", setupMySQL(t)},
        {"PostgreSQL", setupPostgres(t)},
        {"SQLite", setupSQLite(t)},
        {"MSSQL", setupMSSQL(t)},
    }

    for _, dbTest := range databases {
        t.Run(dbTest.name, func(t *testing.T) {
            // Insert test data
            _, _ = dbTest.db.Insert(ctx, "users", map[string]any{
                "name": "Alice",
                "age": 30,
            })

            // Query
            rows, err := v1.NewFluentDB(dbTest.db, ctx).
                Select("users", "name").
                Where(cdt.NewExpr().Column("age").Op(">").Value(25)).
                Execute()

            // Verify
            require.NoError(t, err)
            assert.Len(t, rows, 1)
        })
    }
}
```

### Running Tests

```bash
# Unit tests only (fast)
go test -tags=test ./...

# Coverage
go test -tags=test -cover ./...

# Integration tests (requires Docker)
docker-compose -f docker-compose.test.yml up -d
go test -tags=integration ./tests/...
docker-compose -f docker-compose.test.yml down

# Verbose with custom output
go test -v -run TestSelectBuilder ./...

# Benchmark
go test -bench=^BenchmarkSelectBuilder$ -benchmem ./internal/pkg/builder/...
```

### Coverage Target

**Minimum 80%** for production code:

```sh
$ go test -cover ./...
db/v1: 85% coverage
internal/pkg/builder: 92% coverage
internal/pkg/sqldialect: 88% coverage
pkg/query: 90% coverage
```

---

## Performance Optimization Points

### 1. Query Building Performance

**Bottleneck**: StringBuilder allocations

**Optimization**:

```go
// Pre-allocate buffer to expected size
var sb strings.Builder
sb.Grow(256)  // Common query sizes

// Reuse builders for bulk operations
builder := NewSelectBuilder(dialect)
for _, table := range tables {
    builder.WithTable(table)  // Reused, not reallocated
    sql, args := builder.Build()
    // Execute
}
```

### 2. Parameter Binding Performance

**Bottleneck**: Array allocations for args

**Optimization**:

```go
// Collect args more efficiently
type argCollector struct {
    args []any
    cap  int
}

func (ac *argCollector) Add(val any) {
    if len(ac.args) == ac.cap {
        ac.cap *= 2
        newArgs := make([]any, ac.cap)
        copy(newArgs, ac.args)
        ac.args = newArgs
    }
    ac.args = append(ac.args, val)
}
```

### 3. Connection Pool Optimization

**Bottleneck**: Connection acquisition latency

**Optimization**:

```go
// Configuration for high-traffic
db.SetMaxOpenConns(25 * numCPU)    // Higher for bursty traffic
db.SetMaxIdleConns(5 * numCPU)     // Keep warm connections
db.SetConnMaxLifetime(15 * time.Minute)  // Regular rotation
db.SetConnMaxIdleTime(5 * time.Minute)   // Don't hold indefinitely

// Monitor pool saturation
stats := db.Stats()
utilization := float64(stats.InUse) / float64(stats.OpenConnections)
if utilization > 0.9 {
    log.Warn("Connection pool nearly exhausted", "utilization", utilization)
}
```

### 4. Prepared Statement Caching

**Bottleneck**: Query parsing overhead

**Optimization**:

```go
// pgx automatically caches prepared statements
// For MySQL, consider application-level caching:

type PreparedStmtCache struct {
    cache map[string]*sql.Stmt  // SQL → Prepared stmt
    mu    sync.RWMutex
}

func (psc *PreparedStmtCache) Prepare(ctx context.Context,
    db *sql.DB, sql string) (*sql.Stmt, error) {
    psc.mu.RLock()
    if stmt, ok := psc.cache[sql]; ok {
        psc.mu.RUnlock()
        return stmt, nil
    }
    psc.mu.RUnlock()

    // Prepare if not cached
    stmt, err := db.PrepareContext(ctx, sql)
    if err != nil {
        return nil, err
    }

    psc.mu.Lock()
    psc.cache[sql] = stmt
    psc.mu.Unlock()

    return stmt, nil
}
```

### 5. N+1 Query Prevention

**Bottleneck**: Multiple round trips to database

**Optimization**:

```go
// ✗ BAD: N+1 queries
for _, userID := range userIDs {
    user, _ := db.GetByID(ctx, "users", userID)
    // N queries in loop
}

// ✓ GOOD: Single batch query
rows, _ := v1.NewFluentDB(db).
    Select("users", "id", "name").
    Where(cdt.In("id", userIDs)).
    Get(ctx)
// 1 query, N results
```

### 6. Query Specificity

**Bottleneck**: Large result sets

**Optimization**:

```go
// ✗ BAD: SELECT all columns
rows, _ := v1.NewFluentDB(db).
    Select("users", "*").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    Limit(1000).
    Get(ctx)

// ✓ GOOD: SELECT only needed columns
rows, _ := v1.NewFluentDB(db).
    Select("users", "id", "name", "email").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    Limit(1000).
    Get(ctx)
```

---

## Security Model Deep Dive

### SQL Injection Prevention

**Guarantee**: 100% parameterization, zero concatenation

```go
// ✓ SAFE
condition := cdt.NewExpr().Column("email").Op("=").Value(userEmail)
// Generated: "... WHERE email = ?"
// Args: [userEmail]

// ✗ UNSAFE
unsafeSQL := "SELECT * FROM users WHERE email = '" + userEmail + "'"
// User inputs interpreted as SQL
```

### Credential Handling

```go
// ✓ GOOD: Environment variables
password := os.Getenv("DB_PASSWORD")
cfg := v1.PostgresConfig{Password: password}

// ✗ BAD: Hardcoded
cfg := v1.PostgresConfig{Password: "super-secret"}

// ✓ BEST: Secret manager
secretsClient := vault.NewClient()
secret, _ := secretsClient.Get("database/postgres/password")
cfg := v1.PostgresConfig{Password: secret.Value}
```

### Error Message Sanitization

```go
// ✓ GOOD: Sanitized errors
if err != nil {
    logger.Error("query failed",
        "table", "users",
        "operation", "select",
        "error", sanitizeError(err),
    )
    // Logs: query failed, error: connection refused
}

func sanitizeError(err error) string {
    // Remove connection strings, credentials, etc
    msg := err.Error()
    // Remove password patterns
    msg = regexp.MustCompile(`password[=:]\S+`).ReplaceAllString(msg, "password=***")
    return msg
}

// ✗ BAD: Raw error with config
logger.Error("query failed", "config", cfg.String())
// Leaks password, host, credentials
```

### Security Checklist

- [ ] All user input parameterized
- [ ] Credentials from environment/secrets manager
- [ ] No raw SQL concatenation
- [ ] Error messages sanitized
- [ ] Connection strings not logged
- [ ] Prepared statements used
- [ ] Connection pooling configured securely
- [ ] Rate limiting on sensitive operations
- [ ] Invalid input rejected early
- [ ] HTTPS/TLS for remote databases

---

## OpenTelemetry Integration

### Instrumentation Points

```go
package otel

// Trace each database operation
func (db *DatabaseWrapper) Get(ctx context.Context, query string, args ...any)
 ([]Row, error) {
    tracer := otel.Tracer("fabric")
    ctx, span := tracer.Start(ctx, "database.query",
        trace.WithAttributes(
            attribute.String("db.statement", query),
            attribute.String("db.operation", "select"),
            attribute.Int("db.sql.arg.count", len(args)),
        ),
    )
    defer span.End()

    // Execute query
    rows, err := db.query(ctx, query, args...)

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    } else {
        span.SetStatus(codes.Ok, fmt.Sprintf("returned %d rows", len(rows)))
    }

    return rows, err
}
```

### Metrics Collection

```go
// Record query latency
meter := otel.Meter("fabric")
latencyHist, _ := meter.Float64Histogram("db.client.latency.ms")
rowCountHist, _ := meter.Int64Histogram("db.rows.returned")

// In query execution
start := time.Now()
rows, err := db.Get(ctx, query, args)
duration := time.Since(start).Seconds() * 1000

latencyHist.Record(ctx, duration,
    metric.WithAttributes(
        attribute.String("db.operation", "select"),
        attribute.String("db.table", "users"),
    ),
)
rowCountHist.Record(ctx, int64(len(rows)))
```

---

## Design Patterns

### 1. Fluent Builder

Used for ergonomic query construction:

```go
rows, err := v1.NewFluentDB(db).
    Select("users", "id", "name").
    Where(condition).
    OrderBy("name").
    Limit(10).
    Get(ctx)
```

### 2. Adapter (Logger Adapters)

Multiple logging frameworks behind single interface:

```go
logger := v1.NewSlogAdapter(slog.Default())
logger := v1.NewLogrusAdapter(logrusInstance)
logger := v1.NewZapAdapter(zapLogger)
```

### 3. Strategy (Dialect Pattern)

Different SQL rendering per database:

```go
var dialect Dialect

switch driver {
case "postgres":
    dialect = &PostgresDialect{}
case "mysql":
    dialect = &MySQLDialect{}
// ...
}
```

### 4. Factory (Config Factories)

Easy database initialization:

```go
cfg := v1.PostgresConfig{...}
db, _ := v1.NewDB(cfg, logger)

cfg := v1.MySQLConfig{...}
db, _ := v1.NewDB(cfg, logger)
```

### 5. Interface-Based Design

All public types are interfaces for testability:

```go
type DB interface { ... }
type Tx interface { ... }
type Logger interface { ... }

// Can mock any of these
type MockDB struct { mock.Mock }
```

---

## Common Workflows

### Workflow 1: Adding a New SQL Operator (e.g., GLOB for SQLite)

**1. Define operator constant**:

```go
// File: internal/pkg/operator/operators.go
const (
    GlobOp = "GLOB"
)
```

**2. Update builders**:

```go
// File: internal/pkg/builder/sqlite_builder.go
func (b *SelectBuilder) renderOperator(op string) string {
    switch op {
    // ... existing operators
    case GlobOp:
        return "GLOB"
    }
}
```

**3. Update dialects**:

```go
// File: internal/pkg/sqldialect/sqlite_dialect.go
func (d *SQLiteDialect) SupportedOperators() []string {
    return []string{"=", "!=", ">",…, "GLOB"}
}
```

**4. Add tests**:

```go
// File: tests/integration_test.go
func TestIntegration_GlobOperator(t *testing.T) {
    rows, _ := v1.NewFluentDB(db).
        Select("files", "name").
        Where(cdt.NewExpr().Column("name").Op("GLOB").Value("*.pdf")).
        Get(ctx)
    assert.NotEmpty(t, rows)
}
```

**5. Verify**:

```bash
go test -tags=test ./... && make coverage
```

### Workflow 2: Adding Logger Adapter (e.g., custom logger)

**1. Implement Logger interface**:

```go
// File: db/v1/custom_logger.go
type CustomLogger struct{
    // fields
}

func (l *CustomLogger) Debug(msg string, keyvals ...any)
    { /* implementation */ }
func (l *CustomLogger) Info(msg string, keyvals ...any)
    { /* implementation */ }
func (l *CustomLogger) Warn(msg string, keyvals ...any)
    { /* implementation */ }
func (l *CustomLogger) Error(msg string, keyvals ...any)
    { /* implementation */ }
func (l *CustomLogger) With(key string, value any) Logger
    { /* implementation */ }
```

**2. Add tests**:

```go
// File: db/v1/custom_logger_test.go
func TestCustomLogger_Info(t *testing.T) {
    logger := &CustomLogger{}
    // Test that logs are written correctly
}
```

**3. Verify**:

```bash
go test -tags=test db/v1/... && make coverage
```

### Workflow 3: Fixing a Bug in SQL Generation

**1. Write failing test**:

```go
func TestBug_SelectWithJsonOperator(t *testing.T) {
    // Arrange
    builder := NewSelectBuilder(postgresDialect)
    // ...

    // Act
    sql, _ := builder.Build()

    // Assert - this should pass but currently fails
    assert.Contains(t, sql, "@>")  // Should use PostgreSQL's JSON operator
}
```

**2. Run test to confirm failure**:

```bash
go test -run TestBug_ ./...
# FAIL
```

**3. Fix implementation**:

```go
// Identify which file contains the bug
// Fix SQL generation in the builder
// Or operator rendering in the dialect
```

**4. Verify test passes**:

```bash
go test -run TestBug_ ./...
# PASS
```

**5. Run full test suite to catch regressions**:

```bash
go test ./... && make coverage
```

---

## Decision Trees for Modifications

### Decision: Should I modify builder.go or dialect.go

```text
Does the change affect SQL generation logic?
    ├─ YES: How to render SQL components?
    │   ├─ Database-specific formatting (quoting, keywords)
    │   │   └─→ Modify dialect.go ✓
    │   │
    │   └─ Query structure (SELECT, WHERE, ORDER BY)
    │       └─→ Modify builder.go ✓
    │
    └─ NO: Something else
        ├─ Accumulating query components
        │   └─→ Modify fluentDB.go ✓
        │
        └─ User-facing API
            └─→ Modify db/v1/db.go ✓
```

### Decision: Unit test or integration test

```text
Does the test require a real database?
    ├─ NO: Testing logic, interfaces, mocking
    │   └─→ Unit test (db/v1/*_test.go) ✓
    │       (Fast, no Docker)
    │
    └─ YES: Testing actual database interaction
        ├─ Testing single database
        │   └─→ Dialect-specific test ✓
        │       (internal/pkg/sqldialect/*_test.go)
        │
        └─ Testing all 4 databases
            └─→ Integration test ✓
                (tests/integration_test.go)
                (Requires Docker)
```

### Decision: Add to which builder

```text
Does your feature affect:

    ├─ SelectBuilder (SELECT queries)
    │   └─→ Modify internal/pkg/builder/select.go ✓
    │
    ├─ InsertBuilder (INSERT queries)
    │   └─→ Modify internal/pkg/builder/insert.go ✓
    │
    ├─ UpdateBuilder (UPDATE queries)
    │   └─→ Modify internal/pkg/builder/update.go ✓
    │
    ├─ DeleteBuilder (DELETE queries)
    │   └─→ Modify internal/pkg/builder/delete.go ✓
    │
    └─ All builders?
        ├─ Add to QueryBuilder interface ✓
        └─ Implement in all 4 dialect builders ✓
```

---

## Extension Architecture

### Pattern 1: Adding a Custom Database Driver

**Scenario**: You need to support a custom or proprietary database.

**Steps**:

1. **Define Plugin Factory**:

   ```go
   type CustomSQLDriver struct {}

   func (c *CustomSQLDriver) Name() string {
       return "customsql"
   }

   func (c *CustomSQLDriver) Create(ctx context.Context, cfg any) (any, error) {
       // Parse config and return driver
   }
   ```

2. **Register at Init**:

   ```go
   func init() {
       fabric.RegisterDriver(&CustomSQLDriver{})
   }
   ```

3. **Use in Application**:

   ```go
   db, _ := v1.NewDB(customConfig, logger)
   ```

**Why it works**: Plugin registry checked before built-in drivers;
no Fabric code modification needed.

### Pattern 2: Extending the Dialect System

**Scenario**: You need to support a new SQL dialect or customize
existing renderer logic.

**Steps**:

1. **Implement Dialect Interface**:

   ```go
   type CustomDialect struct {}

   func (d *CustomDialect) QuoteIdentifier(name string) string {
       return "⟨" + name + "⟩"  // Custom quoting
   }

   func (d *CustomDialect) RenderOperator(op string) string {
       // Custom operator rendering
   }

   func (d *CustomDialect) SupportsFeature(feature string) bool {
       // Feature support matrix
   }
   ```

2. **Create Corresponding Builder**:

   ```go
   type CustomBuilder struct {
       dialect CustomDialect
       // ... builder state
   }

   func (b *CustomBuilder) Build() (string, []any, error) {
       // Use dialect for SQL rendering
   }
   ```

3. **Register with Driver Factory**:
   - Include builder in custom driver factory
   - Builder selected based on dialect type

### Pattern 3: Custom Logger Implementation

**Scenario**: You want to integrate a logging system not supported by default adapters.

**Steps**:

1. **Implement Logger Interface**:

   ```go
   type CustomLogger struct {
       underlying *YourLogger
   }

   func (l *CustomLogger) Info(msg string, keyvals ...any) {
       // Delegate to your logger
   }

   func (l *CustomLogger) With(key string, value any) Logger {
       return &CustomLogger{
           underlying: l.underlying.WithField(key, value),
       }
   }
   ```

2. **Initialize and Pass to Fabric**:

   ```go
   customLogger := &CustomLogger{underlying: myLogger}
   db, _ := v1.NewDB(cfg, customLogger)
   ```

### Pattern 4: Adding Custom Query Operators

**Scenario**: You need to use database-specific operators
(e.g., PostgreSQL `~` for regex).

**Steps**:

1. **Define Operator Constant**:

   ```go
   const RegexMatch = "~"
   ```

2. **Use in Condition DSL**:

   ```go
   cdt.NewExpr().Column("email").Op("~").Value(`^[a-z]+@example\.com$`)
   ```

3. **Dialect Handles Rendering**:
   - Dialect's `RenderOperator()` method handles database-specific syntax
   - Other dialects can ignore unsupported operators or throw error

### Pattern 5: Custom Row Scanning Logic

**Scenario**: You need special handling for result row mapping (e.g., custom types).

**Steps**:

1. **Extend Row Interface** (if needed):

   ```go
   type CustomRow struct {
       underlying Row
       // Custom fields
   }

   func (r *CustomRow) Scan(dest ...any) error {
       // Custom pre/post-processing
   }
   ```

2. **Use in Application Logic**:

   ```go
   rows, _ := db.Query(ctx, sqlString, args...)
   for _, row := range rows {
       var value SomeCustomType
       row.Scan(&value)  // Calls your custom logic
   }
   ```

### Pattern 6: Transaction Wrapper for Business Logic

**Scenario**: You want to wrap all transactions with audit logging or metrics.

**Steps**:

1. **Create Wrapper Function**:

   ```go
   func WithAuditedTransaction(ctx context.Context, db DB,
       user string, fn func(Tx) error) error {

       return db.WithTransaction(ctx, func(tx Tx) error {
           log.Info("transaction started", "user", user)
           err := fn(tx)
           if err == nil {
               log.Info("transaction committed", "user", user)
           } else {
               log.Info("transaction rolled back", "user", user, "error", err)
           }
           return err
       })
   }
   ```

2. **Use in Application**:

   ```go
   WithAuditedTransaction(ctx, db, "alice", func(tx Tx) error {
       // Transaction logic
   })
   ```

---

## Dependency Landscape

### Direct Dependencies

| Package                   | Version | Purpose           | Usage           |
| ------------------------- | ------- | ----------------- | --------------- |
| **go-sql-driver/mysql**   | Latest  | MySQL driver      | MySQL support   |
| **jackc/pgx**             | Latest  | PostgreSQL driver | PostgreSQL pool |
| **mattn/go-sqlite3**      | Latest  | SQLite driver     | SQLite file-db  |
| **denisenkom/go-mssqldb** | Latest  | MSSQL driver      | MSSQL support   |

### Optional Dependencies

| Package             | Purpose       | When to use    |
| ------------------- | ------------- | -------------- |
| **sirupsen/logrus** | Log framework | Logrus adapter |
| **uber-go/zap**     | Log framework | Zap adapter    |
| **apex/log**        | Log framework | Apex adapter   |

### No Lock-In

- Core library has **zero external dependencies**
- Adapters depend on respective logging frameworks (optional)
- Easy to add custom implementations

---

## Troubleshooting Guide

### Issue: "Connection Refused" Errors

**Symptoms**: Frequent `connection refused` errors

**Causes**:

1. Database server not running
2. Connection string incorrect (wrong host/port)
3. Network firewall blocking port
4. Connection pool exhausted

**Solutions**:

```bash
# Check database is running
nc -zv localhost 5432          # PostgreSQL
nc -zv localhost 3306          # MySQL
nc -zv localhost 1433          # MSSQL

# Verify connection string
# PostgreSQL: "postgresql://user:password@localhost:5432/dbname"
# MySQL: "user:password@tcp(localhost:3306)/dbname"

# Check connection pool settings
db.SetMaxOpenConns(25)         # Increase if pool exhausted
db.SetConnMaxLifetime(15*time.Minute)  # Rotate connections

# Monitor pool stats
stats := db.Stats()
log.Printf("Pool: Open=%d, InUse=%d, Idle=%d",
    stats.OpenConnections,
    stats.InUse,
    stats.Idle,
)
```

### Issue: "Query Timeout" Errors

**Symptoms**: Queries hang or timeout

**Causes**:

1. Context timeout too short
2. Database query is slow (N+1 queries, missing indexes)
3. Connection pool exhausted (holding connections)

**Solutions**:

```go
// Increase context timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
rows, err := db.Get(ctx, "SELECT ...", args)

// Debug slow query
logger := v1.NewSlogAdapter(slog.Default())
// Logger will show query duration

// Check for N+1 queries
// Use IN instead of loop:
rows, _ := v1.NewFluentDB(db).
    Select("users", "id", "name").
    Where(cdt.In("id", userIDs)).  // Batch, not loop
    Get(ctx)

// Enable query logging to see slow queries
```

### Issue: "Duplicate Key" Errors on Insert

**Symptoms**: `UNIQUE_VIOLATION` or `DUPLICATE_KEY` error

**Causes**:

1. Record with same unique key already exists
2. Natural primary key conflict

**Solutions**:

```go
const (
    // Check if unique constraint was violated
    uniqueViolation = "UNIQUE_VIOLATION"
)

if err := db.Insert(ctx, "users", userData); err != nil {
    if isDatabaseError(err, uniqueViolation) {
        // Handle: duplicate email
        // Option 1: Update instead
        db.Update(ctx, "users", userData,
            cdt.NewExpr().Column("email").Op("=").Value(userData["email"]))

        // Option 2: Use UPSERT (if database supports)
        // Option 3: Return error to user
        return fmt.Errorf("user email already exists")
    }
}
```

### Issue: "Row Not Found" on GetByID

**Symptoms**: Expected to get a result, got empty instead

**Causes**:

1. Record doesn't exist
2. Query condition is wrong
3. Context cancelled before result returned

**Solutions**:

```go
// Check if result is empty
rows, err := v1.NewFluentDB(db).
    Select("users", "id", "name").
    Where(cdt.NewExpr().Column("id").Op("=").Value(userID)).
    Get(ctx)

if err != nil {
    // Database error
    return err
}

if len(rows) == 0 {
    // Record not found
    return fmt.Errorf("user %d not found", userID)
}

// Process result
row := rows[0]
name := row["name"].(string)
```

### Issue: "Invalid Connection String" on NewDB

**Symptoms**: Error immediately on `NewDB()` call

**Causes**:

1. Connection string format incorrect
2. Required config fields missing

**Solutions**:

```go
// Correct format for each database

// PostgreSQL
cfg := v1.PostgresConfig{
    Host:     "localhost",
    Port:     5432,
    User:     "postgres",
    Password: "password",
    DBName:   "myapp",      // Required
}

// MySQL
cfg := v1.MySQLConfig{
    Host:     "localhost",
    Port:     3306,
    User:     "root",
    Password: "password",
    DBName:   "myapp",      // Required
}

// SQLite (simpler)
cfg := v1.SQLiteConfig{
    Path: "/data/app.db",   // File path or ":memory:"
}

// Verify config
db, err := v1.NewDB(cfg, logger)
if err != nil {
    log.Fatalf("invalid config: %v", err)
}
```

### Issue: "Pool Stats Show High In-Use Count"

**Symptoms**: Pool always shows connections in-use

**Causes**:

1. Queries are slow
2. Connections not being returned (context never cancelled)
3. Locks held on resources

**Solutions**:

```go
// Monitor pool health
go func() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        stats := db.Stats()
        utilization := float64(stats.InUse) / float64(stats.OpenConnections)

        if utilization > 0.9 {
            log.Warnf("High pool utilization: %0.1f%%", utilization*100)
        }
    }
}()

// Ensure contexts are cancelled
ctx, cancel := context.WithContext(context.Background())
defer cancel()  // Must call to return connection

rows, _ := db.Get(ctx, "SELECT ...", args)  // Conn returned after Get()
```

---

## Future Extensibility Roadmap

### Short-term (v1.x)

- [ ] Connection pool metrics export (Prometheus)
- [ ] More logger adapters (structured logging)
- [ ] Query result pagination utilities
- [ ] Bulk insert optimization per database
- [ ] Transaction save point support

### Medium-term (v2.0)

- [ ] Query builder caching (prepare frequently-used queries)
- [ ] Async query execution (channels/streams)
- [ ] Connection circuit breaker
- [ ] Built-in connection retry logic
- [ ] Query result caching layer

### Long-term (v3.0+)

- [ ] Query optimization hints
- [ ] Distributed transaction support (2PC)
- [ ] Read replicas support
- [ ] Sharding/partitioning utilities
- [ ] GraphQL query builder integration
- [ ] Machine learning query performance predictions

---

## API Surface Reference

### Main Entry Points

```go
// Database initialization
db, err := v1.NewDB(config, logger)
defer db.Close()

// Query builder entry point
builder := v1.NewFluentDB(db, context.Background())

// Create logger adapter
logger := v1.NewSlogAdapter(slog.Default())
logger := v1.NewLogrusAdapter(logrusInstance)
logger := v1.NewZapAdapter(zapLogger)
logger := v1.NewApexAdapter(apexLogger)
```

### DB Interface Methods

```go
type DB interface {
    // CRUD operations
    Get(ctx context.Context, query string, args ...any) ([]Row, error)
    GetByID(ctx context.Context, table string, id any) (Row, error)
    Insert(ctx context.Context, table string, data map[string]any) (Row, error)
    Inserts(ctx context.Context, table string, dataSlice []map[string]any)
     ([]Row, error)
    Update(ctx context.Context, table string, data map[string]any, conditions ...Condition)
     (int64, error)
    Delete(ctx context.Context, table string, conditions ...Condition) (int64, error)

    // Raw queries
    Query(ctx context.Context, query string, args ...any) (sql.Rows, error)
    Exec(ctx context.Context, query string, args ...any) (sql.Result, error)

    // Transaction management
    Begin(ctx context.Context) (Tx, error)
    WithTransaction(ctx context.Context, fn func(Tx) error) error

    // Connection management
    Ping(ctx context.Context) error
    Close() error
    PoolStats() PoolStats
}
```

### Query Builder Methods

```go
type FluentDB struct {
    // SELECT builder
    Select(table string, columns ...string) *SelectBuilder

    // INSERT builder
    Insert(table string, data map[string]any) *InsertBuilder
    Inserts(table string, dataSlice []map[string]any) *InsertBuilder

    // UPDATE builder
    Update(table string, data map[string]any) *UpdateBuilder

    // DELETE builder
    Delete(table string) *DeleteBuilder
}

type SelectBuilder struct {
    Where(conditions ...Condition) *SelectBuilder
    OrderBy(columns ...string) *SelectBuilder
    GroupBy(columns ...string) *SelectBuilder
    Having(condition Condition) *SelectBuilder
    Limit(limit int) *SelectBuilder
    Offset(offset int) *SelectBuilder
    Distinct() *SelectBuilder
    Execute() ([]Row, error)
}

type InsertBuilder struct {
    Execute() (Row, error)
    ExecuteMany() ([]Row, error)
}

type UpdateBuilder struct {
    Where(conditions ...Condition) *UpdateBuilder
    Execute() (int64, error)
}

type DeleteBuilder struct {
    Where(conditions ...Condition) *DeleteBuilder
    Execute() (int64, error)
}
```

### Condition DSL

```go
// Expression
cdt.NewExpr().Column("age").Op(">").Value(18)

// Logical operators
cdt.NewAnd().Conditions(cond1, cond2, ...)
cdt.NewOr().Conditions(cond1, cond2, ...)
cdt.NewNot().Condition(cond)

// List operations
cdt.NewExpr().Column("status").Op("IN").Value("active", "pending", ...)
cdt.NewExpr().Column("category").Op("NOT IN").Value("deleted", "archived", ...)

// Range
cdt.NewExpr().Column("price").Op("BETWEEN").Value(10.0).Value(100.0)

// NULL checks
cdt.NewExpr().Column("deleted_at").Op("IS NULL")
cdt.NewExpr().Column("email").Op("IS NOT NULL")

// Pattern matching
cdt.NewExpr().Column("email").Op("LIKE").Value("%@example.com")
```

---

## Final Notes for Agents

### Key Mental Models

1. **Layered separation**: Changes rarely affect multiple layers
2. **Dialect abstraction**: Adding SQL features means minimal dialect work
3. **Interface-based**: All public types enable testing and mocking
4. **Type safety through builders**: SQL is generated, not concatenated
5. **Pluggable everything**: Logger, drivers, dialects all extensible

### When to Ask for Help

- **Architectural questions**: Does this change break the layer separation?
- **Security concerns**: Could this enable SQL injection?
- **Performance issues**: Is there a more efficient approach?
- **Breaking changes**: Will this require user code changes?

### Quick Reference: File Purpose

| File                       | Purpose               | When to edit                      |
| -------------------------- | --------------------- | --------------------------------- |
| `db/v1/`                   | Public API            | User-facing changes, new adapters |
| `internal/pkg/builder/`    | SQL generation        | Query structure changes           |
| `internal/pkg/sqldialect/` | DB-specific rendering | Quoting, operators, keywords      |
| `pkg/query/`               | Query DSL             | Condition/option expressions      |
| `tests/`                   | Integration tests     | Real database validation          |

Good luck maintaining Fabric!

```text

This comprehensive architecture document is ready to replace the current one.
It provides:

- **Agent-friendly structure** with clear sections and decision trees
- **Complete technical depth** on all major components
- **Code examples** from actual patterns in the codebase
- **Practical workflows** for common modifications
- **Troubleshooting guide** for real-world issues
- **Security, performance, and testing** deep dives
```
