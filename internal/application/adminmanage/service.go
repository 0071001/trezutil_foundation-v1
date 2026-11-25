// Package adminmanage holds what the Trezutil team creates, changes and removes from the Control Center: the plans
// and the subscriptions to them, manual credit grants, the pricing rules of metered actions, the categories and the
// staff roles. Every change is made by an administrator (operators read), checked here, and written to the audit trail.
//
// Never deleted from here: orders, payments, bookings, the credit ledger and the audit trail (they are records; they
// are cancelled or refunded through their own flows). A plan that was ever used is archived, not deleted.
//
// Plans belong to the Trezutil platform organisation. A subscription is held by a person's account: each month it
// grants the plan's included credits into the credit account their metered actions are charged to (their personal
// space), idempotently, so a retry or a second run of the monthly job never grants twice.
package adminmanage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

var (
	ErrForbidden     = errors.New("only an administrator can make this change")
	ErrNotFound      = errors.New("not found")
	ErrInUse         = errors.New("still in use")
	ErrConflict      = errors.New("conflict")
	ErrLastAdmin     = errors.New("the platform must keep at least one administrator")
	ErrSelfDemote    = errors.New("an administrator cannot remove their own administrator role")
	ErrNoPlatformOrg = errors.New("the Trezutil platform organisation does not exist yet")
)

// InvalidError names the field that is wrong.
type InvalidError struct{ Field, Reason string }

func (e *InvalidError) Error() string { return e.Field + ": " + e.Reason }

func invalid(field, reason string) error { return &InvalidError{Field: field, Reason: reason} }

// Plan is what the Control Center edits.
type Plan struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	PriceMinor      int64  `json:"price_minor"`
	Currency        string `json:"currency"`
	BillingCycle    string `json:"billing_cycle"`
	IncludedCredits int64  `json:"included_credits"` // granted each month to a subscriber
	CreditPack      int64  `json:"credit_pack"`      // credits sold in one purchase (0: not a pack)
	Status          string `json:"status"`           // active | inactive | archived
}

// Subscription of a person's account to a plan.
type Subscription struct {
	ID       string `json:"id"`
	PlanID   string `json:"plan_id"`
	UserID   string `json:"user_id"`
	Status   string `json:"status"`
	PeriodID string `json:"current_period_id"`
}

// Policy is the price in credits of one metered action.
type Policy struct {
	ID         string `json:"id"`
	ActionType string `json:"action_type"`
	Channel    string `json:"channel"`
	ActorType  string `json:"actor_type"`
	Cost       int64  `json:"cost"`
	Active     bool   `json:"active"`
}

// Category as edited here.
type Category struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	ParentID *string `json:"parent_id"`
}

// Store is the persistence the service needs (infrastructure/postgres/admin_manage.go).
type Store interface {
	PlatformOrganization(ctx context.Context) (string, error)

	CreatePlan(ctx context.Context, tenantID string, p *Plan) error
	UpdatePlan(ctx context.Context, p *Plan) error
	FindPlan(ctx context.Context, id string) (*Plan, error)
	PlanInUse(ctx context.Context, id string) (bool, error)
	DeletePlan(ctx context.Context, id string) error

	FindSubscription(ctx context.Context, id string) (*Subscription, error)
	FindSubscriptionByUser(ctx context.Context, tenantID, userID string) (*Subscription, error)
	CreateSubscription(ctx context.Context, tenantID string, s *Subscription) error
	UpdateSubscription(ctx context.Context, s *Subscription) error
	ActiveSubscriptions(ctx context.Context, tenantID string) ([]Subscription, error)

	// UserCreditAccount returns the account a person's metered actions are charged to this period (their personal
	// space), creating it if needed, and that space's id.
	UserCreditAccount(ctx context.Context, userID, planID, periodID string) (accountID, tenantID string, err error)
	Grant(ctx context.Context, accountID, key string, source domain.CreditGrantSource, amount int64, tenantID, periodID, reason string) (bool, error)

	FindPolicy(ctx context.Context, id string) (*Policy, error)
	ActivePolicyFor(ctx context.Context, actionType string) (*Policy, error)
	CreatePolicy(ctx context.Context, p *Policy) error
	UpdatePolicy(ctx context.Context, p *Policy) error
	DeletePolicy(ctx context.Context, id string) error

	FindCategory(ctx context.Context, id string) (*Category, error)
	CategoryUsage(ctx context.Context, id string) (listings, children int, err error)
	UpdateCategory(ctx context.Context, c *Category) error
	DeleteCategory(ctx context.Context, id string) error

	CountRole(ctx context.Context, role domain.Role) (int, error)
	ListSubscriptions(ctx context.Context, tenantID string) ([]SubscriptionRow, error)
	ListPolicies(ctx context.Context) ([]Policy, error)
}

