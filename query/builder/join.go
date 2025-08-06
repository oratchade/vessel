package builder

import (
	"fmt"

	"tounilab.com/db-connector/query"
	"tounilab.com/db-connector/query/condition"
)

type Join struct {
	Type  string // e.g., "INNER", "LEFT", "RIGHT"
	Table string // Table to join
	Alias string // Optional alias
	Left  string // Column from the left table, generally the main table
	Right string // Column from the right table, generally the joined table
}

func (j *Join) ToSQL(table string, dialect condition.SQLDialect) string {
	jn := fmt.Sprintf("%s JOIN %s", dialect.Operator(j.Type), dialect.QuoteIdentifier(j.Table))

	if j.Alias != "" {
		jn += fmt.Sprintf(" AS %s", dialect.QuoteIdentifier(j.Alias))
	}

	on := fmt.Sprintf("%s (%s)", dialect.Operator(query.Using), dialect.QuoteIdentifier(j.Left))

	if j.Left != j.Right {
		on = fmt.Sprintf(
			"ON %s.%s = %s.%s",
			dialect.QuoteIdentifier(table), dialect.QuoteIdentifier(j.Left),
			dialect.QuoteIdentifier(j.Table), dialect.QuoteIdentifier(j.Right),
		)
	}

	return fmt.Sprintf("%s %s", jn, on)
}
