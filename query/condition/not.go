package condition

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

func (n *Not) ToSQL() (string, []any, error) {
	return "", nil, nil
}
