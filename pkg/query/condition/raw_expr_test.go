//go:build test

package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/vessel/pkg/query/condition"
	"tounilab.com/vessel/tests"
)

func TestExpr_ToSQL_RawExprValue(t *testing.T) {
	cases := []struct {
		name    string
		value   condition.RawExpr
		wantSQL string
		wantErr bool
	}{
		{name: "renders inline", value: "NOW()", wantSQL: "`expires_at` > NOW()"},
		{name: "trims surrounding whitespace", value: "  NOW()  ", wantSQL: "`expires_at` > NOW()"},
		{name: "empty", value: "", wantErr: true},
		{name: "whitespace only", value: "   ", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expr := condition.NewExpr().Column("expires_at").Op(">").Value(tc.value)
			sql, args, err := expr.ToSQL(tests.MockDialect{}, 3)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "raw expression")
				assert.Empty(t, sql)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantSQL, sql)
			assert.Empty(t, args, "raw expressions must not bind parameters")
		})
	}
}
