package builder

import (
	cdt "tounilab.com/db-connector/query/condition"
)

type QueryBuilder interface {
	BuildSelect(table string, columns []string, cond cdt.Condition) (string, []any, error)
	BuildInsert(table string, data map[string]any) (string, []any, error)
	BuildUpdate(table string, data map[string]any, cond cdt.Condition) (string, []any, error)
	BuildDelete(table string, cond cdt.Condition) (string, []any, error)
}
