//go:build test

package dberror_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tounilab.com/vessel/db/v1/dberror"
)

// TestMySQLErrorPrefixing verifies MySQL errors include [mysql] prefix.
func TestMySQLErrorPrefixing(t *testing.T) {
	mapper := dberror.MySQLErrorMapper{}
	originalErr := errors.New("Error 1062: Duplicate entry 'john' for key 'email'")

	mappedErr := mapper.MapError(originalErr)
	require.NotNil(t, mappedErr)

	errMsg := mappedErr.Error()
	assert.Contains(t, errMsg, "[mysql]", "MySQL error should contain [mysql] prefix")
	assert.Contains(t, errMsg, "duplicate key violation", "should map to duplicate key sentinel")
}

// TestPostgresErrorPrefixing verifies PostgreSQL errors include [postgres] prefix.
func TestPostgresErrorPrefixing(t *testing.T) {
	mapper := dberror.PostgresErrorMapper{}
	originalErr := errors.New("pq: duplicate key value violates unique constraint")

	mappedErr := mapper.MapError(originalErr)
	require.NotNil(t, mappedErr)

	errMsg := mappedErr.Error()
	assert.Contains(t, errMsg, "[postgres]", "PostgreSQL error should contain [postgres] prefix")
	assert.Contains(t, errMsg, "duplicate key violation", "should map to duplicate key sentinel")
}

// TestSQLiteErrorPrefixing verifies SQLite errors include [sqlite] prefix.
func TestSQLiteErrorPrefixing(t *testing.T) {
	mapper := dberror.SQLiteErrorMapper{}
	originalErr := errors.New("UNIQUE constraint failed: users.email")

	mappedErr := mapper.MapError(originalErr)
	require.NotNil(t, mappedErr)

	errMsg := mappedErr.Error()
	assert.Contains(t, errMsg, "[sqlite]", "SQLite error should contain [sqlite] prefix")
	assert.Contains(t, errMsg, "duplicate key violation", "should map to duplicate key sentinel")
}

// TestMSSQLErrorPrefixing verifies MSSQL errors include [mssql] prefix.
func TestMSSQLErrorPrefixing(t *testing.T) {
	mapper := dberror.MSSQLErrorMapper{}
	originalErr := errors.New("Violation of PRIMARY KEY constraint")

	mappedErr := mapper.MapError(originalErr)
	require.NotNil(t, mappedErr)

	errMsg := mappedErr.Error()
	assert.Contains(t, errMsg, "[mssql]", "MSSQL error should contain [mssql] prefix")
	assert.Contains(t, errMsg, "duplicate key violation", "should map to duplicate key sentinel")
}
