package condition

// Or composes multiple conditions using the logical OR operator.
type Or = And

// NewOr constructs an Or condition ready to accept child conditions.
func NewOr() *Or {
	return &Or{
		operator: "OR",
	}
}
