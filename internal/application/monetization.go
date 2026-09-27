package application

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// monthFromUnix returns the YYYY-MM period identifier for a unix timestamp (UTC).
func monthFromUnix(unix int64) string {
	return time.Unix(unix, 0).UTC().Format("2006-01")
}

// PlanService manages the per-tenant plan catalog (read-only; callers scope by tenant).
type PlanService struct {
	plans  domain.PlanRepository
	logger zerolog.Logger
}

func NewPlanService(plans domain.PlanRepository, logger zerolog.Logger) *PlanService {
	return &PlanService{plans: plans, logger: logger}
}

func (s *PlanService) ListPlans(ctx context.Context, tenantID string) ([]*domain.Plan, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	return s.plans.ListByTenant(ctx, tenantID)
}

func (s *PlanService) FindPlan(ctx context.Context, tenantID, planID string) (*domain.Plan, error) {
	if planID == "" {
		return nil, fmt.Errorf("plan_id is required")
	}
	plan, err := s.plans.FindByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil || plan.TenantID != tenantID || plan.IsArchived() {
		return nil, domain.ErrMonetizationPlanNotFound
	}
	return plan, nil
}

// CreditPurchaseService is the enforcement boundary between Payment Core and the
// credit system. It reacts to a Captured payment (webhook-driven, idempotent) and grants
// credits via the idempotent CreditGranter primitive. It never inspects client-supplied
// credit balances (invariant: client balances never trusted) and never creates payments
// itself (Payment Core owns money movement).
type CreditPurchaseService struct {
	payments  domain.PaymentStatusChecker
	purchases domain.CreditPurchaseRepository
	accounts  domain.CreditAccountCreator
	granter   domain.CreditGranter
	logger    zerolog.Logger
}

type CreditPurchaseServiceConfig struct {
	Payments  domain.PaymentStatusChecker
	Purchases domain.CreditPurchaseRepository
	Accounts  domain.CreditAccountCreator
	Granter   domain.CreditGranter
	Logger    zerolog.Logger
}

func NewCreditPurchaseService(cfg CreditPurchaseServiceConfig) *CreditPurchaseService {
	return &CreditPurchaseService{
		payments:  cfg.Payments,
		purchases: cfg.Purchases,
		accounts:  cfg.Accounts,
		granter:   cfg.Granter,
		logger:    cfg.Logger,
	}
}

const creditPurchaseIdempotencyKeyPrefix = "purchase:"

// OnPaymentCaptured is the idempotent entry point invoked when Payment Core confirms a
// payment reached Captured status. It resolves the credit pack from payment metadata,
// ensures a credit account exists for the period, and grants credits exactly once via
// GrantCreditsTx (ON CONFLICT idempotency key). Replay of the same payment_id is a no-op.
func (s *CreditPurchaseService) OnPaymentCaptured(ctx context.Context, orgID, actorID, paymentID string) (*domain.CreditPurchase, error) {
	payment, err := s.payments.GetPaymentStatus(ctx, orgID, actorID, paymentID, creditPurchaseIdempotencyKeyPrefix+paymentID)
	if err != nil {
		return nil, fmt.Errorf("credit purchase: payment status check failed: %w", err)
	}
	if payment.Status != domain.PaymentStatusCaptured {
		return nil, fmt.Errorf("credit purchase: payment %s is not captured (status=%s)", paymentID, payment.Status)
	}

	planID, ownerTypeStr, ownerID, creditPack := parseCreditPurchaseMetadata(payment)
	if creditPack <= 0 {
		return nil, fmt.Errorf("credit purchase: payment %s metadata missing credit_pack", paymentID)
	}

	periodID := monthFromUnix(payment.CreatedAt)
	account, err := s.accounts.FindOrCreateByOwnerAndPeriod(ctx, domain.ActorType(ownerTypeStr), ownerID, payment.TenantID, planID, periodID)
	if err != nil {
		return nil, fmt.Errorf("credit purchase: credit account resolution failed: %w", err)
	}

	reason := "credit purchase for plan " + planID
	granted, newBalance, err := s.granter.GrantCreditsTx(ctx, account.AccountID,
		creditPurchaseIdempotencyKeyPrefix+paymentID, domain.CreditGrantSourcePurchase,
		creditPack, payment.TenantID, periodID, &payment.ID, reason)
	if err != nil {
		return nil, fmt.Errorf("credit purchase: idempotent grant failed: %w", err)
	}

	if err := s.purchases.MarkCaptured(ctx, payment.ID, payment.TenantID, planID, ownerTypeStr, ownerID, creditPack, time.Now().Unix()); err != nil {
		return nil, fmt.Errorf("credit purchase: failed to record purchase: %w", err)
	}

	if !granted {
		s.logger.Info().Str("payment_id", payment.ID).Msg("credit purchase already granted (idempotent replay)")
	} else {
		s.logger.Info().Str("payment_id", payment.ID).Int64("balance", newBalance).Msg("credit purchase settled")
	}
	return s.purchases.FindByPaymentID(ctx, payment.ID)
}

