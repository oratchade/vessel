//go:build test

package helpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tounilab.com/vessel/internal/pkg/helpers"
)

type mockDialect struct {
	quoteChar      string
	closeQuoteChar string
}

func (m mockDialect) QuoteIdentifier(id string) string {
	return m.quoteChar + id + m.closeQuoteChar
}

func (m mockDialect) Placeholder(index int) string {
	return ""
}

func (m mockDialect) Operator(op string) string {
	return op
}

func (m mockDialect) QuoteString(value string) string {
	return "'" + value + "'"
}

func TestQuoteIdentifierSliceNoPrefix(t *testing.T) {
	dialect := mockDialect{quoteChar: "`", closeQuoteChar: "`"}
	result := helpers.QuoteIdentifierSlice(dialect, []string{"id", "name"}, "")
	assert.Equal(t, []string{"`id`", "`name`"}, result)
}

func TestQuoteIdentifierSliceWithPrefix(t *testing.T) {
	dialect := mockDialect{quoteChar: `"`, closeQuoteChar: `"`}
	result := helpers.QuoteIdentifierSlice(dialect, []string{"id"}, "t.")
	assert.Equal(t, []string{`t."id"`}, result)
}

func TestQuoteIdentifierSliceDottedIdentifiers(t *testing.T) {
	dialect := mockDialect{quoteChar: "`", closeQuoteChar: "`"}
	result := helpers.QuoteIdentifierSlice(dialect, []string{"users.id"}, "")
	assert.Equal(t, []string{"`users`.`id`"}, result)
}

func TestQuoteIdentifierSliceEmpty(t *testing.T) {
	dialect := mockDialect{quoteChar: "`", closeQuoteChar: "`"}
	result := helpers.QuoteIdentifierSlice(dialect, []string{}, "")
	assert.Equal(t, []string{}, result)
}

func TestQuoteIdentifierSliceWithSpaces(t *testing.T) {
	dialect := mockDialect{quoteChar: "[", closeQuoteChar: "]"}
	result := helpers.QuoteIdentifierSlice(dialect, []string{" id "}, "")
	assert.Equal(t, []string{"[id]"}, result)
}
