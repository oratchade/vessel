# FluentDB

This guide covers Vessel's fluent query API as it exists today. It is written
for service code: every example uses the current method names and keeps raw SQL
boundaries explicit.

For dialect support, see [PORTABILITY_MATRIX.md](./PORTABILITY_MATRIX.md).

## Overview

Create a fluent wrapper with `db.NewFluentDB(database)`. The wrapper can be
backed by either a `db.DB` or a `db.Tx`.

```go
import db "tounilab.com/vessel/db/v1"

fdb := db.NewFluentDB(database)
```

Builders mutate their own builder state and return the same builder for
chaining. They execute only when a terminal method is called, such as `Get`,
`GetRaw`, `Exec`, `Upsert`, `UpdateAll`, or `DeleteAll`.

## SELECT

`Select` takes the table first and optional columns after it. If columns are
omitted, Vessel selects `*`.

```go
import cdt "tounilab.com/vessel/pkg/query/condition"

rows, err := fdb.
    Select("users", "id", "name", "email").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    OrderByAsc("name").
    Limit(50).
    Get(ctx)
```

`Get` materializes rows as `[]map[string]any`. For streaming or typed scanning,
use `GetRaw` and pass the returned `*RowsAdapter` to `ScanRowsTo`, `ScanAll`,
or `ScanOne`.

```go
type User struct {
    ID    int
    Name  string
    Email string
}

rawRows, err := fdb.
    Select("users", "id", "name", "email").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    GetRaw(ctx)
if err != nil {
    return err
}

users, err := db.ScanRowsTo[User](ctx, rawRows)
```

`ScanRowsTo` closes the rows adapter after scanning.

## Conditions

Use `pkg/query/condition` for parameterized predicates:

```go
cond := cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("active").Op("=").Value(true),
    cdt.NewOr().Conditions(
        cdt.In("status", "paid", "pending"),
        cdt.IsNull("reviewed_at"),
    ),
)
```

Values are passed as driver parameters. Do not build condition values into SQL
strings.

Useful helpers include:

- `cdt.In`, `cdt.NotIn`
- `cdt.IsNull`, `cdt.IsNotNull`
- `cdt.Like`, `cdt.NotLike`
- `cdt.ILike`, rendered portably as `LOWER(column) LIKE LOWER(?)`
- `cdt.NewBetween().Column("price").From(10).To(100)`
- `cdt.NewNot().Condition(cond)`

## Joins

Column-to-column joins are represented with `cdt.Join` and `cdt.JoinCdts`.
`JoinOn`/`InnerJoinOn`/`LeftJoinOn` accept a condition object for extra `ON`
predicates, not a raw string. Conditions that bind values are rejected by the
current join renderer.

```go
rows, err := fdb.
    Select("orders", "orders.id", "orders.total", "users.email").
    Join(cdt.Join{
        Type:  "INNER",
        Table: "users",
        Conditions: cdt.JoinCdts{
            {Left: "user_id", Right: "id"},
        },
    }).
    Where(cdt.NewExpr().Column("orders.status").Op("=").Value("paid")).
    Get(ctx)
```

When you need aliases, set the alias on the join:

```go
rows, err := fdb.
    Select("orders", "orders.id", "u.email").
    Join(cdt.Join{
        Type:  "INNER",
        Table: "users",
        Alias: "u",
        Conditions: cdt.JoinCdts{
            {Left: "user_id", Right: "id"},
        },
    }).
    Get(ctx)
```

`JoinOn` is useful for value-free extra predicates such as
`cdt.IsNull("u.deleted_at")`.

Keep join predicates simple and covered by tests. For complex vendor-specific
join syntax, use `DB.QueryRaw` at the DB layer.

## Projections, GROUP BY, and HAVING

Use `Column`, `ColumnAs`, `ColumnRaw`, and `ColumnRawAs` to add projections
after the builder is created.

```go
rows, err := fdb.
    Select("orders").
    Column("user_id").
    ColumnRawAs("COUNT(*)", "order_count").
    GroupBy("user_id").
    Having(cdt.NewExpr().Column("COUNT(*)").Op(">").Value(5)).
    Get(ctx)
```

`ColumnRaw` and `ColumnRawAs` are trusted-raw escape hatches. Only pass
allowlisted SQL fragments controlled by the application. User values should
still go through parameterized conditions.

If a HAVING clause cannot be represented by the condition DSL, use
`HavingRaw`, but treat it as an audited raw SQL boundary.

## Count Helpers

`SelectBuilder.Count(ctx)` runs a count query and returns `int64`.

```go
n, err := fdb.
    Select("users").
    Where(cdt.NewExpr().Column("active").Op("=").Value(true)).
    Count(ctx)
```

For streaming access to the count row, use `CountRaw(ctx, alias...)`. To preview
the generated count SQL, use `CountQuery(alias...)`.

## INSERT

Single-row insert:

```go
result, err := fdb.
    Insert().
    Into("users").
    Set("id", userID).
    Set("email", email).
    Set("active", true).
    Exec(ctx)
```

