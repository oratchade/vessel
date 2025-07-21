package condition

type Or struct {
	conditions []Condition
	operator   string
	dialect    SQLDialect
	paramBase  int
}

func NewOr() *Or {
	return &Or{
		paramBase: 1,
		operator:  "OR",
	}
}

func (o *Or) Conditions(conditions ...Condition) *Or {
	o.conditions = append(o.conditions, conditions...)
	return o
}

func (o *Or) Dialect(d SQLDialect) *Or {
	o.dialect = d
	return o
}

func (o *Or) ParamBase(base int) *Or {
	o.paramBase = base
	return o
}

func (o *Or) ToSQL() (string, []any, error) {
	return "", nil, nil
}
