//go:build test

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "tounilab.com/vessel/db/v1"
	"tounilab.com/vessel/pkg/query/options"
)

func TestMutationExecutionRejectsReturning(t *testing.T) {
	opts := &options.QueryOptions{Returning: []string{"id"}}

	for _, operation := range []string{"Insert", "Inserts", "Update", "Delete"} {
		t.Run(operation, func(t *testing.T) {
			err := v1.ExportRejectExecutingReturning(operation, opts)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "RETURNING/OUTPUT execution is not supported")
			assert.Contains(t, err.Error(), operation+"Query")
		})
	}
}

func TestMutationExecutionAllowsNilOrEmptyReturning(t *testing.T) {
	assert.NoError(t, v1.ExportRejectExecutingReturning("Insert", nil))
	assert.NoError(t, v1.ExportRejectExecutingReturning("Insert", &options.QueryOptions{}))
}
