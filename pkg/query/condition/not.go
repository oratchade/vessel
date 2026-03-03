package condition

import "fmt"

// Not negates a child condition.
type Not struct {
	condition Condition
	operator  string
}

// NewNot constructs a Not condition.
func NewNot() *Not {
	return &Not{
		operator: "NOT",
	}
}

// Condition sets the child condition to negate.
func (n *Not) Condition(cond Condition) *Not {
	n.condition = cond
	return n
}

// ToSQL converts this Not condition to a parameterized SQL fragment.
func (n *Not) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	str, args, err := n.condition.ToSQL(dialect, paramBase)
	if err != nil {
		return "", nil, fmt.Errorf("error converting condition to SQL: %w", err)
	}

	return fmt.Sprintf("%s (%s)", dialect.Operator(n.operator), str), args, nil
}
