//go:build test

package sqldialect_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sd "tounilab.com/fabric/internal/pkg/sqldialect"
	"tounilab.com/fabric/pkg/query/condition"
	"tounilab.com/fabric/pkg/query/definition"
	"tounilab.com/fabric/pkg/query/options"
)

func intPtr(v int) *int { return &v }

func TestQuoteIdentifierEscapesDelimiters(t *testing.T) {
	cases := []struct {
		name    string
		dialect condition.SQLDialect
		input   string
		want    string
	}{
		{
			name:    "mysql backtick",
			dialect: sd.MySQLDialect{},
			input:   "user`name",
			want:    "`user``name`",
		},
		{
			name:    "sqlite backtick",
			dialect: sd.SQLiteDialect{},
			input:   "user`name",
			want:    "`user``name`",
		},
		{
			name:    "postgres double quote",
			dialect: sd.PostgresDialect{},
			input:   `user"name`,
			want:    `"user""name"`,
		},
		{
			name:    "mssql closing bracket",
			dialect: sd.MSSQLDialect{},
			input:   "user]name",
			want:    "[user]]name]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.dialect.QuoteIdentifier(tc.input))
		})
	}
}

func TestCapabilitiesForDialects(t *testing.T) {
	cases := []struct {
		name string
		d    condition.SQLDialect
		want sd.Capabilities
	}{
		{
			name: "mysql",
			d:    sd.MySQLDialect{},
			want: sd.Capabilities{
				SelectPagination:       true,
				MutationOrderLimit:     true,
				Upsert:                 true,
				JoinedUpdate:           true,
				JoinedDelete:           true,
				MutationOrderLimitName: "MySQL",
			},
		},
		{
			name: "postgres",
			d:    sd.PostgresDialect{},
			want: sd.Capabilities{
				SelectPagination:      true,
				MutationReturning:     true,
				Upsert:                true,
				JoinedUpdate:          true,
				JoinedDelete:          true,
				JoinedDeleteWithUsing: true,
			},
		},
		{
			name: "sqlite",
			d:    sd.SQLiteDialect{},
			want: sd.Capabilities{
				SelectPagination: true,
				JoinedUpdate:     true,
				Upsert:           true,
			},
		},
		{
			name: "mssql",
			d:    sd.MSSQLDialect{},
			want: sd.Capabilities{
				SelectPagination:  true,
				MutationOutput:    true,
				MutationReturning: true,
				JoinedUpdate:      true,
				JoinedDelete:      true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sd.CapabilitiesFor(tc.d))
		})
	}
}

func TestCapabilitiesForUsesProvider(t *testing.T) {
	dialect := capabilityProviderDialect{
		SQLDialect: sd.MySQLDialect{},
		caps: sd.Capabilities{
			SelectPagination:  true,
			MutationReturning: true,
		},
	}

	assert.Equal(t, dialect.caps, sd.CapabilitiesFor(dialect))
}

type capabilityProviderDialect struct {
	condition.SQLDialect
	caps sd.Capabilities
}

func (c capabilityProviderDialect) Capabilities() sd.Capabilities {
	return c.caps
}

