package v1

import (
	"context"
	"fmt"
	"runtime/debug"
)

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
