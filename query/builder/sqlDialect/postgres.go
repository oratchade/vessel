package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query"
)

type PostgresDialect struct{}

func (d PostgresDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// reason: this function is complex by design and refactoring would reduce clarity
//
//nolint:cyclop
func (d PostgresDialect) Operator(op string) string {
	switch strings.ToLower(op) {
	case query.Equal:
		return "="
	case query.NotEqual:
		return "!="
	case query.LowerThan:
		return "<"
	case query.LowerThanOrEqual:
		return "<="
	case query.GreaterThan:
		return ">"
	case query.GreaterThanOrEqual:
		return ">="
	case query.And:
		return strings.ToUpper(query.And)
	case query.Or:
		return strings.ToUpper(query.Or)
	case query.Like:
		return strings.ToUpper(query.Like)
	case query.NotLike:
		return strings.ToUpper(query.NotLike)
	case query.InsensitiveCaseLike:
		return strings.ToUpper(query.InsensitiveCaseLike) // Postgres has a case-insensitive LIKE
	case query.In:
		return strings.ToUpper(query.In)
	case query.NotIn:
		return strings.ToUpper(query.NotIn)
	case query.Between:
		return strings.ToUpper(query.Between)
	case query.NotBetween:
		return strings.ToUpper(query.NotBetween)
	case query.IsNull:
		return strings.ToUpper(query.IsNull)
	case query.IsNotNull:
		return strings.ToUpper(query.IsNotNull)
	case query.Distinct:
		return strings.ToUpper(query.IsDistinctFrom)
	case query.NotDistinct:
		return "IS NOT DISTINCT FROM"
	case query.Contains:
		return "@>"
	case query.Contained:
		return "<@"
	case query.Overlaps:
		return "&&"
	case query.Regex:
		return "~"
	case query.NotRegex:
		return "!~"
	case query.InsensitiveCaseRegex:
		return "~*"
	case query.NotInsensitiveCaseRegex:
		return "!~*"
	default:
		return op
	}
}
