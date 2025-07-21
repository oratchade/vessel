package condition

import "fmt"

type Not struct {
	condition Condition
	operator  string
	dialect   SQLDialect
	paramBase int
}

func NewNot() *Not {
	return &Not{
		operator:  "NOT",
		paramBase: 1,
	}
}

func (n *Not) Condition(cond Condition) *Not {
	n.condition = cond
	return n
}

func (n *Not) Dialect(d SQLDialect) *Not {
	n.dialect = d
	return n
}

func (n *Not) ParamBase(base int) *Not {
	n.paramBase = base
	return n
}

func (n *Not) ToSQL(paramBase int) (string, []any, error) {
	if paramBase > 0 {
		n.paramBase = paramBase
	}

	str, args, err := n.condition.ToSQL(n.paramBase)
	if err != nil {
		return "", nil, fmt.Errorf("error converting condition to SQL: %w", err)
	}

	return fmt.Sprintf("%s %s", n.dialect.Operator(n.operator), str), args, nil
}
