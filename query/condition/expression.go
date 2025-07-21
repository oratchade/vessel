package condition

import "fmt"

type Expr struct {
	column    string
	operator  string
	value     any
	dialect   SQLDialect
	paramBase int
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

func (e *Expr) Dialect(d SQLDialect) *Expr {
	e.dialect = d
	return e
}

func (e *Expr) ParamBase(base int) *Expr {
	e.paramBase = base
	return e
}

func (e *Expr) ToSQL(paramBase int) (string, []any, error) {
	if e.column == "" || e.operator == "" || e.value == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}

	if paramBase > 0 {
		e.paramBase = paramBase
	}

	placeholder := e.dialect.Placeholder(e.paramBase)
	sql := fmt.Sprintf("%s %s %s", e.column, e.dialect.Operator(e.operator), placeholder)

	return sql, []any{e.value}, nil
}
