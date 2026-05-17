package v1

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"time"

	db "tounilab.com/fabric/db/v1"
	"tounilab.com/fabric/pkg/query/options"
)

type insertBatch struct {
	table     string
	columns   string
	opts      *options.QueryOptions
	requests  []*Query
	rows      []map[string]any
	startedAt time.Time
}

func (b *insertBatch) len() int {
	if b == nil {
		return 0
	}
	return len(b.requests)
}

func (b *insertBatch) empty() bool {
	return b == nil || len(b.requests) == 0
}

func (b *insertBatch) compatible(q *Query) bool {
	if b.empty() {
		return true
	}
	if q == nil || q.Data == nil {
		return false
	}
	return b.table == q.Data.Table &&
		b.columns == insertColumnsKey(q.Data.Data) &&
		reflect.DeepEqual(b.opts, q.Data.Opts)
}

func (b *insertBatch) add(q *Query) {
	if b.empty() {
		b.table = q.Data.Table
		b.columns = insertColumnsKey(q.Data.Data)
		b.opts = q.Data.Opts
		b.startedAt = time.Now()
	}
	b.requests = append(b.requests, q)
	b.rows = append(b.rows, q.Data.Data)
}

func (b *insertBatch) reset() {
	b.table = ""
	b.columns = ""
	b.opts = nil
	b.requests = nil
	b.rows = nil
	b.startedAt = time.Time{}
}

func (b *insertBatch) flush(ctx context.Context, de *DBEntry) {
	if b.empty() {
		return
	}

	result, err := de.db.Inserts(ctx, b.table, b.rows, b.opts)
	responses := b.responses(result, err)
	for i, q := range b.requests {
		de.sendResponseWithTimeout(ctx, q, responses[i])
	}
	b.reset()
}

func (b *insertBatch) responses(result *db.ExecResult, err error) []*QueryResponse {
	responses := make([]*QueryResponse, len(b.requests))
	if err != nil {
		for i := range responses {
			responses[i] = &QueryResponse{Error: err}
		}
		return responses
	}

	rowsAffected := int64(0)
	if result != nil {
		rowsAffected = result.RowsAffected
	}
	for i := range responses {
		responseRowsAffected := rowsAffected
		if rowsAffected == int64(len(b.requests)) {
			responseRowsAffected = 1
		}
		responses[i] = &QueryResponse{
			ExecData: &db.ExecResult{RowsAffected: responseRowsAffected},
		}
	}
	return responses
}

func insertColumnsKey(data map[string]any) string {
	columns := make([]string, 0, len(data))
	for column := range data {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	return strings.Join(columns, "\x00")
}
