// Command identity-service serves phone-OTP sign-in, staff sign-in with email
// and password, JWT sessions (/api/v1/auth/*) and the administration console
// API (/api/v1/admin/*).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	googleauth "github.com/trezutil/trezutil/internal/infrastructure/google"
	"github.com/trezutil/trezutil/internal/application/adminmanage"
	"github.com/trezutil/trezutil/internal/application/exchangerate"
	"github.com/trezutil/trezutil/internal/application/supplieronboarding"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/audit"
	"github.com/trezutil/trezutil/internal/infrastructure/otp"
	"github.com/trezutil/trezutil/internal/infrastructure/whatsapp"
	"github.com/trezutil/trezutil/packages/auth"
	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/database"
	"github.com/trezutil/trezutil/packages/observability"
)

func main() {
	logger := common.Logger("identity-service")

	jwtCfg, err := auth.ConfigFromEnv()
	if err != nil {
		log.Fatalf("identity-service: %v", err)
	}
	tokens := domain.NewJWTTokenService(jwtCfg)

	db, err := database.Open(context.Background())
	if err != nil {
		log.Fatalf("identity-service: %v", err)
	}
	defer db.Close()

	sender := buildOTPSender()
	if sender == nil {
		log.Fatalf("identity-service: no OTP delivery configured (set WHATSAPP_* credentials, or APP_ENV=development to log codes)")
	}

	// The OTP code HMAC key defaults to a derivation of the JWT secret so a
	// single secret suffices; OTP_CODE_SECRET overrides it.
	codeSecret := common.Getenv("OTP_CODE_SECRET", "otp:"+jwtCfg.SecretKey)
	if auth.Strict() {
		// Outside development the OTP hashing key must be its own strong secret:
		// one leaked key must not compromise both tokens and sign-in codes.
		if err := auth.ValidateSecret("OTP_CODE_SECRET", os.Getenv("OTP_CODE_SECRET")); err != nil {
			log.Fatalf("identity-service: %v", err)
		}
		if os.Getenv("OTP_CODE_SECRET") == jwtCfg.SecretKey {
			log.Fatalf("identity-service: OTP_CODE_SECRET must differ from JWT_SECRET")
		}
	}

	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	auditStore := postgres.NewAuditRepository(db)
	auditLog := audit.NewLogger(logger).WithStore(auditStore)
	// Team invitations by phone number become memberships when the number signs in.
	team := application.NewTeamUseCases(postgres.NewTeamRepository(db), orgRepo, auditLog)
	svc, err := identity.NewService(identity.Config{
		Users:          userRepo,
		Challenges:     postgres.NewOTPChallengeRepository(db),
		Sender:         sender,
		Tokens:         tokens,
		Credentials:    userRepo, // staff sign-in with email and password
		CodeSecret:     codeSecret,
		AccessTokenTTL: jwtCfg.AccessTokenTTL,
		MaxPerHour:     common.GetenvInt("OTP_MAX_PER_PHONE_HOUR", identity.DefaultMaxPerPhoneHour),
		// Wrong passwords on a staff account lock it for a while.
		MaxLoginFailures: common.GetenvInt("STAFF_LOGIN_MAX_FAILURES", identity.DefaultMaxLoginFailures),
		Lockout:          time.Duration(common.GetenvInt("STAFF_LOGIN_LOCKOUT_MINUTES", int(identity.DefaultLockout/time.Minute))) * time.Minute,
		Logger:           logger,
		OnPhoneSignIn:    team.AcceptInvitations,
		// Sign in with Google: on when GOOGLE_CLIENT_ID is set (public OAuth client id; no secret is used).
		Google:     googleVerifier(),
		Identities: postgres.NewExternalIdentityRepository(db),
	})
	if err != nil {
		log.Fatalf("identity-service: %v", err)
	}

	h := api.NewIdentityHandlerWithLimit(svc, common.GetenvInt("OTP_MAX_PER_IP_10MIN", 20)).
		WithStaffLoginLimit(common.GetenvInt("STAFF_LOGIN_MAX_PER_IP_10MIN", 10))

	// Seller onboarding (operator-assisted): platform operators create the
	// organization and add the seller's account as owner; the owner then
	// creates the seller profile and stores. Authorization lives in the use cases.
	orgUseCases := application.NewOrganizationUseCases(orgRepo, userRepo, auditLog)
	orgs := api.NewOrganizationHandler(orgUseCases)
	profileAccess := api.NewProfileAccess(userRepo, orgRepo)
	sellers := api.NewSellerHandler(application.NewSellerUseCases(postgres.NewSellerRepository(db), orgRepo, userRepo, auditLog)).WithAccess(profileAccess)
	stores := api.NewStoreHandler(application.NewStoreUseCases(postgres.NewStoreRepository(db), orgRepo, userRepo, auditLog)).WithAccess(profileAccess)
	orgStaff := organizationStaffOnly(orgRepo, userRepo)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/request-otp", h.RequestOTPH)
	mux.HandleFunc("POST /api/v1/auth/verify-otp", h.VerifyOTPH)
	mux.HandleFunc("POST /api/v1/auth/staff/login", h.StaffLoginH)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.RefreshH)
	api.NewGoogleAuthHandler(svc, svc, common.GetenvInt("GOOGLE_SIGN_IN_MAX_PER_IP_10MIN", 30)).Register(mux)
	mux.HandleFunc("GET /api/v1/auth/users/me", h.MeH)
	mux.HandleFunc("PATCH /api/v1/auth/users/me", h.UpdateMeH)
	mux.HandleFunc("POST /api/v1/organizations", orgs.CreateOrganizationH)
	mux.HandleFunc("GET /api/v1/organizations", orgStaff(orgs.GetOrganizationH))
	// Editing the business, a seller profile and a store (checked fields only; the use cases decide who may).
	mux.HandleFunc("PATCH /api/v1/organizations", orgs.PatchOrganizationH())
	mux.HandleFunc("PATCH /api/v1/sellers", sellers.PatchSellerH())
	mux.HandleFunc("PATCH /api/v1/stores", stores.PatchStoreH())
	mux.HandleFunc("GET /api/v1/organizations/members", orgStaff(orgs.GetMembersH))
	mux.HandleFunc("POST /api/v1/organizations/members", orgs.AddMemberH)
	mux.HandleFunc("DELETE /api/v1/organizations/members", orgs.RemoveMemberH)
	mux.HandleFunc("POST /api/v1/sellers", sellers.CreateSellerH)
	mux.HandleFunc("GET /api/v1/sellers", sellers.GetSellerH)
	mux.HandleFunc("GET /api/v1/sellers/me", api.SellerContextH(api.SellerContextSources{
		Memberships: orgRepo, Orgs: orgRepo, Sellers: postgres.NewSellerRepository(db), Stores: postgres.NewStoreRepository(db),
	}))
	mux.HandleFunc("POST /api/v1/stores", stores.CreateStoreH)
	// Supplier self-registration and its review by staff (never verified automatically).
	api.NewSupplierOnboardingHandler(supplieronboarding.NewService(postgres.NewSupplierOnboardingStore(db), orgRepo, userRepo, auditLog), logger).Register(mux)
	// The CDF / USD rate: entered by an administrator, read by everyone (indicative conversions only).
	api.NewExchangeRateHandler(exchangerate.NewService(postgres.NewExchangeRateStore(db), userRepo, auditLog), logger).Register(mux)
	// What the Control Center creates, changes and removes: plans, subscriptions, credit grants, pricing rules,
	// categories, staff roles. Subscriptions get their included credits each month (idempotent hourly check).
	manage := api.NewAdminManageHandler(adminmanage.NewService(postgres.NewAdminManageStore(db), userRepo, auditLog), logger)
	manage.Register(mux)
	go func() {
		manage.RunRenewals(context.Background())
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			manage.RunRenewals(context.Background())
		}
	}()
	mux.HandleFunc("GET /api/v1/stores", stores.GetStoreH)
	// Administration console API: every route checks the caller's role itself.
	adminRepo := postgres.NewAdminRepository(db)
	admin := api.NewAdminHandler(adminRepo, userRepo).
		WithAudit(auditStore).
		WithCommerce(adminRepo).
		WithModeration(identity.NewModeration(userRepo, userRepo, auditLog))
	mux.HandleFunc("GET /api/v1/admin/overview", admin.OverviewH())
	mux.HandleFunc("GET /api/v1/admin/users", admin.UsersH())
	mux.HandleFunc("POST /api/v1/admin/users/{user_id}/suspend", admin.SuspendUserH())
	mux.HandleFunc("POST /api/v1/admin/users/{user_id}/reactivate", admin.ReactivateUserH())
	mux.HandleFunc("GET /api/v1/admin/audit", admin.AuditH())
	mux.HandleFunc("GET /api/v1/admin/orders", admin.OrdersH())
	mux.HandleFunc("GET /api/v1/admin/reservations", admin.ReservationsH())
	mux.HandleFunc("GET /api/v1/admin/organizations", admin.OrganizationsH())
	// Seller workspace read models (money, customers, fulfilment, availability, usage, integrations) and team.
	consoleRepo := postgres.NewConsoleRepository(db)
	console := api.NewConsoleHandler(consoleRepo, userRepo, orgRepo).WithTeam(team, orgUseCases)
	mux.HandleFunc("GET /api/v1/sellers/finance", console.FinanceH())
	// A person's own credits (personal space), distinct from the organisations' (GET /api/v1/sellers/usage).
	mux.Handle("/api/v1/me/credits", api.NewMeCreditsHandler(postgres.NewPersonalSpaceStore(db), consoleRepo, logger))
	mux.HandleFunc("GET /api/v1/sellers/customers", console.CustomersH())
	mux.HandleFunc("GET /api/v1/sellers/fulfilments", console.FulfilmentsH())
	mux.HandleFunc("GET /api/v1/sellers/capacity", console.CapacityH())
	mux.HandleFunc("GET /api/v1/sellers/usage", console.UsageH())
	mux.HandleFunc("GET /api/v1/sellers/integrations", console.IntegrationsH())
	mux.HandleFunc("GET /api/v1/sellers/team", console.TeamH())
	mux.HandleFunc("POST /api/v1/sellers/team/invitations", console.InviteH())
	mux.HandleFunc("DELETE /api/v1/sellers/team/invitations/{id}", console.RevokeInvitationH())
	mux.HandleFunc("PATCH /api/v1/sellers/team/members/{user_id}", console.ChangeRoleH())
	mux.HandleFunc("DELETE /api/v1/sellers/team/members/{user_id}", console.RemoveMemberH())
	// Administration modules: listings, payments, operations, providers, channels, security, AI, monetization, health.
	adminConsole := api.NewAdminConsoleHandler(consoleRepo, userRepo).
		WithAudit(auditStore, auditLog).
		WithHealth(db, api.ParseHealthTargets(common.Getenv("HEALTH_TARGETS", defaultHealthTargets)), configuredIntegrations())
	mux.HandleFunc("GET /api/v1/admin/listings", adminConsole.ListingsH())
	mux.HandleFunc("GET /api/v1/admin/payments", adminConsole.PaymentsH())
	mux.HandleFunc("GET /api/v1/admin/operations", adminConsole.OperationsH())
	mux.HandleFunc("GET /api/v1/admin/providers", adminConsole.ProvidersH())
	mux.HandleFunc("POST /api/v1/admin/providers/{id}/enable", adminConsole.EnableProviderH())
	mux.HandleFunc("POST /api/v1/admin/providers/{id}/disable", adminConsole.DisableProviderH())
	mux.HandleFunc("GET /api/v1/admin/channels", adminConsole.ChannelsH())
	mux.HandleFunc("GET /api/v1/admin/security", adminConsole.SecurityH())
	mux.HandleFunc("GET /api/v1/admin/ai", adminConsole.AIH())
	mux.HandleFunc("GET /api/v1/admin/monetization", adminConsole.MonetizationH())
	mux.HandleFunc("GET /api/v1/admin/health", adminConsole.HealthH())
	// Dashboards: figures computed from the platform's tables. Staff read the whole
	// platform; owners, admins and managers read their own business.
	analytics := api.NewAnalyticsHandler(postgres.NewAnalyticsRepository(db), userRepo, orgRepo)
	mux.HandleFunc("GET /api/v1/admin/analytics", analytics.AdminH())
	mux.HandleFunc("GET /api/v1/sellers/analytics", analytics.SellerH())
	// Managed content of the public site (trust strip, partner band, section order).
	// Reading is public; every change goes through an administrator check.
	site := api.NewSiteHandler(postgres.NewSiteContentRepository(db), userRepo, logger)
	mux.HandleFunc("GET /api/v1/site/content", site.PublicContentH())
	mux.HandleFunc("GET /api/v1/site/assets/{id}", site.AssetH())
	mux.HandleFunc("GET /api/v1/admin/site/content", site.AdminContentH())
	mux.HandleFunc("PUT /api/v1/admin/site/content", site.SaveContentH())
	mux.HandleFunc("POST /api/v1/admin/site/assets", site.UploadAssetH())
	mux.Handle("GET /metrics", observability.MetricsHandler())
	mux.HandleFunc("GET /healthz", common.HealthHandler)
	mux.HandleFunc("GET /readyz", common.ReadyHandler(db))

	authn := auth.NewAuthenticator(tokens)
	srv := common.NewServer(":"+common.Getenv("HTTP_PORT", "8086"), common.Observe("identity-service", logger, authn.Middleware(mux)))
	if err := common.Run(srv, logger, "identity-service"); err != nil {
		log.Fatalf("identity-service: %v", err)
	}
}

