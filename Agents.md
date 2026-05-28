# Agents.md - Vessel Database Library

**Project:** Vessel - Multi-Database SQL Abstraction Library  
**Language:** Go 1.26.0+  
**Last Updated:** March 15, 2026

---

## Overview

This file documents agent instructions and workflows for working with the
Vessel database abstraction library. Use this guide to understand how to
approach common tasks, maintain code quality, and contribute to the project.

---

## Agent Purpose

**Role:** Database Library Developer  
**Responsibility:** Build and maintain a clean, maintainable database
abstraction layer that handles SQL generation and query building without
overcomplicating. Keep the API fluent and intuitive, support multiple
database dialects, and ensure all code is well-tested.

**Key Focus Areas:**

- Implementing fluent query builders (SELECT, INSERT, UPDATE, DELETE)
- Supporting multiple SQL dialects (MySQL, PostgreSQL, SQLite, MSSQL)
- Generating correct SQL for each database type
- Writing comprehensive unit and integration tests
- Maintaining clean interfaces and extensible architecture
- Optimizing performance in hot paths
- Optimizing for maintainability and readability
- Keeping lint output clean before merging changes
- Documenting all features and changes clearly
- Handling errors gracefully and consistently
- Providing a seamless developer experience with type safety and clear APIs
- Ensuring plugin architecture is robust and easy to extend

**What You Don't Need to Handle:**

- Frontend or backend business logic
- Application-level API design
- Server infrastructure
- Driver installation or system setup

---

## Project Structure

```text
vessel/
├── db/v1/                           # Public API (version 1)
│   ├── db.go                        # Core DB/DBActions/Tx interfaces
│   ├── fluentDB.go                  # Fluent query builder
│   │                                 # (SelectBuilder, UpdateBuilder, etc.)
│   ├── config_*.go                  # Database configs
│   │                                 # (MySQL, PostgreSQL, SQLite, MSSQL)
│   ├── row_adapter.go               # Row scanning abstraction
│   ├── logger.go                    # Logging interface
│   └── *_test.go                    # Unit tests (80+ tests)
│
├── internal/pkg/                    # Internal implementation packages
│   ├── builder/                     # SQL query builder (internal)
│   │   ├── builder.go               # Builder interface
│   │   ├── mysql.go, postgres.go    # Dialect implementations
│   │   └── *_builder_test.go        # Builder tests (per-dialect)
│   │
│   ├── sqldialect/                  # SQL dialect abstractions
│   │   ├── sql_dialect.go           # Shared dialect logic
│   │   ├── mysql.go, postgres.go    # Dialect-specific implementations
│   │   └── sql_dialect_test.go      # Dialect tests (per-dialect)
│   │
│   ├── operator/                    # Operator definitions
│   ├── helpers/                     # Utility functions
│   └── otel/                        # OpenTelemetry integration
│
├── pkg/query/                       # Public query DSL packages
│   ├── condition/                   # Condition DSL (Expr, And, Or, In)
│   ├── options/                     # QueryOptions (OrderBy, Limit)
│   └── definition/                  # Constants
│
├── manager/v1/                      # Query manager (higher-level API)
├── tests/                           # Integration tests
├── examples/                        # Usage examples
└── docs/                            # Documentation
```

---

## Agent Guidelines by Task

### 1. **Adding New Database Support**

**Scope:** MySQL, PostgreSQL, SQLite, MSSQL only (established pattern)

**Steps:**

1. Create driver config in `db/v1/{dialect}.go`
2. Implement `SQLDialect` interface in
   `internal/pkg/sqldialect/{dialect}.go`
3. Implement `QueryBuilder` interface in
   `internal/pkg/builder/{dialect}.go`
4. Add unit tests: `{dialect}_test.go`,
   `{dialect}_builder_test.go`
5. Add integration tests in `tests/integration_test.go`
6. Update docs in `README.md` and
   `OPERATORS_COMPATIBILITY.md`

**Key Files to Modify:**

