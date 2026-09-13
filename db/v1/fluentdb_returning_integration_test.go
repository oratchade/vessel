//go:build integration

package v1_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
	cdt "tounilab.com/vessel/pkg/query/condition"
)

type returningMatrixUser struct {
	ID     int64
	Name   string
	Email  string
	Status string
}

func requireReturnedUsers(t *testing.T, rows *v1.RowsAdapter, err error) []returningMatrixUser {
	t.Helper()
	require.NoError(t, err)
	users, err := v1.ScanAll[returningMatrixUser](context.Background(), rows)
	require.NoError(t, err)
	return users
}

func TestFluentDBExecReturningMatrix(t *testing.T) {
	for _, testDB := range fluentMatrixDatabases() {
		t.Run(testDB.name, func(t *testing.T) {
			database := connectFluentMatrixDB(t, testDB)
			defer func() { _ = database.Close() }()
			setupFluentMatrixSchema(t, database, testDB.driver)

			ctx := context.Background()
			fluent := v1.NewFluentDB(database)
			byEmail := func(email string) cdt.Condition {
				return cdt.NewExpr().Column("email").Op("=").Value(email)
			}
			insert := fluent.Insert().
				Into(fluentMatrixUsersTable).
				Set("name", "Returning").
				Set("email", "returning.matrix@example.com").
				Set("age", 30).
				Set("status", "active").
				Returning("id", "name", "email", "status")

			if testDB.driver == "mysql" || testDB.driver == "sqlite" {
				rows, err := insert.ExecReturning(ctx)
				require.Error(t, err)
				assert.Nil(t, rows)
				assert.Contains(t, err.Error(), "not supported")

				existing, err := fluent.Select(fluentMatrixUsersTable, "id").
					Where(byEmail("returning.matrix@example.com")).
					Get(ctx)
				require.NoError(t, err)
				assert.Empty(t, existing, "unsupported dialect must not execute the statement")
				return
			}

			rows, err := insert.ExecReturning(ctx)
			inserted := requireReturnedUsers(t, rows, err)
			require.Len(t, inserted, 1)
			assert.Positive(t, inserted[0].ID)
			assert.Equal(t, "returning.matrix@example.com", inserted[0].Email)
			assert.Equal(t, "active", inserted[0].Status)
			byID := cdt.NewExpr().Column("id").Op("=").Value(inserted[0].ID)

			rows, err = fluent.Update(fluentMatrixUsersTable).
				Set("status", "inactive").
				Where(byID).
				Returning("id", "status").
				ExecReturning(ctx)
			updated := requireReturnedUsers(t, rows, err)
			require.Len(t, updated, 1)
			assert.Equal(t, inserted[0].ID, updated[0].ID)
			assert.Equal(t, "inactive", updated[0].Status)

			if testDB.driver == "postgres" {
				rows, err = fluent.Insert().
					Into(fluentMatrixUsersTable).
					Set("name", "Returning Upserted").
					Set("email", "returning.matrix@example.com").
					Set("age", 31).
					Set("status", "active").
					OnConflict("email").
					DoUpdate("name").
					Returning("id", "name").
					ExecReturning(ctx)
				upserted := requireReturnedUsers(t, rows, err)
				require.Len(t, upserted, 1)
				assert.Equal(t, inserted[0].ID, upserted[0].ID)
				assert.Equal(t, "Returning Upserted", upserted[0].Name)
			}

			errRollback := errors.New("rollback")
			err = database.WithTransaction(ctx, func(tx v1.Tx) error {
				rows, err := fluent.Insert().
					WithTx(tx).
					Into(fluentMatrixUsersTable).
					Set("name", "Returning Tx").
					Set("email", "returning.tx.matrix@example.com").
					Set("age", 40).
					Set("status", "active").
					Returning("id", "email").
					ExecReturning(ctx)
				users := requireReturnedUsers(t, rows, err)
				require.Len(t, users, 1)
				return errRollback
			})
			require.Error(t, err)
			rolledBack, err := fluent.Select(fluentMatrixUsersTable, "id").
				Where(byEmail("returning.tx.matrix@example.com")).
				Get(ctx)
			require.NoError(t, err)
			assert.Empty(t, rolledBack, "ExecReturning with WithTx must run inside the transaction")

			rows, err = fluent.Delete().
				From(fluentMatrixUsersTable).
				Where(byID).
				Returning("id").
				ExecReturning(ctx)
			deleted := requireReturnedUsers(t, rows, err)
			require.Len(t, deleted, 1)
			assert.Equal(t, inserted[0].ID, deleted[0].ID)
		})
	}
}
