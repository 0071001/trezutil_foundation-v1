package adminmanage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type grant struct {
	account, key string
	source       domain.CreditGrantSource
	amount       int64
}

type mem struct {
	plans    map[string]*Plan
	inUse    map[string]bool
	subs     map[string]*Subscription
	grants   map[string]grant
	policies map[string]*Policy
	cats     map[string]*Category
	catUse   map[string][2]int
	admins   int
	seq      int
}

func newMem() *mem {
	return &mem{plans: map[string]*Plan{}, inUse: map[string]bool{}, subs: map[string]*Subscription{}, grants: map[string]grant{},
		policies: map[string]*Policy{}, cats: map[string]*Category{}, catUse: map[string][2]int{}, admins: 2}
}
func (m *mem) id(p string) string { m.seq++; return fmt.Sprintf("%s%d", p, m.seq) }

func (m *mem) PlatformOrganization(context.Context) (string, error) { return "trezutil", nil }
func (m *mem) CreatePlan(_ context.Context, _ string, p *Plan) error {
	for _, o := range m.plans {
		if o.Slug == p.Slug {
			return ErrConflict
		}
	}
	p.ID = m.id("plan")
	c := *p
	m.plans[p.ID] = &c
	return nil
}
func (m *mem) UpdatePlan(_ context.Context, p *Plan) error { c := *p; m.plans[p.ID] = &c; return nil }
func (m *mem) FindPlan(_ context.Context, id string) (*Plan, error) {
	if p, ok := m.plans[id]; ok {
		c := *p
		return &c, nil
	}
	return nil, ErrNotFound
}
func (m *mem) PlanInUse(_ context.Context, id string) (bool, error) { return m.inUse[id], nil }
func (m *mem) DeletePlan(_ context.Context, id string) error  { delete(m.plans, id); return nil }
func (m *mem) FindSubscription(_ context.Context, id string) (*Subscription, error) {
	if s, ok := m.subs[id]; ok {
		c := *s
		return &c, nil
	}
	return nil, nil
}
func (m *mem) FindSubscriptionByUser(_ context.Context, _, user string) (*Subscription, error) {
	for _, s := range m.subs {
		if s.UserID == user {
			c := *s
			return &c, nil
		}
	}
	return nil, nil
}
func (m *mem) CreateSubscription(_ context.Context, _ string, s *Subscription) error {
	s.ID = m.id("sub")
	c := *s
	m.subs[s.ID] = &c
	return nil
}
func (m *mem) UpdateSubscription(_ context.Context, s *Subscription) error { c := *s; m.subs[s.ID] = &c; return nil }
func (m *mem) ActiveSubscriptions(context.Context, string) ([]Subscription, error) {
	out := []Subscription{}
	for _, s := range m.subs {
		if s.Status == "active" {
			out = append(out, *s)
		}
	}
	return out, nil
}
func (m *mem) UserCreditAccount(_ context.Context, user, _, period string) (string, string, error) {
	return "acct-" + user + "-" + period, "space-" + user, nil
}
func (m *mem) Grant(_ context.Context, account, key string, source domain.CreditGrantSource, amount int64, _, _, _ string) (bool, error) {
	if _, dup := m.grants[key]; dup {
		return false, nil
	}
	m.grants[key] = grant{account, key, source, amount}
	return true, nil
}
func (m *mem) FindPolicy(_ context.Context, id string) (*Policy, error) {
	if p, ok := m.policies[id]; ok {
		c := *p
		return &c, nil
	}
	return nil, nil
}
func (m *mem) ActivePolicyFor(_ context.Context, action string) (*Policy, error) {
	for _, p := range m.policies {
		if p.ActionType == action && p.Active {
			return p, nil
		}
	}
	return nil, nil
}
func (m *mem) CreatePolicy(_ context.Context, p *Policy) error {
	p.ID = m.id("pol")
	c := *p
	m.policies[p.ID] = &c
	return nil
}
func (m *mem) UpdatePolicy(_ context.Context, p *Policy) error { c := *p; m.policies[p.ID] = &c; return nil }
func (m *mem) DeletePolicy(_ context.Context, id string) error  { delete(m.policies, id); return nil }
func (m *mem) FindCategory(_ context.Context, id string) (*Category, error) {
	if c, ok := m.cats[id]; ok {
		return c, nil
	}
	return nil, ErrNotFound
}
func (m *mem) CategoryUsage(_ context.Context, id string) (int, int, error) { u := m.catUse[id]; return u[0], u[1], nil }
func (m *mem) UpdateCategory(_ context.Context, c *Category) error         { m.cats[c.ID] = c; return nil }
func (m *mem) DeleteCategory(_ context.Context, id string) error            { delete(m.cats, id); return nil }
func (m *mem) CountRole(context.Context, domain.Role) (int, error)          { return m.admins, nil }
func (m *mem) ListPolicies(context.Context) ([]Policy, error) { return []Policy{}, nil }
func (m *mem) ListSubscriptions(context.Context, string) ([]SubscriptionRow, error) {
	return []SubscriptionRow{}, nil
}

