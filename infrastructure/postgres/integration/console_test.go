package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * The console read models, team invitations and listing moderation on the real schema: every query runs against
 * the migrated database, so a wrong column, join or placeholder fails here rather than in the browser.
 */

const (
	cOwner, cManager, cBuyerA, cBuyerB, cStaff = "c0501e00-0000-4000-8000-0000000000a1", "c0501e00-0000-4000-8000-0000000000a2", "c0501e00-0000-4000-8000-0000000000a3", "c0501e00-0000-4000-8000-0000000000a4", "c0501e00-0000-4000-8000-0000000000a5"
	cOrg, cOtherOrg                            = "c0501e00-0000-4000-8000-00000000000a", "c0501e00-0000-4000-8000-00000000000b"
	cSeller, cStore, cCategory                 = "c0501e00-0000-4000-8000-0000000005a1", "c0501e00-0000-4000-8000-0000000007a1", "c0501e00-0000-4000-8000-00000000c001"
	cListing, cListingBare                     = "c0501e00-0000-4000-8000-0000000001a1", "c0501e00-0000-4000-8000-0000000001a2"
	cProvider                                  = "c0501e00-0000-4000-8000-00000000e001"
)

type consoleFixture struct {
	db     *TestDB
	ctx    context.Context
	repo   *postgres.ConsoleRepository
	orders []string
}

