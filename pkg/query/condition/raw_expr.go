package condition

// RawExpr is a trusted SQL expression in a value position, such as NOW() or
// counter + 1. It is rendered inline instead of being bound as a parameter.
//
// The SQL fragment is caller-owned and is not quoted or parameterized. Only pass
// trusted, allowlisted SQL syntax here; never route user input into it.
type RawExpr string
