package operator

// Operators and SQL keywords used by query builders and dialects. These
// constants represent logical operators, join types and clause keywords that
// are mapped to dialect-specific SQL by the builder layer.

const (
	As                      = "as"
	Equal                   = "eq"
	NotEqual                = "neq"
	LowerThan               = "lt"
	LowerThanOrEqual        = "lte"
	GreaterThan             = "gt"
	GreaterThanOrEqual      = "gte"
	And                     = "and"
	Or                      = "or"
	Like                    = "like"
	NotLike                 = "not like"
	InsensitiveCaseLike     = "ilike"
	In                      = "in"
	NotIn                   = "not in"
	Between                 = "between"
	NotBetween              = "not between"
	IsNull                  = "is null"
	IsNotNull               = "is not null"
	Distinct                = "distinct"
	NotDistinct             = "not distinct"
	IsDistinctFrom          = "is distinct from"
	Contains                = "contains"
	Contained               = "contained"
	Overlaps                = "overlaps"
	Regex                   = "regex"
	NotRegex                = "not regex"
	InsensitiveCaseRegex    = "iregex"
	NotInsensitiveCaseRegex = "not iregex"

	Inner = "inner"
	Left  = "left"
	Right = "right"
	Full  = "full"
	Using = "using"

	Limit     = "limit"
	Offset    = "offset"
	Returning = "returning"
	OrderBy   = "order by"
	GroupBy   = "group by"
	Having    = "having"
	Output    = "output"
)
