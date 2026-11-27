package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// settleDelay: a record younger than this is still in motion.
const settleDelay = time.Hour

// refreshWindow: payments without a final answer are read back from the provider while they are recent enough.
const refreshWindow = 7 * 24 * time.Hour

// Reconciler compares the payment records, and offers the two corrections that move no money: reading a provider's
// current status, and adding the accounting lines a recorded capture or refund is missing.
type Reconciler struct {
	Source     domain.ReconciliationSource
	Payments   domain.PaymentRepository
	Refunds    domain.RefundRepository
	Accounting domain.PaymentAccounting
	Locker     Locker
	Audit      domain.AuditStore
	Logger     zerolog.Logger
}

// Report lists the inconsistencies.
func (r *Reconciler) Report(ctx context.Context) ([]domain.ReconciliationFinding, error) {
	return r.Source.Findings(ctx, time.Now().Add(-settleDelay).Unix())
}

// LedgerRepair is what a ledger repair did, or would do, for one record.
type LedgerRepair struct {
	Finding domain.ReconciliationFinding `json:"finding"`
	Posted  int                          `json:"posted"`
	Error   string                       `json:"error,omitempty"`
}

// RepairLedger adds the missing lines of captures and succeeded refunds. Nothing is changed or removed, and no
// provider is called. With apply false it only lists what it would repair. Every addition is audited under actor.
func (r *Reconciler) RepairLedger(ctx context.Context, actor string, apply bool) ([]LedgerRepair, error) {
	if actor == "" {
		return nil, errors.New("a ledger repair needs the name of the person asking for it")
	}
	findings, err := r.Report(ctx)
	if err != nil {
		return nil, err
	}
	out := []LedgerRepair{}
	for _, f := range findings {
		if f.Kind != domain.ReconCaptureWithoutLedger && f.Kind != domain.ReconRefundWithoutLedger {
			continue
		}
		repair := LedgerRepair{Finding: f}
		if apply {
			if repair.Posted, err = r.repair(ctx, actor, f); err != nil {
				repair.Error = err.Error()
			}
		}
		out = append(out, repair)
	}
	return out, nil
}

func (r *Reconciler) repair(ctx context.Context, actor string, f domain.ReconciliationFinding) (posted int, err error) {
	work := func(ctx context.Context) error {
		// Reload under the lock: only a record that still says the money moved gets lines.
		var entries []*domain.LedgerEntry
		target, id := "payment", f.PaymentID
		now := unixNow()
		if f.Kind == domain.ReconRefundWithoutLedger {
			target, id = "refund", f.RefundID
			refund, err := r.Refunds.FindByID(ctx, f.TenantID, f.RefundID)
			if err != nil {
				return err
			}
			if refund.Status != domain.RefundStatusSucceeded {
				return fmt.Errorf("refund is %s", refund.Status)
			}
			if entries, err = domain.RefundEntries(refund, now); err != nil {
				return err
			}
		} else {
			p, err := r.Payments.FindByID(ctx, f.TenantID, f.PaymentID)
			if err != nil {
				return err
			}
			switch p.Status {
			case domain.PaymentStatusCaptured, domain.PaymentStatusPartiallyRefunded, domain.PaymentStatusRefunded:
			default:
				return fmt.Errorf("payment is %s", p.Status)
			}
			if entries, err = domain.CaptureEntries(p, now); err != nil {
				return err
			}
		}
		if posted, err = r.Accounting.EnsureEntries(ctx, entries); err != nil {
			return err
		}
		if posted == 0 {
			return nil
		}
		return r.Audit.Append(ctx, domain.AuditEntry{ActorID: actor, Action: "payment.ledger.repaired", TargetType: target, TargetID: id,
			OccurredAt: now, Metadata: map[string]interface{}{"organization_id": f.TenantID, "lines_added": posted, "found": f.Detail}})
	}
	if r.Locker == nil {
		err = work(ctx)
	} else {
		err = r.Locker.WithLock(ctx, "payment:"+f.PaymentID, work)
	}
	return posted, err
}

// StatusReader reads a payment's current state from its provider and records it (PaymentService.GetPaymentStatus).
type StatusReader interface {
	GetPaymentStatus(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (*domain.Payment, error)
}

// Pass runs one reconciliation: recent payments without a final answer are read back from their provider (nothing is
// asked to move), then what still disagrees is logged for a person.
func (r *Reconciler) Pass(ctx context.Context, provider StatusReader) {
	findings, err := r.Report(ctx)
	if err != nil {
		r.Logger.Error().Err(err).Msg("reconciliation failed")
		return
	}
	if provider != nil {
		oldest := time.Now().Add(-refreshWindow).Unix()
		read := 0
		for _, f := range findings {
			if f.Kind != domain.ReconStuckPayment && f.Kind != domain.ReconOrderNotPaid {
				continue
			}
			p, err := r.Payments.FindByID(ctx, f.TenantID, f.PaymentID)
			if err != nil || p.CreatedAt < oldest || p.ProviderPaymentID == "" {
				continue
			}
			if _, err := provider.GetPaymentStatus(ctx, f.TenantID, "system:reconciliation", p.ID, ""); err != nil {
				r.Logger.Warn().Err(err).Str("payment", p.ID).Msg("provider status not read")
			}
			read++
		}
		if read > 0 {
			if findings, err = r.Report(ctx); err != nil {
				r.Logger.Error().Err(err).Msg("reconciliation failed")
				return
			}
		}
	}
	if len(findings) == 0 {
		return
	}
	counts := map[string]int{}
	for _, f := range findings {
		counts[string(f.Kind)]++
	}
	r.Logger.Warn().Interface("findings", counts).Msg("payment records need attention: run trezutil-admin reconcile")
}

// Run repeats Pass until the context ends.
func (r *Reconciler) Run(ctx context.Context, provider StatusReader, every time.Duration) {
	if every <= 0 {
		return
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		pass, cancel := context.WithTimeout(ctx, every)
		r.Pass(pass, provider)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
