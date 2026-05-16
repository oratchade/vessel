package condition

import (
	"fmt"
	"regexp"
	"strings"
)

var sqlFunctionPattern = regexp.MustCompile(`(?i)^([a-z_][a-z0-9_]*)\s*\(.*\)$`)

// Expr represents a simple column operator value expression, e.g. `age > 30`.
// It implements the Condition interface and can be converted to SQL with placeholders.
type Expr struct {
	column   string
	operator string
	value    any
}

// NewExpr constructs an empty Expr that can be populated via fluent setters.
func NewExpr() *Expr {
	return &Expr{}
}

// Column sets the column name for this Expr condition.
func (e *Expr) Column(col string) *Expr {
	e.column = col
	return e
}

// Op sets the operator for this Expr condition.
func (e *Expr) Op(op string) *Expr {
	e.operator = op
	return e
}

// Value sets the value for this Expr condition.
func (e *Expr) Value(val any) *Expr {
	e.value = val
	return e
}

// isUnaryOperator returns true for SQL operators that take no value (IS NULL, IS NOT NULL).
func isUnaryOperator(op string) bool {
	upper := strings.ToUpper(strings.TrimSpace(op))
	return upper == "IS NULL" || upper == "IS NOT NULL"
}

func quoteColumn(dialect SQLDialect, column string) string {
	column = strings.TrimSpace(column)
	if column == "*" || sqlFunctionPattern.MatchString(column) || strings.ContainsAny(column, " ()") {
		return column
	}
	if strings.Contains(column, ".") {
		parts := strings.Split(column, ".")
		for i, part := range parts {
			part = strings.TrimSpace(part)
			if part == "*" {
				parts[i] = "*"
			} else {
				parts[i] = dialect.QuoteIdentifier(part)
			}
		}
		return strings.Join(parts, ".")
	}
	return dialect.QuoteIdentifier(column)
}

func requireOperator(dialect SQLDialect, op string) (string, error) {
	sqlOp := dialect.Operator(op)
	if strings.TrimSpace(sqlOp) == "" {
		return "", fmt.Errorf("unsupported operator %q", op)
	}
	return sqlOp, nil
}

// ToSQL converts this Expr condition to a parameterized SQL fragment.
func (e *Expr) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	if e.column == "" || e.operator == "" {
		return "", nil, fmt.Errorf("invalid expression")
	}

	op, err := requireOperator(dialect, e.operator)
	if err != nil {
		return "", nil, err
	}
	column := quoteColumn(dialect, e.column)

	if isUnaryOperator(e.operator) {
		sql := fmt.Sprintf("%s %s", column, op)
		return sql, nil, nil
	}

	if e.value == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}

	placeholder := dialect.Placeholder(paramBase)
	sql := fmt.Sprintf("%s %s %s", column, op, placeholder)

	return sql, []any{e.value}, nil
}
