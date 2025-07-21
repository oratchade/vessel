package condition

import (
	"fmt"
	"strings"
)

type In struct {
	column    string
	operator  string
	values    []any
	dialect   SQLDialect
	paramBase int
}

func NewIn() *In {
	return &In{
		operator:  "IN",
		paramBase: 1,
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

func (i *In) Dialect(d SQLDialect) *In {
	i.dialect = d
	return i
}

func (i *In) ParamBase(base int) *In {
	i.paramBase = base
	return i
}

func (i *In) ToSQL(paramBase int) (string, []any, error) {
	if i.column == "" || i.operator == "" || i.values == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}

	if paramBase > 0 {
		i.paramBase = paramBase
	}

	placeholder := []string{}

	for range i.values {
		placeholder = append(placeholder, i.dialect.Placeholder(i.paramBase))
		i.paramBase++
	}

	sql := fmt.Sprintf(
		"%s %s (%s)",
		i.column, i.dialect.Operator(i.operator), strings.Join(placeholder, ", "),
	)

	return sql, i.values, nil
}