type people map[string][]domain.Role

func (p people) FindByID(_ context.Context, id string) (*domain.User, error) {
	roles, ok := p[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return &domain.User{ID: id, Roles: roles}, nil
}
func (p people) AssignRole(_ context.Context, id string, r domain.Role) error {
	p[id] = append(p[id], r)
	return nil
}
func (p people) RemoveRole(_ context.Context, id string, r domain.Role) error {
	out := []domain.Role{}
	for _, x := range p[id] {
		if x != r {
			out = append(out, x)
		}
	}
	p[id] = out
	return nil
}

type auditSpy struct{ actions []string }

func (a *auditSpy) Record(_ context.Context, e domain.AuditEntry) error { a.actions = append(a.actions, e.Action); return nil }

func setup() (*Service, *mem, people, *auditSpy) {
	m := newMem()
	ppl := people{"admin": {domain.RoleAdmin}, "admin2": {domain.RoleAdmin}, "op": {domain.RoleOperator}, "buyer": {domain.RoleCustomer}}
	spy := &auditSpy{}
	s := NewService(m, ppl, spy)
	s.now = func() time.Time { return time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC) }
	return s, m, ppl, spy
}

var pro = Plan{Name: "Pro", Slug: "pro", PriceMinor: 1000, Currency: "usd", IncludedCredits: 200}

func TestOnlyAnAdministratorChangesAnything(t *testing.T) {
	s, _, _, _ := setup()
	ctx := context.Background()
	for _, who := range []string{"op", "buyer", "nobody"} {
		if _, err := s.CreatePlan(ctx, who, pro); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s created a plan: %v", who, err)
		}
		if err := s.GrantCredits(ctx, who, "buyer", 10, "gesture", ""); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s granted credits: %v", who, err)
		}
		if err := s.GrantRole(ctx, who, "buyer", domain.RoleAdmin); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s granted a role: %v", who, err)
		}
	}
	if _, err := s.Subscriptions(ctx, "op"); err != nil {
		t.Errorf("an operator reads the subscriptions: %v", err)
	}
}

func TestPlansAreValidatedAndAUsedPlanIsArchivedNotDeleted(t *testing.T) {
	s, m, _, spy := setup()
	ctx := context.Background()
	for name, bad := range map[string]Plan{
		"name":     {Name: "x", Slug: "x"},
		"slug":     {Name: "Pro", Slug: "Pro Plan"},
		"currency": {Name: "Pro", Slug: "pro", Currency: "EUR"},
		"credits":  {Name: "Pro", Slug: "pro", IncludedCredits: -1},
	} {
		var inv *InvalidError
		if _, err := s.CreatePlan(ctx, "admin", bad); !errors.As(err, &inv) {
			t.Errorf("%s: accepted %+v", name, bad)
		}
	}
	p, err := s.CreatePlan(ctx, "admin", pro)
	if err != nil || p.Currency != "USD" || p.Status != "active" || p.BillingCycle != "monthly" {
		t.Fatalf("plan %+v %v", p, err)
	}
	if _, err := s.CreatePlan(ctx, "admin", pro); !errors.Is(err, ErrConflict) {
		t.Errorf("two plans with one slug: %v", err)
	}
	m.inUse[p.ID] = true
	archived, err := s.DeletePlan(ctx, "admin", p.ID)
	if err != nil || !archived || m.plans[p.ID] == nil || m.plans[p.ID].Status != "archived" {
		t.Errorf("a used plan must be archived, kept: %v %v %+v", archived, err, m.plans[p.ID])
	}
	free, _ := s.CreatePlan(ctx, "admin", Plan{Name: "Essai", Slug: "essai"})
	if archived, err := s.DeletePlan(ctx, "admin", free.ID); err != nil || archived || m.plans[free.ID] != nil {
		t.Errorf("an unused plan is deleted: %v %v", archived, err)
	}
	if len(spy.actions) == 0 || spy.actions[len(spy.actions)-1] != "plan.deleted" {
		t.Errorf("audit %v", spy.actions)
	}
}

