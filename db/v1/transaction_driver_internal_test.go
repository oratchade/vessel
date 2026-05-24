//go:build test

package v1

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSQLQuerier struct{}

func (fakeSQLQuerier) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}

func (fakeSQLQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func (fakeSQLQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}

type fakePGQuerier struct{}

func (fakePGQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (fakePGQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func TestWithTransactionConcreteDriversReturnBeginErrors(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		db   interface {
			WithTransaction(context.Context, func(Tx) error, ...TransactionOptions) error
		}
		want string
	}{
		{
			name: "mysql",
			db: &MySQL{
				querier:    fakeSQLQuerier{},
				safeLogger: NewSafeLogger(nil),
			},
			want: "mysql.WithTransaction: failed to begin transaction",
		},
		{
			name: "postgres",
			db: &Postgres{
				querier:    fakePGQuerier{},
				safeLogger: NewSafeLogger(nil),
			},
			want: "postgres.WithTransaction: failed to begin transaction",
		},
		{
			name: "sqlite",
			db: &SQLITE{
				querier:    fakeSQLQuerier{},
				safeLogger: NewSafeLogger(nil),
			},
			want: "sqlite.WithTransaction: failed to begin transaction",
		},
		{
			name: "mssql",
			db: &MSSQL{
				querier:    fakeSQLQuerier{},
				safeLogger: NewSafeLogger(nil),
			},
			want: "mssql.WithTransaction: failed to begin transaction",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			err := tc.db.WithTransaction(ctx, func(Tx) error {
				called = true
				return nil
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.False(t, called)
		})
	}
}
