package builder

import (
	cdt "tounilab.com/db-connector/query/condition"
)

type QueryBuilder interface {
	Select(table string, columns []string, cond cdt.Condition) (string, []any, error)
	Insert(table string, data map[string]any) (string, []any, error)
	Update(table string, data map[string]any, cond cdt.Condition) (string, []any, error)
	Delete(table string, cond cdt.Condition) (string, []any, error)
}
