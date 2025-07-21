package condition

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

func (e *Expr) ToSQL() (string, []any, error) {
	return "", nil, nil
}
