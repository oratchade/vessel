# Operators Compatibility Matrix

This document provides a comprehensive reference for SQL operator support
across all
supported database dialects in Fabric (MySQL, PostgreSQL, SQLite, MSSQL).

**Last Updated**: April 18, 2026  
**Fabric Version**: v1.0.0+

---

## Table of Contents

- [Quick Reference](#quick-reference)
- [Comparison Operators](#comparison-operators)
- [Logical Operators](#logical-operators)
- [Pattern Matching](#pattern-matching)
- [Range Operators](#range-operators)
- [Set Operators](#set-operators)
- [Null Checking](#null-checking)
- [Advanced Features](#advanced-features)
- [Examples](#examples)

---

## Quick Reference

| Operator       | MySQL | Pg  | SQLite | MSSQL | Notes                              |
| -------------- | :---: | :-: | :----: | :---: | ---------------------------------- |
| **Comparison** |       |     |        |       |                                    |
| `=`            |  ✅   | ✅  |   ✅   |  ✅   | Equality                           |
| `!=`           |  ✅   | ✅  |   ✅   |  ✅   | Not equal (alias: `<>`)            |
| `<>`           |  ✅   | ✅  |   ✅   |  ✅   | Not equal (standard SQL)           |
| `<`            |  ✅   | ✅  |   ✅   |  ✅   | Less than                          |
| `>`            |  ✅   | ✅  |   ✅   |  ✅   | Greater than                       |
| `<=`           |  ✅   | ✅  |   ✅   |  ✅   | Less than or equal                 |
| `>=`           |  ✅   | ✅  |   ✅   |  ✅   | Greater than or equal              |
| **Logical**    |       |     |        |       |                                    |
| `AND`          |  ✅   | ✅  |   ✅   |  ✅   | Logical AND                        |
| `OR`           |  ✅   | ✅  |   ✅   |  ✅   | Logical OR                         |
| `NOT`          |  ✅   | ✅  |   ✅   |  ✅   | Logical NOT                        |
| **Pattern**    |       |     |        |       |                                    |
| `LIKE`         |  ✅   | ✅  |   ✅   |  ✅   | Pattern matching (case-sens in Pg) |
| `NOT LIKE`     |  ✅   | ✅  |   ✅   |  ✅   | Negated pattern matching           |
| **Range**      |       |     |        |       |                                    |
| `BETWEEN`      |  ✅   | ✅  |   ✅   |  ✅   | Range check (inclusive)            |
| **Set**        |       |     |        |       |                                    |
| `IN`           |  ✅   | ✅  |   ✅   |  ✅   | Membership in set                  |
| `NOT IN`       |  ✅   | ✅  |   ✅   |  ✅   | Negated set membership             |
| **Null**       |       |     |        |       |                                    |
| `IS NULL`      |  ✅   | ✅  |   ✅   |  ✅   | Null check                         |
| `IS NOT NULL`  |  ✅   | ✅  |   ✅   |  ✅   | Not null check                     |

---

## Comparison Operators

Comparison operators are the most fundamental SQL operators, supported uniformly
across all dialects.

### Equality and Inequality

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
)

// Equal to (=)
expr := cdt.NewExpr().Column("status").Op("=").Value("active")
// MySQL: WHERE status = ?
// PostgreSQL: WHERE status = $1
// SQLite: WHERE status = ?
// MSSQL: WHERE status = ?

// Not equal (!= or <>)
expr := cdt.NewExpr().Column("role").Op("!=").Value("admin")
// OR equivalently:
expr := cdt.NewExpr().Column("role").Op("<>").Value("admin")
```

### Relational Operators

```go
// Greater than
expr := cdt.NewExpr().Column("age").Op(">").Value(18)

// Less than
expr := cdt.NewExpr().Column("created_at").Op("<").Value(time.Now())

// Greater than or equal
expr := cdt.NewExpr().Column("score").Op(">=").Value(80)

// Less than or equal
expr := cdt.NewExpr().Column("price").Op("<=").Value(100.00)
```

**Dialect Notes:**

- All databases handle numeric and string comparisons identically
- Date/time comparisons work with native types via parameter binding
- No special quoting required by Fabric (handled automatically)

---

## Logical Operators

Logical operators combine multiple conditions into complex expressions.

### AND Operator

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
)

// Combine conditions with AND
cond := cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("status").Op("=").Value("active"),
    cdt.NewExpr().Column("age").Op(">").Value(18),
)

// Generated SQL:
// WHERE (status = ?) AND (age > ?)

// Chaining multiple conditions
cond := cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("status").Op("=").Value("active"),
    cdt.NewExpr().Column("age").Op(">").Value(18),
    cdt.NewExpr().Column("verified").Op("=").Value(true),
)

// Generated SQL:
// WHERE (status = ?) AND (age > ?) AND (verified = ?)
```

### OR Operator

```go
// Combine conditions with OR
cond := cdt.NewOr().Conditions(
    cdt.NewExpr().Column("role").Op("=").Value("admin"),
    cdt.NewExpr().Column("role").Op("=").Value("moderator"),
)

// Generated SQL:
// WHERE (role = ?) OR (role = ?)
```

### NOT Operator

```go
// Negate a condition
cond := cdt.NewNot().Condition(
    cdt.NewExpr().Column("status").Op("=").Value("deleted"),
)

// Generated SQL:
// WHERE NOT (status = ?)
```

### Complex Combinations

```go
// Combine AND, OR, and NOT
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

// Generated SQL:
// WHERE (active = ?) AND (role = ? OR role = ?) AND NOT (banned = ?)
```

**Dialect Notes:**

- All logical operators are standard SQL and work identically across all databases
- Parentheses are automatically added for proper operator precedence
- No dialect-specific behavior

---

## Pattern Matching

Pattern matching is used for string search operations.

### LIKE Operator

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
)

// Pattern matching with LIKE
expr := cdt.NewExpr().Column("email").Op("LIKE").Value("%@example.com")

// Supported wildcards:
// %  = zero or more characters
// _  = exactly one character

// Examples:
cdt.NewExpr().Column("name").Op("LIKE").Value("John%")    // Starts with "John"
cdt.NewExpr().Column("name").Op("LIKE").Value("%Smith")   // Ends with "Smith"
cdt.NewExpr().Column("name").Op("LIKE").Value("%Middle%") // Contains "Middle"
cdt.NewExpr().Column("code").Op("LIKE").Value("ABC_123")  // Matches "ABCX123" where X is any char
```

**Dialect-Specific Notes:**

| Database       | Case-Sens | Notes                                                       |
| -------------- | :-------: | ----------------------------------------------------------- |
| **MySQL**      |   No\*    | By default, case-insensitive (depends on collation)         |
| **PostgreSQL** |    Yes    | Use `ILIKE` for case-insensitive (Fabric doesn't wrap this) |
| **SQLite**     |   No\*    | By default, case-insensitive (depends on collation)         |
| **MSSQL**      |   No\*    | By default, case-insensitive (depends on collation)         |

\*Can be overridden with collation settings in database configuration.

### NOT LIKE Operator

```go
// Negated pattern matching
expr := cdt.NewExpr().Column("email").Op("NOT LIKE").Value("%@spam.com")
```

---

## Range Operators

### BETWEEN Operator

The `BETWEEN` operator checks if a value falls within an inclusive range.

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
)

// Numeric range
expr := cdt.NewExpr().Column("age").Op("BETWEEN").Value(18).Value(65)
// Generated SQL: WHERE age BETWEEN ? AND ?
// Parameters: [18, 65]

// Date range
startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
endDate := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
expr := cdt.NewExpr().Column("created_at").Op("BETWEEN").Value(startDate).Value(endDate)

// Price range
expr := cdt.NewExpr().Column("price").Op("BETWEEN").Value(10.00).Value(99.99)
```

**Key Points:**

- `BETWEEN` is inclusive on both boundaries
- All databases support `BETWEEN` uniformly
- Always requires two values: min and max

---

## Set Operators

### IN Operator

The `IN` operator checks if a value exists in a set of values.

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
)

// Check if value is in set
expr := cdt.NewExpr().Column("status").Op("IN").Value("active", "pending", "review")
// Generated SQL: WHERE status IN (?, ?, ?)

// With numeric values
expr := cdt.NewExpr().Column("user_id").Op("IN").Value(1, 2, 3, 4, 5)

// With single value (works but typically use = instead)
expr := cdt.NewExpr().Column("role").Op("IN").Value("admin")
```

### NOT IN Operator

```go
// Negated set membership
expr := cdt.NewExpr().Column("status").Op("NOT IN").Value("deleted", "banned", "suspended")
// Generated SQL: WHERE status NOT IN (?, ?, ?)
```

**Performance Notes:**

- For small sets (<10 items): `IN` is efficient
- For large sets (>100 items): Consider `BETWEEN` or JOIN instead
- All databases optimize `IN` with index lookups

---

## Null Checking

### IS NULL Operator

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
)

// Check if value is NULL (no parameter needed)
expr := cdt.NewExpr().Column("deleted_at").Op("IS NULL")
// Generated SQL: WHERE deleted_at IS NULL
```

### IS NOT NULL Operator

```go
// Check if value is not NULL
expr := cdt.NewExpr().Column("email").Op("IS NOT NULL")
// Generated SQL: WHERE email IS NOT NULL
```

**Important Notes:**

- NULL checks don't use parameters (no `Value()` call)
- In SQL, `column = NULL` is always false; use `IS NULL` instead
- All databases support `IS NULL` and `IS NOT NULL` uniformly

## Common Pattern: Soft Deletes

```go
// Find active records (not soft-deleted)
expr := cdt.NewExpr().Column("deleted_at").Op("IS NULL")

// Find deleted records
expr := cdt.NewExpr().Column("deleted_at").Op("IS NOT NULL")
```

---

## Advanced Features

### Operator Combinations

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
)

// Complex query combining multiple operators
cond := cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("status").Op("=").Value("active"),
    cdt.NewExpr().Column("created_at").Op("BETWEEN").Value(startDate).Value(endDate),
    cdt.NewExpr().Column("role").Op("IN").Value("admin", "moderator"),
    cdt.NewExpr().Column("verified").Op("IS NOT NULL"),
)

// Generated SQL:
// WHERE (status = ?) AND (created_at BETWEEN ? AND ?)
//       AND (role IN (?, ?)) AND (verified IS NOT NULL)
```

### Negation with NOT

```go
// Negate entire expressions
cond := cdt.NewNot().Condition(
    cdt.NewExpr().Column("status").Op("IN").Value("deleted", "banned"),
)
// Generated SQL: WHERE NOT (status IN (?, ?))
```

---

## Examples

### User Search Query

```go
import (
    db "tounilab.com/fabric/db/v1"
    cdt "tounilab.com/fabric/pkg/query/condition"
    "time"
)

// Find active users from a specific role, registered in last 30 days
thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

cond := cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("status").Op("=").Value("active"),
    cdt.NewExpr().Column("role").Op("IN").Value("user", "moderator"),
    cdt.NewExpr().Column("created_at").Op(">").Value(thirtyDaysAgo),
    cdt.NewExpr().Column("email_verified").Op("IS NOT NULL"),
)

// Execute on MySQL
results, _ := db.GetRaw(ctx, "users", []string{"*"}, nil, cond, nil)
// SQL Generated: SELECT * FROM users WHERE
// (status = ?) AND (role IN (?, ?)) AND (created_at > ?) AND (email_verified IS NOT NULL)

// Price Range Filter: Find products in price range, excluding certain categories
priceCond := cdt.NewAnd().Conditions(
    cdt.NewExpr().Column("price").Op("BETWEEN").Value(10.00).Value(100.00),
    cdt.NewExpr().Column("category").Op("NOT IN").Value("restricted", "discontinued"),
    cdt.NewExpr().Column("in_stock").Op("=").Value(true),
)

results, _ := db.GetRaw(ctx, "products", []string{"*"}, nil, priceCond, nil)
```

### Content Search

```go
// Find blog posts by title or content, published and not deleted
cond := cdt.NewAnd().Conditions(
    cdt.NewOr().Conditions(
        cdt.NewExpr().Column("title").Op("LIKE").Value("%search%"),
        cdt.NewExpr().Column("content").Op("LIKE").Value("%search%"),
    ),
    cdt.NewExpr().Column("published").Op("=").Value(true),
    cdt.NewExpr().Column("deleted_at").Op("IS NULL"),
)

results, _ := db.GetRaw(ctx, "posts", []string{"*"}, nil, cond, nil)
```

---

## Operator Support by Task

### Finding Records

| Task         | Operator                 | Example                           |
| ------------ | ------------------------ | --------------------------------- |
| Exact match  | `=`                      | `status = 'active'`               |
| Not matching | `!=` or `<>`             | `role != 'guest'`                 |
| Multiple opt | `IN`                     | `status IN ('active', 'pending')` |
| Range        | `BETWEEN`                | `age BETWEEN 18 AND 65`           |
| Existence    | `IS NULL`, `IS NOT NULL` | `email IS NOT NULL`               |
| Pattern      | `LIKE`                   | `name LIKE '%John%'`              |

### Filtering Records

| Task             | Operator | Example                     |
| ---------------- | -------- | --------------------------- |
| Greater than     | `>`      | `created_at > '2024-01-01'` |
| Less than        | `<`      | `price < 100.00`            |
| Greater or equal | `>=`     | `score >= 80`               |
| Less or equal    | `<=`     | `age <= 18`                 |

### Combining Conditions

| Task             | Operator | Example                                 |
| ---------------- | -------- | --------------------------------------- |
| Both conditions  | `AND`    | `status = 'active' AND verified = true` |
| Either condition | `OR`     | `role = 'admin' OR role = 'moderator'`  |
| Opposite         | `NOT`    | `NOT (status = 'deleted')`              |

---

## FAQ

**Q: Why can't I use `=` for NULL checks?**  
A: In SQL, `NULL` is a special value representing "unknown". Comparing anything
to `NULL` (even `NULL = NULL`) returns `NULL`, not true or false. Use `IS NULL`
instead.

**Q: Which operator should I use for case-insensitive search?**  
A: This depends on your database:

- MySQL/SQLite/MSSQL: `LIKE` is case-insensitive by default (depends on collation)
- PostgreSQL: `LIKE` is case-sensitive; consider using `ILIKE` or LOWER()
  function in raw SQL

**Q: How do I handle optional WHERE conditions?**  
A: Build conditions conditionally before passing to the query:

```go
var expr *condition.Expr
if searchTerm != "" {
    expr = cdt.NewExpr().Column("title").Op("LIKE").Value("%" + searchTerm + "%")
}
// Then use expr in query (may be nil, which means no WHERE clause)
```

**Q: Can I use operators in UPDATE or DELETE?**  
A: Yes! All operators work in WHERE clauses for UPDATE and DELETE:

```go
// Update records matching condition
db.Update(ctx, "users", map[string]any{"verified": true},
    cdt.NewExpr().Column("email").Op("LIKE").Value("%@example.com"), nil)
```

---

## Summary

Fabric provides a unified operator interface across all supported databases.
The same Go code generates correct SQL for MySQL, PostgreSQL, SQLite, and MSSQL.
All 20+ operators are supported uniformly, with no dialect-specific syntax
required by the application developer.

For additional information, see:

- [ERROR_HANDLING.md](./ERROR_HANDLING.md) - How errors are handled during query
  execution
- [ARCHITECTURE.md](./ARCHITECTURE.md) - How operators are processed internally
- [DB_MANAGER.md](./DB_MANAGER.md) - Advanced query patterns with DBManager
