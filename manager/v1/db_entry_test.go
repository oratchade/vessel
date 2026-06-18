//go:build test

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "tounilab.com/vessel/manager/v1"
)

func TestQueryRequestConstants(t *testing.T) {
	assert.Equal(t, "get", v1.ReqGet)
	assert.Equal(t, "insert", v1.ReqInsert)
	assert.Equal(t, "upsert", v1.ReqUpsert)
	assert.Equal(t, "upserts", v1.ReqUpserts)
	assert.Equal(t, "update", v1.ReqUpdate)
	assert.Equal(t, "delete", v1.ReqDelete)
	assert.Equal(t, "exec", v1.ReqExec)
}
