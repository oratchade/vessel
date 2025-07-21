package condition

type And struct {
	conditions []Condition
	operator   string
	dialect    SQLDialect
	paramBase  int
}

func NewAnd() *And {
	return &And{
		paramBase: 1,
		operator:  "AND",
	}
}

func (a *And) Conditions(conditions ...Condition) *And {
	a.conditions = append(a.conditions, conditions...)
	return a
}

func (a *And) Dialect(d SQLDialect) *And {
	a.dialect = d
	return a
}

func (a *And) ParamBase(base int) *And {
	a.paramBase = base
	return a
}

func (a *And) ToSQL() (string, []any, error) {
	return "", nil, nil
}
