package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

// ActionJournal is the durable idempotency store of commerce actions (table commerce_action_journal, migration
// 000039): a claimed or completed action stays known across restarts and between instances.
type ActionJournal struct{ db *sql.DB }

// NewActionJournal builds the journal.
func NewActionJournal(db *sql.DB) *ActionJournal { return &ActionJournal{db: db} }

var _ commerce.IdempotencyStore = (*ActionJournal)(nil)

// hashed keeps the primary key short whatever the caller's key is.
func hashed(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Check returns the result of a completed action. A read failure reads as "not found": Claim then fails and nothing
// runs.
func (j *ActionJournal) Check(ctx context.Context, key string) (domain.ActionResultStatus, map[string]interface{}, bool) {
	var raw []byte
	err := j.db.QueryRowContext(ctx, `SELECT result FROM commerce_action_journal WHERE action_key = $1 AND status = 'completed'`,
		hashed(key)).Scan(&raw)
	if err != nil {
		return "", nil, false
	}
	var result domain.CommerceActionResult
	if json.Unmarshal(raw, &result) != nil {
		return "", nil, false
	}
	return result.Status, result.Data, true
}

// Claim takes the key for execution.
func (j *ActionJournal) Claim(ctx context.Context, key, fingerprint string) (commerce.Claim, error) {
	id := hashed(key)
	timeout := commerce.ClaimTimeout.Seconds()
	for attempt := 0; attempt < 3; attempt++ {
		res, err := j.db.ExecContext(ctx, `INSERT INTO commerce_action_journal (action_key, fingerprint, status)
			VALUES ($1, $2, 'in_progress') ON CONFLICT (action_key) DO NOTHING`, id, fingerprint)
		if err != nil {
			return commerce.Claim{}, fmt.Errorf("claim action: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return commerce.Claim{State: commerce.ClaimAcquired}, nil
		}
		var print, status string
		var raw []byte
		var abandoned bool
		err = j.db.QueryRowContext(ctx, `SELECT fingerprint, status, result, claimed_at < NOW() - make_interval(secs => $2)
			FROM commerce_action_journal WHERE action_key = $1`, id, timeout).Scan(&print, &status, &raw, &abandoned)
		if errors.Is(err, sql.ErrNoRows) {
			continue // released between the two statements
		}
		if err != nil {
			return commerce.Claim{}, fmt.Errorf("read action: %w", err)
		}
		switch {
		case print != "" && print != fingerprint:
			return commerce.Claim{State: commerce.ClaimConflict}, nil
		case status == "completed":
			var result domain.CommerceActionResult
			if err := json.Unmarshal(raw, &result); err != nil {
				return commerce.Claim{}, fmt.Errorf("read action result: %w", err)
			}
			return commerce.Claim{State: commerce.ClaimCompleted, Result: &result}, nil
		case !abandoned:
			return commerce.Claim{State: commerce.ClaimInProgress}, nil
		}
		res, err = j.db.ExecContext(ctx, `UPDATE commerce_action_journal SET claimed_at = NOW()
			WHERE action_key = $1 AND status = 'in_progress' AND claimed_at < NOW() - make_interval(secs => $2)`, id, timeout)
		if err != nil {
			return commerce.Claim{}, fmt.Errorf("reclaim action: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return commerce.Claim{State: commerce.ClaimAcquired}, nil
		}
	}
	return commerce.Claim{State: commerce.ClaimInProgress}, nil
}

// Release gives up a claim that did not complete.
func (j *ActionJournal) Release(ctx context.Context, key string) error {
	_, err := j.db.ExecContext(ctx, `DELETE FROM commerce_action_journal WHERE action_key = $1 AND status = 'in_progress'`, hashed(key))
	if err != nil {
		return fmt.Errorf("release action: %w", err)
	}
	return nil
}

// Store completes the key with its result.
func (j *ActionJournal) Store(ctx context.Context, key string, result domain.CommerceActionResult) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode action result: %w", err)
	}
	_, err = j.db.ExecContext(ctx, `INSERT INTO commerce_action_journal (action_key, fingerprint, status, result, completed_at)
		VALUES ($1, '', 'completed', $2::jsonb, NOW())
		ON CONFLICT (action_key) DO UPDATE SET status = 'completed', result = EXCLUDED.result, completed_at = NOW()`,
		hashed(key), string(raw))
	if err != nil {
		return fmt.Errorf("complete action: %w", err)
	}
	return nil
}

// Cleanup removes completed entries older than the given duration.
func (j *ActionJournal) Cleanup(ctx context.Context, olderThan time.Duration) int {
	res, err := j.db.ExecContext(ctx, `DELETE FROM commerce_action_journal
		WHERE status = 'completed' AND completed_at < NOW() - make_interval(secs => $1)`, olderThan.Seconds())
	if err != nil {
		return 0
	}
	n, _ := res.RowsAffected()
	return int(n)
}
