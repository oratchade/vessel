# SQL NULL Types Implementation Guide

This document provides comprehensive guidance on handling SQL NULL values in Fabric
using Go's `sql.Null*` types and proper struct definitions.

**Last Updated**: April 18, 2026  
**Fabric Version**: v1.0.0+

---

## Table of Contents

- [Overview](#overview)
- [Available Null Types](#available-null-types)
- [Basic Usage](#basic-usage)
- [Struct Mapping](#struct-mapping)
- [ScanRowsTo with Nullable Fields](#scanrowsto-with-nullable-fields)
- [Handling Null Values](#handling-null-values)
- [Common Patterns](#common-patterns)
- [Best Practices](#best-practices)
- [FAQ](#faq)

---

## Overview

When querying databases, fields may contain `NULL` values representing missing
or unknown data. Go's `database/sql` package provides `Null*` types
to safely represent these values:

- `sql.NullInt64` - for INTEGER columns
- `sql.NullString` - for VARCHAR/TEXT columns
- `sql.NullFloat64` - for FLOAT/DECIMAL columns
- `sql.NullBool` - for BOOLEAN columns
- `sql.NullTime` - for DATE/DATETIME columns
- `sql.NullByte` - for single byte columns

Each `Null*` type has two fields:

- `Valid bool` - indicates whether the value is NULL (false) or not (true)
- `V` or specialized field - the actual value if Valid is true

---

## Available Null Types

### sql.NullInt64

Used for INTEGER, BIGINT, INT, SMALLINT columns that may be NULL.

```go
import "database/sql"

type User struct {
    ID              int64
    ParentUserID    sql.NullInt64  // May be NULL
    LoginCount      sql.NullInt64  // May be NULL
}

// Check if value is NULL
if user.ParentUserID.Valid {
    parentID := user.ParentUserID.Int64
    fmt.Println("Parent ID:", parentID)
} else {
    fmt.Println("User has no parent")
}
```

### sql.NullString

Used for VARCHAR, TEXT, CHAR columns that may be NULL.

```go
import "database/sql"

type Profile struct {
    ID              int64
    Bio             sql.NullString  // May be NULL
    Website         sql.NullString  // May be NULL
    PhoneNumber     sql.NullString  // May be NULL
}

// Access the value
if profile.Bio.Valid {
    fmt.Println("Bio:", profile.Bio.String)
} else {
    fmt.Println("No bio provided")
}
```

### sql.NullFloat64

Used for FLOAT, DECIMAL, DOUBLE columns that may be NULL.

```go
import "database/sql"

type Product struct {
    ID        int64
    Price     float64         // Non-nullable
    DiscountedPrice sql.NullFloat64  // May be NULL
}

// Check and use
if product.DiscountedPrice.Valid {
    fmt.Printf("Discounted: $%.2f\n", product.DiscountedPrice.Float64)
}
```

### sql.NullBool

Used for BOOLEAN, BIT columns that may be NULL.

```go
import "database/sql"

type Feature struct {
    ID       int64
    Enabled  sql.NullBool  // May be NULL (unknown state)
}

// Triple-state logic
if feature.Enabled.Valid {
    if feature.Enabled.Bool {
        fmt.Println("Feature is enabled")
    } else {
        fmt.Println("Feature is disabled")
    }
} else {
    fmt.Println("Feature state is unknown")
}
```

### sql.NullTime

Used for DATE, DATETIME, TIMESTAMP columns that may be NULL.

```go
import (
    "database/sql"
    "time"
)

type Event struct {
    ID        int64
    CreatedAt time.Time      // Non-nullable (always set by DB)
    DeletedAt sql.NullTime   // Soft-delete timestamp (NULL if not deleted)
    CompletedAt sql.NullTime // Nullable completion time
}

// Check if event is deleted (soft-delete pattern)
if event.DeletedAt.Valid {
    fmt.Println("Event deleted at:", event.DeletedAt.Time)
} else {
    fmt.Println("Event is active")
}

// Check if event is completed
if event.CompletedAt.Valid {
    fmt.Println("Completed:", event.CompletedAt.Time.Format("2006-01-02"))
}
```

### sql.NullByte

Used for CHAR(1), TINYINT columns that may be NULL.

```go
import "database/sql"

type Setting struct {
    ID    int64
    Flag  sql.NullByte  // May be NULL
}
```

---

## Basic Usage

### Define Structs with Nullable Fields

```go
import (
    "database/sql"
    "time"
)

type User struct {
    // Non-nullable fields (always have a value)
    ID        int64
    Email     string
    Username  string
    CreatedAt time.Time

    // Nullable fields (may be NULL in database)
    FirstName    sql.NullString
    LastName     sql.NullString
    PhoneNumber  sql.NullString
    ProfileImage sql.NullString
    Bio          sql.NullString
    Birthday     sql.NullTime
    VerifiedAt   sql.NullTime
    DeletedAt    sql.NullTime
}
```

### Scanning Rows into Structs

```go
import (
    db "tounilab.com/fabric/db/v1"
    "database/sql"
)

// Query database
ctx := context.Background()
rowsAdapter, err := database.GetRaw(ctx, "users",
    []string{"id", "email", "username", "first_name", "phone_number", "verified_at"},
    nil, nil, nil)
if err != nil {
    log.Fatal(err)
}

// Scan into typed structs
users, err := db.ScanRowsTo[User](ctx, rowsAdapter)
if err != nil {
    log.Fatal(err)
}

// Process results
for _, user := range users {
    // Check nullable fields before using
    if user.FirstName.Valid {
        fmt.Println("Name:", user.FirstName.String)
    }
    if user.VerifiedAt.Valid {
        fmt.Println("Verified:", user.VerifiedAt.Time)
    }
}
```

---

## Struct Mapping

### Column-to-Field Alignment

Fabric uses Go's database/sql package for scanning. Fields are mapped by position:

```go
type User struct {
    ID           int64
    Email        string
    FirstName    sql.NullString
    PhoneNumber  sql.NullString
}

// Query selects columns in order: id, email, first_name, phone_number
// Fabric scans into: ID, Email, FirstName, PhoneNumber (by position, not name)
users, err := db.ScanRowsTo[User](ctx, rowsAdapter)
```

**Column Order Must Match Field Order:**

```go
// ✅ CORRECT: Column order matches struct field order
rowsAdapter, _ := database.GetRaw(ctx, "users",
    []string{"id", "email", "first_name", "phone_number"}, // ← order matters
    nil, nil, nil)

// ❌ WRONG: Column order doesn't match
rowsAdapter, _ := database.GetRaw(ctx, "users",
    []string{"first_name", "id", "email", "phone_number"}, // ← different order
    nil, nil, nil)
```

### Type Compatibility

Fabric automatically handles type conversions:

| SQL Type                  | Go Type     | Nullable Type     |
| ------------------------- | ----------- | ----------------- |
| INTEGER, BIGINT, INT      | `int64`     | `sql.NullInt64`   |
| VARCHAR, TEXT, CHAR       | `string`    | `sql.NullString`  |
| FLOAT, DECIMAL, DOUBLE    | `float64`   | `sql.NullFloat64` |
| BOOLEAN, BIT              | `bool`      | `sql.NullBool`    |
| DATE, DATETIME, TIMESTAMP | `time.Time` | `sql.NullTime`    |

---

## ScanRowsTo with Nullable Fields

### Type-Safe Scanning

```go
import (
    db "tounilab.com/fabric/db/v1"
    "database/sql"
    "context"
)

type Product struct {
    ID              int64
    Name            string
    Description     sql.NullString
    Price           float64
    DiscountedPrice sql.NullFloat64
    InStock         bool
    DiscontinuedAt  sql.NullTime
}

func GetProducts(ctx context.Context, database db.DB) ([]Product, error) {
    rowsAdapter, err := database.GetRaw(ctx, "products",
        []string{
            "id",
            "name",
            "description",
            "price",
            "discounted_price",
            "in_stock",
            "discontinued_at",
        },
        nil, nil, nil)
    if err != nil {
        return nil, err
    }

    // ScanRowsTo automatically handles sql.Null* types
    products, err := db.ScanRowsTo[Product](ctx, rowsAdapter)
    if err != nil {
        return nil, err
    }

    return products, nil
}

// Usage
products, _ := GetProducts(ctx, database)
for _, product := range products {
    fmt.Printf("Product: %s ($%.2f", product.Name, product.Price)

    if product.DiscountedPrice.Valid {
        fmt.Printf(" → $%.2f", product.DiscountedPrice.Float64)
    }

    if product.DiscontinuedAt.Valid {
        fmt.Printf(" [DISCONTINUED]")
    }

    fmt.Println()
}
```

### Error Handling

If a column value cannot be scanned into the corresponding struct field,
`ScanRowsTo` returns a detailed error:

```go
products, err := db.ScanRowsTo[Product](ctx, rowsAdapter)
if err != nil {
    // Error messages include column name and type mismatch details
    log.Printf("Failed to scan products: %v", err)
    return nil, fmt.Errorf("scan error: %w", err)
}
```

---

## Handling Null Values

### Checking for NULL

```go
user := users[0]

// Check if field is NULL
if user.PhoneNumber.Valid {
    // Use the value
    fmt.Println("Phone:", user.PhoneNumber.String)
} else {
    // Field is NULL
    fmt.Println("No phone number on file")
}
```

### Providing Default Values

```go
// Convert NULL to default value
firstName := user.FirstName.String
if !user.FirstName.Valid {
    firstName = "Unknown"
}

lastName := user.LastName.String
if !user.LastName.Valid {
    lastName = "User"
}

fullName := fmt.Sprintf("%s %s", firstName, lastName)
```

### Using Pointers as Alternative

Instead of `sql.Null*`, you can use pointers to handle NULL:

```go
type User struct {
    ID        int64
    FirstName *string   // nil = NULL, points to value otherwise
    Email     string
}

// Scanning works the same way
users, err := db.ScanRowsTo[User](ctx, rowsAdapter)
if err != nil {
    log.Fatal(err)
}

// Check if NULL
if user.FirstName != nil {
    fmt.Println("First name:", *user.FirstName)
} else {
    fmt.Println("No first name")
}
```

**When to use pointers vs sql.Null\*:**

- Use `sql.Null*`: For explicit NULL representation with zero value available
- Use pointers: When you want to distinguish between
  "not set" (nil) and "empty" ("")

---

## Common Patterns

### Soft Deletes

Soft-delete pattern uses a nullable `deleted_at` timestamp:

```go
import (
    "database/sql"
    "time"
)

type Article struct {
    ID        int64
    Title     string
    Content   string
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt sql.NullTime  // NULL = active, has value = deleted
}

// Query only active articles
expr := condition.NewExpr().Column("deleted_at").Op("IS NULL")
active, _ := db.GetRaw(ctx, "articles", []string{"*"}, nil, expr, nil)

// Query only deleted articles
expr := condition.NewExpr().Column("deleted_at").Op("IS NOT NULL")
deleted, _ := db.GetRaw(ctx, "articles", []string{"*"}, nil, expr, nil)

// Check if article is soft-deleted
article := articles[0]
if article.DeletedAt.Valid {
    fmt.Printf("Deleted on: %v\n", article.DeletedAt.Time)
}
```

### Optional Profile Fields

User profiles with optional information:

```go
type UserProfile struct {
    ID          int64
    UserID      int64
    Bio         sql.NullString
    Website     sql.NullString
    Location    sql.NullString
    TwitterHandle sql.NullString
    GitHubHandle  sql.NullString
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// Build profile display with only filled fields
func (p *UserProfile) DisplayBio() string {
    if p.Bio.Valid && p.Bio.String != "" {
        return p.Bio.String
    }
    return "No bio provided"
}

func (p *UserProfile) HasWebsite() bool {
    return p.Website.Valid && p.Website.String != ""
}
```

### Nullable Timestamps for Events

Track optional event completion/cancellation:

```go
type Task struct {
    ID          int64
    Title       string
    Description sql.NullString
    AssignedTo  sql.NullInt64  // User ID if assigned
    StartedAt   sql.NullTime   // When work began
    CompletedAt sql.NullTime   // When work finished
    CancelledAt sql.NullTime   // When cancelled (mutually exclusive with CompletedAt)
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// Check task status
func (t *Task) Status() string {
    if t.CancelledAt.Valid {
        return "Cancelled"
    }
    if t.CompletedAt.Valid {
        return "Completed"
    }
    if t.StartedAt.Valid {
        return "In Progress"
    }
    if t.AssignedTo.Valid {
        return "Assigned"
    }
    return "Unassigned"
}
```

### Conditional Fields in Responses

Only include non-NULL fields in API responses:

```go
import "encoding/json"

type UserJSON struct {
    ID        int64  `json:"id"`
    Email     string `json:"email"`
    FirstName *string `json:"first_name,omitempty"`
    LastName  *string `json:"last_name,omitempty"`
    Bio       *string `json:"bio,omitempty"`
}

// Convert database model to JSON response
func UserToJSON(user *User) *UserJSON {
    response := &UserJSON{
        ID:    user.ID,
        Email: user.Email,
    }

    if user.FirstName.Valid {
        response.FirstName = &user.FirstName.String
    }
    if user.LastName.Valid {
        response.LastName = &user.LastName.String
    }
    if user.Bio.Valid {
        response.Bio = &user.Bio.String
    }

    return response
}

// When marshaled to JSON, omitempty skips nil fields:
// {"id":1,"email":"user@example.com","first_name":"John"}
// (Bio is omitted if nil)
```

---

## Best Practices

### 1. Use Explicit Null Types

```go
// ✅ GOOD: Clear that field is nullable
type User struct {
    ID        int64
    FirstName sql.NullString
    LastName  sql.NullString
}

// ❌ AVOID: Unclear if field can be NULL
type User struct {
    ID        int64
    FirstName string  // Is this nullable? Unclear.
    LastName  string
}
```

### 2. Always Check Valid Before Using

```go
// ✅ GOOD
if user.FirstName.Valid {
    fmt.Println(user.FirstName.String)
}

// ❌ AVOID: Crashes if Valid is false
fmt.Println(user.FirstName.String)  // Could be empty/default if NULL
```

### 3. Document Nullable Fields

```go
type User struct {
    ID int64

    // Required fields
    Email    string
    Username string

    // Optional fields (can be NULL in database)
    FirstName sql.NullString  // User's first name if provided
    LastName  sql.NullString  // User's last name if provided
    Bio       sql.NullString  // User's bio/description
}
```

### 4. Consistent Nullable Pattern for Related Fields

```go
// ✅ GOOD: All optional fields use sql.Null*
type Profile struct {
    Bio         sql.NullString
    Website     sql.NullString
    Location    sql.NullString
    PhoneNumber sql.NullString
}

// ❌ AVOID: Mixed nullable patterns
type Profile struct {
    Bio         sql.NullString
    Website     *string        // Inconsistent
    Location    string         // Unclear if nullable
    PhoneNumber sql.NullString
}
```

### 5. Use Zero Value Carefully

```go
type Product struct {
    ID        int64
    Price     float64        // Non-nullable, default 0.0
    Discount  sql.NullFloat64 // Nullable, only set if discount exists
}

// This distinction is important:
// A product with Price=0 is different from Price being NULL
// A product with Discount=0 (Valid=true) means actual 0% discount
// A product with Discount=NULL (Valid=false) means no discount info
```

### 6. Soft Deletes with Timestamps

```go
// ✅ GOOD: Clear deletion tracking
type Article struct {
    ID        int64
    CreatedAt time.Time      // Never NULL
    UpdatedAt time.Time      // Updated on each change
    DeletedAt sql.NullTime   // NULL if active, has timestamp if deleted
}

// ✅ Query active articles easily
condition.NewExpr().Column("deleted_at").Op("IS NULL")
```

---

## FAQ

**Q: What's the difference between `sql.NullString` and `*string`?**

A: Both represent nullable strings, but with different semantics:

```go
type Option1 struct {
    Value sql.NullString  // nil = NULL, String = value, Valid = true/false
}

type Option2 struct {
    Value *string  // nil = NULL, pointer = value
}

// sql.NullString is more explicit about NULL state:
ns := sql.NullString{String: "hello", Valid: true}
ns2 := sql.NullString{String: "", Valid: false}  // Explicitly NULL, not empty

// Pointers are simpler:
ps := "hello"
ps2 := (*string)(nil)  // NULL

// Use sql.Null* for:
// - API contracts requiring explicit NULL representation
// - Database NULL semantics clarity
// - Database field mappings

// Use pointers for:
// - Simpler code
// - Go-idiomatic optional values
```

**Q: How do I insert a NULL value into the database?**

A: Use `sql.Null*` types with `Valid=false`:

```go
user := map[string]any{
    "id":         1,
    "email":      "user@example.com",
    "first_name": sql.NullString{String: "", Valid: false},  // Will be NULL
}
database.Insert(ctx, "users", user, nil)
```

**Q: What happens if I scan a NULL value into a non-nullable field?**

A: ScanRowsTo returns an error with details:

```go
type User struct {
    ID    int64
    Email string  // Non-nullable
}

// If email column is NULL, ScanRowsTo fails:
users, err := db.ScanRowsTo[User](ctx, rowsAdapter)
// err: "scan: error reading column Email: unsupported scan of NULL into type string"
```

**Q: Can I use struct tags to customize NULL handling?**

A: The standard database/sql package doesn't use struct tags for NULL handling.
Use field names and positions instead. For custom mapping, create a conversion function:

```go
type UserDB struct {
    ID        int64
    FirstName sql.NullString
}

type UserAPI struct {
    ID        int64  `json:"id"`
    FirstName string `json:"first_name"`
}

// Convert nullable DB model to non-nullable API model
func ToAPI(user *UserDB) *UserAPI {
    api := &UserAPI{ID: user.ID}
    if user.FirstName.Valid {
        api.FirstName = user.FirstName.String
    }
    return api
}
```

**Q: What's the performance impact of using sql.Null\* types?**

A: Minimal. sql.Null\* types are struct wrappers with:

- 1 bool field (Valid)
- 1 value field (String, Int64, etc.)

Memory overhead is negligible. No performance difference in scanning or queries.

---

## Summary

Handling SQL NULL values properly is essential for robust database applications:

1. **Define your schema clearly**: Use `sql.Null*` for nullable columns
2. **Always check Valid**: Before accessing the value
3. **Document nullable fields**: Make intent clear in code
4. **Use consistent patterns**: Throughout your codebase
5. **Test NULL cases**: Ensure handling in unit tests

For more information:

- [database/sql documentation](https://pkg.go.dev/database/sql)
- [OPERATORS_COMPATIBILITY.md](./OPERATORS_COMPATIBILITY.md) - Query NULL values
  with `IS NULL`
- [ERROR_HANDLING.md](./ERROR_HANDLING.md) - Handle scanning errors
- [ARCHITECTURE.md](./ARCHITECTURE.md) - How ScanRowsTo works internally
