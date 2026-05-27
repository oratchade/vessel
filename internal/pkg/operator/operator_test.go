//go:build test

package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tounilab.com/vessel/internal/pkg/operator"
)

func TestComparisonOperators(t *testing.T) {
	assert.Equal(t, "eq", operator.Equal)
	assert.Equal(t, "neq", operator.NotEqual)
	assert.Equal(t, "lt", operator.LowerThan)
	assert.Equal(t, "lte", operator.LowerThanOrEqual)
	assert.Equal(t, "gt", operator.GreaterThan)
	assert.Equal(t, "gte", operator.GreaterThanOrEqual)
}

func TestLogicalOperators(t *testing.T) {
	assert.Equal(t, "and", operator.And)
	assert.Equal(t, "or", operator.Or)
}

func TestPatternOperators(t *testing.T) {
	assert.Equal(t, "like", operator.Like)
	assert.Equal(t, "not like", operator.NotLike)
	assert.Equal(t, "ilike", operator.InsensitiveCaseLike)
}

func TestSetOperators(t *testing.T) {
	assert.Equal(t, "in", operator.In)
	assert.Equal(t, "not in", operator.NotIn)
}

func TestRangeOperators(t *testing.T) {
	assert.Equal(t, "between", operator.Between)
	assert.Equal(t, "not between", operator.NotBetween)
}

func TestNullOperators(t *testing.T) {
	assert.Equal(t, "is null", operator.IsNull)
	assert.Equal(t, "is not null", operator.IsNotNull)
}

func TestDistinctOperators(t *testing.T) {
	assert.Equal(t, "distinct", operator.Distinct)
	assert.Equal(t, "not distinct", operator.NotDistinct)
	assert.Equal(t, "is distinct from", operator.IsDistinctFrom)
}

func TestJSONOperators(t *testing.T) {
	assert.Equal(t, "contains", operator.Contains)
	assert.Equal(t, "contained", operator.Contained)
	assert.Equal(t, "overlaps", operator.Overlaps)
}

func TestRegexOperators(t *testing.T) {
	assert.Equal(t, "regex", operator.Regex)
	assert.Equal(t, "not regex", operator.NotRegex)
	assert.Equal(t, "iregex", operator.InsensitiveCaseRegex)
	assert.Equal(t, "not iregex", operator.NotInsensitiveCaseRegex)
}

func TestJoinTypes(t *testing.T) {
	assert.Equal(t, "inner", operator.Inner)
	assert.Equal(t, "left", operator.Left)
	assert.Equal(t, "right", operator.Right)
	assert.Equal(t, "full", operator.Full)
	assert.Equal(t, "using", operator.Using)
}

func TestClauseKeywords(t *testing.T) {
	assert.Equal(t, "limit", operator.Limit)
	assert.Equal(t, "offset", operator.Offset)
	assert.Equal(t, "returning", operator.Returning)
	assert.Equal(t, "order by", operator.OrderBy)
	assert.Equal(t, "group by", operator.GroupBy)
	assert.Equal(t, "having", operator.Having)
	assert.Equal(t, "output", operator.Output)
}
