package sqldialect

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query"
)

type MSSQLDialect struct{}

func (d MSSQLDialect) Placeholder(index int) string {
	return fmt.Sprintf("@p%d", index)
}

// reason: this function is complex by design and refactoring would reduce clarity
//
//nolint:cyclop
func (d MSSQLDialect) Operator(op string) string {
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
		return strings.ToUpper(query.NotLike) // MSSQL does not support NOT LIKE, so we use LIKE
	case query.InsensitiveCaseLike:
		return strings.ToUpper(query.Like) // MSSQL does not have a case-insensitive LIKE, so we use LIKE
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
		return strings.ToUpper(query.IsDistinctFrom) // emulate
	case query.NotDistinct:
		return strings.ToUpper(query.Like) // MSSQL does not support IS NOT DISTINCT FROM, so we use LIKE
	case query.Contains:
		return strings.ToUpper(query.Like) // MSSQL does not support @> like Postgres, so we use LIKE
	case query.Contained:
		return strings.ToUpper(query.Like) // MSSQL does not support <@ like Postgres, so we use LIKE
	case query.Overlaps:
		return strings.ToUpper(query.Like) // MSSQL does not support && like Postgres, so we use LIKE
	case query.Regex, query.NotRegex, query.InsensitiveCaseRegex, query.NotInsensitiveCaseRegex:
		return "" // not supported
	default:
		return op
	}
}
