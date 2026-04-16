//go:build test

package v1_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "tounilab.com/fabric/db/v1"
	cdt "tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/options"
)

// TestNewFluentDB tests FluentDB creation
func TestNewFluentDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)

	fluentDB := v1.NewFluentDB(db)

	assert.NotNil(t, fluentDB)
}

// TestSelectBuilderInitialization tests that Select properly initializes SelectBuilder
func TestSelectBuilderInitialization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name    string
		table   string
		columns []string
		check   func(*testing.T, []string)
	}{
		{
			name:    "no columns defaults to all",
			table:   "users",
			columns: []string{},
			check: func(t *testing.T, cols []string) {
				assert.Equal(t, []string{"*"}, cols)
			},
		},
		{
			name:    "single column",
			table:   "users",
			columns: []string{"id"},
			check: func(t *testing.T, cols []string) {
				assert.Equal(t, []string{"id"}, cols)
			},
		},
		{
			name:    "multiple columns",
			table:   "users",
			columns: []string{"id", "name", "email"},
			check: func(t *testing.T, cols []string) {
				assert.Equal(t, []string{"id", "name", "email"}, cols)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Select(tc.table, tc.columns...)
			assert.NotNil(t, builder)

			// Get the columns from the built query by checking the internal state
			// We'll do this by attempting a Get(context.Background()) and observing the call
			db.EXPECT().Get(context.Background(), tc.table, gomock.Any(), nil, nil, nil).
				DoAndReturn(func(ctx context.Context, table string, cols []string, joins []cdt.Join, cond cdt.Condition, opts *options.QueryOptions) ([]map[string]any, error) { //nolint:lll
					tc.check(t, cols)
					return []map[string]any{}, nil
				}).Times(1)

			_, _ = builder.Get(context.Background())
		})
	}
}

// TestSelectBuilderChaining tests that SelectBuilder methods return the builder for chaining
func TestSelectBuilderChaining(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Select("users", "id", "name")

	// Test chaining returns the same builder
	result := builder.OrderBy("id", "ASC").Limit(10).Offset(5)
	assert.Equal(t, builder, result)
}

// TestSelectBuilderWhere tests WHERE condition building
func TestSelectBuilderWhere(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name       string
		conditions []cdt.Condition
		expectCall bool
		checkCond  func(*testing.T, cdt.Condition)
	}{
		{
			name:       "single where condition",
			conditions: []cdt.Condition{cdt.NewExpr().Column("id").Op("=").Value(1)},
			expectCall: true,
			checkCond: func(t *testing.T, cond cdt.Condition) {
				assert.NotNil(t, cond)
			},
		},
		{
			name:       "multiple where conditions combine with AND",
			conditions: []cdt.Condition{cdt.NewExpr().Column("id").Op("=").Value(1), cdt.NewExpr().Column("name").Op("=").Value("John")}, //nolint:lll
			expectCall: true,
			checkCond: func(t *testing.T, cond cdt.Condition) {
				assert.NotNil(t, cond)
			},
		},
		{
			name:       "nil condition is ignored",
			conditions: []cdt.Condition{nil},
			expectCall: true,
			checkCond: func(t *testing.T, cond cdt.Condition) {
				assert.Nil(t, cond)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Select("users")
			for _, c := range tc.conditions {
				builder = builder.Where(c)
			}

			if tc.expectCall {
				db.EXPECT().Get(context.Background(), "users", gomock.Any(), nil, gomock.Any(), nil).
					DoAndReturn(func(ctx context.Context, table string, cols []string, joins []cdt.Join, cond cdt.Condition, opts *options.QueryOptions) ([]map[string]any, error) { //nolint:lll
						tc.checkCond(t, cond)
						return []map[string]any{}, nil
					}).Times(1)
				_, _ = builder.Get(context.Background())
			}
		})
	}
}

// TestSelectBuilderJoins tests JOIN building
func TestSelectBuilderJoins(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name        string
		joins       []cdt.Join
		expectCount int
	}{
		{
			name: "single join",
			joins: []cdt.Join{
				{Type: "INNER", Table: "orders", Conditions: cdt.JoinCdts{{Left: "users.id", Right: "orders.user_id"}}},
			},
			expectCount: 1,
		},
		{
			name: "multiple joins",
			joins: []cdt.Join{
				{Type: "INNER", Table: "orders", Conditions: cdt.JoinCdts{{Left: "users.id", Right: "orders.user_id"}}},
				{Type: "LEFT", Table: "products", Conditions: cdt.JoinCdts{{Left: "orders.product_id", Right: "products.id"}}}, //nolint:lll
			},
			expectCount: 2,
		},
		{
			name:        "empty joins list",
			joins:       []cdt.Join{},
			expectCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Select("users")

			if len(tc.joins) > 0 {
				builder = builder.Joins(tc.joins)
			}

			db.EXPECT().Get(context.Background(), "users", gomock.Any(), gomock.Any(), nil, nil).
				DoAndReturn(func(ctx context.Context, table string, cols []string, joins []cdt.Join, cond cdt.Condition, opts *options.QueryOptions) ([]map[string]any, error) { //nolint:lll
					assert.Len(t, joins, tc.expectCount)
					return []map[string]any{}, nil
				}).Times(1)

			_, _ = builder.Get(context.Background())
		})
	}
}

