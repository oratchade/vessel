package condition

type Condition interface {
	ToSQL(dialect SQLDialect, paramBase int) (string, []any, error)
}

type SQLDialect interface {
	Placeholder(index int) string
	Operator(op string) string
}

/*
	conditions := And{
	    Expr{"age", ">", 30},
	    Or{
	        Expr{"status", "=", "active"},
	        Expr{"status", "=", "pending"},
	    },
	    Not{
	        Cond: Expr{"deleted_at", strings.ToUpper(query.IsNotNull), nil},
	    },
		In{
			column:   "id",
			operator: strings.ToUpper(query.In),
			values:   []any{1, 2, 3},
		},
		Between{
			column:    "created_at",
			operator:  strings.ToUpper(query.Between),
			from:      "2023-01-01",
			to:        "2023-12-31",
		},
	}
*/
