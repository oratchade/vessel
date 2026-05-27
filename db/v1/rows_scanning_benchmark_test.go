//go:build test

package v1_test

import (
	"context"
	"testing"

	v1 "tounilab.com/vessel/db/v1"
)

func BenchmarkScanAll(b *testing.B) {
	type User struct {
		ID    int    `db:"id"`
		Email string `db:"email"`
		Name  string `db:"name"`
	}

	rows := make([][]any, 100)
	for i := range rows {
		rows[i] = []any{int64(i), "user@example.com", "Ada"}
	}

	for b.Loop() {
		adapter := v1.NewRowsAdapterWithMockRows(NewMockRows([]string{"id", "email", "name"}, rows))
		_, _ = v1.ScanAll[User](context.Background(), adapter)
	}
}
