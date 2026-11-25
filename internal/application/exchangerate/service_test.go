package exchangerate

import (
	"context"
	"errors"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type memStore struct{ rates []domain.ExchangeRate }

func (m *memStore) Latest(context.Context) (*domain.ExchangeRate, error) {
	if len(m.rates) == 0 {
		return nil, domain.ErrNoExchangeRate
	}
	r := m.rates[len(m.rates)-1]
	return &r, nil
}
func (m *memStore) Add(_ context.Context, r *domain.ExchangeRate) error {
	r.ID = "r" + string(rune('0'+len(m.rates)))
	m.rates = append(m.rates, *r)
	return nil
}
func (m *memStore) History(context.Context, int) ([]domain.ExchangeRate, error) { return m.rates, nil }

type users map[string][]domain.Role

func (u users) FindByID(_ context.Context, id string) (*domain.User, error) {
	roles, ok := u[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return &domain.User{ID: id, Roles: roles}, nil
}

type auditSpy struct{ entries []domain.AuditEntry }

func (a *auditSpy) Record(_ context.Context, e domain.AuditEntry) error { a.entries = append(a.entries, e); return nil }

func TestAStoredRateWithAnExtraZeroIsNotUsed(t *testing.T) {
	svc := NewService(&memStore{rates: []domain.ExchangeRate{{Rate: 22500}}}, users{}, nil)
	if _, err := svc.Current(context.Background()); !errors.Is(err, domain.ErrNoExchangeRate) {
		t.Fatalf("an implausible rate must read as none, got %v", err)
	}
}

func TestOnlyAnAdministratorSetsTheRateAndEachChangeIsAudited(t *testing.T) {
	store, spy := &memStore{}, &auditSpy{}
	svc := NewService(store, users{"admin": {domain.RoleAdmin}, "op": {domain.RoleOperator}, "buyer": {domain.RoleCustomer}}, spy)
	ctx := context.Background()

	if _, err := svc.Current(ctx); !errors.Is(err, domain.ErrNoExchangeRate) {
		t.Fatalf("no rate entered yet must read as none, got %v", err)
	}
	for _, who := range []string{"op", "buyer", "stranger"} {
		if _, err := svc.Set(ctx, who, 2850, ""); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s set the rate: %v", who, err)
		}
	}
	for _, bad := range []float64{0, -1, 28, 285, 22500, 2850000} {
		if _, err := svc.Set(ctx, "admin", bad, ""); !errors.Is(err, ErrInvalidRate) {
			t.Errorf("rate %v accepted: %v", bad, err)
		}
	}
	if _, err := svc.Set(ctx, "admin", 2800, "BCC"); err != nil {
		t.Fatal(err)
	}
	r, err := svc.Set(ctx, "admin", 2850.456, " marché ")
	if err != nil {
		t.Fatal(err)
	}
	if r.Rate != 2850.46 || r.Base != "USD" || r.Quote != "CDF" || r.Note != "marché" || r.SetBy != "admin" {
		t.Errorf("stored %+v", r)
	}
	cur, _ := svc.Current(ctx)
	if cur.Rate != 2850.46 {
		t.Errorf("current rate %v", cur.Rate)
	}
	if len(spy.entries) != 2 || spy.entries[1].Action != "exchange_rate.set" || spy.entries[1].Metadata["previous_rate"] != 2800.0 {
		t.Errorf("audit %+v", spy.entries)
	}
	if _, err := svc.History(ctx, "buyer", 10); !errors.Is(err, ErrForbidden) {
		t.Error("a buyer read the history")
	}
	if h, err := svc.History(ctx, "op", 10); err != nil || len(h) != 2 {
		t.Errorf("an operator reads the history: %v %d", err, len(h))
	}
}
