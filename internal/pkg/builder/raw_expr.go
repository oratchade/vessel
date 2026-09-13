package builder

import (
	"fmt"
	"sort"
	"strings"

	cdt "tounilab.com/vessel/pkg/query/condition"
)

// setClauses renders "column = value" assignments in sorted column order,
// numbering placeholders from 1. It returns the next free placeholder index.
func setClauses(dialect cdt.SQLDialect, data map[string]any) ([]string, []any, int, error) {
	columns := make([]string, 0, len(data))
	for col := range data {
		columns = append(columns, col)
	}
	sort.Strings(columns)

	index := 1
	sets, values := make([]string, 0, len(columns)), make([]any, 0, len(columns))
	for _, col := range columns {
		placeholder, args, err := bindValue(dialect, data[col], index)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("column %q: %w", col, err)
		}
		sets = append(sets, fmt.Sprintf("%s = %s", dialect.QuoteIdentifier(col), placeholder))
		values = append(values, args...)
		index += len(args)
	}
	return sets, values, index, nil
}

// bindValue renders value as the placeholder at index, or inline when it is a
// trusted cdt.RawExpr. Callers advance their placeholder index by len(args).
func bindValue(dialect cdt.SQLDialect, value any, index int) (string, []any, error) {
	raw, ok := value.(cdt.RawExpr)
	if !ok {
		return dialect.Placeholder(index), []any{value}, nil
	}
	sql := strings.TrimSpace(string(raw))
	if sql == "" {
		return "", nil, fmt.Errorf("raw expression cannot be empty")
	}
	return sql, nil, nil
}
