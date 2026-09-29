// Command identity-service serves phone-OTP sign-in and JWT sessions
// (/api/v1/auth/*).
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/otp"
	"github.com/trezutil/trezutil/internal/infrastructure/whatsapp"
	"github.com/trezutil/trezutil/packages/auth"
	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/database"
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

	svc, err := identity.NewService(identity.Config{
		Users:          postgres.NewUserRepository(db),
		Challenges:     postgres.NewOTPChallengeRepository(db),
		Sender:         sender,
		Tokens:         tokens,
		CodeSecret:     codeSecret,
		AccessTokenTTL: jwtCfg.AccessTokenTTL,
		MaxPerHour:     common.GetenvInt("OTP_MAX_PER_PHONE_HOUR", identity.DefaultMaxPerPhoneHour),
		Logger:         logger,
	})
	if err != nil {
		log.Fatalf("identity-service: %v", err)
	}

	h := api.NewIdentityHandlerWithLimit(svc, common.GetenvInt("OTP_MAX_PER_IP_10MIN", 20))

	// Seller onboarding (operator-assisted): platform operators create the
	// organization and add the seller's account as owner; the owner then
	// creates the seller profile and stores. Authorization lives in the use cases.
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	orgs := api.NewOrganizationHandler(application.NewOrganizationUseCases(orgRepo, userRepo, nil))
	sellers := api.NewSellerHandler(application.NewSellerUseCases(postgres.NewSellerRepository(db), orgRepo, userRepo, nil))
	stores := api.NewStoreHandler(application.NewStoreUseCases(postgres.NewStoreRepository(db), orgRepo, userRepo, nil))
	orgStaff := organizationStaffOnly(orgRepo, userRepo)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/request-otp", h.RequestOTPH)
	mux.HandleFunc("POST /api/v1/auth/verify-otp", h.VerifyOTPH)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.RefreshH)
	mux.HandleFunc("GET /api/v1/auth/users/me", h.MeH)
	mux.HandleFunc("POST /api/v1/organizations", orgs.CreateOrganizationH)
	mux.HandleFunc("GET /api/v1/organizations", orgStaff(orgs.GetOrganizationH))
	mux.HandleFunc("GET /api/v1/organizations/members", orgStaff(orgs.GetMembersH))
	mux.HandleFunc("POST /api/v1/organizations/members", orgs.AddMemberH)
	mux.HandleFunc("DELETE /api/v1/organizations/members", orgs.RemoveMemberH)
	mux.HandleFunc("POST /api/v1/sellers", sellers.CreateSellerH)
	mux.HandleFunc("GET /api/v1/sellers", sellers.GetSellerH)
	mux.HandleFunc("GET /api/v1/sellers/me", api.SellerContextH(api.SellerContextSources{
		Memberships: orgRepo, Orgs: orgRepo, Sellers: postgres.NewSellerRepository(db), Stores: postgres.NewStoreRepository(db),
	}))
	mux.HandleFunc("POST /api/v1/stores", stores.CreateStoreH)
	mux.HandleFunc("GET /api/v1/stores", stores.GetStoreH)
	mux.HandleFunc("GET /healthz", common.HealthHandler)
	mux.HandleFunc("GET /readyz", common.ReadyHandler(db))

	authn := auth.NewAuthenticator(tokens)
	srv := common.NewServer(":"+common.Getenv("HTTP_PORT", "8086"), common.AccessLog(logger, authn.Middleware(mux)))
	if err := common.Run(srv, logger, "identity-service"); err != nil {
		log.Fatalf("identity-service: %v", err)
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
