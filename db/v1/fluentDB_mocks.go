//go:build test

package v1

import (
	"github.com/golang/mock/gomock"
)

// MockDBActions is a composite mock that implements the dbActions interface.
// It embeds the three component mocks (Mockreader, Mockwriter, Mockintrospector).
type MockDBActions struct {
	*Mockreader
	*Mockwriter
	*Mockupserter
	*Mockintrospector
}

// NewMockDBActions creates a new MockDBActions instance combining Mockreader,
// Mockwriter, and Mockintrospector mocks.
func NewMockDBActions(ctrl *gomock.Controller) *MockDBActions {
	m := &MockDBActions{
		Mockreader:       NewMockreader(ctrl),
		Mockwriter:       NewMockwriter(ctrl),
		Mockupserter:     NewMockupserter(ctrl),
		Mockintrospector: NewMockintrospector(ctrl),
	}
	return m
}

// CompositeRecorder combines the mock recorders from reader, writer, and introspector.
// This allows tests to call db.EXPECT().Get(...), db.EXPECT().Insert(...), etc. seamlessly.
type CompositeRecorder struct {
	readerRecorder       *MockreaderMockRecorder
	writerRecorder       *MockwriterMockRecorder
	upserterRecorder     *MockupserterMockRecorder
	introspectorRecorder *MockintrospectorMockRecorder
}

// EXPECT returns a composite recorder that combines expectations for all three interfaces.
func (m *MockDBActions) EXPECT() *CompositeRecorder {
	return &CompositeRecorder{
		readerRecorder:       m.Mockreader.EXPECT(),
		writerRecorder:       m.Mockwriter.EXPECT(),
		upserterRecorder:     m.Mockupserter.EXPECT(),
		introspectorRecorder: m.Mockintrospector.EXPECT(),
	}
}

// Get methods - delegate to reader recorder
func (c *CompositeRecorder) Get(ctx, table, columns, joins, conditions, opts any) *gomock.Call {
	return c.readerRecorder.Get(ctx, table, columns, joins, conditions, opts)
}

func (c *CompositeRecorder) GetRaw(ctx, table, columns, joins, conditions, opts any) *gomock.Call {
	return c.readerRecorder.GetRaw(ctx, table, columns, joins, conditions, opts)
}

// Insert methods - delegate to writer recorder
func (c *CompositeRecorder) Insert(ctx, table, data, opts any) *gomock.Call {
	return c.writerRecorder.Insert(ctx, table, data, opts)
}

func (c *CompositeRecorder) Inserts(ctx, table, data, opts any) *gomock.Call {
	return c.writerRecorder.Inserts(ctx, table, data, opts)
}

func (c *CompositeRecorder) Upsert(ctx, table, data, upsertOpts, opts any) *gomock.Call {
	return c.upserterRecorder.Upsert(ctx, table, data, upsertOpts, opts)
}

func (c *CompositeRecorder) UpsertQuery(table, data, upsertOpts, opts any) *gomock.Call {
	return c.upserterRecorder.UpsertQuery(table, data, upsertOpts, opts)
}

func (c *CompositeRecorder) Update(ctx, table, data, joins, conditions, opts any) *gomock.Call {
	return c.writerRecorder.Update(ctx, table, data, joins, conditions, opts)
}

func (c *CompositeRecorder) Delete(ctx, table, joins, conditions, opts any) *gomock.Call {
	return c.writerRecorder.Delete(ctx, table, joins, conditions, opts)
}

func (c *CompositeRecorder) Exec(ctx, query any, args ...any) *gomock.Call {
	return c.writerRecorder.Exec(ctx, query, args...)
}

// GetQuery methods - delegate to introspector recorder
func (c *CompositeRecorder) GetQuery(table, columns, joins, conditions, opts any) *gomock.Call {
	return c.introspectorRecorder.GetQuery(table, columns, joins, conditions, opts)
}

func (c *CompositeRecorder) GetByIDQuery(table, id, joins, opts any) *gomock.Call {
	return c.introspectorRecorder.GetByIDQuery(table, id, joins, opts)
}

func (c *CompositeRecorder) InsertQuery(table, data, opts any) *gomock.Call {
	return c.introspectorRecorder.InsertQuery(table, data, opts)
}

func (c *CompositeRecorder) InsertsQuery(table, data, opts any) *gomock.Call {
	return c.introspectorRecorder.InsertsQuery(table, data, opts)
}

func (c *CompositeRecorder) UpdateQuery(table, data, joins, conditions, opts any) *gomock.Call {
	return c.introspectorRecorder.UpdateQuery(table, data, joins, conditions, opts)
}

func (c *CompositeRecorder) DeleteQuery(table, joins, conditions, opts any) *gomock.Call {
	return c.introspectorRecorder.DeleteQuery(table, joins, conditions, opts)
}

func (c *CompositeRecorder) Explain(ctx, query any, args ...any) *gomock.Call {
	return c.introspectorRecorder.Explain(ctx, query, args...)
}
