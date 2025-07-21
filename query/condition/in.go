package condition

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

func (i *In) ToSQL() (string, []any, error) {
	return "", nil, nil
}
