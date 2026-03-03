# SQL Operator Compatibility Matrix

This document defines which query operators are supported by each database dialect in db-connector.

## Operator Support by Database

| Operator            | MySQL | PostgreSQL | SQLite | MSSQL | Notes                                             |
| ------------------- | :---: | :--------: | :----: | :---: | ------------------------------------------------- |
| `eq` (=)            |  ✅   |     ✅     |   ✅   |  ✅   | Basic equality comparison                         |
| `neq` (!=, <>)      |  ✅   |     ✅     |   ✅   |  ✅   | Not equal                                         |
| `lt` (<)            |  ✅   |     ✅     |   ✅   |  ✅   | Less than                                         |
| `lte` (<=)          |  ✅   |     ✅     |   ✅   |  ✅   | Less than or equal                                |
| `gt` (>)            |  ✅   |     ✅     |   ✅   |  ✅   | Greater than                                      |
| `gte` (>=)          |  ✅   |     ✅     |   ✅   |  ✅   | Greater than or equal                             |
| `like`              |  ✅   |     ✅     |   ✅   |  ✅   | Case-sensitive pattern matching                   |
| `not like`          |  ✅   |     ✅     |   ✅   |  ✅   | Negative pattern matching                         |
| `ilike`             |  ❌   |     ✅     |   ❌   |  ❌   | Case-insensitive LIKE (PostgreSQL only)           |
| `in`                |  ✅   |     ✅     |   ✅   |  ✅   | IN set membership                                 |
| `not in`            |  ✅   |     ✅     |   ✅   |  ✅   | NOT IN set membership                             |
| `between`           |  ✅   |     ✅     |   ✅   |  ✅   | Range check                                       |
| `not between`       |  ✅   |     ✅     |   ✅   |  ✅   | Negative range check                              |
| `is null`           |  ✅   |     ✅     |   ✅   |  ✅   | NULL comparison                                   |
| `is not null`       |  ✅   |     ✅     |   ✅   |  ✅   | NOT NULL comparison                               |
| `distinct`          |  ⚠️   |     ✅     |   ⚠️   |  ⚠️   | Distinct comparison (emulated with LIKE)          |
| `not distinct`      |  ❌   |     ✅     |   ❌   |  ❌   | Null-safe not distinct (Postgres only)            |
| `is distinct from`  |  ⚠️   |     ✅     |   ⚠️   |  ⚠️   | IS DISTINCT FROM comparison                       |
| `contains` (@>)     |  ❌   |     ✅     |   ❌   |  ❌   | Array/JSONB contains (PostgreSQL only)            |
| `contained` (<@)    |  ❌   |     ✅     |   ❌   |  ❌   | Array/JSONB is contained by (PostgreSQL only)     |
| `overlaps` (&&)     |  ❌   |     ✅     |   ❌   |  ❌   | Array overlap (PostgreSQL only)                   |
| `regex` (~)         |  ✅   |     ✅     |   ❌   |  ❌   | Regular expression match                          |
| `not regex`         |  ✅   |     ✅     |   ❌   |  ❌   | Regular expression non-match                      |
| `iregex` (~\*)      |  ❌   |     ✅     |   ❌   |  ❌   | Case-insensitive regex (PostgreSQL only)          |
| `not iregex` (!~\*) |  ❌   |     ✅     |   ❌   |  ❌   | Negative case-insensitive regex (PostgreSQL only) |

## Legend

| Symbol | Meaning                                            |
| ------ | -------------------------------------------------- |
| ✅     | Fully supported with native SQL                    |
| ⚠️     | Partially supported (emulated behavior)            |
| ❌     | Not supported (will return empty or LIKE fallback) |

## Dialect-Specific Notes

### MySQL