func newConsoleFixture(t *testing.T) *consoleFixture {
	db := NewTestDB(t)
	t.Cleanup(db.Close)
	db.Cleanup(t)
	ctx := context.Background()
	f := &consoleFixture{db: db, ctx: ctx, repo: postgres.NewConsoleRepository(db.DB)}

	for id, phone := range map[string]string{cOwner: "+243810000061", cManager: "+243810000062", cBuyerA: "+243810000063", cBuyerB: "+243810000064", cStaff: "+243810000065"} {
		sellerUser(t, db, ctx, id, phone)
	}
	exec(t, db, `UPDATE users SET display_name = 'Marie Kalala' WHERE id = $1`, cBuyerA)
	exec(t, db, `UPDATE users SET display_name = 'Grace Admin', email = 'grace@trezutil.test' WHERE id = $1`, cStaff)
	exec(t, db, `INSERT INTO roles (name, description) VALUES ('admin', 'Platform administrator') ON CONFLICT DO NOTHING`)
	exec(t, db, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'admin'`, cStaff)

	createTestOrganization(t, db, ctx, cOrg)
	createTestOrganization(t, db, ctx, cOtherOrg)
	orgs := postgres.NewOrganizationRepository(db.DB)
	for user, role := range map[string]domain.OrganizationRole{cOwner: domain.OrganizationRoleOwner, cManager: domain.OrganizationRoleManager} {
		if err := orgs.AddMember(ctx, domain.NewOrganizationMember(user, cOrg, role)); err != nil {
			t.Fatal(err)
		}
	}
	createTestSeller(t, db, ctx, cSeller, cOwner, cOrg)
	createTestStore(t, db, ctx, cStore, cOrg)
	createTestCategory(t, db, ctx, cCategory)
	createTestListing(t, db, ctx, cListing, cOrg, cSeller, cStore, cCategory)
	createTestListing(t, db, ctx, cListingBare, cOrg, cSeller, cStore, cCategory)
	exec(t, db, `UPDATE listings SET status = 'published', title = 'Wax fabric', vertical = 'RETAIL' WHERE id = $1`, cListing)
	exec(t, db, `UPDATE listings SET title = 'Free sample', price_amount = 0 WHERE id = $1`, cListingBare)
	exec(t, db, `INSERT INTO listing_media (listing_id, type, url, is_primary) VALUES ($1, 'image', 'https://img.test/a.jpg', true)`, cListing)

	uc := application.NewOrderUseCases(postgres.NewOrderRepository(db.DB), postgres.NewListingRepository(db.DB), postgres.NewSellerRepository(db.DB),
		orgs, postgres.NewUserRepository(db.DB), postgres.NewInventoryRepository(db.DB), postgres.NewFulfillmentRepository(db.DB), nil)
	for i, buyer := range []string{cBuyerA, cBuyerA, cBuyerB} {
		o, err := uc.CreateOrder(ctx, buyer, cListing, 1)
		if err != nil {
			t.Fatalf("order: %v", err)
		}
		f.orders = append(f.orders, o.ID)
		exec(t, db, `UPDATE orders SET created_at = NOW() - make_interval(hours => $2), updated_at = NOW() - make_interval(hours => $2) WHERE id = $1`, o.ID, 72-24*i)
	}
	// A: completed and paid; A: paid three days ago, never confirmed; B: still awaiting payment.
	exec(t, db, `UPDATE orders SET status = 'completed' WHERE id = $1`, f.orders[0])
	exec(t, db, `UPDATE orders SET status = 'paid', updated_at = NOW() - interval '60 hours' WHERE id = $1`, f.orders[1])

	exec(t, db, `INSERT INTO providers (id, organization_id, code, name, type, status, capabilities, configuration, secret_ref, health_status)
		VALUES ($1, $2, 'stripe', 'Stripe', 'PAYMENT', 'ACTIVE', '{PAYMENT}', '{"settings":{"publishable_key":"pk_test"}}', 'env:STRIPE_SECRET_KEY', 'HEALTHY')`, cProvider, cOrg)
	exec(t, db, `INSERT INTO payments (organization_id, order_id, amount_minor, currency, payment_method_type, payment_method_id, provider_id, idempotency_key, status, captured_at, refunded_amount_minor)
		VALUES ($1, $2, 10000, 'USD', 'CARD', 'pm_1', $3, 'k1', 'partially_refunded', NOW(), 2500)`, cOrg, f.orders[0], cProvider)
	exec(t, db, `INSERT INTO payments (organization_id, order_id, amount_minor, currency, payment_method_type, payment_method_id, provider_id, idempotency_key, status, failed_at, failure_reason)
		VALUES ($1, $2, 10000, 'USD', 'CARD', 'pm_2', $3, 'k2', 'failed', NOW(), 'card_declined')`, cOrg, f.orders[2], cProvider)
	exec(t, db, `INSERT INTO provider_executions (provider_id, organization_id, capability, operation, status, error_code, duration_ms)
		VALUES ($1, $2, 'PAYMENT', 'create', 'SUCCESS', NULL, 120), ($1, $2, 'PAYMENT', 'create', 'FAILED', 'card_declined', 300)`, cProvider, cOrg)

	exec(t, db, `INSERT INTO reservations (organization_id, customer_id, listing_id, status, start_at, end_at) VALUES ($1, $2, $3, 'confirmed', NOW() + interval '1 day', NOW() + interval '2 days')`, cOrg, cBuyerB, cListing)
	exec(t, db, `INSERT INTO fulfillments (tenant_id, order_id, type, status, reference) VALUES ($1, $2, 'DELIVERY', 'PENDING', 'FUL-1'), ($1, $3, 'PICKUP', 'COMPLETED', 'FUL-2')`, cOrg, f.orders[1], f.orders[0])
	exec(t, db, `INSERT INTO inventory_resources (id, tenant_id, listing_id, resource_type, total, reserved, consumed) VALUES ('c0501e00-0000-4000-8000-00000000f001', $1, $2, 'COUNTABLE', 10, 2, 3)`, cOrg, cListing)
	exec(t, db, `INSERT INTO capacity_units (inventory_id, start_at, end_at, capacity, reserved) VALUES
		('c0501e00-0000-4000-8000-00000000f001', NOW() + interval '1 day', NOW() + interval '2 days', 4, 1),
		('c0501e00-0000-4000-8000-00000000f001', NOW() + interval '90 days', NOW() + interval '91 days', 4, 0)`)
	exec(t, db, `INSERT INTO channel_accounts (organization_id, channel, external_id) VALUES ($1, 'whatsapp', '243999000111')`, cOrg)
	exec(t, db, `INSERT INTO webhook_events (channel, provider_message_id, idempotency_key, status, error_message) VALUES ('whatsapp', 'm1', 'i1', 'processed', NULL), ('whatsapp', 'm2', 'i2', 'failed', 'token expired')`)
	exec(t, db, `INSERT INTO conversations (organization_id, customer_id, channel, external_id) VALUES ($1, $2, 'whatsapp', 'wa-1')`, cOrg, cBuyerA)
	return f
}

func TestConsole_MoneyCustomersFulfilment(t *testing.T) {
	f := newConsoleFixture(t)
	ctx, repo := f.ctx, f.repo
	now := time.Now()

	totals, err := repo.MoneyTotals(ctx, cOrg, now.Add(-30*24*time.Hour).Unix(), now.Add(time.Minute).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 || totals[0].Currency != "USD" || totals[0].CapturedMinor != 10000 || totals[0].RefundedMinor != 2500 || totals[0].NetMinor != 7500 ||
		totals[0].Failed != 1 || totals[0].Orders != 2 {
		t.Fatalf("totals = %+v", totals)
	}
	other, err := repo.MoneyTotals(ctx, cOtherOrg, now.Add(-30*24*time.Hour).Unix(), now.Unix())
	if err != nil || len(other) != 0 {
		t.Fatalf("another business sees money: %+v %v", other, err)
	}

	pays, total, err := repo.Payments(ctx, domain.PaymentFilter{OrganizationID: cOrg, Status: "failed"}, domain.Page{Limit: 10})
	if err != nil || total != 1 || pays[0].FailureReason != "card_declined" || pays[0].ProviderCode != "stripe" {
		t.Fatalf("failed payments = %+v %d %v", pays, total, err)
	}
	// the search word is data, never SQL
	if _, total, err := repo.Payments(ctx, domain.PaymentFilter{Query: "x' OR '1'='1"}, domain.Page{Limit: 10}); err != nil || total != 0 {
		t.Fatalf("injection matched %d (%v)", total, err)
	}

	customers, total, err := repo.Customers(ctx, cOrg, "", domain.Page{Limit: 10})
	if err != nil || total != 2 {
		t.Fatalf("customers = %d %v", total, err)
	}
	var marie *domain.CustomerRow
	for i := range customers {
		if customers[i].CustomerID == cBuyerA {
			marie = &customers[i]
		}
	}
	if marie == nil || marie.DisplayName != "Marie Kalala" || marie.Orders != 2 || marie.Reservations != 0 || marie.Spent["USD"] == 0 {
		t.Fatalf("Marie = %+v", marie)
	}
	if found, n, _ := repo.Customers(ctx, cOrg, "kalala", domain.Page{Limit: 10}); n != 1 || found[0].CustomerID != cBuyerA {
		t.Fatalf("search by name = %d", n)
	}

	open, n, err := repo.Fulfillments(ctx, cOrg, "open", domain.Page{Limit: 10})
	if err != nil || n != 1 || open[0].Reference != "FUL-1" || open[0].CustomerName != "Marie Kalala" || open[0].OrderStatus != "paid" {
		t.Fatalf("open fulfilments = %+v %d %v", open, n, err)
	}
	if all, n, _ := repo.Fulfillments(ctx, cOrg, "", domain.Page{Limit: 10}); n != 2 || all[0].Status != "PENDING" {
		t.Fatalf("open ones first: %+v", all)
	}
}

func TestConsole_CapacityUsageIntegrationsTeam(t *testing.T) {
	f := newConsoleFixture(t)
	ctx, repo := f.ctx, f.repo
	now := time.Now()

	res, err := repo.Capacity(ctx, cOrg, now.Unix(), now.Add(14*24*time.Hour).Unix())
	if err != nil || len(res) != 1 || res[0].Total != 10 || len(res[0].Windows) != 1 || res[0].Windows[0].Reserved != 1 || res[0].ListingTitle != "Wax fabric" {
		t.Fatalf("capacity = %+v %v", res, err)
	}

	usage, err := repo.Usage(ctx, cOrg)
	if err != nil || usage.Subscription != nil || len(usage.Accounts) != 0 {
		t.Fatalf("usage of a business with no plan = %+v %v", usage, err)
	}

	providers, err := repo.Providers(ctx, cOrg, now.Add(-7*24*time.Hour).Unix())
	if err != nil || len(providers) != 1 {
		t.Fatalf("providers = %+v %v", providers, err)
	}
	p := providers[0]
	if p.Attempts != 2 || p.Failures != 1 || p.LastError != "card_declined" || p.P95Ms != 300 || len(p.Capabilities) != 1 {
		t.Fatalf("provider = %+v", p)
	}
	channels, err := repo.ChannelAccounts(ctx, cOrg)
	if err != nil || len(channels) != 1 || channels[0].ExternalID != "••••0111" {
		t.Fatalf("channels = %+v %v", channels, err)
	}

	team, err := repo.Team(ctx, cOrg)
	if err != nil || len(team) != 2 || team[0].Role != "owner" || strings.Contains(team[0].MaskedPhone, "0000061") || !strings.HasSuffix(team[0].MaskedPhone, "61") {
		t.Fatalf("team = %+v %v", team, err)
	}

	// the switch only moves from the status the caller saw
	if org, err := repo.SetProviderStatus(ctx, cProvider, "ACTIVE", "INACTIVE"); err != nil || org != cOrg {
		t.Fatalf("disable = %q %v", org, err)
	}
	if _, err := repo.SetProviderStatus(ctx, cProvider, "ACTIVE", "INACTIVE"); !errors.Is(err, domain.ErrAccountState) {
		t.Fatalf("second disable = %v", err)
	}
}

func TestConsole_AdministrationReads(t *testing.T) {
	f := newConsoleFixture(t)
	ctx, repo := f.ctx, f.repo
	now := time.Now()

	listings, total, err := repo.AdminListings(ctx, domain.AdminListingFilter{}, domain.Page{Limit: 10})
	if err != nil || total != 2 {
		t.Fatalf("listings = %d %v", total, err)
	}
	if bare, n, _ := repo.AdminListings(ctx, domain.AdminListingFilter{Flag: "no_image"}, domain.Page{Limit: 10}); n != 1 || bare[0].ID != cListingBare ||
		len(bare[0].Flags) != 2 {
		t.Fatalf("no_image = %+v", bare)
	}
	if pub, n, _ := repo.AdminListings(ctx, domain.AdminListingFilter{Status: "published", Vertical: "RETAIL", Query: "wax"}, domain.Page{Limit: 10}); n != 1 || pub[0].Images != 1 || pub[0].OrganizationName == "" {
		t.Fatalf("published retail = %+v", pub)
	}
	_ = listings

	ex, err := repo.Exceptions(ctx, now.Add(-7*24*time.Hour).Unix(), now.Add(-48*time.Hour).Unix(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if ex.Counts["failed_payment"] != 1 || ex.Counts["failed_webhook"] != 1 || ex.Counts["unconfirmed_order"] != 1 || ex.Counts["failed_payment_webhook"] != 0 {
		t.Fatalf("exceptions = %+v", ex.Counts)
	}

	ch, err := repo.ChannelHealth(ctx, now.Unix())
	if err != nil || len(ch) != 1 || ch[0].Channel != "whatsapp" || ch[0].Events24h != 2 || ch[0].Failed24h != 1 || ch[0].LastError != "token expired" ||
		ch[0].Accounts != 1 || ch[0].Conversations7d != 1 {
		t.Fatalf("channel health = %+v %v", ch, err)
	}

	staff, err := repo.StaffAccounts(ctx)
	if err != nil || len(staff) != 1 || staff[0].Email != "grace@trezutil.test" || staff[0].Roles[0] != "admin" {
		t.Fatalf("staff = %+v %v", staff, err)
	}

	ai, err := repo.ConversationSummary(ctx, now.Unix(), 30)
	if err != nil || ai.Conversations != 1 || ai.ByChannel["whatsapp"] != 1 || ai.AIExecutions != 0 || len(ai.PerDay) != 1 {
		t.Fatalf("ai = %+v %v", ai, err)
	}

	m, err := repo.Monetization(ctx, now.Add(-30*24*time.Hour).Unix())
	if err != nil || len(m.Plans) != 0 || m.Accounts != 0 {
		t.Fatalf("monetization = %+v %v", m, err)
	}
	insertTestPlan(t, f.db, ctx, "c0501e00-0000-4000-8000-00000000d001", cOrg, "starter", 0, 100, 0)
	if m, err = repo.Monetization(ctx, now.Add(-30*24*time.Hour).Unix()); err != nil || len(m.Plans) != 1 || m.Plans[0].QuotaAmount != 100 {
		t.Fatalf("plans = %+v %v", m, err)
	}
}

func TestTeam_InvitationBecomesMembershipAtSignIn(t *testing.T) {
	f := newConsoleFixture(t)
	ctx := f.ctx
	orgs := postgres.NewOrganizationRepository(f.db.DB)
	team := application.NewTeamUseCases(postgres.NewTeamRepository(f.db.DB), orgs, nil)

	// a manager cannot invite; an owner can, never as owner, and an admin role only by an owner
	if _, err := team.Invite(ctx, cManager, cOrg, "+243810000099", domain.OrganizationRoleStaff); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("manager invite = %v", err)
	}
	if _, err := team.Invite(ctx, cOwner, cOrg, "+243810000099", domain.OrganizationRoleOwner); !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("owner role = %v", err)
	}
	if _, err := team.Invite(ctx, cOwner, cOrg, "0810000099", domain.OrganizationRoleSales); err != nil {
		t.Fatalf("invite: %v", err)
	}
	// a second invitation for the same number replaces the role
	if _, err := team.Invite(ctx, cOwner, cOrg, "+243 810 000 099", domain.OrganizationRoleManager); err != nil {
		t.Fatalf("re-invite: %v", err)
	}
	inv, err := team.Invitations(ctx, cOwner, cOrg)
	if err != nil || len(inv) != 1 || inv[0].Role != "manager" || strings.Contains(inv[0].MaskedPhone, "000099") {
		t.Fatalf("invitations = %+v %v", inv, err)
	}

	// the number signs in for the first time: the account is new, the membership is there
	newcomer := "c0501e00-0000-4000-8000-0000000000a9"
	sellerUser(t, f.db, ctx, newcomer, "+243810000099")
	if err := team.AcceptInvitations(ctx, newcomer, "+243810000099"); err != nil {
		t.Fatal(err)
	}
	if role, err := orgs.GetMemberRole(ctx, cOrg, newcomer); err != nil || role != domain.OrganizationRoleManager {
		t.Fatalf("role after sign-in = %q %v", role, err)
	}
	if inv, _ := team.Invitations(ctx, cOwner, cOrg); len(inv) != 0 {
		t.Fatalf("accepted invitation still open: %+v", inv)
	}
	// signing in again changes nothing
	if err := team.AcceptInvitations(ctx, newcomer, "+243810000099"); err != nil {
		t.Fatal(err)
	}

	// role changes: never the owner, never oneself; the change is conditional
	if err := team.ChangeRole(ctx, cOwner, cOrg, newcomer, domain.OrganizationRoleStaff); err != nil {
		t.Fatalf("change role: %v", err)
	}
	if err := team.ChangeRole(ctx, cOwner, cOrg, cOwner, domain.OrganizationRoleStaff); !errors.Is(err, domain.ErrOwnerProtected) {
		t.Fatalf("own role = %v", err)
	}

	// an admin cannot remove the owner
	exec(t, f.db, `UPDATE organization_members SET membership_role = 'admin' WHERE organization_id = $1 AND user_id = $2`, cOrg, cManager)
	members := application.NewOrganizationUseCases(orgs, postgres.NewUserRepository(f.db.DB), nil)
	if err := members.RemoveMember(ctx, cManager, cOrg, cOwner); !errors.Is(err, domain.ErrOwnerProtected) {
		t.Fatalf("admin removing the owner = %v", err)
	}

	// revoking an invitation closes it once
	id, err := team.Invite(ctx, cOwner, cOrg, "+243810000098", domain.OrganizationRoleStaff)
	if err != nil {
		t.Fatal(err)
	}
	if err := team.RevokeInvitation(ctx, cOwner, cOrg, id); err != nil {
		t.Fatal(err)
	}
	if err := team.RevokeInvitation(ctx, cOwner, cOrg, id); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Fatalf("second revoke = %v", err)
	}
}

func TestListingModeration_OnTheRealSchema(t *testing.T) {
	f := newConsoleFixture(t)
	ctx := f.ctx
	db := f.db.DB
	uc := application.NewListingUseCases(postgres.NewListingRepository(db), postgres.NewCategoryRepository(db),
		postgres.NewAttributeDefinitionRepository(db), postgres.NewAttributeValueRepository(db), postgres.NewListingMediaRepository(db),
		postgres.NewListingAvailabilityRepository(db), postgres.NewLocationRepository(db), postgres.NewListingLocationRepository(db),
		postgres.NewOrganizationRepository(db), postgres.NewSellerRepository(db), postgres.NewStoreRepository(db), postgres.NewUserRepository(db), nil)

	if err := uc.SuspendListing(ctx, cOwner, cListing, "counterfeit"); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("seller suspending = %v", err)
	}
	if err := uc.SuspendListing(ctx, cStaff, cListing, ""); !errors.Is(err, domain.ErrReasonRequired) {
		t.Fatalf("no reason = %v", err)
	}
	if err := uc.SuspendListing(ctx, cStaff, cListing, "Counterfeit brand"); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	// the seller cannot bring it back by resuming or publishing
	if err := uc.ResumeListing(ctx, cOwner, cListing); err == nil {
		t.Fatal("a seller resumed a suspended listing")
	}
	if err := uc.PublishListing(ctx, cOwner, cListing); err == nil {
		t.Fatal("a seller published a suspended listing")
	}
	if err := uc.ReinstateListing(ctx, cStaff, cListing, ""); err != nil {
		t.Fatalf("reinstate: %v", err)
	}
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM listings WHERE id = $1`, cListing).Scan(&status); err != nil || status != "paused" {
		t.Fatalf("after reinstatement = %q %v", status, err)
	}
	// the seller closes it for good
	if err := uc.CloseListing(ctx, cOwner, cListing); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := uc.CloseListing(ctx, cOwner, cListing); !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Fatalf("second close = %v", err)
	}
}
