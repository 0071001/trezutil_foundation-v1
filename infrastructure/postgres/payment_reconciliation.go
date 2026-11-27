package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// reconciliationLimit bounds each check, so a report stays readable and cheap on a damaged database.
const reconciliationLimit = 500

// PaymentReconciliation implements domain.ReconciliationSource. It only reads.
type PaymentReconciliation struct{ db *sql.DB }

// NewPaymentReconciliation builds the source.
func NewPaymentReconciliation(db *sql.DB) *PaymentReconciliation {
	return &PaymentReconciliation{db: db}
}

var _ domain.ReconciliationSource = (*PaymentReconciliation)(nil)

const capturedStatuses = `('captured', 'partially_refunded', 'refunded')`

// Each query returns the organization, the payment or an empty string, the refund or an empty string, and a detail.
var reconciliationChecks = []struct {
	kind  domain.ReconciliationKind
	aged  bool // takes the "settled before" bound
	query string
}{
	{domain.ReconCaptureWithoutLedger, false, `
		SELECT p.organization_id, p.id::text, '', p.status || ' ' || p.amount_minor || ' ' || p.currency
		FROM payments p
		WHERE p.status IN ` + capturedStatuses + ` AND (
		      NOT EXISTS (SELECT 1 FROM payment_ledger l WHERE l.payment_id = p.id AND l.refund_id IS NULL AND l.status = 'posted' AND l.account = 'cash' AND l.entry_type = 'DEBIT')
		   OR NOT EXISTS (SELECT 1 FROM payment_ledger l WHERE l.payment_id = p.id AND l.refund_id IS NULL AND l.status = 'posted' AND l.account = 'revenue' AND l.entry_type = 'CREDIT'))
		ORDER BY p.created_at`},
	{domain.ReconRefundWithoutLedger, false, `
		SELECT r.organization_id, r.payment_id::text, r.id::text, r.amount_minor || ' ' || r.currency
		FROM refunds r
		WHERE r.status = 'succeeded' AND (
		      NOT EXISTS (SELECT 1 FROM payment_ledger l WHERE l.refund_id = r.id AND l.status = 'posted' AND l.account = 'refund' AND l.entry_type = 'DEBIT')
		   OR NOT EXISTS (SELECT 1 FROM payment_ledger l WHERE l.refund_id = r.id AND l.status = 'posted' AND l.account = 'cash' AND l.entry_type = 'CREDIT'))
		ORDER BY r.created_at`},
	{domain.ReconDuplicatePosting, false, `
		SELECT organization_id, COALESCE(payment_id::text, ''), COALESCE(refund_id::text, ''),
		       account || ' ' || entry_type || ' posted ' || COUNT(*) || ' times: ' || string_agg(idempotency_key, ', ' ORDER BY idempotency_key)
		FROM payment_ledger WHERE status = 'posted'
		GROUP BY organization_id, payment_id, refund_id, account, entry_type
		HAVING COUNT(*) > 1`},
	{domain.ReconPostingAmount, false, `
		SELECT l.organization_id, COALESCE(p.id::text, r.payment_id::text, ''), COALESCE(r.id::text, ''),
		       l.account || ' ' || l.entry_type || ' ' || l.amount_minor || ' ' || l.currency || ', record ' ||
		       COALESCE(r.amount_minor, p.amount_minor) || ' ' || COALESCE(r.currency, p.currency)
		FROM payment_ledger l
		LEFT JOIN payments p ON p.id = l.payment_id AND l.refund_id IS NULL
		LEFT JOIN refunds r ON r.id = l.refund_id
		WHERE l.status = 'posted'
		  AND ((r.id IS NOT NULL AND (l.amount_minor <> r.amount_minor OR l.currency <> r.currency))
		    OR (r.id IS NULL AND p.id IS NOT NULL AND (l.amount_minor <> p.amount_minor OR l.currency <> p.currency)))`},
	{domain.ReconPostingWithoutMoney, false, `
		SELECT DISTINCT p.organization_id, p.id::text, '', 'payment is ' || p.status
		FROM payments p JOIN payment_ledger l ON l.payment_id = p.id AND l.refund_id IS NULL AND l.status = 'posted'
		WHERE p.status NOT IN ` + capturedStatuses},
	{domain.ReconRefundedTotal, false, `
		SELECT p.organization_id, p.id::text, '', 'payment says ' || p.refunded_amount_minor || ', succeeded refunds total ' || COALESCE(SUM(r.amount_minor), 0)
		FROM payments p LEFT JOIN refunds r ON r.payment_id = p.id AND r.status = 'succeeded'
		GROUP BY p.id
		HAVING p.refunded_amount_minor <> COALESCE(SUM(r.amount_minor), 0)`},
	{domain.ReconPendingRefund, true, `
		SELECT organization_id, payment_id::text, id::text,
		       amount_minor || ' ' || currency || ', provider reference ' || COALESCE(NULLIF(provider_refund_id, ''), 'none')
		FROM refunds WHERE status = 'pending' AND created_at < $1
		ORDER BY created_at`},
	{domain.ReconStuckPayment, true, `
		SELECT organization_id, id::text, '', status || ', provider reference ' || COALESCE(NULLIF(provider_payment_id, ''), 'none')
		FROM payments WHERE status IN ('created', 'pending', 'authorized') AND updated_at < $1
		ORDER BY created_at DESC`},
	{domain.ReconOrderNotPaid, true, `
		SELECT p.organization_id, p.id::text, '', 'order ' || o.id::text || ' is ' || o.status
		FROM payments p JOIN orders o ON o.id = p.order_id
		WHERE p.status = 'captured' AND o.status = 'pending_payment' AND p.updated_at < $1`},
	{domain.ReconWebhookUnprocessed, true, `
		SELECT organization_id, '', '', provider_code || ' ' || provider_msg_id || ' ' || status || ': ' || COALESCE(error_message, '')
		FROM payment_webhook_events
		WHERE status IN ('failed', 'review', 'pending', 'processing') AND created_at < $1
		ORDER BY created_at DESC`},
}

// Findings runs every check.
func (r *PaymentReconciliation) Findings(ctx context.Context, settledBefore int64) ([]domain.ReconciliationFinding, error) {
	out := []domain.ReconciliationFinding{}
	for _, check := range reconciliationChecks {
		var args []interface{}
		if check.aged {
			args = append(args, toTime(settledBefore))
		}
		rows, err := r.db.QueryContext(ctx, fmt.Sprintf("%s LIMIT %d", check.query, reconciliationLimit), args...)
		if err != nil {
			return nil, fmt.Errorf("reconciliation %s: %w", check.kind, err)
		}
		for rows.Next() {
			f := domain.ReconciliationFinding{Kind: check.kind}
			if err := rows.Scan(&f.TenantID, &f.PaymentID, &f.RefundID, &f.Detail); err != nil {
				rows.Close()
				return nil, fmt.Errorf("reconciliation %s: %w", check.kind, err)
			}
			out = append(out, f)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("reconciliation %s: %w", check.kind, err)
		}
	}
	return out, nil
}
