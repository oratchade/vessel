package v1

import cdt "tounilab.com/vessel/pkg/query/condition"

// RawExpr is a trusted SQL expression rendered inline wherever a value would
// otherwise be bound: Set, SetMap, Values, ValuesBulk, DoUpdateSet, and
// condition Expr values.
//
// The SQL fragment is caller-owned and is not quoted or parameterized. Only pass
// trusted, allowlisted SQL syntax here; never route user input into it.
//
//	NewFluentDB(db).Update("jobs").
//	    Set("completed_at", RawExpr("NOW()")).
//	    Where(cdt.NewExpr().Column("expires_at").Op(">").Value(RawExpr("NOW()"))).
//	    Exec(ctx)
type RawExpr = cdt.RawExpr