- `db/v1/{dialect}.go` - Driver configuration
- `internal/pkg/sqldialect/{dialect}.go` - SQL generation
- `internal/pkg/builder/{dialect}.go` - Query building

**Testing Requirements:**

- ✅ Unit tests must pass (use `-tags=test` flag)
- ✅ Integration tests required for real database
- ✅ All tests pass in CI/CD

---

### 2. **Bug Fixes**

**Process:**

1. Identify the affected module (db/v1, builder, sqldialect, etc.)
2. Write a failing test first (TDD approach)
3. Fix the bug in the implementation
4. Ensure all related tests pass
5. Run full test suite: `go test -tags=test ./...`
6. Update relevant documentation if behavior changed

**Key Test Files:**

- `db/v1/fluentDB_test.go` - Builder API tests
- `internal/pkg/builder/builder_test.go` - Builder tests
- `internal/pkg/sqldialect/sql_dialect_test.go` - SQL
  generation tests

**Important:** Never commit without running full test suite
and linter

---

### 3. **Adding New Query Features**

#### Example: Adding LIMIT/OFFSET support

**Process:**

1. **Define:** Add feature to `QueryOptions` struct in
   `pkg/query/options/options.go`
2. **Builder:** Add builder method to `SelectBuilder`,
   `UpdateBuilder`, `DeleteBuilder` in `db/v1/fluentDB.go`
3. **SQL Generation:** Update SQL generation in
   `internal/pkg/sqldialect/sql_dialect.go`
4. **Per-Dialect:** Update each dialect if special handling is
   needed (e.g., MSSQL OFFSET/FETCH syntax)
5. **Test:** Add comprehensive tests to `fluentDB_test.go` and `sql_dialect_test.go`
6. **Document:** Update `README.md` and migration guide

**Key Pattern:**

```go
// 1. Add to options
type QueryOptions struct {
    NewFeature  TypeHere
}

// 2. Add builder method
func (sb *SelectBuilder) NewFeature(value TypeHere) *SelectBuilder {
    sb.opts.NewFeature = value
    return sb
}

// 3. Update SQL generation
// In sql_dialect.go supportedOptions()
if opts.NewFeature != nil {
    // Generate SQL fragment
}

// 4. Test across dialects
// In sql_dialect_test.go
```

---

### 4. **Fixing SQL Generation Issues**

**Location:** `internal/pkg/sqldialect/sql_dialect.go`

**Process:**

1. Understand the `supportedOptions()` function (handles LIMIT,
   OFFSET, ORDER BY, etc.)
2. Check dialect-specific implementations in `{dialect}.go`
3. Verify `QuoteIdentifier()` is used for all column/table names
4. Test with all 4 dialects (MySQL, PostgreSQL, SQLite, MSSQL)
5. Pay special attention to:
   - Identifier quoting (backticks vs quotes vs brackets)
   - OFFSET/FETCH syntax (MSSQL-specific)
   - NULL handling in HAVING clauses

**Common Issues:**

- ✗ Not quoting identifiers → SQL injection risks
- ✗ Wrong operator syntax → Database errors
- ✗ Dialect-specific syntax → MSSQL OFFSET/FETCH instead of LIMIT/OFFSET

**Test Pattern:**

```go
testCases := []struct {
    name       string
    opts       *options.QueryOptions
    expectSQL  string
}{
    {
        name: "description",
        opts: &options.QueryOptions{...},
        expectSQL: "SELECT ... expected SQL",
    },
}
```

---

### 5. **Performance Improvements**

**Profile Before:** Use `go test -bench=.` if benchmarks exist

**Areas to Focus:**

1. **Row Scanning** (`db/v1/row_adapter.go`) - Hot path for data retrieval
2. **SQL Building** (`internal/pkg/builder/`) - Path for every query
3. **Connection Pooling** - Database pooling configuration

**Important Constraints:**

- No breaking changes to public API
- All tests must pass
- Benchmarks must show improvement or equal performance

---

### 6. **Documentation Updates**

**Documentation Files:**