Bulk insert uses `ValuesBulk`:

```go
result, err := fdb.
    Insert().
    Into("users").
    ValuesBulk([]map[string]any{
        {"id": id1, "email": "a@example.com"},
        {"id": id2, "email": "b@example.com"},
    }).
    Exec(ctx)
```

## InsertAndFetch

`InsertAndFetch` inserts one row and fetches it by an application-provided key.
It returns a `map[string]any`.

```go
user, err := fdb.
    Insert().
    Into("users").
    Set("id", userID).
    Set("email", email).
    InsertAndFetch(ctx, "id", "id", "email")
```

The insert and follow-up select are separate statements unless the builder is
bound to a transaction with `WithTx`. For atomic create-and-fetch workflows, run
it inside `WithTransaction`.

## UPDATE

`Update` takes the table name. `Exec` requires a `Where` condition. Use
`UpdateAll` only when an unfiltered update is intentional.

```go
result, err := fdb.
    Update("users").
    Set("last_seen", time.Now()).
    Where(cdt.NewExpr().Column("id").Op("=").Value(userID)).
    Exec(ctx)
```

The right-hand side of `Set` is parameterized. There is no public `db.Raw`
assignment helper. If you need expressions such as `seen_count = seen_count +
1`, use `DB.Exec` or `DB.QueryRaw`/`Exec` with a reviewed raw SQL statement.

## DELETE

`Delete().Exec(ctx)` requires a `Where` condition. Use `DeleteAll` only when an
unfiltered delete is intentional.

```go
result, err := fdb.
    Delete().
    From("sessions").
    Where(cdt.NewExpr().Column("expired_at").Op("<").Value(time.Now())).
    Exec(ctx)
```

## UPSERT

Upsert is configured on `InsertBuilder` with `OnConflict` and either
`DoUpdate`, `DoUpdateSet`, or `DoNothing`. Use `ValuesBulk` with the same
conflict methods for bulk upsert.

```go
result, err := fdb.
    Insert().
    Into("users").
    Set("id", userID).
    Set("email", email).
    OnConflict("id").
    DoUpdate("email").
    Upsert(ctx)
```

```go
result, err := fdb.
    Insert().
    Into("users").
    ValuesBulk([]map[string]any{
        {"id": id1, "email": "a@example.com"},
        {"id": id2, "email": "b@example.com"},
    }).
    OnConflict("id").
    DoUpdate("email").
    Upserts(ctx)
```

Dialect behavior:

- PostgreSQL and SQLite render `ON CONFLICT`.
- MySQL renders `ON DUPLICATE KEY UPDATE`.
- MSSQL returns an explicit unsupported error.

## RETURNING / OUTPUT Preview

`Returning` is for query preview only. `InsertQuery`, `UpdateQuery`,
`DeleteQuery`, `UpsertQuery`, and `UpsertsQuery` can include PostgreSQL
`RETURNING` or MSSQL `OUTPUT` where supported by the builder. Mutation
execution methods reject `Returning` because they return `ExecResult`, not
rows.

```go
sql, args, err := fdb.
    Update("users").
    Set("active", false).
    Where(cdt.NewExpr().Column("last_seen").Op("<").Value(cutoff)).
    Returning("id", "email").
    Query()
```

If production code needs returned rows from a mutation, use a dialect-specific
raw query that your tests cover.

## Transactions

Create the fluent wrapper from the transaction or attach a transaction with
`WithTx`.

```go
err := database.WithTransaction(ctx, func(tx db.Tx) error {
    ftx := db.NewFluentDB(tx)

    if _, err := ftx.Insert().
        Into("orders").
        Set("id", orderID).
        Set("user_id", userID).
        Exec(ctx); err != nil {
        return err
    }

    _, err := ftx.Insert().
        Into("order_items").
        Set("order_id", orderID).
        Set("sku", sku).
        Exec(ctx)
    return err
})
```

`WithTransaction` commits on nil return and rolls back on error or panic. Do
not call `Commit` or `Rollback` manually inside the callback.

## Production Practices

- Put timeouts or deadlines on contexts passed to fluent terminal methods.
- Use `GetRaw` plus typed scanning for larger result sets so cleanup is explicit
  and memory use is bounded.
- Treat `ColumnRaw`, `ColumnRawAs`, `HavingRaw`, `DB.QueryRaw`, and `DB.Exec` as
  review-required raw SQL boundaries.
- Prefer `WithTransaction` for request-scoped writes. Use manual `Begin` only
  when a transaction must cross function boundaries.
- Use `Query()`/`InsertQuery()`/`UpdateQuery()`/`DeleteQuery()` in tests to lock
  down generated SQL for important queries.
- Avoid relying on unordered `Limit`. Always pair pagination with a stable
  `OrderBy`.

## See Also

- [PORTABILITY_MATRIX.md](./PORTABILITY_MATRIX.md)
- [ERROR_HANDLING.md](./ERROR_HANDLING.md)
- [RESOURCE_POOLING.md](./RESOURCE_POOLING.md)
