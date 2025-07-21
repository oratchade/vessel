package sqldialect

import (
	"fmt"
	"strings"
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
		return "ILIKE"
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
		return strings.ToUpper(isDistinctFrom)
	case notDistinct:
		return "IS NOT DISTINCT FROM"
	case contains:
		return "@>"
	case contained:
		return "<@"
	case overlaps:
		return "&&"
	case regex:
		return "~"
	case notRegex:
		return "!~"
	case insensitiveCaseRegex:
		return "~*"
	case notInsensitiveCaseRegex:
		return "!~*"
	default:
		return op
	}
}