// TestSelectBuilderOrderBy tests ORDER BY building
func TestSelectBuilderOrderBy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name            string
		orders          []struct{ col, dir string }
		expectedOrderBy []options.OrderBy
	}{
		{
			name:   "single order by ASC",
			orders: []struct{ col, dir string }{{"id", "ASC"}},
			expectedOrderBy: []options.OrderBy{
				{Column: "id", Direction: "ASC"},
			},
		},
		{
			name:   "single order by DESC",
			orders: []struct{ col, dir string }{{"id", "DESC"}},
			expectedOrderBy: []options.OrderBy{
				{Column: "id", Direction: "DESC"},
			},
		},
		{
			name: "multiple order by",
			orders: []struct{ col, dir string }{
				{"id", "ASC"},
				{"name", "DESC"},
			},
			expectedOrderBy: []options.OrderBy{
				{Column: "id", Direction: "ASC"},
				{Column: "name", Direction: "DESC"},
			},
		},
		{
			name:   "empty direction defaults to ASC",
			orders: []struct{ col, dir string }{{"id", ""}},
			expectedOrderBy: []options.OrderBy{
				{Column: "id", Direction: "ASC"},
			},
		},
		{
			name:   "lowercase direction normalized to uppercase",
			orders: []struct{ col, dir string }{{"id", "desc"}},
			expectedOrderBy: []options.OrderBy{
				{Column: "id", Direction: "DESC"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Select("users")
			for _, o := range tc.orders {
				builder = builder.OrderBy(o.col, o.dir)
			}

			db.EXPECT().Get(context.Background(), "users", gomock.Any(), nil, nil, gomock.Any()).
				DoAndReturn(func(ctx context.Context, table string, cols []string, joins []cdt.Join, cond cdt.Condition, opts *options.QueryOptions) ([]map[string]any, error) { //nolint:lll
					assert.NotNil(t, opts)
					assert.Equal(t, tc.expectedOrderBy, opts.OrderBy)
					return []map[string]any{}, nil
				}).Times(1)

			_, _ = builder.Get(context.Background())
		})
	}
}

// TestSelectBuilderLimitOffset tests LIMIT and OFFSET
func TestSelectBuilderLimitOffset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name           string
		limit          *int
		offset         *int
		expectedLimit  *int
		expectedOffset *int
	}{
		{
			name:           "no limit or offset",
			limit:          nil,
			offset:         nil,
			expectedLimit:  nil,
			expectedOffset: nil,
		},
		{
			name:           "with limit",
			limit:          intPtr(10),
			offset:         nil,
			expectedLimit:  intPtr(10),
			expectedOffset: nil,
		},
		{
			name:           "with offset",
			limit:          nil,
			offset:         intPtr(5),
			expectedLimit:  nil,
			expectedOffset: intPtr(5),
		},
		{
			name:           "with limit and offset",
			limit:          intPtr(10),
			offset:         intPtr(5),
			expectedLimit:  intPtr(10),
			expectedOffset: intPtr(5),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Select("users")
			if tc.limit != nil {
				builder = builder.Limit(*tc.limit)
			}
			if tc.offset != nil {
				builder = builder.Offset(*tc.offset)
			}

			db.EXPECT().Get(context.Background(), "users", gomock.Any(), nil, nil, gomock.Any()).
				DoAndReturn(func(ctx context.Context, table string, cols []string, joins []cdt.Join, cond cdt.Condition, opts *options.QueryOptions) ([]map[string]any, error) { //nolint:lll
					if tc.expectedLimit == nil {
						if opts != nil {
							assert.Nil(t, opts.Limit)
						}
					} else {
						assert.NotNil(t, opts)
						assert.Equal(t, *tc.expectedLimit, *opts.Limit)
					}
					if tc.expectedOffset == nil {
						if opts != nil {
							assert.Nil(t, opts.Offset)
						}
					} else {
						assert.NotNil(t, opts)
						assert.Equal(t, *tc.expectedOffset, *opts.Offset)
					}
					return []map[string]any{}, nil
				}).Times(1)

			_, _ = builder.Get(context.Background())
		})
	}
}

