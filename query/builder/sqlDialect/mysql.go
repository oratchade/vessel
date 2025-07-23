package sqldialect

import (
	"strings"

	"tounilab.com/db-connector/query"
)

// MySQL and SQLite use the same placeholder syntax
// and similar operators, so we can use the same dialect for both.

type MySQLDialect struct{}

func (d MySQLDialect) Placeholder(_ int) string {
	return "?"
}

// reason: this function is complex by design and refactoring would reduce clarity
//
//nolint:cyclop
func (d MySQLDialect) Operator(op string) string {
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
		return strings.ToUpper(query.Like) // MySQL does not have a case-insensitive LIKE, so we use LIKE
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
		return strings.ToUpper(query.IsDistinctFrom) // MySQL does not support IS DISTINCT FROM, but we can emulate it
	case query.NotDistinct:
		return "IS NOT DISTINCT FROM"
	case query.Contains:
		return strings.ToUpper(query.Like) // MySQL does not support @> like Postgres, so we use LIKE
	case query.Contained:
		return strings.ToUpper(query.Like) // MySQL does not support <@ like Postgres, so we use LIKE
	case query.Overlaps:
		return strings.ToUpper(query.Like) // MySQL does not support && like Postgres, so we use LIKE
	case query.Regex:
		return "REGEXP"
	case query.NotRegex:
		return "NOT REGEXP"
	case query.InsensitiveCaseRegex:
		return "REGEXP" // MySQL does not have a case-insensitive regex operator, so we use REGEXP
	case query.NotInsensitiveCaseRegex:
		return "NOT REGEXP"
	default:
		return op
	}
}
