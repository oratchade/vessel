package condition

type Between struct {
	column    string
	operator  string
	from      any
	to        any
	dialect   SQLDialect
	paramBase int
}

func NewBetween() *Between {
	return &Between{
		operator:  "BETWEEN",
		paramBase: 1,
	}
}

func (b *Between) Column(col string) *Between {
	b.column = col
	return b
}

func (b *Between) From(val any) *Between {
	b.from = val
	return b
}

func (b *Between) To(val any) *Between {
	b.to = val
	return b
}

func (b *Between) Dialect(d SQLDialect) *Between {
	b.dialect = d
	return b
}

func (b *Between) ParamBase(base int) *Between {
	b.paramBase = base
	return b
}

func (b *Between) ToSQL() (string, []any, error) {
	return "", nil, nil
}
