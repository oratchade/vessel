//go:build test

package v1

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/vessel/db/v1/dberror"
	"tounilab.com/vessel/pkg/query/definition"
)

var errReturningQuery = errors.New("returning query failed")

type recordingSQLQuerier struct {
	fakeSQLQuerier

	query string
}

func (r *recordingSQLQuerier) QueryContext(_ context.Context, query string, _ ...any) (*sql.Rows, error) {
	r.query = query
	return nil, errReturningQuery
}

type recordingPGQuerier struct {
	fakePGQuerier

	query string
}

func (r *recordingPGQuerier) Query(_ context.Context, query string, _ ...any) (pgx.Rows, error) {
	r.query = query
	return nil, errReturningQuery
}

func TestDriversExecReturningDialectGating(t *testing.T) {
	const query = `INSERT INTO "users" ("name") VALUES ($1) RETURNING "id";`
	mysqlQuerier, sqliteQuerier, mssqlQuerier := &recordingSQLQuerier{}, &recordingSQLQuerier{}, &recordingSQLQuerier{}
	pgQuerier := &recordingPGQuerier{}

	tests := []struct {
		name      string
		executor  ReturningExecutor
		executed  func() string
		supported bool
		wantErr   string
	}{
		{
			name: "mysql rejects without executing",
			executor: &MySQL{
				querier:     mysqlQuerier,
				safeLogger:  NewSafeLogger(nil),
				errorMapper: dberror.GetMapper(definition.DriverMySQL),
			},
			executed: func() string { return mysqlQuerier.query },
			wantErr:  "mysql.ExecReturning: mutation RETURNING/OUTPUT execution is not supported",
		},
		{
			name: "sqlite rejects without executing",
			executor: &SQLITE{
				querier:     sqliteQuerier,
				safeLogger:  NewSafeLogger(nil),
				errorMapper: dberror.GetMapper(definition.DriverSQLite),
			},
			executed: func() string { return sqliteQuerier.query },
			wantErr:  "sqlite.ExecReturning: mutation RETURNING/OUTPUT execution is not supported",
		},
		{
			name: "postgres executes as a row query",
			executor: &Postgres{
				querier:     pgQuerier,
				safeLogger:  NewSafeLogger(nil),
				errorMapper: dberror.GetMapper(definition.DriverPostgres),
			},
			executed:  func() string { return pgQuerier.query },
			supported: true,
			wantErr:   "postgres.QueryRaw: failed to execute query",
		},
		{
			name: "mssql executes as a row query",
			executor: &MSSQL{
				querier:     mssqlQuerier,
				safeLogger:  NewSafeLogger(nil),
				errorMapper: dberror.GetMapper(definition.DriverMSSQL),
			},
			executed:  func() string { return mssqlQuerier.query },
			supported: true,
			wantErr:   "mssql.QueryRaw: failed to execute query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := tt.executor.ExecReturning(context.Background(), query, "Ada")

			require.Error(t, err)
			assert.Nil(t, rows)
			assert.Contains(t, err.Error(), tt.wantErr)
			if tt.supported {
				assert.Equal(t, query, tt.executed())
			} else {
				assert.Empty(t, tt.executed())
			}
		})
	}
}
