package v1

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"runtime/debug"
)

var savepointNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// TransactionOptions controls transaction behavior for Begin and WithTransaction.
type TransactionOptions struct {
	// Isolation is the database isolation level. Leave zero to use the driver default.
	Isolation sql.IsolationLevel
	// ReadOnly requests a read-only transaction where the driver supports it.
	ReadOnly bool
}

func firstTransactionOptions(opts []TransactionOptions) TransactionOptions {
	if len(opts) == 0 {
		return TransactionOptions{}
	}
	return opts[0]
}

func runTransaction(ctx context.Context, operation string, tx Tx, fn func(Tx) error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			rollbackErr := tx.Rollback(ctx)
			panicErr := fmt.Errorf(
				"%s: transaction callback panicked: %v\n%s",
				operation,
				p,
				debug.Stack(),
			)
			if rollbackErr != nil {
				err = fmt.Errorf("%w; rollback failed: %w", panicErr, rollbackErr)
				return
			}
			err = panicErr
		}
	}()

	if err = fn(tx); err != nil {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil {
			return fmt.Errorf("%s: execution failed: %w; rollback failed: %w", operation, err, rollbackErr)
		}
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: failed to commit transaction: %w", operation, err)
	}
	return nil
}

func savepoint(ctx context.Context, tx Tx, name string, isMSSQL bool) error {
	if err := validateSavepointName(name); err != nil {
		return err
	}
	query := "SAVEPOINT " + name
	if isMSSQL {
		query = "SAVE TRANSACTION " + name
	}
	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("savepoint: %w", err)
	}
	return nil
}

func rollbackToSavepoint(ctx context.Context, tx Tx, name string, isMSSQL bool) error {
	if err := validateSavepointName(name); err != nil {
		return err
	}
	query := "ROLLBACK TO SAVEPOINT " + name
	if isMSSQL {
		query = "ROLLBACK TRANSACTION " + name
	}
	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("rollback to savepoint: %w", err)
	}
	return nil
}

func releaseSavepoint(ctx context.Context, tx Tx, name string, isMSSQL bool) error {
	if err := validateSavepointName(name); err != nil {
		return err
	}
	if isMSSQL {
		return fmt.Errorf("release savepoint: MSSQL does not support releasing savepoints")
	}
	if _, err := tx.Exec(ctx, "RELEASE SAVEPOINT "+name); err != nil {
		return fmt.Errorf("release savepoint: %w", err)
	}
	return nil
}

func validateSavepointName(name string) error {
	if !savepointNamePattern.MatchString(name) {
		return fmt.Errorf("savepoint name %q must match %s", name, savepointNamePattern.String())
	}
	return nil
}
