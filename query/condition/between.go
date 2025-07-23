package condition

import "fmt"

type Between struct {
	column   string
	operator string
	from     any
	to       any
}

func NewBetween() *Between {
	return &Between{
		operator: "BETWEEN",
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

func (b *Between) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	if b.column == "" || b.operator == "" || b.from == nil || b.to == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}

	fromPlaceholder := dialect.Placeholder(paramBase)
	toPlaceholder := dialect.Placeholder(paramBase + 1)
	sql := fmt.Sprintf(
		"%s %s %s %s %s",
		b.column, dialect.Operator(b.operator), fromPlaceholder, dialect.Operator("AND"), toPlaceholder,
	)

	return sql, []any{b.from, b.to}, nil
}
