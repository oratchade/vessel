package query

import (
	"fmt"

	"tounilab.com/db-connector/query/condition"
)

func QuoteIdentifierSlice(dialect condition.SQLDialect, identifiers []string, prefix string) []string {
	quoted := make([]string, len(identifiers))
	for i, id := range identifiers {
		quoted[i] = dialect.QuoteIdentifier(fmt.Sprintf("%s%s", prefix, id))
	}
	return quoted
}
