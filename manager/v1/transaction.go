package v1

import (
	"context"
	"errors"
	"fmt"

	db "tounilab.com/vessel/db/v1"
)

// ErrEntryNotFound is returned when a named database entry is not configured.
var ErrEntryNotFound = errors.New("database entry not found")

// ErrEntryReadOnly is returned when a transaction targets a readonly entry.
var ErrEntryReadOnly = errors.New("database entry is readonly")

// ErrEntryUnhealthy is returned when a transaction targets an entry that is failing health checks.
var ErrEntryUnhealthy = errors.New("database entry is unhealthy")

// WithTransaction runs fn inside a database transaction on the named readwrite entry.
//
// Every statement fn issues through the Tx, including SELECT ... FOR UPDATE, runs on the
// transaction's single connection: worker queues and read/write routing do not apply, so
// a row lock is held for the rest of the callback and is never taken on a replica.
// The transaction commits when fn returns nil and rolls back when fn returns an error or
// panics; a panic is returned as an error.
//
// WithTransaction never retries. Retry by wrapping the whole call, never by replaying
// individual statements inside fn. Finish transactions before Stop, which closes connections.
func (dm *DBManager) WithTransaction(
	ctx context.Context,
	entryName string,
	fn func(db.Tx) error,
	opts ...db.TransactionOptions,
) error {
	if err := dm.ensureRunning(); err != nil {
		return fmt.Errorf("DBManager.WithTransaction: %w", err)
	}
	if fn == nil {
		return errors.New("DBManager.WithTransaction: transaction callback is nil")
	}
	entry, err := dm.transactionEntry(entryName)
	if err != nil {
		return fmt.Errorf("DBManager.WithTransaction: %w", err)
	}
	if err := entry.db.WithTransaction(ctx, fn, opts...); err != nil {
		return fmt.Errorf("DBManager.WithTransaction(%s): %w", entryName, err)
	}
	return nil
}

func (dm *DBManager) transactionEntry(name string) (*DBEntry, error) {
	if entry, ok := dm.readWriteEntries[name]; ok {
		if !entry.Health() {
			return nil, fmt.Errorf("%q: %w", name, ErrEntryUnhealthy)
		}
		return entry, nil
	}
	if _, ok := dm.readOnlyEntries[name]; ok {
		return nil, fmt.Errorf("%q: %w", name, ErrEntryReadOnly)
	}
	return nil, fmt.Errorf("%q: %w", name, ErrEntryNotFound)
}
