//go:build test

package sqldialect_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sd "tounilab.com/db-connector/query/builder/sqldialect"
	"tounilab.com/db-connector/query/condition"
	"tounilab.com/db-connector/query/definition"
	"tounilab.com/db-connector/query/options"
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
			wantFrag: "LIMIT ? OFFSET ? ORDER BY `name`",
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
			wantFrag: "LIMIT $1 OFFSET $2 ORDER BY \"username\"",
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
			wantFrag: "FETCH NEXT @p1 ROWS ONLY OFFSET @p2 ROWS ORDER BY [id]",
			wantArgs: []any{3, 2},
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
