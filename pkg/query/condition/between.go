package condition

import "fmt"

// Between represents a SQL BETWEEN condition (column BETWEEN from AND to).
type Between struct {
	column   string
	operator string
	from     any
	to       any
}

// NewBetween constructs a Between condition builder.
func NewBetween() *Between {
	return &Between{
		operator: "BETWEEN",
	}
}

// Column sets the column name for this Between condition.
func (b *Between) Column(col string) *Between {
	b.column = col
	return b
}

// From sets the lower bound for this Between condition.
func (b *Between) From(val any) *Between {
	b.from = val
	return b
}

// To sets the upper bound for this Between condition.
func (b *Between) To(val any) *Between {
	b.to = val
	return b
}

// ToSQL converts this Between condition to a parameterized SQL fragment.
func (b *Between) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	if b.column == "" || b.operator == "" || b.from == nil || b.to == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}

	op, err := requireOperator(dialect, b.operator)
	if err != nil {
		return "", nil, err
	}
	andOp, err := requireOperator(dialect, "AND")
	if err != nil {
		return "", nil, err
	}

	fromPlaceholder := dialect.Placeholder(paramBase)
	toPlaceholder := dialect.Placeholder(paramBase + 1)
	sql := fmt.Sprintf(
		"%s %s %s %s %s",
		quoteColumn(dialect, b.column), op, fromPlaceholder, andOp, toPlaceholder,
	)

	return sql, []any{b.from, b.to}, nil
}
