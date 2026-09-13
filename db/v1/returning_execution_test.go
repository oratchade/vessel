//go:build test

package v1_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
	cdt "tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/pkg/query/options"
)

func TestMutationExecutionRejectsReturning(t *testing.T) {
	opts := &options.QueryOptions{Returning: []string{"id"}}

	for _, operation := range []string{"Insert", "Inserts", "Update", "Delete"} {
		t.Run(operation, func(t *testing.T) {
			err := v1.ExportRejectExecutingReturning(operation, opts)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "RETURNING/OUTPUT execution is not supported")
			assert.Contains(t, err.Error(), operation+"Query")
		})
	}
}

func TestMutationExecutionAllowsNilOrEmptyReturning(t *testing.T) {
	assert.NoError(t, v1.ExportRejectExecutingReturning("Insert", nil))
	assert.NoError(t, v1.ExportRejectExecutingReturning("Insert", &options.QueryOptions{}))
}

type returningActions struct {
	*v1.MockDBActions

	query string
	args  []any
	rows  *v1.RowsAdapter
	err   error
}

func (r *returningActions) ExecReturning(_ context.Context, query string, args ...any) (*v1.RowsAdapter, error) {
	r.query, r.args = query, args
	return r.rows, r.err
}

type returningTx struct {
	*v1.MockTx

	query string
	rows  *v1.RowsAdapter
}

func (r *returningTx) ExecReturning(_ context.Context, query string, _ ...any) (*v1.RowsAdapter, error) {
	r.query = query
	return r.rows, nil
}

func TestMutationBuildersExecReturning(t *testing.T) {
	const query = "MUTATION RETURNING"
	ctx := context.Background()
	args := []any{"Ada"}
	returningID := &options.QueryOptions{Returning: []string{"id"}}
	byID := cdt.NewExpr().Column("id").Op("=").Value(1)

	tests := []struct {
		name   string
		expect func(m *v1.MockDBActions)
		exec   func(f *v1.FluentDB) (*v1.RowsAdapter, error)
	}{
		{
			name: "insert",
			expect: func(m *v1.MockDBActions) {
				m.EXPECT().InsertQuery("users", map[string]any{"name": "Ada"}, returningID).Return(query, args, nil)
			},
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Insert().Into("users").Set("name", "Ada").Returning("id").ExecReturning(ctx)
			},
		},
		{
			name: "bulk insert",
			expect: func(m *v1.MockDBActions) {
				m.EXPECT().
					InsertsQuery("users", []map[string]any{{"name": "Ada"}, {"name": "Grace"}}, returningID).
					Return(query, args, nil)
			},
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Insert().
					Into("users").
					ValuesBulk([]map[string]any{{"name": "Ada"}, {"name": "Grace"}}).
					Returning("id").
					ExecReturning(ctx)
			},
		},
		{
			name: "upsert",
			expect: func(m *v1.MockDBActions) {
				m.EXPECT().
					UpsertQuery("users", map[string]any{"email": "ada@example.com", "name": "Ada"}, gomock.Any(), returningID).
					Return(query, args, nil)
			},
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Insert().
					Into("users").
					Set("email", "ada@example.com").
					Set("name", "Ada").
					OnConflict("email").
					DoUpdate("name").
					Returning("id").
					ExecReturning(ctx)
			},
		},
		{
			name: "update",
			expect: func(m *v1.MockDBActions) {
				m.EXPECT().
					UpdateQuery("users", map[string]any{"name": "Ada"}, nil, gomock.Any(), returningID).
					Return(query, args, nil)
			},
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Update("users").Set("name", "Ada").Where(byID).Returning("id").ExecReturning(ctx)
			},
		},
		{
			name: "delete",
			expect: func(m *v1.MockDBActions) {
				m.EXPECT().DeleteQuery("users", nil, gomock.Any(), returningID).Return(query, args, nil)
			},
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Delete().From("users").Where(byID).Returning("id").ExecReturning(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mock := v1.NewMockDBActions(ctrl)
			tt.expect(mock)
			actions := &returningActions{MockDBActions: mock, rows: &v1.RowsAdapter{}}

			rows, err := tt.exec(v1.NewFluentDB(actions))

			require.NoError(t, err)
			assert.Same(t, actions.rows, rows)
			assert.Equal(t, query, actions.query)
			assert.Equal(t, args, actions.args)
		})
	}
}

