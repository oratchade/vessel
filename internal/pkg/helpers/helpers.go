// Package helpers provides utility functions for SQL query building and formatting.
package helpers

import (
	"strings"

	"tounilab.com/fabric/pkg/query/condition"
)

// QuoteIdentifierSlice returns a slice of identifiers where each dotted segment
// is quoted using the provided dialect. If a non-empty prefix is provided it
// is prepended (unquoted) to each resulting identifier.
func QuoteIdentifierSlice(dialect condition.SQLDialect, identifiers []string, prefix string) []string {
	quoted := make([]string, len(identifiers))
	for i, id := range identifiers {
		// split dotted identifiers and quote each segment individually
		parts := strings.Split(id, ".")
		for j, p := range parts {
			part := strings.TrimSpace(p)
			if part == "*" {
				parts[j] = "*"
				continue
			}
			parts[j] = dialect.QuoteIdentifier(part)
		}
		quotedID := strings.Join(parts, ".")
		if prefix != "" {
			quoted[i] = prefix + quotedID
		} else {
			quoted[i] = quotedID
		}
	}
	return quoted
}
