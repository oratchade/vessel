package condition

import "fmt"

type And struct {
	conditions []Condition
	operator   string
	dialect    SQLDialect
	paramBase  int
}

func NewAnd() *And {
	return &And{
		paramBase: 1,
		operator:  "AND",
	}
}

func (a *And) Conditions(conditions ...Condition) *And {
	a.conditions = append(a.conditions, conditions...)
	return a
}

func (a *And) Dialect(d SQLDialect) *And {
	a.dialect = d
	return a
}

func (a *And) ParamBase(base int) *And {
	a.paramBase = base
	return a
}

func (a *And) ToSQL(paramBase int) (string, []any, error) {
	var values []any
	strCdt := ""

	for i, cdt := range a.conditions {
		if i > 0 {
			strCdt = fmt.Sprintf("%s %s", strCdt, a.dialect.Operator(a.operator))
		}

		str, args, err := cdt.ToSQL(a.paramBase)
		if err != nil {
			return "", nil, fmt.Errorf("error converting condition %d to SQL: %w", i, err)
		}

		strCdt = fmt.Sprintf("%s (%s)", strCdt, str)
		values = append(values, args...)
		a.paramBase += len(args)
	}

	return fmt.Sprintf("(%s)", strCdt), values, nil
}
