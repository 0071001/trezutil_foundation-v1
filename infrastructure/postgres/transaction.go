package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// TxManager manages database transactions.
type TxManager struct {
	db *sql.DB
}

// NewTxManager creates a new transaction manager.
func NewTxManager(db *sql.DB) *TxManager {
	return &TxManager{db: db}
}

// RunInTx executes a function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
func (m *TxManager) RunInTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction failed: %v, rollback failed: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Tx wraps a sql.Tx with context for use in repositories.
type Tx struct {
	*sql.Tx
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, query, args...)
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.Tx.QueryContext(ctx, query, args...)
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.Tx.QueryRowContext(ctx, query, args...)
}
