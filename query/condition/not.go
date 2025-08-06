package condition

import "fmt"

type Not struct {
	condition Condition
	operator  string
}

func NewNot() *Not {
	return &Not{
		operator: "NOT",
	}
}

func (n *Not) Condition(cond Condition) *Not {
	n.condition = cond
	return n
}

func (n *Not) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	str, args, err := n.condition.ToSQL(dialect, paramBase)
	if err != nil {
		return "", nil, fmt.Errorf("error converting condition to SQL: %w", err)
	}

	return fmt.Sprintf("%s (%s)", dialect.Operator(n.operator), str), args, nil
}