// TestSelectBuilderValidation tests query option validation at execution time
func TestSelectBuilderValidation(t *testing.T) {
	// Test that ExportValidateQueryOptions properly rejects invalid directions
	opts := &options.QueryOptions{
		OrderBy: []options.OrderBy{
			{Column: "id", Direction: "INVALID"},
		},
	}
	err := v1.ExportValidateQueryOptions(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid direction")

	// Test that negative Limit is rejected
	opts = &options.QueryOptions{
		Limit: ptrInt(-1),
	}
	err = v1.ExportValidateQueryOptions(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Limit")
	require.Contains(t, err.Error(), "cannot be negative")

	// Test that negative Offset is rejected
	opts = &options.QueryOptions{
		Offset: ptrInt(-5),
	}
	err = v1.ExportValidateQueryOptions(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Offset")
	require.Contains(t, err.Error(), "cannot be negative")

	// Test that zero values are allowed
	opts = &options.QueryOptions{
		Limit:  ptrInt(0),
		Offset: ptrInt(0),
	}
	err = v1.ExportValidateQueryOptions(opts)
	require.NoError(t, err)

	// Test that valid ASC/DESC directions are allowed
	opts = &options.QueryOptions{
		OrderBy: []options.OrderBy{
			{Column: "id", Direction: "ASC"},
			{Column: "name", Direction: "DESC"},
		},
	}
	err = v1.ExportValidateQueryOptions(opts)
	require.NoError(t, err)

	// Test that nil options are allowed
	err = v1.ExportValidateQueryOptions(nil)
	require.NoError(t, err)
}

// ptrInt is a helper to create an int pointer
func ptrInt(v int) *int {
	return &v
}

// TestSelectBuilderGet tests Get execution
func TestSelectBuilderGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name         string
		table        string
		mockReturn   []map[string]any
		mockError    error
		expectError  bool
		expectedRows []map[string]any
	}{
		{
			name:         "successful get",
			table:        "users",
			mockReturn:   []map[string]any{{"id": 1, "name": "John"}},
			mockError:    nil,
			expectError:  false,
			expectedRows: []map[string]any{{"id": 1, "name": "John"}},
		},
		{
			name:         "empty result",
			table:        "users",
			mockReturn:   []map[string]any{},
			mockError:    nil,
			expectError:  false,
			expectedRows: []map[string]any{},
		},
		{
			name:         "database error",
			table:        "users",
			mockReturn:   nil,
			mockError:    errors.New("database error"),
			expectError:  true,
			expectedRows: nil,
		},
		{
			name:         "no table specified error",
			table:        "",
			mockReturn:   nil,
			mockError:    nil,
			expectError:  true,
			expectedRows: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Select(tc.table, "id", "name")

			if tc.table != "" {
				db.EXPECT().Get(context.Background(), tc.table, gomock.Any(), nil, nil, nil).
					Return(tc.mockReturn, tc.mockError).
					Times(1)
			}

			rows, err := builder.Get(context.Background())

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedRows, rows)
			}
		})
	}
}

// TestSelectBuilderOne tests One execution
func TestSelectBuilderOne(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name        string
		table       string
		mockReturn  []map[string]any
		mockError   error
		expectError bool
		expectedRow map[string]any
	}{
		{
			name:        "successful one",
			table:       "users",
			mockReturn:  []map[string]any{{"id": 1, "name": "John"}},
			mockError:   nil,
			expectError: false,
			expectedRow: map[string]any{"id": 1, "name": "John"},
		},
		{
			name:        "no rows found",
			table:       "users",
			mockReturn:  []map[string]any{},
			mockError:   nil,
			expectError: true,
			expectedRow: nil,
		},
		{
			name:        "database error",
			table:       "users",
			mockReturn:  nil,
			mockError:   errors.New("database error"),
			expectError: true,
			expectedRow: nil,
		},
		{
			name:        "no table specified",
			table:       "",
			mockReturn:  nil,
			mockError:   nil,
			expectError: true,
			expectedRow: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Select(tc.table, "id", "name")

			if tc.table != "" {
				db.EXPECT().Get(context.Background(), tc.table, gomock.Any(), nil, nil, gomock.Any()).
					Return(tc.mockReturn, tc.mockError).
					Times(1)
			}

			row, err := builder.One(context.Background())

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, row)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedRow, row)
			}
		})
	}
}

// TestSelectBuilderCount tests Count execution
func TestSelectBuilderCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name          string
		table         string
		mockReturn    []map[string]any
		mockError     error
		expectError   bool
		expectedCount int64
	}{
		{
			name:          "count returns int64",
			table:         "users",
			mockReturn:    []map[string]any{{"count": int64(5)}},
			mockError:     nil,
			expectError:   false,
			expectedCount: 5,
		},
		{
			name:          "count returns int",
			table:         "users",
			mockReturn:    []map[string]any{{"count": 5}},
			mockError:     nil,
			expectError:   false,
			expectedCount: 5,
		},
		{
			name:          "count returns float64",
			table:         "users",
			mockReturn:    []map[string]any{{"count": 5.0}},
			mockError:     nil,
			expectError:   false,
			expectedCount: 5,
		},
		{
			name:          "empty result",
			table:         "users",
			mockReturn:    []map[string]any{},
			mockError:     nil,
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:          "database error",
			table:         "users",
			mockReturn:    nil,
			mockError:     errors.New("database error"),
			expectError:   true,
			expectedCount: 0,
		},
		{
			name:          "no table specified",
			table:         "",
			mockReturn:    nil,
			mockError:     nil,
			expectError:   true,
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Select(tc.table)

			if tc.table != "" {
				db.EXPECT().Get(context.Background(), tc.table, gomock.Any(), nil, nil, nil).
					Return(tc.mockReturn, tc.mockError).
					Times(1)
			}

			count, err := builder.Count(context.Background())

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCount, count)
			}
		})
	}
}