| File                              | Purpose                    |
| --------------------------------- | -------------------------- |
| `README.md`                       | Feature overview and quick |
| `CHANGELOG.md`                    | Version history            |
| `docs/ARCHITECTURE.md`            | Current architecture docs  |
| `docs/ERROR_HANDLING.md`          | Error handling             |
| `docs/OPERATORS_COMPATIBILITY.md` | Operator support           |
| `docs/SQL_NULL_TYPES.md`          | Null type handling         |
| `CONTRIBUTING.md`                 | Contributing guide         |

**When to Update Documentation:**

- ✅ New features added
- ✅ API changes made
- ✅ Bug fixes that change behavior
- ✅ Example code updated
- ✅ Performance improvements documented

**Format Requirements:**

- Use GitHub-flavored markdown
- Code examples must be correct and tested
- Cross-reference related documentation
- Keep examples up-to-date with code

---

### 7. **Testing Requirements**

**Test Execution:**

```bash
# Unit tests only
go test -tags=test ./db/v1 ./internal/pkg/builder ./internal/pkg/sqldialect

# With coverage
go test -tags=test -cover ./...

# Integration tests (requires Docker/databases)
go test -tags=test ./tests

# Linting
make lint

# All quality checks
make test
```

**Test Coverage Expectations:**

- ✅ New code: Minimum 80% coverage
- ✅ Bug fixes: New tests for the bug
- ✅ Features: Integration tests for real databases
- ✅ All dialects tested (MySQL, PostgreSQL, SQLite, MSSQL)

**Test Naming Convention:**

```go
func Test{Feature}_{Scenario}(t *testing.T) {
    // Example: TestSelectBuilderOrderBy
}

func TestSelectBuilderOrderBy_SingleColumn(t *testing.T) {
    // Specific scenario
}
```

---

### 8. **Code Quality Standards**

**Linting (40+ enabled linters):**

```bash
make lint  # Run golangci-lint
```

**Code Style:**

- ✅ Document exported APIs and behavior changes clearly
- ✅ Use interfaces for abstraction
- ✅ Proper error handling with context
- ✅ No global state (except for registry in plugin system)
- ✅ Immutable where possible

**Error Handling:**

- Use sentinel errors from `db/v1/dberror/errors.go`
- Wrap errors with context using `fmt.Errorf`
- Map database-specific errors to Vessel errors
- Never ignore errors silently

**Example:**

```go
if err != nil {
    return fmt.Errorf("failed to scan row: %w", err)
}
```

---

### 9. **Working with Builders**

**Fluent API Pattern** (in `db/v1/fluentDB.go`):

```go
// SelectBuilder example
func (sb *SelectBuilder) OrderBy(column, direction string) *SelectBuilder {
    // Implementation
    return sb  // Return for chaining
}

// Usage
rows, err := fdb.Select("users", "id", "name").
    OrderBy("name", "ASC").
    Limit(10).
    Get()
```

**Builder Characteristics:**

- ✅ Methods return `*Builder` for chaining
- ✅ Order of method calls doesn't matter
- ✅ Each method modifies builder state
- ✅ Terminal methods (Get, Exec) execute the query

**Key Builders:**

- `SelectBuilder` - For SELECT queries
- `UpdateBuilder` - For UPDATE queries
- `DeleteBuilder` - For DELETE queries
- `InsertBuilder` - For INSERT queries

---

### 10. **Database Configuration**

**Config Types** (in `db/v1/config_*.go`):

- `MysqlConfig` - MySQL connection config
- `PostgresConfig` - PostgreSQL connection config
- `SQLiteConfig` - SQLite connection config
- `MSSQLConfig` - MSSQL connection config

**Creation Pattern:**

```go
db, err := db.NewDB(db.MysqlConfig{
    User:     "user",
    Password: "password",
    Host:     "localhost",
    Port:     3306,
    Database: "mydb",
}, logger)
```

**Important:**

- Each config type validates settings
- Connection pooling configured per database
- OpenTelemetry can be disabled via `OTEL_ENABLED` env var

---

## Common Workflows

### Workflow: Adding a New Operator

