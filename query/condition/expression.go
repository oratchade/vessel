package condition

import "fmt"

type Expr struct {
	column   string
	operator string
	value    any
}

func NewExpr() *Expr {
	return &Expr{}
}

func (e *Expr) Column(col string) *Expr {
	e.column = col
	return e
}

func (e *Expr) Op(op string) *Expr {
	e.operator = op
	return e
}

func (e *Expr) Value(val any) *Expr {
	e.value = val
	return e
}

func (e *Expr) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	if e.column == "" || e.operator == "" || e.value == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}

	placeholder := dialect.Placeholder(paramBase)
	sql := fmt.Sprintf("%s %s %s", e.column, dialect.Operator(e.operator), placeholder)

	return sql, []any{e.value}, nil
}
