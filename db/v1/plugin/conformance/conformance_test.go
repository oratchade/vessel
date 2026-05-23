//go:build test

package conformance_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/db/v1/plugin/conformance"
)

type testConfig struct{}

func (testConfig) Driver() string { return "test-plugin" }
func (testConfig) DSN() string    { return "" }

type testFactory struct {
	result any
	err    error
}

func (testFactory) Name() string { return "test-plugin" }

func (f testFactory) Create(context.Context, any) (any, error) {
	return f.result, f.err
}

func TestCheckFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := db.NewMockDB(ctrl)
	mockDB.EXPECT().Close().Return(nil)

	err := conformance.CheckFactory(context.Background(), testFactory{result: mockDB}, testConfig{})
	require.NoError(t, err)
}

func TestCheckFactoryRejectsWrongResult(t *testing.T) {
	err := conformance.CheckFactory(context.Background(), testFactory{result: "not-db"}, testConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected db.DB")
}