func TestMutationBuildersExecReturningErrors(t *testing.T) {
	ctx := context.Background()
	errExec := errors.New("exec failed")
	errBuild := errors.New("build failed")
	byID := cdt.NewExpr().Column("id").Op("=").Value(1)
	insertReturning := func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
		return f.Insert().Into("users").Set("name", "Ada").Returning("id").ExecReturning(ctx)
	}
	expectInsertQuery := func(sql string, err error) func(m *v1.MockDBActions) {
		return func(m *v1.MockDBActions) {
			m.EXPECT().InsertQuery("users", gomock.Any(), gomock.Any()).Return(sql, nil, err)
		}
	}

	tests := []struct {
		name            string
		withoutExecutor bool
		execErr         error
		expect          func(m *v1.MockDBActions)
		exec            func(f *v1.FluentDB) (*v1.RowsAdapter, error)
		wantErr         string
		wantIs          error
	}{
		{
			name: "insert without Returning",
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Insert().Into("users").Set("name", "Ada").ExecReturning(ctx)
			},
			wantErr: "InsertBuilder.ExecReturning: Returning columns not specified",
		},
		{
			name: "update without Returning",
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Update("users").Set("name", "Ada").Where(byID).ExecReturning(ctx)
			},
			wantErr: "UpdateBuilder.ExecReturning: Returning columns not specified",
		},
		{
			name: "delete without Returning",
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Delete().From("users").Where(byID).ExecReturning(ctx)
			},
			wantErr: "DeleteBuilder.ExecReturning: Returning columns not specified",
		},
		{
			name: "update without WHERE",
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Update("users").Set("name", "Ada").Returning("id").ExecReturning(ctx)
			},
			wantErr: "UpdateBuilder.ExecReturning: WHERE condition required",
		},
		{
			name: "delete without WHERE",
			exec: func(f *v1.FluentDB) (*v1.RowsAdapter, error) {
				return f.Delete().From("users").Returning("id").ExecReturning(ctx)
			},
			wantErr: "DeleteBuilder.ExecReturning: WHERE condition required",
		},
		{
			name:    "query build failure",
			expect:  expectInsertQuery("", errBuild),
			exec:    insertReturning,
			wantErr: "InsertBuilder.ExecReturning:",
			wantIs:  errBuild,
		},
		{
			name:            "DBActions without ReturningExecutor",
			withoutExecutor: true,
			expect:          expectInsertQuery("INSERT RETURNING", nil),
			exec:            insertReturning,
			wantErr:         "does not implement ReturningExecutor",
		},
		{
			name:    "executor failure",
			execErr: errExec,
			expect:  expectInsertQuery("INSERT RETURNING", nil),
			exec:    insertReturning,
			wantErr: "InsertBuilder.ExecReturning: failed to execute mutation",
			wantIs:  errExec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mock := v1.NewMockDBActions(ctrl)
			if tt.expect != nil {
				tt.expect(mock)
			}
			fake := &returningActions{MockDBActions: mock, err: tt.execErr}
			var actions v1.DBActions = fake
			if tt.withoutExecutor {
				actions = mock
			}

			rows, err := tt.exec(v1.NewFluentDB(actions))

			require.Error(t, err)
			assert.Nil(t, rows)
			assert.Contains(t, err.Error(), tt.wantErr)
			if tt.wantIs != nil {
				assert.ErrorIs(t, err, tt.wantIs)
			}
			if tt.execErr == nil {
				assert.Empty(t, fake.query, "statement must not execute")
			}
		})
	}
}

func TestMutationBuilderExecReturningUsesTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	tx := &returningTx{MockTx: v1.NewMockTx(ctrl), rows: &v1.RowsAdapter{}}
	tx.EXPECT().
		UpdateQuery("users", map[string]any{"name": "Ada"}, nil, gomock.Any(), gomock.Any()).
		Return("UPDATE RETURNING", []any{"Ada"}, nil)
	database := &returningActions{MockDBActions: v1.NewMockDBActions(ctrl)}

	rows, err := v1.NewFluentDB(database).
		Update("users").
		WithTx(tx).
		Set("name", "Ada").
		Where(cdt.NewExpr().Column("id").Op("=").Value(1)).
		Returning("id").
		ExecReturning(context.Background())

	require.NoError(t, err)
	assert.Same(t, tx.rows, rows)
	assert.Equal(t, "UPDATE RETURNING", tx.query)
	assert.Empty(t, database.query)
}
