//go:build test

package dberror_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	dberror "tounilab.com/fabric/db/v1/dberror"
)

// testErrorMapperCase represents a single test case for error mapping.
type testErrorMapperCase struct {
	name        string
	err         error
	expectedErr error
}

// testErrorMapperFunc is a function that maps an error using a specific ErrorMapper.
type testErrorMapperFunc func(error) error

// runErrorMapperTests runs a series of error mapping test cases.
func runErrorMapperTests(t *testing.T, mapper testErrorMapperFunc, cases []testErrorMapperCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped := mapper(tc.err)

			if tc.expectedErr == nil {
				assert.Equal(t, tc.err, mapped)
			} else {
				assert.ErrorIs(t, mapped, tc.expectedErr)
			}
		})
	}
}

func TestMySQLErrorMapper(t *testing.T) {
	mapper := dberror.MySQLErrorMapper{}
	cases := []testErrorMapperCase{
		{
			name:        "nil error",
			err:         nil,
			expectedErr: nil,
		},
		{
			name:        "duplicate key via error code 1062",
			err:         errors.New("Error 1062: Duplicate entry"),
			expectedErr: dberror.ErrDuplicateKey,
		},
		{
			name:        "duplicate key via message",
			err:         errors.New("Duplicate entry for key"),
			expectedErr: dberror.ErrDuplicateKey,
		},
		{
			name:        "foreign key violation",
			err:         errors.New("Error 1452: foreign key constraint fails"),
			expectedErr: dberror.ErrForeignKeyViolation,
		},
		{
			name:        "connection refused",
			err:         errors.New("connection refused"),
			expectedErr: dberror.ErrConnectionFailed,
		},
		{
			name:        "syntax error",
			err:         errors.New("Error 1064: syntax error"),
			expectedErr: dberror.ErrSyntaxError,
		},
		{
			name:        "unknown error",
			err:         errors.New("some other error"),
			expectedErr: nil,
		},
	}
	runErrorMapperTests(t, mapper.MapError, cases)
}

func TestPostgresErrorMapper(t *testing.T) {
	mapper := dberror.PostgresErrorMapper{}
	cases := []testErrorMapperCase{
		{
			name:        "nil error",
			err:         nil,
			expectedErr: nil,
		},
		{
			name:        "duplicate key via SQLSTATE 23505",
			err:         errors.New("ERROR: duplicate key value violates unique constraint"),
			expectedErr: dberror.ErrDuplicateKey,
		},
		{
			name:        "duplicate key via message",
			err:         errors.New("duplicate key value violates"),
			expectedErr: dberror.ErrDuplicateKey,
		},
		{
			name:        "foreign key violation",
			err:         errors.New("ERROR 23503: foreign key constraint violation"),
			expectedErr: dberror.ErrForeignKeyViolation,
		},
		{
			name:        "connection refused",
			err:         errors.New("connect: connection refused"),
			expectedErr: dberror.ErrConnectionFailed,
		},
		{
			name:        "syntax error via SQLSTATE 42601",
			err:         errors.New("ERROR 42601: syntax error"),
			expectedErr: dberror.ErrSyntaxError,
		},
		{
			name:        "unknown error",
			err:         errors.New("some other error"),
			expectedErr: nil,
		},
	}
	runErrorMapperTests(t, mapper.MapError, cases)
}

func TestSQLiteErrorMapper(t *testing.T) {
	mapper := dberror.SQLiteErrorMapper{}
	cases := []testErrorMapperCase{
		{
			name:        "nil error",
			err:         nil,
			expectedErr: nil,
		},
		{
			name:        "unique constraint failed",
			err:         errors.New("UNIQUE constraint failed: users.email"),
			expectedErr: dberror.ErrDuplicateKey,
		},
		{
			name:        "foreign key constraint failed",
			err:         errors.New("FOREIGN KEY constraint failed"),
			expectedErr: dberror.ErrForeignKeyViolation,
		},
		{
			name:        "unable to open database",
			err:         errors.New("unable to open database file"),
			expectedErr: dberror.ErrConnectionFailed,
		},
		{
			name:        "syntax error",
			err:         errors.New("syntax error in SELECT statement"),
			expectedErr: dberror.ErrSyntaxError,
		},
		{
			name:        "unknown error",
			err:         errors.New("some other error"),
			expectedErr: nil,
		},
	}
	runErrorMapperTests(t, mapper.MapError, cases)
}

func TestMSSQLErrorMapper(t *testing.T) {
	mapper := dberror.MSSQLErrorMapper{}
	cases := []testErrorMapperCase{
		{
			name:        "nil error",
			err:         nil,
			expectedErr: nil,
		},
		{
			name:        "primary key constraint via error code 2627",
			err:         errors.New("Violation of PRIMARY KEY constraint Msg 2627"),
			expectedErr: dberror.ErrDuplicateKey,
		},
		{
			name:        "duplicate key via error code 2601",
			err:         errors.New("Cannot insert duplicate key row with index [2601]"),
			expectedErr: dberror.ErrDuplicateKey,
		},
		{
			name: "foreign key constraint violation",
			err: errors.New("The INSERT, UPDATE, or DELETE statement conflicted " +
				"with a FOREIGN KEY constraint Msg 547"),
			expectedErr: dberror.ErrForeignKeyViolation,
		},
		{
			name:        "connection error",
			err:         errors.New("cannot open database"),
			expectedErr: dberror.ErrConnectionFailed,
		},
		{
			name:        "syntax error",
			err:         errors.New("Incorrect syntax near keyword SELECT"),
			expectedErr: dberror.ErrSyntaxError,
		},
		{
			name:        "unknown error",
			err:         errors.New("some other error"),
			expectedErr: nil,
		},
	}
	runErrorMapperTests(t, mapper.MapError, cases)
}

func TestGetMapper(t *testing.T) {
	testCases := []struct {
		dialect      string
		expectedType string
	}{
		{"mysql", "mysql"},
		{"postgres", "postgres"},
		{"postgresql", "postgres"}, // alias for postgres
		{"sqlite", "sqlite"},
		{"sqlserver", "mssql"},
		{"mssql", "mssql"},   // alias for sqlserver
		{"unknown", "mysql"}, // defaults to MySQL
	}

	for _, tc := range testCases {
		t.Run(tc.dialect, func(t *testing.T) {
			mapper := dberror.GetMapper(tc.dialect)
			// Check the type by checking which mapper it is
			switch tc.expectedType {
			case "mysql":
				assert.IsType(t, dberror.MySQLErrorMapper{}, mapper)
			case "postgres":
				assert.IsType(t, dberror.PostgresErrorMapper{}, mapper)
			case "sqlite":
				assert.IsType(t, dberror.SQLiteErrorMapper{}, mapper)
			case "mssql":
				assert.IsType(t, dberror.MSSQLErrorMapper{}, mapper)
			}
		})
	}
}

func TestErrorChaining(t *testing.T) {
	// Verify that errors are properly wrapped and can be checked with errors.Is()
	mapper := dberror.MySQLErrorMapper{}
	originalErr := errors.New("Error 1062: Duplicate entry for key 'email'")

	mappedErr := mapper.MapError(originalErr)

	// Should be able to check the sentinel error
	assert.ErrorIs(t, mappedErr, dberror.ErrDuplicateKey)

	// Should preserve the original error in the chain
	assert.ErrorIs(t, mappedErr, originalErr)
}