func (s *CreditPurchaseService) GetCreditPurchase(ctx context.Context, id string) (*domain.CreditPurchase, error) {
	return s.purchases.FindByID(ctx, id)
}

// parseCreditPurchaseMetadata extracts the credit-pack parameters stored on the
// payment by the initiating client/handler. owner_type/owner_id are the purchasing
// actor; the tenant is taken from payment.TenantID (server-resolved, not client).
func parseCreditPurchaseMetadata(payment *domain.Payment) (planID, ownerType, ownerID string, creditPack int64) {
	planID, _ = payment.Metadata["plan_id"].(string)
	ownerType, _ = payment.Metadata["owner_type"].(string)
	ownerID, _ = payment.Metadata["owner_id"].(string)
	switch v := payment.Metadata["credit_pack"].(type) {
	case int64:
		creditPack = v
	case int:
		creditPack = int64(v)
	case string:
		if n, convErr := strconv.ParseInt(v, 10, 64); convErr == nil {
			creditPack = n
		}
	case float64:
		creditPack = int64(v)
	}
	return planID, ownerType, ownerID, creditPack
}

// SubscriptionService manages the subscription lifecycle and billing-period credit
// allocation. Allocation grants are idempotent (keyed by tenant:period:owner) so renewal
// webhooks or retries never double-grant a monthly quota.
type SubscriptionService struct {
	plans    domain.PlanRepository
	subs     domain.SubscriptionRepository
	periods  domain.BillingPeriodRepository
	accounts domain.CreditAccountCreator
	grant    domain.CreditGranter
	logger   zerolog.Logger
}

type SubscriptionServiceConfig struct {
	Plans    domain.PlanRepository
	Subs     domain.SubscriptionRepository
	Periods  domain.BillingPeriodRepository
	Accounts domain.CreditAccountCreator
	Grant    domain.CreditGranter
	Logger   zerolog.Logger
}

func NewSubscriptionService(cfg SubscriptionServiceConfig) *SubscriptionService {
	return &SubscriptionService{
		plans:    cfg.Plans,
		subs:     cfg.Subs,
		periods:  cfg.Periods,
		accounts: cfg.Accounts,
		grant:    cfg.Grant,
		logger:   cfg.Logger,
	}
}

// CreateSubscription creates an active subscription and an initial open billing period.
func (s *SubscriptionService) CreateSubscription(ctx context.Context, tenantID, planID, ownerType, ownerID string) (*domain.Subscription, *domain.BillingPeriod, error) {
	if tenantID == "" || planID == "" || ownerID == "" {
		return nil, nil, fmt.Errorf("tenant_id, plan_id, owner_id are required")
	}
	plan, err := s.plans.FindByID(ctx, planID)
	if err != nil {
		return nil, nil, fmt.Errorf("subscription: plan lookup failed: %w", err)
	}
	if plan == nil || plan.TenantID != tenantID || plan.IsArchived() {
		return nil, nil, domain.ErrMonetizationPlanNotFound
	}

	periodID := monthFromUnix(time.Now().Unix())
	sub := domain.NewSubscription(tenantID, planID, ownerType, ownerID, periodID, time.Now().Unix())
	if err := s.subs.Create(ctx, sub); err != nil {
		return nil, nil, fmt.Errorf("subscription create failed: %w", err)
	}
	period := s.newOpenPeriod(ctx, sub, plan, periodID)
	return sub, period, nil
}

