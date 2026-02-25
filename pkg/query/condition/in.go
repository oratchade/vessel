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

func (i *In) Column(col string) *In {
	i.column = col
	return i
}

func (i *In) Values(vals ...any) *In {
	i.values = vals
	return i
}

func (i *In) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	if i.column == "" || i.operator == "" || i.values == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}

	placeholder := []string{}

	for range i.values {
		placeholder = append(placeholder, dialect.Placeholder(paramBase))
		paramBase++
	}

	sql := fmt.Sprintf(
		"%s %s (%s)",
		i.column, dialect.Operator(i.operator), strings.Join(placeholder, ", "),
	)

	return sql, i.values, nil
}