func TestASubscriptionGrantsItsCreditsOncePerMonthIntoTheChargedAccount(t *testing.T) {
	s, m, _, _ := setup()
	ctx := context.Background()
	p, _ := s.CreatePlan(ctx, "admin", pro)
	sub, err := s.Subscribe(ctx, "admin", "buyer", p.ID)
	if err != nil || sub.Status != "active" || sub.PeriodID != "2026-11" {
		t.Fatalf("subscribe %+v %v", sub, err)
	}
	if len(m.grants) != 1 {
		t.Fatalf("grants %v", m.grants)
	}
	for _, g := range m.grants {
		if g.account != "acct-buyer-2026-11" || g.amount != 200 || g.source != domain.CreditGrantSourceAllocation {
			t.Errorf("grant %+v", g)
		}
	}
	// The monthly job, run again and again, never grants twice in a month.
	for i := 0; i < 3; i++ {
		if n, err := s.RenewAll(ctx); err != nil || n != 1 {
			t.Fatalf("renew %d %v", n, err)
		}
	}
	if len(m.grants) != 1 {
		t.Errorf("the same month was granted twice: %v", m.grants)
	}
	// Next month: granted again.
	s.now = func() time.Time { return time.Date(2026, 12, 1, 1, 0, 0, 0, time.UTC) }
	if _, err := s.RenewAll(ctx); err != nil || len(m.grants) != 2 {
		t.Errorf("december not granted: %v %v", err, m.grants)
	}
	// Cancelled: no more grants.
	if _, err := s.Cancel(ctx, "admin", sub.ID); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC) }
	if n, _ := s.RenewAll(ctx); n != 0 || len(m.grants) != 2 {
		t.Errorf("a cancelled subscription was renewed: %d %v", n, m.grants)
	}
}

func TestMovingToAnotherPlanKeepsOneSubscriptionAndRefusesInactivePlans(t *testing.T) {
	s, m, _, _ := setup()
	ctx := context.Background()
	a, _ := s.CreatePlan(ctx, "admin", pro)
	b, _ := s.CreatePlan(ctx, "admin", Plan{Name: "Business", Slug: "business", IncludedCredits: 1000})
	off, _ := s.CreatePlan(ctx, "admin", Plan{Name: "Ancien", Slug: "ancien", Status: "inactive"})
	first, _ := s.Subscribe(ctx, "admin", "buyer", a.ID)
	second, err := s.Subscribe(ctx, "admin", "buyer", b.ID)
	if err != nil || second.ID != first.ID || len(m.subs) != 1 || m.subs[first.ID].PlanID != b.ID {
		t.Errorf("change of plan: %+v %v %d", second, err, len(m.subs))
	}
	var inv *InvalidError
	if _, err := s.Subscribe(ctx, "admin", "buyer", off.ID); !errors.As(err, &inv) {
		t.Errorf("subscribed to an inactive plan: %v", err)
	}
	if _, err := s.Subscribe(ctx, "admin", "ghost", a.ID); !errors.As(err, &inv) {
		t.Errorf("subscribed an unknown account: %v", err)
	}
}

