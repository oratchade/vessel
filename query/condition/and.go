package condition

import (
	"fmt"
	"strings"
)

type And struct {
	conditions []Condition
	operator   string
}

func NewAnd() *And {
	return &And{
		operator: "AND",
	}
}

func (a *And) Conditions(conditions ...Condition) *And {
	a.conditions = append(a.conditions, conditions...)
	return a
}

func (a *And) ParamBase(base int) *And {
	return a
}

func (a *And) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	values, parts := make([]any, 0), make([]string, 0)

	for _, cdt := range a.conditions {
		str, args, err := cdt.ToSQL(dialect, paramBase)
		if err != nil {
			return "", nil, fmt.Errorf("error converting and condition to SQL: %w", err)
		}
		parts = append(parts, fmt.Sprintf("(%s)", str))
		values = append(values, args...)
		paramBase += len(args)
	}

	return strings.Join(parts, fmt.Sprintf(" %s ", dialect.Operator(a.operator))), values, nil
}
