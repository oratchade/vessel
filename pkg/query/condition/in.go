package condition

import (
	"fmt"
	"strings"
)

// InCondition represents a SQL IN condition (column IN (...)).
type InCondition struct {
	column   string
	operator string
	values   []any
}

// NewIn constructs an In condition builder.
func NewIn() *InCondition {
	return &InCondition{
		operator: "IN",
	}
}

// In creates a column IN (...) condition.
func In(column string, values ...any) Condition {
	return NewIn().Column(column).Values(values...)
}

// NotIn creates a column NOT IN (...) condition.
func NotIn(column string, values ...any) Condition {
	return (&InCondition{operator: "NOT IN"}).Column(column).Values(values...)
}

// Column sets the column name for this In condition.
func (i *InCondition) Column(col string) *InCondition {
	i.column = col
	return i
}

// Values sets the values for this In condition.
func (i *InCondition) Values(vals ...any) *InCondition {
	i.values = vals
	return i
}

// ToSQL converts this In condition to a parameterized SQL fragment.
func (i *InCondition) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	if i.column == "" || i.operator == "" || len(i.values) == 0 {
		return "", nil, fmt.Errorf("invalid expression")
	}

	placeholder := []string{}

	for range i.values {
		placeholder = append(placeholder, dialect.Placeholder(paramBase))
		paramBase++
	}

	op, err := requireOperator(dialect, i.operator)
	if err != nil {
		return "", nil, err
	}

	sql := fmt.Sprintf(
		"%s %s (%s)",
		quoteColumn(dialect, i.column), op, strings.Join(placeholder, ", "),
	)

	return sql, i.values, nil
}