func TestManualCreditsNeedAReasonAndAreGrantedOncePerRequest(t *testing.T) {
	s, m, _, spy := setup()
	ctx := context.Background()
	var inv *InvalidError
	if err := s.GrantCredits(ctx, "admin", "buyer", 50, "", "k1"); !errors.As(err, &inv) {
		t.Error("granted without a reason")
	}
	if err := s.GrantCredits(ctx, "admin", "buyer", 0, "geste", "k1"); !errors.As(err, &inv) {
		t.Error("granted zero")
	}
	for i := 0; i < 2; i++ {
		if err := s.GrantCredits(ctx, "admin", "buyer", 50, "Geste commercial", "k1"); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.grants) != 1 || m.grants["manual:k1"].source != domain.CreditGrantSourceManual {
		t.Errorf("grants %v", m.grants)
	}
	if spy.actions[len(spy.actions)-1] != "credits.granted" {
		t.Errorf("audit %v", spy.actions)
	}
}

func TestOneActiveRulePerActionAndOnlyKnownActions(t *testing.T) {
	s, _, _, _ := setup()
	ctx := context.Background()
	var inv *InvalidError
	if _, err := s.CreatePolicy(ctx, "admin", Policy{ActionType: "DELETE_EVERYTHING", Cost: 1, Active: true}); !errors.As(err, &inv) {
		t.Error("priced an unknown action")
	}
	p, err := s.CreatePolicy(ctx, "admin", Policy{ActionType: "search_listings", Cost: 2, Active: true})
	if err != nil || p.ActionType != "SEARCH_LISTINGS" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := s.CreatePolicy(ctx, "admin", Policy{ActionType: "SEARCH_LISTINGS", Cost: 3, Active: true}); !errors.Is(err, ErrConflict) {
		t.Errorf("two active rules for one action: %v", err)
	}
	off, err := s.CreatePolicy(ctx, "admin", Policy{ActionType: "SEARCH_LISTINGS", Cost: 3})
	if err != nil {
		t.Fatal(err)
	}
	off.Active = true
	if _, err := s.UpdatePolicy(ctx, "admin", *off); !errors.Is(err, ErrConflict) {
		t.Errorf("switched on a second rule: %v", err)
	}
	p.Cost = 5
	if _, err := s.UpdatePolicy(ctx, "admin", *p); err != nil {
		t.Errorf("updating the active rule itself: %v", err)
	}
	if err := s.DeletePolicy(ctx, "admin", p.ID); err != nil {
		t.Error(err)
	}
}

func TestCategoriesAreDeletedOnlyWhenEmpty(t *testing.T) {
	s, m, _, _ := setup()
	ctx := context.Background()
	m.cats["c1"] = &Category{ID: "c1", Name: "Ciment", Slug: "ciment"}
	m.catUse["c1"] = [2]int{3, 0}
	if err := s.DeleteCategory(ctx, "admin", "c1"); !errors.Is(err, ErrInUse) {
		t.Errorf("deleted a category with offers: %v", err)
	}
	self := "c1"
	if _, err := s.UpdateCategory(ctx, "admin", Category{ID: "c1", Name: "Ciment", Slug: "ciment", ParentID: &self}); err == nil {
		t.Error("a category became its own parent")
	}
	m.catUse["c1"] = [2]int{0, 0}
	if err := s.DeleteCategory(ctx, "admin", "c1"); err != nil || m.cats["c1"] != nil {
		t.Errorf("empty category not deleted: %v", err)
	}
}

func TestStaffRolesKeepAnAdministrator(t *testing.T) {
	s, m, ppl, _ := setup()
	ctx := context.Background()
	if err := s.GrantRole(ctx, "admin", "buyer", domain.RoleOperator); err != nil || !(&domain.User{Roles: ppl["buyer"]}).HasRole(domain.RoleOperator) {
		t.Errorf("grant: %v %v", err, ppl["buyer"])
	}
	var inv *InvalidError
	if err := s.GrantRole(ctx, "admin", "buyer", domain.RoleSeller); !errors.As(err, &inv) {
		t.Error("granted a non-staff role from the Control Center")
	}
	if err := s.RevokeRole(ctx, "admin", "admin", domain.RoleAdmin); !errors.Is(err, ErrSelfDemote) {
		t.Errorf("an admin removed their own admin role: %v", err)
	}
	m.admins = 1
	if err := s.RevokeRole(ctx, "admin", "admin2", domain.RoleAdmin); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("the last admin was removed: %v", err)
	}
	m.admins = 2
	if err := s.RevokeRole(ctx, "admin", "admin2", domain.RoleAdmin); err != nil {
		t.Error(err)
	}
}