- **Regex operators**: Uses `REGEXP` and `NOT REGEXP`
- **Case-insensitive regex**: Not available; uses `REGEXP` as fallback
- **Case-insensitive LIKE**: Not available; uses `LIKE` as fallback
- **NOT DISTINCT FROM**: Not available; not implemented (returns empty or LIKE fallback)
- **Array/JSON operators**: Not available; uses `LIKE` as fallback

### PostgreSQL

- **Full support**: All operators supported natively
- **Regex operators**: Uses Postgres-specific `~`, `!~`, `~*`, `!~*`
- **Array/JSON operators**: Supports `@>` (contains), `<@` (contained by), `&&` (overlaps)
- **NULL handling**: `IS NOT DISTINCT FROM` properly handles NULLs

### SQLite

- **Regex operators**: Not available; would require custom functions
- **No advanced types**: No native array or JSONB support (uses text only)
- **Shares MySQL dialect**: Uses MySQL operator mappings for compatibility
- **PRAGMA compilation options**: Regex might be available with `--enable-all-baked-in-functions`

### MSSQL (SQL Server)

- **Regex operators**: Not available; would require `LIKE` with wildcards or `PATINDEX()`
- **NOT DISTINCT FROM**: Not available; not implemented
- **Array/JSON**: Limited JSON support (uses `LIKE` fallback for JSON operators)
- **LIMIT/OFFSET**: Uses `OFFSET ... ROWS FETCH NEXT ... ROWS ONLY`
- **RETURNING**: Mapped to `OUTPUT` clause

## Implementation Notes

### Operator Mapping by Dialect

**MySQL/SQLite Dialect (`MySQLDialect`):**

- Operators not natively available are mapped to `LIKE` or `REGEXP` when possible
- `Contains`, `Contained`, `Overlaps` → mapped to `LIKE` (fallback)
- `Distinct` → mapped to `IS DISTINCT FROM` (unsupported in MySQL, returns literal)

**PostgreSQL Dialect (`PostgresDialect`):**

- All operators have native implementations
- Supports the full range of PostgreSQL-specific operators

**MSSQL Dialect (`MSSQLDialect`):**

- `NotDistinct`, `Contains`, `Contained`, `Overlaps` → mapped to `LIKE` (fallback)
- Regex operators not supported (returns empty string)
- Uses MSSQL-specific syntax (`OFFSET/FETCH`, `OUTPUT`)

## Using Operators Safely

### Recommendation: Check Dialect Support Before Using

```go
// Good: Check if the operator is supported
dialect := NewDatabaseDialect()

// For operators that vary by database, consider using database-agnostic
// alternatives when portability is critical

// ✅ Portable: Use standard comparison operators
builder.Where("age", ">", 18)

// ⚠️ Database-specific: Check documentation before using
// Avoid these unless targeting a specific database:
// - regex, iregex (not in SQLite/MSSQL)
// - contains, contained, overlaps (PostgreSQL only)
// - is distinct from, not distinct from (PostgreSQL only)
```

### Portability Guidelines

**For maximum portability across all databases:**

- Use: `eq`, `neq`, `lt`, `lte`, `gt`, `gte`, `like`, `in`, `between`, `is null`

**For PostgreSQL-specific queries:**

- Use: `contains`, `overlaps`, `regex`, `iregex`, `is distinct from`

**For MySQL/SQLite queries:**

- Use: `regex` (supported), `like` (case-sensitive)
- Avoid: `ilike`, `contains`, `regex` on SQLite

**For MSSQL queries:**

- Use: Standard comparison operators and `like`
- Avoid: `regex`, `contains`, `is distinct from`

## Future Enhancements

1. **Add validation**: Warn or error if using unsupported operators for a dialect
2. **Add operator aliases**: Allow portable ways to express operations (e.g., `null_safe_eq`)
3. **Add regex support**: Implement custom functions for SQLite/MSSQL regex
4. **Add JSON operators**: Extend MSSQL JSON support beyond LIKE mappings
5. **Document performance**: Add notes on operator performance per database
