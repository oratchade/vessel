package condition

import (
	"fmt"
	"strings"
)

// In represents a SQL IN condition (column IN (...)).
type In struct {
	column   string
	operator string
	values   []any
}

// NewIn constructs an In condition builder.
func NewIn() *In {
	return &In{
		operator: "IN",
	}
}

// Column sets the column name for this In condition.
func (i *In) Column(col string) *In {
	i.column = col
	return i
}

// Values sets the values for this In condition.
func (i *In) Values(vals ...any) *In {
	i.values = vals
	return i
}

// ToSQL converts this In condition to a parameterized SQL fragment.
func (i *In) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	if i.column == "" || i.operator == "" || i.values == nil {
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
