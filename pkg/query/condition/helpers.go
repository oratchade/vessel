package condition

import "fmt"

// IsNull creates a column IS NULL condition.
func IsNull(column string) Condition {
	return NewExpr().Column(column).Op("IS NULL")
}

// IsNotNull creates a column IS NOT NULL condition.
func IsNotNull(column string) Condition {
	return NewExpr().Column(column).Op("IS NOT NULL")
}

// Like creates a column LIKE pattern condition.
func Like(column string, pattern any) Condition {
	return NewExpr().Column(column).Op("LIKE").Value(pattern)
}

// NotLike creates a column NOT LIKE pattern condition.
func NotLike(column string, pattern any) Condition {
	return NewExpr().Column(column).Op("NOT LIKE").Value(pattern)
}

// ILike creates a portable case-insensitive LIKE condition.
//
// Fabric renders this as LOWER(column) LIKE LOWER(?) for all built-in dialects.
func ILike(column string, pattern any) Condition {
	return &insensitiveLike{column: column, pattern: pattern}
}

type insensitiveLike struct {
	column  string
	pattern any
}

func (i *insensitiveLike) ToSQL(dialect SQLDialect, paramBase int) (string, []any, error) {
	if i.column == "" || i.pattern == nil {
		return "", nil, fmt.Errorf("invalid expression")
	}
	column := quoteColumn(dialect, i.column)
	placeholder := dialect.Placeholder(paramBase)
	return fmt.Sprintf("LOWER(%s) LIKE LOWER(%s)", column, placeholder), []any{i.pattern}, nil
}