func TestSupportedOptions_SelectLimitOffsetOrderBy(t *testing.T) {
	cases := []struct {
		name    string
		dialect interface {
			condition.SQLDialect
			SupportedOptions(definition.QueryType, *options.QueryOptions, int) (string, []any, error)
		}
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
				OrderBy: []options.OrderBy{{Column: "name", Direction: "ASC"}},
			},
			wantFrag: "ORDER BY `name` ASC LIMIT ? OFFSET ?",
			wantArgs: []any{10, 5},
		},
		{
			name:    "postgres",
			dialect: sd.PostgresDialect{},
			opts: &options.QueryOptions{
				Limit:   intPtr(20),
				Offset:  intPtr(7),
				OrderBy: []options.OrderBy{{Column: "username", Direction: "ASC"}},
			},
			wantFrag: "ORDER BY \"username\" ASC LIMIT $1 OFFSET $2",
			wantArgs: []any{20, 7},
		},
		{
			name:    "mssql",
			dialect: sd.MSSQLDialect{},
			opts: &options.QueryOptions{
				Limit:   intPtr(3),
				Offset:  intPtr(2),
				OrderBy: []options.OrderBy{{Column: "id", Direction: "ASC"}},
			},
			wantFrag: "ORDER BY [id] ASC OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY",
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
			wantFrag: `RETURNING "id", "name"`,
		},
		{
			name:     "postgres-delete",
			dialect:  sd.PostgresDialect{},
			qt:       definition.QueryTypeDelete,
			wantFrag: `RETURNING "id", "name"`,
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
	count := "COUNT(*) > 10"
	cases := []struct {
		name    string
		dialect interface {
			condition.SQLDialect
			SupportedOptions(definition.QueryType, *options.QueryOptions, int) (string, []any, error)
		}
		opts     *options.QueryOptions
		wantFrag string
	}{
		{
			name:    "mysql-groupby-having-orderby",
			dialect: sd.MySQLDialect{},
			opts: &options.QueryOptions{
				GroupBy: []string{"country", "city"},
				Having:  func() *string { return &count }(),
				OrderBy: []options.OrderBy{
					{Column: "country", Direction: "ASC"},
					{Column: "city", Direction: "ASC"},
				},
			},
			wantFrag: "GROUP BY `country`, `city` HAVING COUNT(*) > 10 ORDER BY `country` ASC, `city` ASC",
		},
		{
			name:    "postgres-groupby-having-orderby",
			dialect: sd.PostgresDialect{},
			opts: &options.QueryOptions{
				GroupBy: []string{"country", "city"},
				Having:  func() *string { return &count }(),
				OrderBy: []options.OrderBy{
					{Column: "country", Direction: "ASC"},
					{Column: "city", Direction: "ASC"},
				},
			},
			wantFrag: "GROUP BY \"country\", \"city\" HAVING COUNT(*) > 10 ORDER BY \"country\" ASC, \"city\" ASC",
		},
		{
			name:    "mssql-groupby-having-orderby",
			dialect: sd.MSSQLDialect{},
			opts: &options.QueryOptions{
				GroupBy: []string{"country", "city"},
				Having:  func() *string { return &count }(),
				OrderBy: []options.OrderBy{
					{Column: "country", Direction: "ASC"},
					{Column: "city", Direction: "ASC"},
				},
			},
			wantFrag: "GROUP BY [country], [city] HAVING COUNT(*) > 10 ORDER BY [country] ASC, [city] ASC",
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

func TestSupportedOptions_ParameterizedHavingUsesParamBase(t *testing.T) {
	opts := &options.QueryOptions{
		GroupBy: []string{"department"},
		HavingCondition: condition.NewExpr().
			Column("COUNT(*)").
			Op(">").
			Value(2),
	}

	frag, args, err := sd.PostgresDialect{}.SupportedOptions(definition.QueryTypeSelect, opts, 3)

	assert.NoError(t, err)
	assert.Equal(t, `GROUP BY "department" HAVING COUNT(*) > $3`, frag)
	assert.Equal(t, []any{2}, args)
}

func TestSupportedOptions_RawAndParameterizedHaving(t *testing.T) {
	raw := "COUNT(*) > 1"
	opts := &options.QueryOptions{
		GroupBy: []string{"department"},
		Having:  &raw,
		HavingCondition: condition.NewExpr().
			Column("SUM(score)").
			Op("<").
			Value(100),
	}

	frag, args, err := sd.MySQLDialect{}.SupportedOptions(definition.QueryTypeSelect, opts, 1)

	assert.NoError(t, err)
	assert.Equal(t, "GROUP BY `department` HAVING COUNT(*) > 1 AND SUM(score) < ?", frag)
	assert.Equal(t, []any{100}, args)
}

func TestReturningOutput_UpdateCase(t *testing.T) {
	// Ensure Postgres Update uses unprefixed RETURNING columns.
	d := sd.PostgresDialect{}
	opts := &options.QueryOptions{Returning: []string{"id"}}
	frag, args, err := d.SupportedOptions(definition.QueryTypeUpdate, opts, 1)
	assert.NoError(t, err)
	assert.Equal(t, "RETURNING \"id\"", frag)
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
				OrderBy: []options.OrderBy{{Column: "id", Direction: "ASC"}},
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
				OrderBy: []options.OrderBy{{Column: "id", Direction: "ASC"}},
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
				assert.Equal(t, "MSSQL pagination requires ORDER BY clause", err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMSSQLLimitWithoutOffsetAddsZeroOffset(t *testing.T) {
	d := sd.MSSQLDialect{}
	opts := &options.QueryOptions{
		Limit:   intPtr(5),
		OrderBy: []options.OrderBy{{Column: "id", Direction: "ASC"}},
	}

	frag, args, err := d.SupportedOptions(definition.QueryTypeSelect, opts, 1)

	assert.NoError(t, err)
	assert.Equal(t, "ORDER BY [id] ASC OFFSET 0 ROWS FETCH NEXT @p1 ROWS ONLY", frag)
	assert.Equal(t, []any{5}, args)
}
