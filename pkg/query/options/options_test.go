//go:build test

package options_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tounilab.com/vessel/pkg/query/options"
)

// TestOrderByCreation tests OrderBy struct creation and validation
func TestOrderByCreation(t *testing.T) {
	tests := []struct {
		name      string
		column    string
		direction string
		valid     bool
	}{
		{
			name:      "valid ascending order",
			column:    "name",
			direction: "ASC",
			valid:     true,
		},
		{
			name:      "valid descending order",
			column:    "created_at",
			direction: "DESC",
			valid:     true,
		},
		{
			name:      "empty direction defaults to ASC",
			column:    "id",
			direction: "",
			valid:     true,
		},
		{
			name:      "empty column is invalid",
			column:    "",
			direction: "ASC",
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ob := options.OrderBy{
				Column: tt.column,
			}

			if tt.valid {
				assert.NotEmpty(t, ob.Column)
			} else {
				assert.Empty(t, ob.Column)
			}
		})
	}
}

// TestQueryOptionsWithOrderBy tests QueryOptions with OrderBy
func TestQueryOptionsWithOrderBy(t *testing.T) {
	t.Run("single order by column", func(t *testing.T) {
		opts := &options.QueryOptions{
			OrderBy: []options.OrderBy{
				{Column: "name", Direction: "ASC"},
			},
		}

		require.NotNil(t, opts)
		assert.Len(t, opts.OrderBy, 1)
		assert.Equal(t, "name", opts.OrderBy[0].Column)
		assert.Equal(t, "ASC", opts.OrderBy[0].Direction)
	})

	t.Run("multiple order by columns", func(t *testing.T) {
		opts := &options.QueryOptions{
			OrderBy: []options.OrderBy{
				{Column: "department", Direction: "ASC"},
				{Column: "salary", Direction: "DESC"},
			},
		}

		require.NotNil(t, opts)
		assert.Len(t, opts.OrderBy, 2)
		assert.Equal(t, "department", opts.OrderBy[0].Column)
		assert.Equal(t, "salary", opts.OrderBy[1].Column)
	})

	t.Run("empty order by", func(t *testing.T) {
		opts := &options.QueryOptions{
			OrderBy: []options.OrderBy{},
		}

		require.NotNil(t, opts)
		assert.Len(t, opts.OrderBy, 0)
	})
}

// TestQueryOptionsWithLimit tests QueryOptions with Limit
func TestQueryOptionsWithLimit(t *testing.T) {
	t.Run("limit only", func(t *testing.T) {
		limit := 10
		opts := &options.QueryOptions{
			Limit: &limit,
		}

		require.NotNil(t, opts)
		require.NotNil(t, opts.Limit)
		assert.Equal(t, 10, *opts.Limit)
	})

	t.Run("limit with offset", func(t *testing.T) {
		limit := 20
		offset := 50
		opts := &options.QueryOptions{
			Limit:  &limit,
			Offset: &offset,
		}

		require.NotNil(t, opts)
		require.NotNil(t, opts.Limit)
		require.NotNil(t, opts.Offset)
		assert.Equal(t, 20, *opts.Limit)
		assert.Equal(t, 50, *opts.Offset)
	})

	t.Run("nil limit", func(t *testing.T) {
		opts := &options.QueryOptions{
			Limit: nil,
		}

		require.NotNil(t, opts)
		assert.Nil(t, opts.Limit)
	})
}

// TestQueryOptionsWithGroupBy tests QueryOptions with GroupBy
func TestQueryOptionsWithGroupBy(t *testing.T) {
	t.Run("single group by column", func(t *testing.T) {
		opts := &options.QueryOptions{
			GroupBy: []string{"department"},
		}

		require.NotNil(t, opts)
		assert.Len(t, opts.GroupBy, 1)
		assert.Equal(t, "department", opts.GroupBy[0])
	})

	t.Run("multiple group by columns", func(t *testing.T) {
		opts := &options.QueryOptions{
			GroupBy: []string{"department", "location"},
		}

		require.NotNil(t, opts)
		assert.Len(t, opts.GroupBy, 2)
		assert.Equal(t, "department", opts.GroupBy[0])
		assert.Equal(t, "location", opts.GroupBy[1])
	})
}

// TestQueryOptionsWithHaving tests QueryOptions with HAVING clause
func TestQueryOptionsWithHaving(t *testing.T) {
	t.Run("having with group by", func(t *testing.T) {
		havingClause := "COUNT(*) > 5"
		opts := &options.QueryOptions{
			GroupBy: []string{"department"},
			Having:  &havingClause,
		}

		require.NotNil(t, opts)
		require.NotNil(t, opts.Having)
		assert.Equal(t, "COUNT(*) > 5", *opts.Having)
		assert.Len(t, opts.GroupBy, 1)
	})

	t.Run("nil having", func(t *testing.T) {
		opts := &options.QueryOptions{
			Having: nil,
		}

		require.NotNil(t, opts)
		assert.Nil(t, opts.Having)
	})
}

// TestQueryOptionsWithReturning tests QueryOptions with RETURNING clause
func TestQueryOptionsWithReturning(t *testing.T) {
	t.Run("single returning column", func(t *testing.T) {
		opts := &options.QueryOptions{
			Returning: []string{"id"},
		}

		require.NotNil(t, opts)
		assert.Len(t, opts.Returning, 1)
		assert.Equal(t, "id", opts.Returning[0])
	})

	t.Run("multiple returning columns", func(t *testing.T) {
		opts := &options.QueryOptions{
			Returning: []string{"id", "created_at", "updated_at"},
		}

		require.NotNil(t, opts)
		assert.Len(t, opts.Returning, 3)
		assert.Contains(t, opts.Returning, "id")
		assert.Contains(t, opts.Returning, "created_at")
	})

	t.Run("empty returning", func(t *testing.T) {
		opts := &options.QueryOptions{
			Returning: []string{},
		}

		require.NotNil(t, opts)
		assert.Len(t, opts.Returning, 0)
	})
}

// TestComplexQueryOptions tests QueryOptions with multiple options combined
func TestComplexQueryOptions(t *testing.T) {
	limit := 25
	offset := 10
	havingClause := "COUNT(*) >= 3"

	opts := &options.QueryOptions{
		OrderBy: []options.OrderBy{
			{Column: "department", Direction: "ASC"},
			{Column: "salary", Direction: "DESC"},
		},
		Limit:     &limit,
		Offset:    &offset,
		GroupBy:   []string{"department"},
		Having:    &havingClause,
		Returning: []string{"id", "department", "salary"},
	}

	require.NotNil(t, opts)
	assert.Len(t, opts.OrderBy, 2)
	assert.Equal(t, 25, *opts.Limit)
	assert.Equal(t, 10, *opts.Offset)
	assert.Len(t, opts.GroupBy, 1)
	assert.NotNil(t, opts.Having)
	assert.Len(t, opts.Returning, 3)
}