// TestSelectBuilderWithTx tests transaction support
func TestSelectBuilderWithTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	tx := v1.NewMockTx(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Select("users")
	result := builder.WithTx(tx)

	assert.Equal(t, builder, result)

	// Verify that subsequent Get(context.Background()) call uses tx instead of db
	tx.EXPECT().Get(context.Background(), "users", gomock.Any(), nil, nil, nil).
		Return([]map[string]any{}, nil).
		Times(1)

	_, _ = result.Get(context.Background())
}

// TestInsertBuilderInitialization tests that Insert properly initializes InsertBuilder
func TestInsertBuilderInitialization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Insert()
	assert.NotNil(t, builder)
}

// TestInsertBuilderChaining tests that InsertBuilder methods return the builder for chaining
func TestInsertBuilderChaining(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Insert()
	result := builder.Into("users").Set("name", "John").Set("age", 30)
	assert.Equal(t, builder, result)
}

// TestInsertBuilderInto tests Into method
func TestInsertBuilderInto(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name  string
		table string
	}{
		{"users table", "users"},
		{"products table", "products"},
		{"empty table", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Insert().Into(tc.table)
			assert.NotNil(t, builder)
		})
	}
}

// TestInsertBuilderValues tests Values method
func TestInsertBuilderValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name         string
		data         map[string]any
		expectedKeys []string
	}{
		{
			name:         "single field",
			data:         map[string]any{"name": "John"},
			expectedKeys: []string{"name"},
		},
		{
			name:         "multiple fields",
			data:         map[string]any{"name": "John", "age": 30, "email": "john@example.com"},
			expectedKeys: []string{"name", "age", "email"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Insert().Into("users").Values(tc.data)

			db.EXPECT().Insert(context.Background(), "users", gomock.Any(), nil).
				DoAndReturn(func(ctx context.Context, table string, data map[string]any, opts *options.QueryOptions) (*v1.ExecResult, error) { //nolint:lll
					assert.Equal(t, "John", data["name"])
					if len(tc.expectedKeys) > 1 {
						assert.Equal(t, 30, data["age"])
						assert.Equal(t, "john@example.com", data["email"])
					}
					return &v1.ExecResult{RowsAffected: 1}, nil
				}).Times(1)

			_, _ = builder.Exec(context.Background())
		})
	}
}

// TestInsertBuilderSetMap tests SetMap method
func TestInsertBuilderSetMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	dataMap := map[string]any{
		"name":  "John",
		"age":   30,
		"email": "john@example.com",
	}

	builder := fluentDB.Insert().Into("users").SetMap(dataMap)

	db.EXPECT().Insert(context.Background(), "users", gomock.Any(), nil).
		DoAndReturn(func(ctx context.Context, table string, data map[string]any, opts *options.QueryOptions) (*v1.ExecResult, error) { //nolint:lll
			assert.Equal(t, "John", data["name"])
			assert.Equal(t, 30, data["age"])
			assert.Equal(t, "john@example.com", data["email"])
			return &v1.ExecResult{RowsAffected: 1}, nil
		}).Times(1)

	_, _ = builder.Exec(context.Background())
}

// TestInsertBuilderValuesBulk tests ValuesBulk method
func TestInsertBuilderValuesBulk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	data := []map[string]any{
		{"name": "John", "age": 30},
		{"name": "Jane", "age": 28},
		{"name": "Bob", "age": 35},
	}

	builder := fluentDB.Insert().Into("users").ValuesBulk(data)

	db.EXPECT().Inserts(context.Background(), "users", gomock.Any(), nil).
		DoAndReturn(func(ctx context.Context, table string, bulkData []map[string]any, opts *options.QueryOptions) (*v1.ExecResult, error) { //nolint:lll
			assert.Len(t, bulkData, 3)
			assert.Equal(t, "John", bulkData[0]["name"])
			assert.Equal(t, "Jane", bulkData[1]["name"])
			assert.Equal(t, "Bob", bulkData[2]["name"])
			return &v1.ExecResult{RowsAffected: 3}, nil
		}).Times(1)

	_, _ = builder.Exec(context.Background())
}

