package builder

import (
	"fmt"
	"strings"

	"tounilab.com/db-connector/query"
	"tounilab.com/db-connector/query/condition"
)

type JoinCdt struct {
	Left  string // Column from the left table, generally the main table
	Right string // Column from the right table, generally the joined table
}

type JoinCdts []JoinCdt

func (j JoinCdts) on(table string, joinTable string, dialect condition.SQLDialect) string {
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
		return fmt.Sprintf("%s (%s)", dialect.Operator(query.Using), strings.Join(columns, ", "))
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

type Join struct {
	Type       string // e.g., "INNER", "LEFT", "RIGHT"
	Table      string // Table to join
	Alias      string // Optional alias
	Conditions JoinCdts
}

func (j *Join) ToSQL(table string, dialect condition.SQLDialect) string {
	jn := fmt.Sprintf("%s JOIN %s", dialect.Operator(j.Type), dialect.QuoteIdentifier(j.Table))

	if j.Alias != "" {
		jn += fmt.Sprintf(" AS %s", dialect.QuoteIdentifier(j.Alias))
	}

	on := j.Conditions.on(table, j.Table, dialect)

	return fmt.Sprintf("%s %s", jn, on)
}
