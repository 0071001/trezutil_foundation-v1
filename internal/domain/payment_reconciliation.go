package domain

import "context"

/*
 * Reconciliation reads the payment records and says where they disagree with each other. It never moves money: the
 * only correction it offers adds accounting lines that a recorded capture or refund is missing, on a person's request,
 * and audits it. Everything else is listed for a person to check with the provider.
 */

// ReconciliationKind names one inconsistency.
type ReconciliationKind string

const (
	ReconCaptureWithoutLedger ReconciliationKind = "capture_without_ledger"  // captured, accounting lines missing
	ReconRefundWithoutLedger  ReconciliationKind = "refund_without_ledger"   // succeeded refund, accounting lines missing
	ReconDuplicatePosting     ReconciliationKind = "duplicate_posting"       // the same line posted more than once
	ReconPostingAmount        ReconciliationKind = "posting_amount_mismatch" // a line differs from its payment or refund
	ReconPostingWithoutMoney  ReconciliationKind = "posting_without_capture" // capture lines on a payment never captured
	ReconRefundedTotal        ReconciliationKind = "refunded_total_mismatch" // refunded amount differs from succeeded refunds
	ReconPendingRefund        ReconciliationKind = "refund_pending"          // refund without a final answer
	ReconStuckPayment         ReconciliationKind = "payment_stuck"           // payment without a final answer
	ReconOrderNotPaid         ReconciliationKind = "captured_order_unpaid"   // money taken, order still awaiting payment
	ReconWebhookUnprocessed   ReconciliationKind = "webhook_unprocessed"     // provider event failed, abandoned or kept for review
)

// ReconciliationFinding is one inconsistency on one record.
type ReconciliationFinding struct {
	Kind      ReconciliationKind `json:"kind"`
	TenantID  string             `json:"organization_id"`
	PaymentID string             `json:"payment_id,omitempty"`
	RefundID  string             `json:"refund_id,omitempty"`
	Detail    string             `json:"detail"`
}

// ReconciliationSource lists the inconsistencies. Records younger than settledBefore are still in motion and are not
// reported as stuck.
type ReconciliationSource interface {
	Findings(ctx context.Context, settledBefore int64) ([]ReconciliationFinding, error)
}
