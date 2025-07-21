package condition

import "fmt"

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

func (b *Between) ToSQL(paramBase int) (string, []any, error) {
	if b.column == "" || b.operator == "" || b.from == nil || b.to == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}

	if paramBase > 0 {
		b.paramBase = paramBase
	}

	fromPlaceholder := b.dialect.Placeholder(b.paramBase)
	toPlaceholder := b.dialect.Placeholder(b.paramBase + 1)
	sql := fmt.Sprintf(
		"%s %s %s %s %s",
		b.column, b.dialect.Operator(b.operator), fromPlaceholder, b.dialect.Operator("AND"), toPlaceholder,
	)

	return sql, []any{b.from, b.to}, nil
}
