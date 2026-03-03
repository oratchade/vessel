//go:build test

package sqldialect_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sd "tounilab.com/db-connector/internal/pkg/sqldialect"
	"tounilab.com/db-connector/pkg/query/condition"
	"tounilab.com/db-connector/pkg/query/definition"
	"tounilab.com/db-connector/pkg/query/options"
)

func intPtr(v int) *int { return &v }

func TestSupportedOptions_SelectLimitOffsetOrderBy(t *testing.T) {
	cases := []struct {
		name     string
		dialect  condition.SQLDialect
		opts     *options.QueryOptions
		wantFrag string
		wantArgs []any
	}{
		{
			name:    "mysql",
			dialect: sd.MySQLDialect{},
			opts: &options.QueryOptions{
				Limit:   intPtr(10),
				Offset:  intPtr(5),
				OrderBy: []string{"name"},
			},
			wantFrag: "ORDER BY `name` LIMIT ? OFFSET ?",
			wantArgs: []any{10, 5},
		},
		{
			name:    "postgres",
			dialect: sd.PostgresDialect{},
			opts: &options.QueryOptions{
				Limit:   intPtr(20),
				Offset:  intPtr(7),
				OrderBy: []string{"username"},
			},
			wantFrag: "ORDER BY \"username\" LIMIT $1 OFFSET $2",
			wantArgs: []any{20, 7},
		},
		{
			name:    "mssql",
			dialect: sd.MSSQLDialect{},
			opts: &options.QueryOptions{
				Limit:   intPtr(3),
				Offset:  intPtr(2),
				OrderBy: []string{"id"},
			},
			wantFrag: "ORDER BY [id] OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY",
			wantArgs: []any{2, 3},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frag, args, err := tc.dialect.SupportedOptions(definition.QueryTypeSelect, tc.opts, 1)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantFrag, frag)
			assert.Equal(t, tc.wantArgs, args)
		})
	}
}

func TestReturningOutput_PrefixQuoting(t *testing.T) {
	cases := []struct {
		name    string
		dialect interface {
			SupportedOptions(definition.QueryType, *options.QueryOptions, int) (string, []any, error)
		}
		qt       definition.QueryType
		wantFrag string
	}{
		{
			name:     "postgres-insert",
			dialect:  sd.PostgresDialect{},
			qt:       definition.QueryTypeInsert,
			wantFrag: `RETURNING inserted."id", inserted."name"`,
		},
		{
			name:     "postgres-delete",
			dialect:  sd.PostgresDialect{},
			qt:       definition.QueryTypeDelete,
			wantFrag: `RETURNING deleted."id", deleted."name"`,
		},
		{
			name:     "mssql-insert",
			dialect:  sd.MSSQLDialect{},
			qt:       definition.QueryTypeInsert,
			wantFrag: `OUTPUT inserted.[id], inserted.[name]`,
		},
		{
			name:     "mssql-delete",
			dialect:  sd.MSSQLDialect{},
			qt:       definition.QueryTypeDelete,
			wantFrag: `OUTPUT deleted.[id], deleted.[name]`,
		},
	}

	opts := &options.QueryOptions{Returning: []string{"id", "name"}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frag, args, err := tc.dialect.SupportedOptions(tc.qt, opts, 1)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantFrag, frag)
			assert.Empty(t, args)
		})
	}
}

func TestSupportedOptions_GroupByHavingAndMultipleOrderBy(t *testing.T) {
	count := "count>10"
	cases := []struct {
		name     string
		dialect  condition.SQLDialect
		opts     *options.QueryOptions
		wantFrag string
	}{
		{
			name:    "mysql-groupby-having-orderby",
			dialect: sd.MySQLDialect{},
			opts: &options.QueryOptions{
				GroupBy: []string{"country", "city"},
				Having:  func() *string { return &count }(),
				OrderBy: []string{"country", "city"},
			},
			wantFrag: "GROUP BY `country`, `city` HAVING `count>10` ORDER BY `country`, `city`",
		},
		{
			name:    "postgres-groupby-having-orderby",
			dialect: sd.PostgresDialect{},
			opts: &options.QueryOptions{
				GroupBy: []string{"country", "city"},
				Having:  func() *string { return &count }(),
				OrderBy: []string{"country", "city"},
			},
			wantFrag: "GROUP BY \"country\", \"city\" HAVING \"count>10\" ORDER BY \"country\", \"city\"",
		},
		{
			name:    "mssql-groupby-having-orderby",
			dialect: sd.MSSQLDialect{},
			opts: &options.QueryOptions{
				GroupBy: []string{"country", "city"},
				Having:  func() *string { return &count }(),
				OrderBy: []string{"country", "city"},
			},
			wantFrag: "GROUP BY [country], [city] HAVING [count>10] ORDER BY [country], [city]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Note: supportedOptions returns parts in order: Limit/Offset then OrderBy, Having, GroupBy
			frag, _, err := tc.dialect.SupportedOptions(definition.QueryTypeSelect, tc.opts, 1)
			assert.NoError(t, err)
			// normalize spacing for comparison
			assert.Equal(t, tc.wantFrag, frag)
		})
	}
}

func TestReturningOutput_UpdateCase(t *testing.T) {
	// Ensure Update uses inserted. prefix for returning when applicable
	d := sd.PostgresDialect{}
	opts := &options.QueryOptions{Returning: []string{"id"}}
	frag, args, err := d.SupportedOptions(definition.QueryTypeUpdate, opts, 1)
	assert.NoError(t, err)
	assert.Equal(t, "RETURNING inserted.\"id\"", frag)
	assert.Empty(t, args)
}

func TestMSSQLOffsetRequiresOrderBy(t *testing.T) {
	// Test that MSSQL requires ORDER BY when using OFFSET
	d := sd.MSSQLDialect{}

	testCases := []struct {
		name      string
		opts      *options.QueryOptions
		expectErr bool
	}{
		{
			name: "OFFSET with ORDER BY - should succeed",
			opts: &options.QueryOptions{
				Offset:  intPtr(10),
				OrderBy: []string{"id"},
			},
			expectErr: false,
		},
		{
			name: "OFFSET without ORDER BY - should error",
			opts: &options.QueryOptions{
				Offset: intPtr(10),
			},
			expectErr: true,
		},
		{
			name: "ORDER BY without OFFSET - should succeed",
			opts: &options.QueryOptions{
				OrderBy: []string{"id"},
			},
			expectErr: false,
		},
		{
			name: "LIMIT with OFFSET without ORDER BY - should error",
			opts: &options.QueryOptions{
				Limit:  intPtr(5),
				Offset: intPtr(10),
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := d.SupportedOptions(definition.QueryTypeSelect, tc.opts, 1)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Equal(t, "MSSQL OFFSET requires ORDER BY clause", err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
