# Explain Query Analysis Example

This example demonstrates how to use the `Explain()` method to analyze query execution plans across multiple database backends without executing the queries or retrieving data.

## Overview

The `Explain()` method is part of the `DBQueries` interface and provides safe, non-executing query plan analysis. This is useful for:

- **Query Debugging**: Preview generated SQL and verify correctness before execution
- **Performance Analysis**: Understand how the database engine plans to execute queries
- **Index Verification**: Check if your WHERE conditions use appropriate indexes
- **Audit Logging**: Log all queries without side effects
- **SQL Injection Prevention**: Verify that all queries are properly parameterized

## How It Works

The `Explain()` method works in three steps:

1. **Generate Query**: Use `xxxQuery()` methods (GetQuery, UpdateQuery, DeleteQuery, etc.) to generate parameterized SQL
2. **Execute Explain**: Pass the query and parameters to `Explain()` to get the execution plan
3. **Read Plan**: Iterate through the plan rows to analyze the execution strategy

### Key Safety Features

- **Non-Executing**: Uses database-specific non-executing plan variants
  - PostgreSQL: `EXPLAIN` (estimated plan, no execution)
  - MySQL: `EXPLAIN` (estimated plan, no execution)
  - SQLite: `EXPLAIN QUERY PLAN` (plan only, no execution)
  - MSSQL: `SET STATISTICS SHOWPLAN_TEXT` (plan only, no execution)

- **No Data Retrieval**: Plan analysis does not return actual data rows
- **Parameterized SQL**: All queries use parameter binding to prevent SQL injection

## Example Scenarios

### 1. Simple SELECT Analysis

```go
query, args, err := database.GetQuery(
    "users",
    []string{"id", "name", "email"},
    nil,
    condition.NewExpr().Column("age").Op(">").Value(25),
    nil,
)

plan, err := database.Explain(ctx, query, args...)
defer plan.Close()

for plan.Next() {
    var line string
    plan.Scan(&line)
    log.Println(line)
}
```

### 2. Complex Multi-Condition Query

```go
cond := condition.NewOr().Conditions(
    condition.NewAnd().Conditions(
        condition.NewExpr().Column("age").Op(">").Value(25),
        condition.NewExpr().Column("city").Op("=").Value("NYC"),
    ),
    condition.NewExpr().Column("salary").Op(">=").Value(50000),
)

query, args, _ := database.GetQuery("users", []string{"id", "name", "age", "city", "salary"}, nil, cond, nil)
plan, _ := database.Explain(ctx, query, args...)
```

### 3. Bulk INSERT Analysis

```go
data := []map[string]any{
    {"id": 1, "name": "Alice", "email": "alice@example.com"},
    {"id": 2, "name": "Bob", "email": "bob@example.com"},
}

query, args, _ := database.InsertsQuery("users", data, nil)
plan, _ := database.Explain(ctx, query, args...)
```

### 4. UPDATE Query Analysis

```go
cond := condition.NewExpr().Column("years_experience").Op(">=").Value(5)
query, args, _ := database.UpdateQuery(
    "employees",
    map[string]any{"salary": 75000},
    cond,
    nil,
)
plan, _ := database.Explain(ctx, query, args...)
```

## Running the Example

```bash
# Navigate to the db-connector directory
cd db-connector

# Run the example (this outputs expected results and explanations)
go run ./examples/explain-example
```

The example outputs:
- Code snippets showing how to use `Explain()`
- Expected outputs from different database backends
- Benefits and use cases for each scenario

## Database-Specific Outputs

### PostgreSQL
```
Seq Scan on users  (cost=0.00..35.50 rows=333 width=32)
  Filter: (age > 25)
Planning Time: 0.234 ms
Execution Time: 0.451 ms
```

### MySQL
```
id  select_type  table  partitions  type   possible_keys  key   key_len  ref   rows  filtered  Extra
1   SIMPLE       users  NULL        ALL    idx_age        NULL  NULL     NULL  1000  50.00     Using where
```

### SQLite
```
SCAN users WHERE age > ?
```

### MSSQL
```
  |--Clustered Index Scan(OBJECT:([db].[dbo].[users]))
       WHERE:[age] > (25)
```

## Key Benefits

✓ **Safe**: Non-executing, parameterized queries prevent SQL injection and data leaks  
✓ **Fast**: No actual query execution required  
✓ **Portable**: Works across PostgreSQL, MySQL, SQLite, and MSSQL  
✓ **Debuggable**: Verify generated SQL matches expectations  
✓ **Auditable**: Log all queries without side effects  
✓ **Optimizable**: Identify missing indexes and optimization opportunities  

## Related Documentation

- [Query Introspection Guide](https://github.com/oratchade/db-connector#query-introspection-and-performance-analysis) in README.md
- [DBQueries Interface](https://github.com/oratchade/db-connector/docs/CODE_REVIEW.md#dbqueries-interface) in CODE_REVIEW.md
- [Operators Compatibility](https://github.com/oratchade/db-connector/docs/OPERATORS_COMPATIBILITY.md)

## Common Use Cases

1. **Development**: Preview queries during development and debugging
2. **Testing**: Verify query generation without side effects
3. **Performance Tuning**: Analyze execution plans to optimize queries
4. **Compliance**: Log all queries for audit trails
5. **CI/CD**: Validate query correctness in automated tests