// TestInsertBuilderExec tests Exec execution
func TestInsertBuilderExec(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name          string
		setup         func(*v1.InsertBuilder) *v1.InsertBuilder
		mockSetup     func()
		expectError   bool
		expectedError string
	}{
		{
			name: "successful insert",
			setup: func(b *v1.InsertBuilder) *v1.InsertBuilder {
				return b.Into("users").Set("name", "John")
			},
			mockSetup: func() {
				db.EXPECT().Insert(context.Background(), "users", gomock.Any(), nil).
					Return(&v1.ExecResult{RowsAffected: 1}, nil).
					Times(1)
			},
			expectError: false,
		},
		{
			name: "no table specified",
			setup: func(b *v1.InsertBuilder) *v1.InsertBuilder {
				return b.Set("name", "John")
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "table not specified",
		},
		{
			name: "no data provided",
			setup: func(b *v1.InsertBuilder) *v1.InsertBuilder {
				return b.Into("users")
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "no data provided",
		},
		{
			name: "insert error",
			setup: func(b *v1.InsertBuilder) *v1.InsertBuilder {
				return b.Into("users").Set("name", "John")
			},
			mockSetup: func() {
				db.EXPECT().Insert(context.Background(), "users", gomock.Any(), nil).
					Return(nil, errors.New("insert failed")).
					Times(1)
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()
			builder := tc.setup(fluentDB.Insert())
			result, err := builder.Exec(context.Background())

			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != "" {
					assert.Contains(t, err.Error(), tc.expectedError)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestInsertBuilderWithTx tests transaction support
func TestInsertBuilderWithTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	tx := v1.NewMockTx(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Insert()
	result := builder.WithTx(tx)

	assert.Equal(t, builder, result)

	// Verify that subsequent Exec(context.Background()) call uses tx instead of db
	tx.EXPECT().Insert(context.Background(), "users", gomock.Any(), nil).
		Return(&v1.ExecResult{RowsAffected: 1}, nil).
		Times(1)

	_, _ = result.Into("users").Set("name", "John").Exec(context.Background())
}

// TestUpdateBuilderInitialization tests that Update properly initializes UpdateBuilder
func TestUpdateBuilderInitialization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Update("users")
	assert.NotNil(t, builder)
}

// TestUpdateBuilderChaining tests that UpdateBuilder methods return the builder for chaining
func TestUpdateBuilderChaining(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Update("users")
	result := builder.Set("name", "John").Where(cdt.NewExpr().Column("id").Op("=").Value(1))
	assert.Equal(t, builder, result)
}

// TestUpdateBuilderSet tests Set method
func TestUpdateBuilderSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Update("users").
		Set("name", "John").
		Set("age", 30).
		Where(cdt.NewExpr().Column("id").Op("=").Value(1))

	db.EXPECT().Update(context.Background(), "users", gomock.Any(), nil, gomock.Any(), nil).
		DoAndReturn(func(ctx context.Context, table string, data map[string]any, joins []cdt.Join, cond cdt.Condition, opts *options.QueryOptions) (*v1.ExecResult, error) { //nolint:lll
			assert.Equal(t, "John", data["name"])
			assert.Equal(t, 30, data["age"])
			return &v1.ExecResult{RowsAffected: 1}, nil
		}).Times(1)

	_, _ = builder.Exec(context.Background())
}

// TestUpdateBuilderSetMap tests SetMap method
func TestUpdateBuilderSetMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	dataMap := map[string]any{
		"name": "John",
		"age":  30,
	}

	builder := fluentDB.Update("users").
		SetMap(dataMap).
		Where(cdt.NewExpr().Column("id").Op("=").Value(1))

	db.EXPECT().Update(context.Background(), "users", gomock.Any(), nil, gomock.Any(), nil).
		DoAndReturn(func(ctx context.Context, table string, data map[string]any, joins []cdt.Join, cond cdt.Condition, opts *options.QueryOptions) (*v1.ExecResult, error) { //nolint:lll
			assert.Equal(t, "John", data["name"])
			assert.Equal(t, 30, data["age"])
			return &v1.ExecResult{RowsAffected: 1}, nil
		}).Times(1)

	_, _ = builder.Exec(context.Background())
}

// TestUpdateBuilderWhere tests WHERE conditions
func TestUpdateBuilderWhere(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name        string
		conditions  []cdt.Condition
		shouldError bool
	}{
		{
			name:        "single where condition",
			conditions:  []cdt.Condition{cdt.NewExpr().Column("id").Op("=").Value(1)},
			shouldError: false,
		},
		{
			name:        "no where condition",
			conditions:  []cdt.Condition{},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Update("users").Set("name", "John")
			for _, c := range tc.conditions {
				builder = builder.Where(c)
			}

			if !tc.shouldError {
				db.EXPECT().Update(context.Background(), "users", gomock.Any(), nil, gomock.Any(), nil).
					Return(&v1.ExecResult{RowsAffected: 1}, nil).
					Times(1)
			}

			_, err := builder.Exec(context.Background())

			if tc.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "WHERE condition required")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestUpdateBuilderExec tests Exec execution
func TestUpdateBuilderExec(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name          string
		setup         func(*v1.UpdateBuilder) *v1.UpdateBuilder
		mockSetup     func()
		expectError   bool
		expectedError string
	}{
		{
			name: "successful update",
			setup: func(b *v1.UpdateBuilder) *v1.UpdateBuilder {
				return b.Set("name", "John").Where(cdt.NewExpr().Column("id").Op("=").Value(1))
			},
			mockSetup: func() {
				db.EXPECT().Update(context.Background(), "users", gomock.Any(), nil, gomock.Any(), nil).
					Return(&v1.ExecResult{RowsAffected: 1}, nil).
					Times(1)
			},
			expectError: false,
		},
		{
			name: "no table specified",
			setup: func(b *v1.UpdateBuilder) *v1.UpdateBuilder {
				return fluentDB.Update("").Set("name", "John").Where(cdt.NewExpr().Column("id").Op("=").Value(1))
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "table not specified",
		},
		{
			name: "no data to update",
			setup: func(b *v1.UpdateBuilder) *v1.UpdateBuilder {
				return b.Where(cdt.NewExpr().Column("id").Op("=").Value(1))
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "no data to update",
		},
		{
			name: "no where condition",
			setup: func(b *v1.UpdateBuilder) *v1.UpdateBuilder {
				return b.Set("name", "John")
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "WHERE condition required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()
			builder := tc.setup(fluentDB.Update("users"))
			result, err := builder.Exec(context.Background())

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestUpdateBuilderWithTx tests transaction support
func TestUpdateBuilderWithTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	tx := v1.NewMockTx(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Update("users")
	result := builder.WithTx(tx)

	assert.Equal(t, builder, result)

	// Verify that subsequent Exec(context.Background()) call uses tx instead of db
	tx.EXPECT().Update(context.Background(), "users", gomock.Any(), nil, gomock.Any(), nil).
		Return(&v1.ExecResult{RowsAffected: 1}, nil).
		Times(1)

	_, _ = result.Set("name", "John").Where(cdt.NewExpr().Column("id").Op("=").Value(1)).Exec(context.Background())
}

// TestDeleteBuilderInitialization tests that Delete properly initializes DeleteBuilder
func TestDeleteBuilderInitialization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Delete()
	assert.NotNil(t, builder)
}

// TestDeleteBuilderChaining tests that DeleteBuilder methods return the builder for chaining
func TestDeleteBuilderChaining(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Delete()
	result := builder.From("users").Where(cdt.NewExpr().Column("id").Op("=").Value(1))
	assert.Equal(t, builder, result)
}

// TestDeleteBuilderFrom tests From method
func TestDeleteBuilderFrom(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Delete().From("users")
	assert.NotNil(t, builder)
}

// TestDeleteBuilderWhere tests WHERE conditions
func TestDeleteBuilderWhere(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name        string
		conditions  []cdt.Condition
		shouldError bool
	}{
		{
			name:        "single where condition",
			conditions:  []cdt.Condition{cdt.NewExpr().Column("id").Op("=").Value(1)},
			shouldError: false,
		},
		{
			name:        "no where condition",
			conditions:  []cdt.Condition{},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fluentDB.Delete().From("users")
			for _, c := range tc.conditions {
				builder = builder.Where(c)
			}

			if !tc.shouldError {
				db.EXPECT().Delete(context.Background(), "users", nil, gomock.Any(), nil).
					Return(&v1.ExecResult{RowsAffected: 1}, nil).
					Times(1)
			}

			_, err := builder.Exec(context.Background())

			if tc.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "WHERE condition required")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDeleteBuilderExec tests Exec execution
func TestDeleteBuilderExec(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name          string
		setup         func(*v1.DeleteBuilder) *v1.DeleteBuilder
		mockSetup     func()
		expectError   bool
		expectedError string
	}{
		{
			name: "successful delete",
			setup: func(b *v1.DeleteBuilder) *v1.DeleteBuilder {
				return b.From("users").Where(cdt.NewExpr().Column("id").Op("=").Value(1))
			},
			mockSetup: func() {
				db.EXPECT().Delete(context.Background(), "users", nil, gomock.Any(), nil).
					Return(&v1.ExecResult{RowsAffected: 1}, nil).
					Times(1)
			},
			expectError: false,
		},
		{
			name: "no table specified",
			setup: func(b *v1.DeleteBuilder) *v1.DeleteBuilder {
				return b.Where(cdt.NewExpr().Column("id").Op("=").Value(1))
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "table not specified",
		},
		{
			name: "no where condition",
			setup: func(b *v1.DeleteBuilder) *v1.DeleteBuilder {
				return b.From("users")
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "WHERE condition required",
		},
		{
			name: "database error",
			setup: func(b *v1.DeleteBuilder) *v1.DeleteBuilder {
				return b.From("users").Where(cdt.NewExpr().Column("id").Op("=").Value(1))
			},
			mockSetup: func() {
				db.EXPECT().Delete(context.Background(), "users", nil, gomock.Any(), nil).
					Return(nil, errors.New("delete failed")).
					Times(1)
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()
			builder := tc.setup(fluentDB.Delete())
			result, err := builder.Exec(context.Background())

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestDeleteBuilderWithTx tests transaction support
func TestDeleteBuilderWithTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	tx := v1.NewMockTx(ctrl)
	fluentDB := v1.NewFluentDB(db)

	builder := fluentDB.Delete()
	result := builder.WithTx(tx)

	assert.Equal(t, builder, result)

	// Verify that subsequent Exec(context.Background()) call uses tx instead of db
	tx.EXPECT().Delete(context.Background(), "users", nil, gomock.Any(), nil).
		Return(&v1.ExecResult{RowsAffected: 1}, nil).
		Times(1)

	_, _ = result.From("users").Where(cdt.NewExpr().Column("id").Op("=").Value(1)).Exec(context.Background())
}

// TestUpdateBuilderUpdateAll tests the UpdateAll method for unfiltered updates
func TestUpdateBuilderUpdateAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name          string
		setup         func(*v1.UpdateBuilder) *v1.UpdateBuilder
		mockSetup     func()
		expectError   bool
		expectedError string
	}{
		{
			name: "successful update all - no where condition",
			setup: func(b *v1.UpdateBuilder) *v1.UpdateBuilder {
				return b.Set("status", "active")
			},
			mockSetup: func() {
				db.EXPECT().Update(context.Background(), "users", gomock.Any(), nil, nil, nil).
					Return(&v1.ExecResult{RowsAffected: 100}, nil).
					Times(1)
			},
			expectError: false,
		},
		{
			name: "update all with existing where condition is ignored",
			setup: func(b *v1.UpdateBuilder) *v1.UpdateBuilder {
				return b.Set("status", "inactive").Where(cdt.NewExpr().Column("id").Op(">").Value(50))
			},
			mockSetup: func() {
				db.EXPECT().Update(context.Background(), "users", gomock.Any(), nil, gomock.Any(), nil).
					Return(&v1.ExecResult{RowsAffected: 100}, nil).
					Times(1)
			},
			expectError: false,
		},
		{
			name: "no table specified",
			setup: func(b *v1.UpdateBuilder) *v1.UpdateBuilder {
				return fluentDB.Update("").Set("status", "active")
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "table not specified",
		},
		{
			name: "no data to update",
			setup: func(b *v1.UpdateBuilder) *v1.UpdateBuilder {
				return b
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "no data to update",
		},
		{
			name: "database error",
			setup: func(b *v1.UpdateBuilder) *v1.UpdateBuilder {
				return b.Set("status", "active")
			},
			mockSetup: func() {
				db.EXPECT().Update(context.Background(), "users", gomock.Any(), nil, nil, nil).
					Return(nil, errors.New("update failed")).
					Times(1)
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()
			builder := tc.setup(fluentDB.Update("users"))
			result, err := builder.UpdateAll(context.Background())

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Greater(t, result.RowsAffected, int64(0))
			}
		})
	}
}

// TestDeleteBuilderDeleteAll tests the DeleteAll method for unfiltered deletes
func TestDeleteBuilderDeleteAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	testCases := []struct {
		name          string
		setup         func(*v1.DeleteBuilder) *v1.DeleteBuilder
		mockSetup     func()
		expectError   bool
		expectedError string
	}{
		{
			name: "successful delete all - no where condition",
			setup: func(b *v1.DeleteBuilder) *v1.DeleteBuilder {
				return b.From("users")
			},
			mockSetup: func() {
				db.EXPECT().Delete(context.Background(), "users", nil, nil, nil).
					Return(&v1.ExecResult{RowsAffected: 100}, nil).
					Times(1)
			},
			expectError: false,
		},
		{
			name: "delete all with existing where condition is ignored",
			setup: func(b *v1.DeleteBuilder) *v1.DeleteBuilder {
				return b.From("users").Where(cdt.NewExpr().Column("status").Op("=").Value("inactive"))
			},
			mockSetup: func() {
				db.EXPECT().Delete(context.Background(), "users", nil, gomock.Any(), nil).
					Return(&v1.ExecResult{RowsAffected: 100}, nil).
					Times(1)
			},
			expectError: false,
		},
		{
			name: "no table specified",
			setup: func(b *v1.DeleteBuilder) *v1.DeleteBuilder {
				return b
			},
			mockSetup:     func() {},
			expectError:   true,
			expectedError: "table not specified",
		},
		{
			name: "database error",
			setup: func(b *v1.DeleteBuilder) *v1.DeleteBuilder {
				return b.From("users")
			},
			mockSetup: func() {
				db.EXPECT().Delete(context.Background(), "users", nil, nil, nil).
					Return(nil, errors.New("delete failed")).
					Times(1)
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()
			builder := tc.setup(fluentDB.Delete())
			result, err := builder.DeleteAll(context.Background())

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Greater(t, result.RowsAffected, int64(0))
			}
		})
	}
}

// Helper function
func intPtr(i int) *int {
	return &i
}

// Benchmarks for FluentDB to verify zero-overhead claim of the builder pattern

// BenchmarkSelectBuilderConstruction benchmarks the time to construct a complex select query
func BenchmarkSelectBuilderConstruction(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fluentDB.
			Select("users", "id", "name", "email").
			Where(condition).
			OrderBy("id", "ASC").
			Limit(10).
			Offset(5)
	}
}

// BenchmarkInsertBuilderConstruction benchmarks the time to construct an insert query
func BenchmarkInsertBuilderConstruction(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fluentDB.Insert().
			Into("users").
			Set("name", "John").
			Set("email", "john@example.com").
			Set("age", 30)
	}
}

// BenchmarkUpdateBuilderConstruction benchmarks the time to construct an update query
func BenchmarkUpdateBuilderConstruction(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fluentDB.Update("users").
			Set("name", "Jane").
			Set("age", 31).
			Where(condition)
	}
}

// BenchmarkDeleteBuilderConstruction benchmarks the time to construct a delete query
func BenchmarkDeleteBuilderConstruction(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	condition := cdt.NewExpr().Column("id").Op("=").Value(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fluentDB.Delete().
			From("users").
			Where(condition).
			Limit(5)
	}
}

// BenchmarkSelectBuilderWithMultipleJoins benchmarks select with multiple joins
func BenchmarkSelectBuilderWithMultipleJoins(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	joins := []cdt.Join{
		{Type: "INNER", Table: "orders", Conditions: cdt.JoinCdts{{Left: "users.id", Right: "orders.user_id"}}},
		{Type: "LEFT", Table: "products", Conditions: cdt.JoinCdts{{Left: "orders.product_id", Right: "products.id"}}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fluentDB.
			Select("users", "id", "name").
			Joins(joins).
			OrderBy("id", "ASC").
			Limit(10)
	}
}

// BenchmarkSelectBuilderWithComplexConditions benchmarks select with multiple where conditions
func BenchmarkSelectBuilderWithComplexConditions(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	cond1 := cdt.NewExpr().Column("age").Op(">").Value(18)
	cond2 := cdt.NewExpr().Column("active").Op("=").Value(true)
	cond3 := cdt.NewExpr().Column("name").Op("!=").Value("admin")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fluentDB.
			Select("users").
			Where(cond1).
			Where(cond2).
			Where(cond3).
			OrderBy("id", "ASC").
			OrderBy("name", "DESC").
			Limit(25)
	}
}

// BenchmarkInsertBuilderBulk benchmarks bulk insert construction
func BenchmarkInsertBuilderBulk(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	bulkData := []map[string]any{
		{"name": "John", "email": "john@example.com", "age": 30},
		{"name": "Jane", "email": "jane@example.com", "age": 28},
		{"name": "Bob", "email": "bob@example.com", "age": 35},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fluentDB.Insert().
			Into("users").
			ValuesBulk(bulkData)
	}
}

// BenchmarkSelectBuilderExecution benchmarks select query execution time (construction + execution)
func BenchmarkSelectBuilderExecution(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	// Mock the Get method to return immediately
	db.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]map[string]any{{"id": 1, "name": "John"}}, nil).
		AnyTimes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fluentDB.
			Select("users", "id", "name").
			Where(cdt.NewExpr().Column("id").Op("=").Value(1)).
			Get(context.Background())
	}
}

// BenchmarkInsertBuilderExecution benchmarks insert query execution time
func BenchmarkInsertBuilderExecution(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	// Mock the Insert method to return immediately
	db.EXPECT().Insert(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&v1.ExecResult{RowsAffected: 1}, nil).
		AnyTimes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fluentDB.Insert().
			Into("users").
			Set("name", "John").
			Set("email", "john@example.com").
			Exec(context.Background())
	}
}

// BenchmarkUpdateBuilderExecution benchmarks update query execution time
func BenchmarkUpdateBuilderExecution(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	// Mock the Update method to return immediately
	db.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&v1.ExecResult{RowsAffected: 1}, nil).
		AnyTimes()

	cond := cdt.NewExpr().Column("id").Op("=").Value(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fluentDB.Update("users").
			Set("name", "Jane").
			Set("age", 31).
			Where(cond).
			Exec(context.Background())
	}
}

// BenchmarkDeleteBuilderExecution benchmarks delete query execution time
func BenchmarkDeleteBuilderExecution(b *testing.B) {
	ctrl := gomock.NewController(&testing.T{})
	defer ctrl.Finish()

	db := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(db)

	// Mock the Delete method to return immediately
	db.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&v1.ExecResult{RowsAffected: 1}, nil).
		AnyTimes()

	cond := cdt.NewExpr().Column("id").Op("=").Value(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fluentDB.Delete().
			From("users").
			Where(cond).
			Exec(context.Background())
	}
}