// SubscriptionRow is a subscription as the Control Center lists it.
type SubscriptionRow struct {
	ID        string    `json:"id"`
	PlanID    string    `json:"plan_id"`
	PlanName  string    `json:"plan_name"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	UserEmail string    `json:"user_email"`
	UserPhone string    `json:"user_phone"`
	Status    string    `json:"status"`
	PeriodID  string    `json:"current_period_id"`
	StartedAt time.Time `json:"started_at"`
}

// Users reads accounts and changes their roles.
type Users interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	AssignRole(ctx context.Context, userID string, role domain.Role) error
	RemoveRole(ctx context.Context, userID string, role domain.Role) error
}

// Service applies the rules.
type Service struct {
	store Store
	users Users
	audit domain.AuditLogger
	now   func() time.Time
}

// NewService builds the service; audit may be nil.
func NewService(store Store, users Users, audit domain.AuditLogger) *Service {
	return &Service{store: store, users: users, audit: audit, now: time.Now}
}

// BillableActions are the actions a pricing rule can apply to (the metered runtime actions and the AI's work).
var BillableActions = []string{
	"AI_INTERPRETATION", "AI_REPLY_WRITING",
	string(domain.ActionSearchListings), string(domain.ActionGetListing), string(domain.ActionCompareListings),
	string(domain.ActionCheckAvailability), string(domain.ActionCreateOrder), string(domain.ActionCreateReservation),
	string(domain.ActionCheckInventory),
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (s *Service) admin(ctx context.Context, actorID string) error {
	u, err := s.users.FindByID(ctx, actorID)
	if err != nil || u == nil || !u.HasRole(domain.RoleAdmin) {
		return ErrForbidden
	}
	return nil
}

func (s *Service) record(ctx context.Context, actor, action, targetType, targetID string, meta map[string]interface{}) {
	if s.audit != nil {
		_ = s.audit.Record(ctx, domain.AuditEntry{ActorID: actor, Action: action, TargetType: targetType, TargetID: targetID, Metadata: meta})
	}
}

func (s *Service) period() string { return s.now().UTC().Format("2006-01") }

// Subscriptions lists the platform's subscriptions (staff: admin or operator).
func (s *Service) Subscriptions(ctx context.Context, actorID string) ([]SubscriptionRow, error) {
	u, err := s.users.FindByID(ctx, actorID)
	if err != nil || u == nil || !(u.HasRole(domain.RoleAdmin) || u.HasRole(domain.RoleOperator)) {
		return nil, ErrForbidden
	}
	org, err := s.store.PlatformOrganization(ctx)
	if errors.Is(err, ErrNoPlatformOrg) {
		return []SubscriptionRow{}, nil
	}
	if err != nil {
		return nil, err
	}
	return s.store.ListSubscriptions(ctx, org)
}

// Policies lists the pricing rules with their ids (staff: admin or operator).
func (s *Service) Policies(ctx context.Context, actorID string) ([]Policy, error) {
	u, err := s.users.FindByID(ctx, actorID)
	if err != nil || u == nil || !(u.HasRole(domain.RoleAdmin) || u.HasRole(domain.RoleOperator)) {
		return nil, ErrForbidden
	}
	return s.store.ListPolicies(ctx)
}

// --- plans ------------------------------------------------------------------------------------------------------

func checkPlan(p *Plan) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Slug = strings.TrimSpace(strings.ToLower(p.Slug))
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	if p.Currency == "" {
		p.Currency = "USD"
	}
	if p.BillingCycle == "" {
		p.BillingCycle = string(domain.BillingCycleMonthly)
	}
	if p.Status == "" {
		p.Status = string(domain.PlanStatusActive)
	}
	switch {
	case len([]rune(p.Name)) < 2 || len([]rune(p.Name)) > 80:
		return invalid("name", "2 to 80 characters")
	case !slugRe.MatchString(p.Slug) || len(p.Slug) > 60:
		return invalid("slug", "lowercase letters, digits and hyphens")
	case p.PriceMinor < 0 || p.PriceMinor > 1_000_000_00:
		return invalid("price_minor", "out of range")
	case p.Currency != "USD" && p.Currency != "CDF":
		return invalid("currency", "USD or CDF")
	case p.BillingCycle != string(domain.BillingCycleMonthly) && p.BillingCycle != string(domain.BillingCycleYearly):
		return invalid("billing_cycle", "monthly or yearly")
	case p.IncludedCredits < 0 || p.IncludedCredits > 10_000_000:
		return invalid("included_credits", "out of range")
	case p.CreditPack < 0 || p.CreditPack > 10_000_000:
		return invalid("credit_pack", "out of range")
	case p.Status != string(domain.PlanStatusActive) && p.Status != string(domain.PlanStatusInactive) && p.Status != string(domain.PlanStatusArchived):
		return invalid("status", "active, inactive or archived")
	}
	return nil
}

// CreatePlan adds a plan to the Trezutil catalogue.
func (s *Service) CreatePlan(ctx context.Context, actorID string, p Plan) (*Plan, error) {
	if err := s.admin(ctx, actorID); err != nil {
		return nil, err
	}
	if err := checkPlan(&p); err != nil {
		return nil, err
	}
	org, err := s.store.PlatformOrganization(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreatePlan(ctx, org, &p); err != nil {
		return nil, err
	}
	s.record(ctx, actorID, "plan.created", "plan", p.ID, map[string]interface{}{"name": p.Name, "price_minor": p.PriceMinor, "currency": p.Currency, "included_credits": p.IncludedCredits})
	return &p, nil
}

// UpdatePlan changes a plan. Existing subscribers keep it; new monthly grants use the new included credits.
func (s *Service) UpdatePlan(ctx context.Context, actorID string, p Plan) (*Plan, error) {
	if err := s.admin(ctx, actorID); err != nil {
		return nil, err
	}
	old, err := s.store.FindPlan(ctx, p.ID)
	if err != nil || old == nil {
		return nil, ErrNotFound
	}
	if err := checkPlan(&p); err != nil {
		return nil, err
	}
	if err := s.store.UpdatePlan(ctx, &p); err != nil {
		return nil, err
	}
	s.record(ctx, actorID, "plan.updated", "plan", p.ID, map[string]interface{}{"before": old, "after": p})
	return &p, nil
}

// DeletePlan removes a plan nobody ever used; a plan with subscriptions or purchases is archived instead
// (archived = true in the answer), so its history stays readable.
func (s *Service) DeletePlan(ctx context.Context, actorID, id string) (archived bool, err error) {
	if err := s.admin(ctx, actorID); err != nil {
		return false, err
	}
	p, err := s.store.FindPlan(ctx, id)
	if err != nil || p == nil {
		return false, ErrNotFound
	}
	used, err := s.store.PlanInUse(ctx, id)
	if err != nil {
		return false, err
	}
	if used {
		p.Status = string(domain.PlanStatusArchived)
		if err := s.store.UpdatePlan(ctx, p); err != nil {
			return false, err
		}
		s.record(ctx, actorID, "plan.archived", "plan", id, map[string]interface{}{"name": p.Name, "reason": "in use"})
		return true, nil
	}
	if err := s.store.DeletePlan(ctx, id); err != nil {
		return false, err
	}
	s.record(ctx, actorID, "plan.deleted", "plan", id, map[string]interface{}{"name": p.Name})
	return false, nil
}

// --- subscriptions ----------------------------------------------------------------------------------------------

// Subscribe puts a person's account on a plan (or moves it to another plan) and grants this month's included
// credits at once. One subscription per account.
func (s *Service) Subscribe(ctx context.Context, actorID, userID, planID string) (*Subscription, error) {
	if err := s.admin(ctx, actorID); err != nil {
		return nil, err
	}
	if u, err := s.users.FindByID(ctx, userID); err != nil || u == nil {
		return nil, invalid("user_id", "unknown account")
	}
	plan, err := s.store.FindPlan(ctx, planID)
	if err != nil || plan == nil {
		return nil, invalid("plan_id", "unknown plan")
	}
	if plan.Status != string(domain.PlanStatusActive) {
		return nil, invalid("plan_id", "the plan is not active")
	}
	org, err := s.store.PlatformOrganization(ctx)
	if err != nil {
		return nil, err
	}
	period := s.period()
	sub, err := s.store.FindSubscriptionByUser(ctx, org, userID)
	if err != nil {
		return nil, err
	}
	action := "subscription.created"
	before := ""
	if sub == nil {
		sub = &Subscription{PlanID: planID, UserID: userID, Status: string(domain.SubscriptionStatusActive), PeriodID: period}
		if err := s.store.CreateSubscription(ctx, org, sub); err != nil {
			return nil, err
		}
	} else {
		before = sub.PlanID
		action = "subscription.changed"
		sub.PlanID, sub.Status, sub.PeriodID = planID, string(domain.SubscriptionStatusActive), period
		if err := s.store.UpdateSubscription(ctx, sub); err != nil {
			return nil, err
		}
	}
	s.record(ctx, actorID, action, "subscription", sub.ID, map[string]interface{}{"user_id": userID, "plan_id": planID, "previous_plan_id": before})
	if err := s.allocate(ctx, sub, plan, period); err != nil {
		return sub, err
	}
	return sub, nil
}

// Cancel ends a subscription: no further monthly grants; credits already granted stay.
func (s *Service) Cancel(ctx context.Context, actorID, subscriptionID string) (*Subscription, error) {
	if err := s.admin(ctx, actorID); err != nil {
		return nil, err
	}
	sub, err := s.store.FindSubscription(ctx, subscriptionID)
	if err != nil || sub == nil {
		return nil, ErrNotFound
	}
	sub.Status = string(domain.SubscriptionStatusCanceled)
	if err := s.store.UpdateSubscription(ctx, sub); err != nil {
		return nil, err
	}
	s.record(ctx, actorID, "subscription.canceled", "subscription", sub.ID, map[string]interface{}{"user_id": sub.UserID, "plan_id": sub.PlanID})
	return sub, nil
}

// allocate grants a plan's included credits for one period, once (keyed by subscription, plan and period).
func (s *Service) allocate(ctx context.Context, sub *Subscription, plan *Plan, period string) error {
	if plan.IncludedCredits <= 0 {
		return nil
	}
	account, tenant, err := s.store.UserCreditAccount(ctx, sub.UserID, plan.ID, period)
	if err != nil {
		return fmt.Errorf("credit account: %w", err)
	}
	key := fmt.Sprintf("plan:%s:%s:%s", sub.ID, plan.ID, period)
	_, err = s.store.Grant(ctx, account, key, domain.CreditGrantSourceAllocation, plan.IncludedCredits, tenant, period, "plan "+plan.Slug)
	return err
}

// RenewAll grants the current month's included credits to every active subscription (the monthly job; safe to run
// as often as wanted). It returns how many subscriptions it went through.
func (s *Service) RenewAll(ctx context.Context) (int, error) {
	org, err := s.store.PlatformOrganization(ctx)
	if err != nil {
		return 0, err
	}
	subs, err := s.store.ActiveSubscriptions(ctx, org)
	if err != nil {
		return 0, err
	}
	period := s.period()
	done := 0
	for i := range subs {
		sub := &subs[i]
		plan, err := s.store.FindPlan(ctx, sub.PlanID)
		if err != nil || plan == nil || plan.Status == string(domain.PlanStatusArchived) {
			continue
		}
		if err := s.allocate(ctx, sub, plan, period); err != nil {
			return done, err
		}
		if sub.PeriodID != period {
			sub.PeriodID = period
			_ = s.store.UpdateSubscription(ctx, sub)
		}
		done++
	}
	return done, nil
}

// GrantCredits adds credits to a person's account for this month (a gesture, a correction), with a reason.
func (s *Service) GrantCredits(ctx context.Context, actorID, userID string, amount int64, reason, requestKey string) error {
	if err := s.admin(ctx, actorID); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if amount <= 0 || amount > 1_000_000 {
		return invalid("amount", "between 1 and 1,000,000 credits")
	}
	if len([]rune(reason)) < 3 || len([]rune(reason)) > 300 {
		return invalid("reason", "3 to 300 characters")
	}
	if u, err := s.users.FindByID(ctx, userID); err != nil || u == nil {
		return invalid("user_id", "unknown account")
	}
	period := s.period()
	account, tenant, err := s.store.UserCreditAccount(ctx, userID, "", period)
	if err != nil {
		return fmt.Errorf("credit account: %w", err)
	}
	key := "manual:" + strings.TrimSpace(requestKey)
	if strings.TrimSpace(requestKey) == "" {
		key = fmt.Sprintf("manual:%s:%d", userID, s.now().UnixNano())
	}
	granted, err := s.store.Grant(ctx, account, key, domain.CreditGrantSourceManual, amount, tenant, period, reason)
	if err != nil {
		return err
	}
	if granted {
		s.record(ctx, actorID, "credits.granted", "user", userID, map[string]interface{}{"amount": amount, "reason": reason, "period": period})
	}
	return nil
}

// --- pricing rules ----------------------------------------------------------------------------------------------

func checkPolicy(p *Policy) error {
	p.ActionType = strings.ToUpper(strings.TrimSpace(p.ActionType))
	if p.Channel == "" {
		p.Channel = string(domain.ChannelOther)
	}
	if p.ActorType == "" {
		p.ActorType = string(domain.ActorTypeCustomer)
	}
	known := false
	for _, a := range BillableActions {
		if a == p.ActionType {
			known = true
		}
	}
	switch {
	case !known:
		return invalid("action_type", "not a metered action")
	case p.Cost < 0 || p.Cost > 100_000:
		return invalid("cost", "between 0 and 100,000 credits")
	}
	return nil
}

// CreatePolicy prices an action. Only one active rule per action (the runtime applies the first active one).
func (s *Service) CreatePolicy(ctx context.Context, actorID string, p Policy) (*Policy, error) {
	if err := s.admin(ctx, actorID); err != nil {
		return nil, err
	}
	if err := checkPolicy(&p); err != nil {
		return nil, err
	}
	if p.Active {
		if other, err := s.store.ActivePolicyFor(ctx, p.ActionType); err == nil && other != nil {
			return nil, ErrConflict
		}
	}
	if err := s.store.CreatePolicy(ctx, &p); err != nil {
		return nil, err
	}
	s.record(ctx, actorID, "usage_policy.created", "usage_policy", p.ID, map[string]interface{}{"action_type": p.ActionType, "cost": p.Cost, "active": p.Active})
	return &p, nil
}

// UpdatePolicy changes a rule's cost or switches it on or off.
func (s *Service) UpdatePolicy(ctx context.Context, actorID string, p Policy) (*Policy, error) {
	if err := s.admin(ctx, actorID); err != nil {
		return nil, err
	}
	old, err := s.store.FindPolicy(ctx, p.ID)
	if err != nil || old == nil {
		return nil, ErrNotFound
	}
	if err := checkPolicy(&p); err != nil {
		return nil, err
	}
	if p.Active {
		if other, err := s.store.ActivePolicyFor(ctx, p.ActionType); err == nil && other != nil && other.ID != p.ID {
			return nil, ErrConflict
		}
	}
	if err := s.store.UpdatePolicy(ctx, &p); err != nil {
		return nil, err
	}
	s.record(ctx, actorID, "usage_policy.updated", "usage_policy", p.ID, map[string]interface{}{"before": old, "after": p})
	return &p, nil
}

// DeletePolicy removes a rule: the action becomes free.
func (s *Service) DeletePolicy(ctx context.Context, actorID, id string) error {
	if err := s.admin(ctx, actorID); err != nil {
		return err
	}
	old, err := s.store.FindPolicy(ctx, id)
	if err != nil || old == nil {
		return ErrNotFound
	}
	if err := s.store.DeletePolicy(ctx, id); err != nil {
		return err
	}
	s.record(ctx, actorID, "usage_policy.deleted", "usage_policy", id, map[string]interface{}{"action_type": old.ActionType, "cost": old.Cost})
	return nil
}

// --- categories -------------------------------------------------------------------------------------------------

// UpdateCategory renames a category or moves it under another one.
func (s *Service) UpdateCategory(ctx context.Context, actorID string, c Category) (*Category, error) {
	if err := s.admin(ctx, actorID); err != nil {
		return nil, err
	}
	old, err := s.store.FindCategory(ctx, c.ID)
	if err != nil || old == nil {
		return nil, ErrNotFound
	}
	c.Name = strings.TrimSpace(c.Name)
	c.Slug = strings.TrimSpace(strings.ToLower(c.Slug))
	if len([]rune(c.Name)) < 2 || len([]rune(c.Name)) > 80 {
		return nil, invalid("name", "2 to 80 characters")
	}
	if !slugRe.MatchString(c.Slug) {
		return nil, invalid("slug", "lowercase letters, digits and hyphens")
	}
	if c.ParentID != nil && (*c.ParentID == "" || *c.ParentID == c.ID) {
		if *c.ParentID == c.ID {
			return nil, invalid("parent_id", "a category cannot be its own parent")
		}
		c.ParentID = nil
	}
	if err := s.store.UpdateCategory(ctx, &c); err != nil {
		return nil, err
	}
	s.record(ctx, actorID, "category.updated", "category", c.ID, map[string]interface{}{"before": old, "after": c})
	return &c, nil
}

// DeleteCategory removes an empty category (no offers, no sub-categories).
func (s *Service) DeleteCategory(ctx context.Context, actorID, id string) error {
	if err := s.admin(ctx, actorID); err != nil {
		return err
	}
	c, err := s.store.FindCategory(ctx, id)
	if err != nil || c == nil {
		return ErrNotFound
	}
	listings, children, err := s.store.CategoryUsage(ctx, id)
	if err != nil {
		return err
	}
	if listings > 0 || children > 0 {
		return ErrInUse
	}
	if err := s.store.DeleteCategory(ctx, id); err != nil {
		return err
	}
	s.record(ctx, actorID, "category.deleted", "category", id, map[string]interface{}{"name": c.Name})
	return nil
}

// --- staff roles ------------------------------------------------------------------------------------------------

func staffRole(r domain.Role) bool {
	return r == domain.RoleAdmin || r == domain.RoleOperator || r == domain.RoleModerator
}

// GrantRole gives an account a staff role (admin, operator, moderator).
func (s *Service) GrantRole(ctx context.Context, actorID, userID string, role domain.Role) error {
	if err := s.admin(ctx, actorID); err != nil {
		return err
	}
	if !staffRole(role) {
		return invalid("role", "admin, operator or moderator")
	}
	if u, err := s.users.FindByID(ctx, userID); err != nil || u == nil {
		return ErrNotFound
	}
	if err := s.users.AssignRole(ctx, userID, role); err != nil && !errors.Is(err, domain.ErrRoleAlreadyAssigned) {
		return err
	}
	s.record(ctx, actorID, "user.role_granted", "user", userID, map[string]interface{}{"role": string(role)})
	return nil
}

// RevokeRole removes a staff role. An administrator keeps their own admin role, and the platform keeps one admin.
func (s *Service) RevokeRole(ctx context.Context, actorID, userID string, role domain.Role) error {
	if err := s.admin(ctx, actorID); err != nil {
		return err
	}
	if !staffRole(role) {
		return invalid("role", "admin, operator or moderator")
	}
	if role == domain.RoleAdmin {
		if userID == actorID {
			return ErrSelfDemote
		}
		if n, err := s.store.CountRole(ctx, domain.RoleAdmin); err == nil && n <= 1 {
			return ErrLastAdmin
		}
	}
	if err := s.users.RemoveRole(ctx, userID, role); err != nil {
		return err
	}
	s.record(ctx, actorID, "user.role_revoked", "user", userID, map[string]interface{}{"role": string(role)})
	return nil
}
