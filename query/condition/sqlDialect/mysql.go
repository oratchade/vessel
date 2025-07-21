package sqldialect

import (
	"strings"
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
	case equal:
		return "="
	case notEqual:
		return "!="
	case lowerThan:
		return "<"
	case lowerThanOrEqual:
		return "<="
	case greaterThan:
		return ">"
	case greaterThanOrEqual:
		return ">="
	case like:
		return strings.ToUpper(like)
	case notLike:
		return strings.ToUpper(notLike)
	case insensitiveCaseLike:
		return strings.ToUpper(like) // MySQL does not have a case-insensitive LIKE, so we use LIKE
	case in:
		return strings.ToUpper(in)
	case notIn:
		return strings.ToUpper(notIn)
	case between:
		return strings.ToUpper(between)
	case notBetween:
		return strings.ToUpper(notBetween)
	case isNull:
		return strings.ToUpper(isNull)
	case isNotNull:
		return strings.ToUpper(isNotNull)
	case distinct:
		return strings.ToUpper(isDistinctFrom) // MySQL does not support IS DISTINCT FROM, but we can emulate it
	case notDistinct:
		return "IS NOT DISTINCT FROM"
	case contains:
		return strings.ToUpper(like) // MySQL does not support @> like Postgres, so we use LIKE
	case contained:
		return strings.ToUpper(like) // MySQL does not support <@ like Postgres, so we use LIKE
	case overlaps:
		return strings.ToUpper(like) // MySQL does not support && like Postgres, so we use LIKE
	case regex:
		return "REGEXP"
	case notRegex:
		return "NOT REGEXP"
	case insensitiveCaseRegex:
		return "REGEXP" // MySQL does not have a case-insensitive regex operator, so we use REGEXP
	case notInsensitiveCaseRegex:
		return "NOT REGEXP"
	default:
		return op
	}
}
