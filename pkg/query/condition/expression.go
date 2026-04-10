package condition

import (
	"fmt"
	"strings"
)

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

// ToSQL converts this Expr condition to a parameterized SQL fragment.
func (e *Expr) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	if e.column == "" || e.operator == "" {
		return "", nil, fmt.Errorf("invalid expression")
	}

	op := dialect.Operator(e.operator)

	if isUnaryOperator(e.operator) {
		sql := fmt.Sprintf("%s %s", e.column, op)
		return sql, nil, nil
	}

	if e.value == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}

	placeholder := dialect.Placeholder(paramBase)
	sql := fmt.Sprintf("%s %s %s", e.column, op, placeholder)

	return sql, []any{e.value}, nil
}
