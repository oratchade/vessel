package sqldialect

import (
	"fmt"
	"strings"
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
	case and:
		return strings.ToUpper(and)
	case or:
		return strings.ToUpper(or)
	case like:
		return strings.ToUpper(like)
	case notLike:
		return strings.ToUpper(notLike) // MSSQL does not support NOT LIKE, so we use LIKE
	case insensitiveCaseLike:
		return strings.ToUpper(like) // MSSQL does not have a case-insensitive LIKE, so we use LIKE
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
		return strings.ToUpper(isDistinctFrom) // emulate
	case notDistinct:
		return strings.ToUpper(like) // MSSQL does not support IS NOT DISTINCT FROM, so we use LIKE
	case contains:
		return strings.ToUpper(like) // MSSQL does not support @> like Postgres, so we use LIKE
	case contained:
		return strings.ToUpper(like) // MSSQL does not support <@ like Postgres, so we use LIKE
	case overlaps:
		return strings.ToUpper(like) // MSSQL does not support && like Postgres, so we use LIKE
	case regex, notRegex, insensitiveCaseRegex, notInsensitiveCaseRegex:
		return "" // not supported
	default:
		return op
	}
}
