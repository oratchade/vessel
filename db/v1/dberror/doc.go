// Package dberror provides Vessel sentinel errors and database-driver error
// mapping helpers.
//
// The package maps driver-specific errors from MySQL, PostgreSQL, SQLite, and
// Microsoft SQL Server to stable errors such as ErrNotFound, ErrDuplicateKey,
// ErrForeignKeyViolation, ErrConnectionFailed, ErrConstraintViolation,
// ErrSyntaxError, and ErrQueryTimeout. Call GetMapper with a dialect name when
// driver-specific errors need to be normalized before returning them from a
// custom Vessel driver.
package dberror
