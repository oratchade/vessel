package condition

import (
	"fmt"
	"strings"

	"tounilab.com/fabric/internal/pkg/operator"
)

// JoinCdt represents a pair of columns used for JOIN conditions.
type JoinCdt struct {
	Left  string // Column from the left table, generally the main table
	Right string // Column from the right table, generally the joined table
}

// JoinCdts is a slice of JoinCdt and provides helpers for JOIN condition rendering.
type JoinCdts []JoinCdt

// on generates the ON or USING clause for this join condition.
func (j JoinCdts) on(table string, joinTable string, dialect SQLDialect) string {
	similar := true
	columns := []string{}
	for _, cdt := range j {
		if cdt.Left != cdt.Right {
			similar = false
			break
		}
		columns = append(columns, dialect.QuoteIdentifier(cdt.Left))
	}

	if similar {
		return fmt.Sprintf("%s (%s)", dialect.Operator(operator.Using), strings.Join(columns, ", "))
	}

	columns = []string{}
	for _, cdt := range j {
		columns = append(columns, fmt.Sprintf(
			"%s.%s = %s.%s",
			dialect.QuoteIdentifier(table), dialect.QuoteIdentifier(cdt.Left),
			dialect.QuoteIdentifier(joinTable), dialect.QuoteIdentifier(cdt.Right),
		))
	}

	return fmt.Sprintf("ON %s", strings.Join(columns, " AND "))
}

// Reverse returns a new JoinCdts with Left and Right columns swapped.
func (j JoinCdts) Reverse() JoinCdts {
	reversed := make(JoinCdts, len(j))
	for i, cdt := range j {
		reversed[i] = JoinCdt{
			Left:  cdt.Right,
			Right: cdt.Left,
		}
	}
	return reversed
}

// Join describes a SQL JOIN clause including type, table, alias and join conditions.
type Join struct {
	Type       string // e.g., "INNER", "LEFT", "RIGHT"
	Table      string // Table to join
	Alias      string // Optional alias
	Conditions JoinCdts
	On         Condition // Optional extra ON condition
}

// ToSQL converts this Join to a SQL JOIN clause for the specified table.
func (j *Join) ToSQL(table string, dialect SQLDialect) string {
	sql, _, err := j.ToSQLWithArgs(table, dialect, 1)
	if err != nil {
		return fmt.Sprintf("%s JOIN %s", dialect.Operator(j.Type), dialect.QuoteIdentifier(j.Table))
	}
	return sql
}

// ToSQLWithArgs converts this Join to a SQL JOIN clause and returns ON-clause
// arguments. Join predicates with values are not supported by the current
// builder contract and return a clear error.
func (j *Join) ToSQLWithArgs(table string, dialect SQLDialect, paramBase int) (string, []any, error) {
	jn := fmt.Sprintf("%s JOIN %s", dialect.Operator(j.Type), dialect.QuoteIdentifier(j.Table))

	if j.Alias != "" {
		jn += fmt.Sprintf(" AS %s", dialect.QuoteIdentifier(j.Alias))
	}

	joinTable := j.Table
	if j.Alias != "" {
		joinTable = j.Alias
	}

	if len(j.Conditions) > 0 {
		on := j.Conditions.on(table, joinTable, dialect)
		if j.On == nil {
			return fmt.Sprintf("%s %s", jn, on), nil, nil
		}
	}

	var parts []string
	for _, cdt := range j.Conditions {
		parts = append(parts, fmt.Sprintf(
			"%s.%s = %s.%s",
			dialect.QuoteIdentifier(table), dialect.QuoteIdentifier(cdt.Left),
			dialect.QuoteIdentifier(joinTable), dialect.QuoteIdentifier(cdt.Right),
		))
	}
	var args []any
	if j.On != nil {
		onSQL, onArgs, err := j.On.ToSQL(dialect, paramBase)
		if err != nil {
			return "", nil, fmt.Errorf("join ON condition: %w", err)
		}
		if len(onArgs) > 0 {
			return "", nil, fmt.Errorf("join ON condition values are not supported")
		}
		parts = append(parts, onSQL)
		args = onArgs
	}

	if len(parts) == 0 {
		return "", nil, fmt.Errorf("join requires conditions or ON condition")
	}

	return fmt.Sprintf("%s ON %s", jn, strings.Join(parts, " AND ")), args, nil
}