// defaultHealthTargets are the services the console's health report probes (the Docker deployment's names).
const defaultHealthTargets = "gateway=http://api-gateway:8080/healthz,search=http://search-service:8081/readyz," +
	"conversation=http://conversation-service:8091/readyz,commerce=http://order-service:8085/readyz,payment=http://payment-service:8083/readyz"

// configuredIntegrations says which external integrations have their settings present. Values are never read
// beyond "set or not"; the *_CONFIGURED flags are passed by the deployment so no secret reaches this service.
func configuredIntegrations() api.ConfiguredFlags {
	set := func(names ...string) bool {
		for _, n := range names {
			if strings.TrimSpace(os.Getenv(n)) == "" {
				return false
			}
		}
		return true
	}
	return api.ConfiguredFlags{
		"whatsapp_sender":       set("WHATSAPP_PHONE_NUMBER_ID", "WHATSAPP_ACCESS_TOKEN"),
		"whatsapp_otp_template": set("WHATSAPP_OTP_TEMPLATE"),
		"stripe":                os.Getenv("STRIPE_CONFIGURED") == "true",
		"ai_assistant":          os.Getenv("AI_CONFIGURED") == "true",
		"metrics_token":         os.Getenv("METRICS_CONFIGURED") == "true",
	}
}

func buildOTPSender() domain.OTPProvider {
	cfg := whatsapp.SenderConfig{
		BaseURL:         common.Getenv("WHATSAPP_API_BASE_URL", "https://graph.facebook.com/v20.0"),
		PhoneNumberID:   common.Getenv("WHATSAPP_PHONE_NUMBER_ID", ""),
		AccessToken:     common.Getenv("WHATSAPP_ACCESS_TOKEN", ""),
		OTPTemplate:     common.Getenv("WHATSAPP_OTP_TEMPLATE", ""),
		OTPTemplateLang: common.Getenv("WHATSAPP_OTP_TEMPLATE_LANG", "en"),
		Timeout:         10 * time.Second,
	}
	if cfg.Configured() {
		return whatsapp.NewSender(cfg)
	}
	if database.IsDevelopment() {
		return otp.DevLogSender{Logger: common.Logger("identity-service")}
	}
	return nil
}

// organizationStaffOnly restricts organization reads (contact details,
// member list) to that organization's members or platform admins/operators.
func organizationStaffOnly(orgs domain.OrganizationRepository, users domain.UserRepository) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.RequireUser(w, r)
			if !ok {
				return
			}
			allowed := false
			if u, err := users.FindByID(r.Context(), userID); err == nil && (u.HasRole(domain.RoleAdmin) || u.HasRole(domain.RoleOperator)) {
				allowed = true
			}
			if !allowed {
				if member, err := orgs.HasMember(r.Context(), r.URL.Query().Get("organization_id"), userID); err == nil && member {
					allowed = true
				}
			}
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":{"code":"FORBIDDEN","message":"Organization membership required"}}`))
				return
			}
			next(w, r)
		}
	}
}

// googleVerifier returns the Google ID token verifier, or nil (sign-in with Google off) without GOOGLE_CLIENT_ID.
func googleVerifier() identity.GoogleVerifier {
	clientID := strings.TrimSpace(common.Getenv("GOOGLE_CLIENT_ID", ""))
	if clientID == "" {
		return nil
	}
	return googleauth.NewVerifier(clientID)
}
