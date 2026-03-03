# SQL.Null\* Types Support Implementation

## Overview

This document describes the implementation of `sql.Null*` type support in the db-connector library's scanning mechanism.

## What Was Changed

Enhanced the `setFieldFromValue()` function in [v1/db/row_adapter.go](../v1/db/row_adapter.go) to automatically detect and properly handle Go's nullable SQL types from the `database/sql` package.

## Supported SQL.Null\* Types

The implementation supports all standard Go nullable SQL types:

- `sql.NullString` - for nullable string columns
- `sql.NullInt64` - for nullable integer columns
- `sql.NullFloat64` - for nullable floating-point columns
- `sql.NullBool` - for nullable boolean columns
- `sql.NullByte` - for nullable byte columns
- `sql.NullTime` - for nullable timestamp columns

## How It Works

### 1. Type Detection

A new helper function `isSQLNullType()` checks if a field's type is one of the supported `sql.Null*` types:

```go
func isSQLNullType(t reflect.Type) bool {
    switch t {
    case reflect.TypeOf(sql.NullString{}),
         reflect.TypeOf(sql.NullInt64{}),
         reflect.TypeOf(sql.NullFloat64{}),
         reflect.TypeOf(sql.NullBool{}),
         reflect.TypeOf(sql.NullByte{}),
         reflect.TypeOf(sql.NullTime{}):
        return true
    }
    return false
}
```

### 2. Value Assignment

A new function `setSQLNullField()` handles the assignment of values to `sql.Null*` types:

- **Null values (nil):** Sets `Valid = false`, leaves the value field empty
- **Non-null values:** Sets the value field and `Valid = true`
- **Type coercion:** Converts strings to appropriate types (int, float, bool, etc.)

## Usage Example

Define struct fields using `sql.Null*` types:

```go
type User struct {
    ID       int            `db:"id"`
    Name     sql.NullString `db:"name"`      // Can be NULL in database
    Email    sql.NullString `db:"email"`     // Can be NULL in database
    Age      sql.NullInt64  `db:"age"`       // Can be NULL in database
    Score    sql.NullFloat64 `db:"score"`    // Can be NULL in database
    Active   sql.NullBool   `db:"active"`    // Can be NULL in database
    LastSeen sql.NullTime   `db:"last_seen"` // Can be NULL in database
}
```

Then use `ScanRowsTo` normally:

```go
rows, err := db.Query("SELECT id, name, email, age, score, active, last_seen FROM users")
if err != nil {
    // handle error
}

users, err := db.ScanRowsTo[User](rowsAdapter)
if err != nil {
    // handle error
}

// Check if a field was NULL in the database
if users[0].Name.Valid {
    println("User name:", users[0].Name.String)
} else {
    println("User name: not available (NULL in database)")
}

if users[0].Age.Valid && users[0].Age.Int64 > 0 {
    println("User age:", users[0].Age.Int64)
}
```

## Benefits

1. **Explicit NULL handling:** The `.Valid` field clearly indicates whether a database value was NULL
2. **No pointer overhead:** Unlike `*string`, nullable types have all data in a single struct
3. **Type safety:** The type system ensures you check `.Valid` before using the value
4. **Standard library:** Uses Go's standard `database/sql` nullable types that are familiar to Go developers
5. **Automatic conversion:** String values from database drivers are automatically parsed to the correct type

## Implementation Details

### How NULL values are handled

When a database row has a NULL value:

1. The scanner receives `nil` from `rows.Scan()`
2. `setFieldFromValue(f, nil)` is called
3. The function detects it's a `sql.Null*` type via `isSQLNullType()`
4. It calls `setSQLNullField(f, nil)` which sets `Valid = false`

### How non-NULL values are handled

When a database row has a value:

1. The scanner receives the value (could be `string`, `int64`, `float64`, etc.)
2. `setFieldFromValue(f, value)` is called
3. The function detects it's a `sql.Null*` type via `isSQLNullType()`
4. It calls `setSQLNullField(f, value)` which:
   - Converts the value to the appropriate type if needed
   - Sets the value field
   - Sets `Valid = true`

### Type conversions in `setSQLNullField()`

- **NullString:** Converts via string concatenation (`fmt.Sprint(cv)`)
- **NullInt64:** Parses strings using `strconv.ParseInt()`
- **NullFloat64:** Parses strings using `strconv.ParseFloat()`
- **NullBool:** Parses strings using `strconv.ParseBool()`
- **NullByte:** Extracts first byte from byte slice or single byte values
- **NullTime:** Uses JSON unmarshaling for time values

## Backward Compatibility

This implementation is **fully backward compatible**:

- Existing code using regular fields (int, string, \*string, etc.) continues to work unchanged
- The new functionality only activates when a field's type **is** a `sql.Null*` type
- No breaking changes to the public API

## Testing

To test the functionality:

```go
// Create a test struct with null-able fields
type TestRecord struct {
    Value sql.NullString
}

// After ScanRowsTo, you can verify:
```
