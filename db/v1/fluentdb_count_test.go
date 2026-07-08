//go:build test

package v1_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	v1 "tounilab.com/vessel/db/v1"
)

// TestSelectBuilderCountDriverReturnTypes covers COUNT(*) values delivered as
// the non-int64 types real drivers use (byte slices and strings from text
// protocols, unsigned integers), which previously fell through to an error.
func TestSelectBuilderCountDriverReturnTypes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := v1.NewMockDBActions(ctrl)
	fluentDB := v1.NewFluentDB(mockDB)

	testCases := []struct {
		name          string
		countValue    any
		expectedCount int64
	}{
		{name: "bytes", countValue: []byte("42"), expectedCount: 42},
		{name: "string", countValue: "7", expectedCount: 7},
		{name: "uint64", countValue: uint64(9), expectedCount: 9},
		{name: "int32", countValue: int32(3), expectedCount: 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB.EXPECT().
				Get(context.Background(), "users", gomock.Any(), nil, nil, nil).
				Return([]map[string]any{{"count": tc.countValue}}, nil).
				Times(1)

			count, err := fluentDB.Select("users").Count(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedCount, count)
		})
	}
}
