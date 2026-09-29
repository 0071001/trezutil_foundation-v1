package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrLockTimeout is returned when an advisory lock could not be acquired in time.
var ErrLockTimeout = errors.New("resource is busy; retry shortly")

// AdvisoryLocker serializes work on a key across every service instance using
// a PostgreSQL transaction-scoped advisory lock. The holder keeps the lock on
// its own connection for the duration of fn; it is released on rollback (or if
// the process dies and the connection drops).
//
// Waiters poll with pg_try_advisory_xact_lock and give their connection back
// between attempts. A blocking pg_advisory_xact_lock would park one pooled
// connection per waiter, and once the pool is exhausted the holder could not
// obtain a connection to finish its own work: a pool-starvation deadlock.
type AdvisoryLocker struct {
	db      *sql.DB
	timeout time.Duration
}

// NewAdvisoryLocker creates an AdvisoryLocker with a 15s acquisition timeout.
func NewAdvisoryLocker(db *sql.DB) *AdvisoryLocker {
	return &AdvisoryLocker{db: db, timeout: 15 * time.Second}
}

// WithLock runs fn while holding the advisory lock for key.
func (l *AdvisoryLocker) WithLock(ctx context.Context, key string, fn func(context.Context) error) error {
	deadline := time.Now().Add(l.timeout)
	backoff := 5 * time.Millisecond
	for {
		tx, err := l.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("advisory lock begin: %w", err)
		}
		var acquired bool
		if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))`, key).Scan(&acquired); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("advisory lock acquire: %w", err)
		}
		if acquired {
			fnErr := fn(ctx)
			// Nothing is written through tx; ending it only releases the lock.
			_ = tx.Rollback()
			return fnErr
		}
		_ = tx.Rollback()

		if time.Now().After(deadline) {
			return ErrLockTimeout
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 100*time.Millisecond {
			backoff *= 2
		}
	}
}