1. Define operator in `internal/pkg/operator/operator.go`
2. Map operator to SQL syntax in `internal/pkg/sqldialect/{dialect}.go`
3. Add to condition DSL in `pkg/query/condition/`
4. Test in `condition_test.go`
5. Update `OPERATORS_COMPATIBILITY.md`

### Workflow: Fixing a Dialect Bug

1. Find the test case in `internal/pkg/sqldialect/sql_dialect_test.go`
2. Add test for specific dialect
3. Fix implementation in `internal/pkg/sqldialect/{dialect}.go`
4. Verify with integration tests
5. Update docs if behavior changed

### Workflow: Adding Transaction Feature

1. Add method to `Tx` interface in `db/v1/db.go`
2. Implement in each dialect's transaction handler
3. Add tests in `db/v1/db_test.go`
4. Add integration tests in `tests/integration_test.go`
5. Document in `README.md` transactions section

---

## Testing Best Practices

### Unit Tests

- Test single feature in isolation
- Use table-driven tests for multiple cases
- Mock external dependencies

### Integration Tests

- Use real database instances
- Clean up data after tests
- Test multi-dialect compatibility

### Test Helpers

Located in `internal/pkg/builder/test_helpers.go`:

- `intPtr()` - Helper for int pointers
- Other utilities for test setup

---

## Performance Considerations

**Critical Paths:**

1. **Row Scanning** - Optimize in `db/v1/row_adapter.go`
2. **Query Building** - Optimize in builder implementations
3. **Connection Pooling** - Monitor with `PoolStats()`

**OpenTelemetry Impact:**

- Minimal when enabled (built-in instrumentation)
- Zero overhead when disabled (`OTEL_ENABLED=false`)

---

## Release Checklist

Before releasing a new version:

- [ ] All tests passing: `go test -tags=test ./...`
- [ ] Linting clean: `make lint`
- [ ] Coverage acceptable: `go test -cover ./...`
- [ ] Documentation updated
- [ ] Changelog updated in `CHANGELOG.md`
- [ ] Example code tested and working
- [ ] Integration tests pass
- [ ] No breaking changes (or documented migration)

---

## Key Design Patterns

1. **Strategy Pattern** - SQLDialect for per-database implementations
2. **Builder Pattern** - FluentDB for readable query construction
3. **Composite Pattern** - Conditions (And, Or, Not, etc.)
4. **Adapter Pattern** - Row scanning abstraction
5. **Factory Pattern** - Plugin driver registry
6. **Bridge Pattern** - Abstraction from implementation

---

## Important Notes

### ⚠️ Breaking Changes

- Require documentation, migration guide, and changelog entry
- Should be bundled into major version release
- Example: OrderBy struct redesign → v1.1.0

### ⚠️ Code Style

- Keep exported API documentation accurate and useful
- No global state except plugin registry
- Prefer composition over inheritance

### ⚠️ Error Handling

- Always wrap errors with context
- Use sentinel errors from dberror package
- Log errors with proper context

### ⚠️ Database Support

- All new features must work on all 4 dialects
- Test on real database instances
- Handle dialect-specific syntax carefully

---

## Resources

- **README.md** - Feature overview and quick start
- **docs/ARCHITECTURE.md** - Current architecture notes
- **CONTRIBUTING.md** - Contribution guidelines
- **CHANGELOG.md** - Version history

---

## Version Info

- **Go Version:** 1.26.0+
- **License:** MIT
- **Status:** v0.x, used by the author, not broadly battle hardened
- **Test Coverage:** Unit and integration coverage across supported dialects
- **Last Updated:** March 15, 2026

---

## Summary

Vessel is a personal SQL toolkit with:

- ✅ Multi-database support (MySQL, PostgreSQL, SQLite, MSSQL)
- ✅ Type-safe fluent query builder
- ✅ Unit and integration tests
- ✅ Practical documentation
- ✅ Linting in the normal quality gate
- ✅ Production-style service APIs

When working with Vessel, prioritize:

1. Avoiding unnecessary breaking changes
2. Testing across all dialects
3. Clear documentation
4. Type safety and error handling
5. Performance without breaking API
