package condition

import "fmt"

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

func (o *Or) ToSQL(paramBase int) (string, []any, error) {
	var values []any
	strCdt := ""

	for i, cdt := range o.conditions {
		if i > 0 {
			strCdt = fmt.Sprintf("%s %s", strCdt, o.dialect.Operator(o.operator))
		}

		str, args, err := cdt.ToSQL(o.paramBase)
		if err != nil {
			return "", nil, fmt.Errorf("error converting condition %d to SQL: %w", i, err)
		}

		strCdt = fmt.Sprintf("%s (%s)", strCdt, str)
		values = append(values, args...)
		o.paramBase += len(args)
	}

	return strCdt, values, nil
}
