package payment

import (
	"context"
	"errors"
	"sync"

	"github.com/trezutil/trezutil/internal/domain"
)

// mockAccounting mirrors the PostgreSQL accounting store: all of a commit happens, or none of it. failNext makes the
// next commits fail, as a database error would.
type mockAccounting struct {
	mu       sync.Mutex
	payments *mockPaymentRepo
	refunds  *mockRefundRepo
	ledger   *mockLedgerRepo
	failNext int
}

var errDatabase = errors.New("database unavailable")

func newMockAccounting(p *mockPaymentRepo, r *mockRefundRepo, l *mockLedgerRepo) *mockAccounting {
	return &mockAccounting{payments: p, refunds: r, ledger: l}
}

func (m *mockAccounting) failing() bool {
	if m.failNext > 0 {
		m.failNext--
		return true
	}
	return false
}

func (m *mockAccounting) CommitCapture(ctx context.Context, p *domain.Payment, entries []*domain.LedgerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing() {
		return errDatabase
	}
	if err := m.payments.Update(ctx, p); err != nil {
		return err
	}
	m.insert(entries)
	return nil
}

func (m *mockAccounting) CommitRefund(ctx context.Context, r *domain.Refund, p *domain.Payment, entries []*domain.LedgerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing() {
		return errDatabase
	}
	if stored, err := m.payments.FindByID(ctx, p.TenantID, p.ID); err == nil && stored.Version != p.Version-1 {
		return domain.ErrOptimisticLock
	}
	if err := m.refunds.Update(ctx, r); err != nil {
		return err
	}
	if err := m.payments.Update(ctx, p); err != nil {
		return err
	}
	m.insert(entries)
	return nil
}

func (m *mockAccounting) EnsureEntries(ctx context.Context, entries []*domain.LedgerEntry) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing() {
		return 0, errDatabase
	}
	return m.insert(entries), nil
}

func (m *mockAccounting) insert(entries []*domain.LedgerEntry) int {
	m.ledger.mu.Lock()
	defer m.ledger.mu.Unlock()
	same := func(a, b *string) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
	posted := 0
next:
	for _, e := range entries {
		for _, x := range m.ledger.entries {
			if x.TenantID == e.TenantID && x.Account == e.Account && x.EntryType == e.EntryType && same(x.PaymentID, e.PaymentID) && same(x.RefundID, e.RefundID) {
				continue next
			}
		}
		cp := *e
		m.ledger.entries = append(m.ledger.entries, &cp)
		posted++
	}
	return posted
}