// RenewPeriod opens (or reuses) the current open billing period and grants the plan's
// monthly allocation idempotently. Called on subscription start, renewal, or retry.
func (s *SubscriptionService) RenewPeriod(ctx context.Context, tenantID, ownerType, ownerID, planID string) (*domain.Subscription, *domain.BillingPeriod, error) {
	sub, err := s.subs.FindByOwner(ctx, tenantID, ownerType, ownerID)
	if err != nil {
		return nil, nil, fmt.Errorf("subscription lookup failed: %w", err)
	}
	if sub == nil {
		return s.CreateSubscription(ctx, tenantID, planID, ownerType, ownerID)
	}
	if sub.PlanID != planID {
		return nil, nil, domain.ErrMonetizationPlanNotFound
	}
	if !sub.IsActive() {
		return nil, nil, domain.ErrSubscriptionNotActive
	}

	periodID := monthFromUnix(time.Now().Unix())
	now := time.Now().Unix()
	period := s.openPeriod(ctx, sub, planID, periodID, now)

	if err := s.grantAllocation(ctx, sub, period, ownerType, ownerID, tenantID, planID, now); err != nil {
		return sub, period, err
	}
	return sub, period, nil
}

func (s *SubscriptionService) newOpenPeriod(ctx context.Context, sub *domain.Subscription, plan *domain.Plan, periodID string) *domain.BillingPeriod {
	return s.openPeriod(ctx, sub, plan.ID, periodID, time.Now().Unix())
}

func (s *SubscriptionService) openPeriod(ctx context.Context, sub *domain.Subscription, planID, periodID string, now int64) *domain.BillingPeriod {
	existing, _ := s.periods.FindByID(ctx, periodID, sub.TenantID)
	if existing != nil {
		return existing
	}
	monthStart, mErr := time.ParseInLocation("2006-01", periodID, time.UTC)
	if mErr != nil {
		monthStart = time.Unix(now, 0).UTC()
	}
	startsAt := monthStart.Unix()
	endsAt := monthStart.AddDate(0, 1, 0).Unix() - 1
	period := domain.NewBillingPeriod(periodID, sub.TenantID, sub.ID, planID, startsAt, endsAt, now)
	if err := s.periods.Create(ctx, period); err != nil {
		s.logger.Warn().Err(err).Str("period_id", periodID).Msg("failed to create billing period")
		return period
	}
	return period
}

// grantAllocation grants the plan's free-tier quota (allocation) to the owner's credit
// account for the period, idempotently keyed by tenant:period:owner.
func (s *SubscriptionService) grantAllocation(ctx context.Context, sub *domain.Subscription, period *domain.BillingPeriod, ownerType, ownerID, tenantID, planID string, now int64) error {
	plan, err := s.plans.FindByID(ctx, planID)
	if err != nil || plan == nil {
		return domain.ErrMonetizationPlanNotFound
	}
	if plan.QuotaAmount <= 0 {
		return nil
	}
	account, err := s.accounts.FindOrCreateByOwnerAndPeriod(ctx, domain.ActorType(ownerType), ownerID, tenantID, planID, period.ID)
	if err != nil {
		return fmt.Errorf("credit account resolution failed: %w", err)
	}
	key := fmt.Sprintf("allocation:%s:%s:%s", tenantID, period.ID, ownerID)
	granted, balance, err := s.grant.GrantCreditsTx(ctx, account.AccountID, key,
		domain.CreditGrantSourceAllocation, plan.QuotaAmount, tenantID, period.ID, nil, "subscription allocation")
	if err != nil {
		return fmt.Errorf("subscription allocation grant failed: %w", err)
	}
	s.logger.Info().Bool("granted", granted).Int64("balance", balance).Str("owner", ownerID).Str("period", period.ID).Msg("subscription allocation recorded")
	return nil
}
