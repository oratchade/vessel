package condition

import (
	"fmt"
	"strings"
)

// Or composes multiple conditions using the logical OR operator.
type Or struct {
	conditions []Condition
	operator   string
}

// NewOr constructs an Or condition ready to accept child conditions.
func NewOr() *Or {
	return &Or{
		operator: "OR",
	}
}

func (o *Or) Conditions(conditions ...Condition) *Or {
	o.conditions = append(o.conditions, conditions...)
	return o
}

func (o *Or) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	values, parts := make([]any, 0), make([]string, 0)

	for _, cdt := range o.conditions {
		str, args, err := cdt.ToSQL(dialect, paramBase)
		if err != nil {
			return "", nil, fmt.Errorf("error converting or condition to SQL: %w", err)
		}
		parts = append(parts, fmt.Sprintf("(%s)", str))
		values = append(values, args...)
		paramBase += len(args)
	}

	return strings.Join(parts, fmt.Sprintf(" %s ", dialect.Operator(o.operator))), values, nil
}
