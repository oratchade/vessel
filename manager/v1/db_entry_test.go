//go:build test

package v1_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	db "tounilab.com/fabric/db/v1"
	v1 "tounilab.com/fabric/manager/v1"
)

// MockDB implements db.DB interface for testing
type MockDB struct {
	closed bool
}

func (m *MockDB) Ping(ctx context.Context) error { return nil }

func (m *MockDB) Close() error { m.closed = true; return nil }

func (m *MockDB) Get(
	ctx context.Context,
	table string,
	columns []string,
	joins []any,
	conditions any,
	opts any,
) ([]map[string]any, error) {
	return nil, nil
}

func (m *MockDB) GetRaw(
	ctx context.Context,
	table string,
	columns []string,
	joins []any,
	conditions any,
	opts any,
) (any, error) {
	return nil, nil
}

func (m *MockDB) GetByID(
	ctx context.Context,
	table string,
	id any,
	joins []any,
	opts any,
) ([]map[string]any, error) {
	return nil, nil
}

func (m *MockDB) GetByIDRaw(
	ctx context.Context,
	table string,
	id any,
	joins []any,
	opts any,
) (any, error) {
	return nil, nil
}

func (m *MockDB) Insert(
	ctx context.Context,
	table string,
	data map[string]any,
	opts any,
) (*db.ExecResult, error) {
	return &db.ExecResult{RowsAffected: 1}, nil
}

func (m *MockDB) Inserts(
	ctx context.Context,
	table string,
	data []map[string]any,
	opts any,
) (*db.ExecResult, error) {
	return &db.ExecResult{RowsAffected: int64(len(data))}, nil
}

func (m *MockDB) Update(
	ctx context.Context,
	table string,
	data map[string]any,
	conditions any,
	opts any,
) (*db.ExecResult, error) {
	return &db.ExecResult{RowsAffected: 1}, nil
}

func (m *MockDB) Delete(
	ctx context.Context,
	table string,
	conditions any,
	opts any,
) (*db.ExecResult, error) {
	return &db.ExecResult{RowsAffected: 1}, nil
}

func (m *MockDB) Query(
	ctx context.Context,
	query string,
	args ...any,
) ([]map[string]any, error) {
	return nil, nil
}

func (m *MockDB) QueryRaw(
	ctx context.Context,
	query string,
	args ...any,
) (any, error) {
	return nil, nil
}

func (m *MockDB) Exec(
	ctx context.Context,
	query string,
	args ...any,
) (*db.ExecResult, error) {
	return &db.ExecResult{RowsAffected: 1}, nil
}

func (m *MockDB) Statistics() (*db.PoolStatistics, error) {
	return &db.PoolStatistics{}, nil
}

func TestQueryRequestConstants(t *testing.T) {
	assert.Equal(t, "get", v1.ReqGet)
	assert.Equal(t, "insert", v1.ReqInsert)
	assert.Equal(t, "update", v1.ReqUpdate)
	assert.Equal(t, "delete", v1.ReqDelete)
	assert.Equal(t, "exec", v1.ReqExec)
}
