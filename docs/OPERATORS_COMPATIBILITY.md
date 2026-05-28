# Operators Compatibility

This guide describes the condition helpers and operators Vessel supports across
MySQL, PostgreSQL, SQLite, and MSSQL.

## Quick Reference

| Feature | Helper or operator | MySQL | PostgreSQL | SQLite | MSSQL |
| --- | --- | --- | --- | --- | --- |
| Equality | `Op("=")` | Yes | Yes | Yes | Yes |
| Inequality | `Op("!=")`, `Op("<>")` | Yes | Yes | Yes | Yes |
| Comparisons | `<`, `>`, `<=`, `>=` | Yes | Yes | Yes | Yes |
| Logical AND | `cdt.NewAnd` | Yes | Yes | Yes | Yes |
| Logical OR | `cdt.NewOr` | Yes | Yes | Yes | Yes |
| Logical NOT | `cdt.NewNot` | Yes | Yes | Yes | Yes |
| LIKE | `cdt.Like`, `Op("LIKE")` | Yes | Yes | Yes | Yes |
| NOT LIKE | `cdt.NotLike`, `Op("NOT LIKE")` | Yes | Yes | Yes | Yes |
| Case-insensitive LIKE | `cdt.ILike` | Yes | Yes | Yes | Yes |
| BETWEEN | `cdt.NewBetween` | Yes | Yes | Yes | Yes |
| IN | `cdt.In`, `NewIn` | Yes | Yes | Yes | Yes |
| NOT IN | `cdt.NotIn` | Yes | Yes | Yes | Yes |
| IS NULL | `cdt.IsNull`, `Op("IS NULL")` | Yes | Yes | Yes | Yes |
| IS NOT NULL | `cdt.IsNotNull`, `Op("IS NOT NULL")` | Yes | Yes | Yes | Yes |

Values are passed as driver parameters. Column names are rendered by the
dialect when the condition is used through Vessel's builders.

## Basic Expressions

```go
active := cdt.NewExpr().Column("status").Op("=").Value("active")
adult := cdt.NewExpr().Column("age").Op(">=").Value(18)
```

Generated placeholders depend on the dialect:

- MySQL, SQLite, MSSQL: `?`
- PostgreSQL: `$1`, `$2`, ...

## Logical Conditions

Use `And`, `Or`, and `Not` to compose conditions:

```go
cond := cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("active").Op("=").Value(true),
    cdt.NewOr().Conditions(
        cdt.NewExpr().Column("role").Op("=").Value("admin"),
        cdt.NewExpr().Column("role").Op("=").Value("moderator"),
    ),
    cdt.NewNot().Condition(
        cdt.NewExpr().Column("banned").Op("=").Value(true),
    ),
)
```

The builder adds parentheses where needed to preserve operator precedence.

## Pattern Matching

```go
email := cdt.Like("email", "%@example.com")
notSpam := cdt.NotLike("email", "%@spam.example")
search := cdt.ILike("name", "%alice%")
```

`ILike` is intentionally portable: Vessel renders it as
`LOWER(column) LIKE LOWER(?)` for the built-in dialects. It does not use
PostgreSQL's native `ILIKE`, so the same condition can be used across
databases.

Plain `LIKE` case-sensitivity still depends on the database collation and
configuration.

## Ranges

Use `NewBetween` for range filters:

```go
price := cdt.NewBetween().Column("price").From(10).To(100)

created := cdt.NewBetween().
    Column("created_at").
    From(startTime).
    To(endTime)
```

`BETWEEN` is inclusive on both bounds.

## Sets

```go
status := cdt.In("status", "active", "pending", "review")
excluded := cdt.NotIn("category", "restricted", "discontinued")
```

`In` and `NotIn` require at least one value. For empty application slices,
decide at the application layer whether the condition should match nothing,
skip the filter, or return an error.

## NULL Checks

```go
visible := cdt.IsNull("deleted_at")
verified := cdt.IsNotNull("verified_at")
```

Do not use `Op("=").Value(nil)` for NULL checks. In SQL, `column = NULL` does
not behave like normal equality.

## Query Example

```go
cond := cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("status").Op("=").Value("active"),
    cdt.In("role", "user", "moderator"),
    cdt.NewExpr().Column("created_at").Op(">").Value(thirtyDaysAgo),
    cdt.IsNotNull("email_verified"),
)

rows, err := database.GetRaw(
    ctx,
    "users",
    []string{"id", "email"},
    nil,
    cond,
    nil,
)
if err != nil {
    return err
}

users, err := db.ScanRowsTo[User](ctx, rows)
```

## UPDATE And DELETE

The same conditions work for mutation filters. The `DB` methods take joins
before the condition:

```go
_, err := database.Update(
    ctx,
    "users",
    map[string]any{"verified": true},
    nil,
    cdt.Like("email", "%@example.com"),
    nil,
)
```

```go
_, err := database.Delete(
    ctx,
    "sessions",
    nil,
    cdt.NewExpr().Column("expires_at").Op("<").Value(time.Now()),
    nil,
)
```

Fluent builders require `Where` for filtered mutation execution:

```go
_, err := db.NewFluentDB(database).
    Delete().
    From("sessions").
    Where(cdt.NewExpr().Column("expires_at").Op("<").Value(time.Now())).
    Exec(ctx)
```

## Raw SQL Boundary

The condition DSL does not model every vendor feature. Use raw SQL for database
specific syntax such as PostgreSQL arrays, `ANY`, `DISTINCT ON`, recursive CTEs,
or vendor-specific functions.

Raw SQL APIs are caller-owned and should only receive trusted or allowlisted
SQL:

- `DB.QueryRaw`
- `DB.Exec`
- `ColumnRaw`
- `ColumnRawAs`
- `HavingRaw`

## See Also

- [FLUENTDB.md](./FLUENTDB.md)
- [PORTABILITY_MATRIX.md](./PORTABILITY_MATRIX.md)
- [ERROR_HANDLING.md](./ERROR_HANDLING.md)
