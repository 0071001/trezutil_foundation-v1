// Command conversation-service runs conversational commerce for every channel
// (/api/v1/conversations, WhatsApp webhooks at /api/v1/webhooks/whatsapp) and
// the vertical pack catalogue (/api/v1/packs).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application"
	aiapp "github.com/trezutil/trezutil/internal/application/ai"
	"github.com/trezutil/trezutil/internal/application/availability"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/application/hotel"
	"github.com/trezutil/trezutil/internal/application/memory"
	"github.com/trezutil/trezutil/internal/application/missionexec"
	"github.com/trezutil/trezutil/internal/application/missionorch"
	"github.com/trezutil/trezutil/internal/application/missionplanning"
	"github.com/trezutil/trezutil/internal/application/reservations"
	"github.com/trezutil/trezutil/internal/application/vertical"
	whatsappapp "github.com/trezutil/trezutil/internal/application/whatsapp"
	"github.com/trezutil/trezutil/internal/domain"
	anthropicai "github.com/trezutil/trezutil/internal/infrastructure/ai/anthropic"
	failoverai "github.com/trezutil/trezutil/internal/infrastructure/ai/failover"
	geminiai "github.com/trezutil/trezutil/internal/infrastructure/ai/gemini"
	"github.com/trezutil/trezutil/internal/infrastructure/audit"
	whatsappinfra "github.com/trezutil/trezutil/internal/infrastructure/whatsapp"
	searchapp "github.com/trezutil/trezutil/internal/search/application"
	"github.com/trezutil/trezutil/internal/search/infrastructure/opensearch"
	"github.com/trezutil/trezutil/packages/auth"
	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/database"
	"github.com/trezutil/trezutil/packages/observability"
)

func main() {
	logger := common.Logger("conversation-service")

	authn, err := auth.NewAuthenticatorFromEnv()
	if err != nil {
		log.Fatalf("conversation-service: %v", err)
	}

	db, err := database.Open(context.Background())
	if err != nil {
		log.Fatalf("conversation-service: %v", err)
	}
	defer db.Close()

	// Repositories
	listingRepo := postgres.NewListingRepository(db)
	categoryRepo := postgres.NewCategoryRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	storeRepo := postgres.NewStoreRepository(db)
	userRepo := postgres.NewUserRepository(db)
	inventoryRepo := postgres.NewInventoryRepository(db)
	capacityRepo := postgres.NewCapacityRepository(db)
	fulfillmentRepo := postgres.NewFulfillmentRepository(db)
	reservationRepo := postgres.NewReservationRepository(db)

	// Usage / credits (metering applies only to actions with an active usage policy).
	usageReservations := postgres.NewUsageReservationRepository(db)
	usageSvc := application.NewUsageService(
		postgres.NewPlanRepository(db),
		postgres.NewCreditAccountRepository(db),
		postgres.NewUsageLedgerRepository(db),
		postgres.NewUsagePolicyRepository(db),
		usageReservations,
		systemClock{},
	).WithPersonalSpaces(postgres.NewPersonalSpaceStore(db)) // a person's own credits live in their personal space
	go releaseOrphanedCreditReservations(logger, usageReservations)

	// Commerce use cases executed by the runtime. They are the same
	// application services the REST APIs use; the runtime adds identity
	// enforcement, validation, confirmation, idempotency and usage.
	auditLog := audit.NewLogger(logger).WithStore(postgres.NewAuditRepository(db))
	listingUseCases := application.NewListingUseCases(
		listingRepo, categoryRepo,
		postgres.NewAttributeDefinitionRepository(db), postgres.NewAttributeValueRepository(db),
		postgres.NewListingMediaRepository(db), postgres.NewListingAvailabilityRepository(db),
		postgres.NewLocationRepository(db), postgres.NewListingLocationRepository(db),
		orgRepo, sellerRepo, storeRepo, userRepo, auditLog,
	).WithVerticals(vertical.DefaultCatalog())
	orderUseCases := application.NewOrderUseCases(postgres.NewOrderRepository(db), listingRepo, sellerRepo, orgRepo, userRepo, inventoryRepo, fulfillmentRepo, auditLog)
	availabilityProvider := availability.NewCapacityAvailabilityProvider(inventoryRepo, capacityRepo).WithReservations(reservationRepo)
	availabilityUseCases := availability.NewAvailabilityUseCases(listingRepo, availabilityProvider, nil)
	reservationUseCases := reservations.NewReservationUseCases(
		reservationRepo, listingRepo, sellerRepo, userRepo, orgRepo,
		availabilityProvider, capacityRepo, inventoryRepo, fulfillmentRepo, auditLog,
		reservations.ConfirmationPolicy{AutoConfirm: true},
	).WithRules(hotel.NewService(hotel.Config{RoomTypes: postgres.NewHotelRoomTypeRepository(db)}))
	searchUseCases := buildSearch(logger)

	registry := commerce.NewHandlerRegistry()
	registry.Register(commerce.NewSearchHandler(searchUseCases))
	registry.Register(commerce.NewSuggestHandler(searchUseCases))
	registry.Register(commerce.NewGetListingHandler(listingUseCases))
	registry.Register(commerce.NewPublishListingHandler(listingUseCases))
	registry.Register(commerce.NewPauseListingHandler(listingUseCases))
	registry.Register(commerce.NewResumeListingHandler(listingUseCases))
	registry.Register(commerce.NewCreateOrderHandler(orderUseCases))
	registry.Register(commerce.NewGetOrderHandler(orderUseCases))
	registry.Register(commerce.NewCancelOrderHandler(orderUseCases))
	registry.Register(commerce.NewCreateReservationHandler(reservationUseCases))
	registry.Register(commerce.NewGetReservationHandler(reservationUseCases))
	registry.Register(commerce.NewCancelReservationHandler(reservationUseCases))
	registry.Register(commerce.NewCheckAvailabilityHandler(availabilityUseCases))
	// CREATE_LISTING needs a structured payload and is served by the listings
	// API; payment actions need a payment provider (see KNOWN_LIMITATIONS).

	runtime := commerce.NewCommerceActionRuntime(commerce.RuntimeConfig{
		Logger:             logger,
		HandlerRegistry:    registry,
		IdempotencyStore:   commerce.NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: commerce.NewDefaultConfirmationPolicy(),
		ParameterValidator: commerce.NewDefaultParameterValidator(),
		UsageService:       usageSvc,
	})

	// Vertical packs
	// One pack catalogue (application/vertical.PackDefinitions); the vertical
	// catalogue composes these packs.
	packRegistry := vertical.NewPackRegistry(vertical.PackRegistryConfig{Logger: logger})
	if err := vertical.LoadAllPacks(packRegistry); err != nil {
		logger.Warn().Err(err).Msg("Failed to register packs")
	}
	policyEngine := vertical.NewPolicyEngine(vertical.PolicyEngineConfig{Logger: logger})
	packResolver := vertical.NewPackResolver(vertical.PackResolverConfig{
		Logger:   logger,
		Registry: packRegistry,
		Policies: policyEngine,
		PackRepo: &allPacksEnabledRepo{},
	})

	// Commercial memory: kept per account on the server and read by the interpreter as hints (never instructions).
	memorySvc := memory.NewService(postgres.NewMemoryStore(db))
	// The same model providers interpret requests and word the answers (streamed); without AI both fall back.
	classifier, replyWriter := buildIntentClassifier(logger, usageSvc, categoryRepo, postgres.NewExchangeRateStore(db))
	processor := conversation.NewConversationProcessor(conversation.ProcessorConfig{
		Logger:            logger,
		ConversationStore: postgres.NewConversationStore(db),
		ContextStore:      postgres.NewConversationContextStore(db),
		MessageStore:      postgres.NewMessageStore(db),
		WebhookEventStore: postgres.NewWebhookEventStore(db),
		IntentClassifier:  classifier,
		ActionResolver:    conversation.NewActionResolver(conversation.NewDefaultIntentValidator()),
		Runtime:           runtime,
		PackResolver:      packResolver,
		Memory:            memorySvc,
		ReplyWriter:       replyWriter,
	})

	// Replies follow the locale sent with each message; this answers the channels
	// that carry none (WhatsApp). French is the platform's primary language.
	conversation.DefaultLanguage = common.Getenv("DEFAULT_LANGUAGE", "fr")

	convStore := postgres.NewConversationStore(db)
	convHandler := api.NewConversationHandler(conversation.NewProcessorService(processor), logger).
		WithHistory(conversation.NewHistoryService(convStore, convStore, postgres.NewMessageStore(db)).WithWriter(convStore))
	packHandler := api.NewVerticalPackHandler(api.NewVerticalPackService(packRegistry, packResolver, policyEngine))

	// WhatsApp channel
	waAdapter := whatsappinfra.NewAdapter(whatsappinfra.Config{
		VerifyToken:   common.Getenv("WHATSAPP_VERIFY_TOKEN", ""),
		WebhookSecret: common.Getenv("WHATSAPP_APP_SECRET", common.Getenv("WHATSAPP_WEBHOOK_SECRET", "")),
	})
	var waSender whatsappapp.OutboundSender
	senderCfg := whatsappinfra.SenderConfig{
		BaseURL:       common.Getenv("WHATSAPP_API_BASE_URL", "https://graph.facebook.com/v20.0"),
		PhoneNumberID: common.Getenv("WHATSAPP_PHONE_NUMBER_ID", ""),
		AccessToken:   common.Getenv("WHATSAPP_ACCESS_TOKEN", ""),
	}
	if senderCfg.Configured() {
		waSender = whatsappinfra.NewSender(senderCfg)
	} else {
		logger.Warn().Msg("WhatsApp outbound not configured: replies are processed but not delivered")
	}
	waBridge := whatsappapp.NewBridge(whatsappapp.BridgeConfig{
		Adapter:         waAdapter,
		ChannelAccounts: postgres.NewChannelAccountRepository(db),
		Users:           userRepo,
		Events:          postgres.NewWebhookEventStore(db),
		Processor:       processor,
		Sender:          waSender,
		Logger:          logger,
	})
	waHandler := api.NewWhatsAppHandler(waBridge, waAdapter)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/conversations/message", convHandler.HandleProcessMessage)
	memoryHandler := api.NewMemoryHandler(memorySvc, logger)
	mux.Handle("/api/v1/memory", memoryHandler)
	mux.Handle("/api/v1/memory/", memoryHandler)
	mux.HandleFunc("/api/v1/conversations", func(w http.ResponseWriter, r *http.Request) {
		api.HandleConversationRoutes(convHandler, w, r)
	})
	mux.HandleFunc("/api/v1/conversations/", func(w http.ResponseWriter, r *http.Request) {
		api.HandleConversationRoutes(convHandler, w, r)
	})
	packRoutes := requireOperatorForWrites(userRepo, func(w http.ResponseWriter, r *http.Request) {
		api.HandleVerticalPacksRoutes(packHandler, w, r)
	})
	// Organiser: a person's own missions (goals that may need several commerce actions). Recording a mission runs
	// no action and uses no credit.
	// Planning proposes steps from templates (no model) and only asks the runtime registry which actions it has: no
	// action is run here.
	missionRepo := postgres.NewMissionRepository(db)
	// Running ONE mission step is a request to the same runtime the conversation uses: it keeps its authorization,
	// credits and idempotency; the executor only translates the step and records the outcome.
	stepExecutor := missionexec.New(missionRepo, missionRepo, missionRepo, missionRepo, runtime, logger)
	orchestrator := missionorch.New(missionRepo, missionRepo, stepExecutor, logger)
	missions := api.NewMissionHandler(application.NewMissionUseCases(missionRepo).
		WithPlanning(missionplanning.NewRuleBased(registry.Has), missionRepo).
		WithStepStates(stepExecutor)).WithExecution(stepExecutor).WithOrchestration(orchestrator)
	mux.HandleFunc("POST /api/v1/missions", missions.CreateH)
	mux.HandleFunc("GET /api/v1/missions", missions.ListH)
	mux.HandleFunc("GET /api/v1/missions/{id}", missions.GetH)
	mux.HandleFunc("PATCH /api/v1/missions/{id}", missions.UpdateH)
	mux.HandleFunc("POST /api/v1/missions/{id}/steps", missions.AddStepH)
	mux.HandleFunc("GET /api/v1/missions/{id}/steps", missions.StepsH)
	mux.HandleFunc("POST /api/v1/missions/{id}/plan", missions.PlanH)
	mux.HandleFunc("GET /api/v1/missions/{id}/plan", missions.GetPlanH)
	mux.HandleFunc("POST /api/v1/missions/{id}/steps/{step_id}/execute", missions.ExecuteStepH)
	mux.HandleFunc("POST /api/v1/missions/{id}/run", missions.RunH)
	mux.HandleFunc("POST /api/v1/missions/{id}/resume", missions.ResumeH)
	mux.HandleFunc("GET /api/v1/missions/{id}/execution", missions.ExecutionH)
	mux.HandleFunc("/api/v1/packs", packRoutes)
	mux.HandleFunc("/api/v1/packs/", packRoutes)
	mux.HandleFunc("GET /api/v1/webhooks/whatsapp", api.VerifySubscriptionH(common.Getenv("WHATSAPP_VERIFY_TOKEN", "")))
	mux.HandleFunc("POST /api/v1/webhooks/whatsapp", waHandler.WebhookH)
	mux.Handle("GET /metrics", observability.MetricsHandler())
	mux.HandleFunc("GET /healthz", common.HealthHandler)
	mux.HandleFunc("GET /readyz", common.ReadyHandler(db))

	srv := common.NewServer(":"+common.Getenv("HTTP_PORT", common.Getenv("PORT", "8091")), common.Observe("conversation-service", logger, authn.Middleware(mux)))
	if err := common.Run(srv, logger, "conversation-service"); err != nil {
		log.Fatalf("conversation-service: %v", err)
	}
}

func buildSearch(logger zerolog.Logger) commerce.SearchUseCases {
	adapter, err := opensearch.NewOpenSearchAdapter(opensearch.Config{
		URL:          common.Getenv("OPENSEARCH_URL", "http://localhost:9200"),
		Username:     common.Getenv("OPENSEARCH_USERNAME", ""),
		Password:     common.Getenv("OPENSEARCH_PASSWORD", ""),
		Index:        common.Getenv("OPENSEARCH_INDEX", "trezutil-listings-v1"),
		Alias:        common.Getenv("OPENSEARCH_ALIAS", "trezutil-listings"),
		Timeout:      10 * time.Second,
		TLS:          common.Getenv("OPENSEARCH_TLS", "false") == "true",
		MaxRetries:   common.GetenvInt("OPENSEARCH_MAX_RETRIES", 3),
		ReindexBatch: 1000,
	})
	if err != nil {
		log.Fatalf("conversation-service: OpenSearch adapter: %v", err)
	}
	return searchapp.NewSearchService(adapter,
		searchapp.NewNoopVisibilityResolver(), searchapp.NewAllowAllAuthorizationResolver(),
		nil, searchapp.NewNoopCursorCodec(), searchapp.NewSystemClock())
}

// buildIntentClassifier wires the configured model provider(s) behind the AI
// gateway (router, capability checks, allowlist, confidence threshold, usage
// metering). AI_PROVIDER is one provider ("anthropic", "gemini") or a
// comma-separated priority list ("gemini,anthropic"): a call goes to the first
// provider and, if it fails (no credit, rate limit, outage), to the next.
// Without AI_PROVIDER, or when no provider is configured, it returns the
// deterministic keyword classifier; the processor also falls back to keywords
// whenever the model is unsure or unavailable.
func buildIntentClassifier(logger zerolog.Logger, usage domain.UsageService, categories domain.CategoryRepository, rates domain.ExchangeRateStore) (conversation.IntentClassifier, conversation.ReplyWriter) {
	names := splitProviders(common.Getenv("AI_PROVIDER", ""))
	if len(names) == 0 {
		logger.Info().Msg("AI_PROVIDER not set; using keyword classification")
		return &conversation.KeywordClassifier{}, nil
	}
	var providers []domain.AIProvider
	for _, name := range names {
		if p := buildProvider(logger, name, len(names) == 1); p != nil {
			providers = append(providers, p)
		}
	}
	if len(providers) == 0 {
		logger.Warn().Msg("no AI provider is usable; using keyword classification")
		return &conversation.KeywordClassifier{}, nil
	}
	provider, err := failoverai.New(logger, providers...)
	if err != nil {
		logger.Warn().Err(err).Msg("AI providers not composed; using keyword classification")
		return &conversation.KeywordClassifier{}, nil
	}

	router := aiapp.NewModelRouter(logger)
	router.RegisterProvider(provider)
	gateway := aiapp.NewAIGateway(router, aiapp.AIGatewayConfig{
		Logger:  logger,
		Timeout: time.Duration(common.GetenvInt("AI_TIMEOUT_SECONDS", 30)) * time.Second,
	})
	orchestrator := aiapp.NewAIOrchestrator(aiapp.AIOrchestratorConfig{
		Logger:       logger,
		Router:       router,
		Gateway:      gateway,
		UsageService: usage,
	})
	order := make([]string, 0, len(providers))
	for _, p := range providers {
		order = append(order, p.ID()+"/"+p.Models()[0].ModelID)
	}
	logger.Info().Strs("ai_providers", order).Msg("AI intent classification enabled")
	// The interpreter is told which categories exist, so it can name the one a person means.
	lister := func(ctx context.Context) ([]aiapp.CategoryOption, error) {
		active, err := categories.FindActive(ctx)
		if err != nil {
			return nil, err
		}
		names := make(map[string]string, len(active))
		for _, c := range active {
			names[c.ID] = c.Name
		}
		out := make([]aiapp.CategoryOption, 0, len(active))
		for _, c := range active {
			o := aiapp.CategoryOption{ID: c.ID, Name: c.Name}
			if c.ParentID != nil {
				o.Parent = names[*c.ParentID]
			}
			out = append(out, o)
		}
		return out, nil
	}
	// It reads a budget in francs at the rate an administrator entered (none: it does not convert).
	rate := func(ctx context.Context) (float64, bool, error) {
		r, err := rates.Latest(ctx)
		if errors.Is(err, domain.ErrNoExchangeRate) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return r.Rate, true, nil
	}
	return aiapp.NewIntentClassifierAdapter(orchestrator, aiapp.WithCategories(lister), aiapp.WithExchangeRate(rate)), aiapp.NewReplyWriter(gateway, usage, logger)
}

func splitProviders(list string) []string {
	var out []string
	seen := map[string]bool{}
	for _, n := range strings.Split(list, ",") {
		n = strings.ToLower(strings.TrimSpace(n))
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// buildProvider builds and verifies one provider. A provider whose model or key
// the API definitely rejects is fatal when it is the only one configured (a
// deployment mistake that must not degrade silently) and is skipped, with an
// error log, when a fallback exists. It returns nil for a provider that is not
// configured.
func buildProvider(logger zerolog.Logger, name string, only bool) domain.AIProvider {
	reject := func(err error, msg string) {
		if only {
			logger.Fatal().Err(err).Msg(msg)
		}
		logger.Error().Err(err).Str("ai_provider", name).Msg(msg + "; skipping this provider")
	}
	verify := func(fn func(context.Context) error) error {
		if common.Getenv("AI_VERIFY_ON_START", "true") == "false" {
			return nil
		}
		vctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return fn(vctx)
	}
	timeout := time.Duration(common.GetenvInt("AI_REQUEST_TIMEOUT_SECONDS", 15)) * time.Second

	switch name {
	case anthropicai.ProviderID:
		p, err := anthropicai.New(anthropicai.Config{
			APIKey:         common.Getenv("ANTHROPIC_API_KEY", ""),
			Model:          common.Getenv("AI_MODEL", anthropicai.DefaultModel),
			BaseURL:        common.Getenv("AI_BASE_URL", ""),
			Effort:         common.Getenv("AI_EFFORT", "low"),
			RequestTimeout: timeout,
			MaxRetries:     common.GetenvInt("AI_MAX_RETRIES", 2),
		})
		if errors.Is(err, anthropicai.ErrInvalidModel) {
			reject(err, "invalid AI_MODEL")
			return nil
		}
		if err != nil {
			logger.Warn().Err(err).Str("ai_provider", name).Msg("AI provider not configured")
			return nil
		}
		switch verr := verify(p.Verify); {
		case errors.Is(verr, anthropicai.ErrInvalidModel), errors.Is(verr, anthropicai.ErrKeyRejected):
			reject(verr, "AI provider configuration rejected by the API")
			return nil
		case verr != nil:
			logger.Warn().Err(verr).Str("ai_provider", name).Msg("AI provider could not be verified at startup; continuing (fallbacks cover outages)")
		}
		return p
	case geminiai.ProviderID:
		p, err := geminiai.New(geminiai.Config{
			APIKey:         common.Getenv("GEMINI_API_KEY", ""),
			Model:          common.Getenv("GEMINI_MODEL", geminiai.DefaultModel),
			BaseURL:        common.Getenv("GEMINI_BASE_URL", ""),
			RequestTimeout: timeout,
		})
		if errors.Is(err, geminiai.ErrInvalidModel) {
			reject(err, "invalid GEMINI_MODEL")
			return nil
		}
		if err != nil {
			logger.Warn().Err(err).Str("ai_provider", name).Msg("AI provider not configured")
			return nil
		}
		switch verr := verify(p.Verify); {
		case errors.Is(verr, geminiai.ErrInvalidModel), errors.Is(verr, geminiai.ErrKeyRejected):
			reject(verr, "AI provider configuration rejected by the API")
			return nil
		case verr != nil:
			logger.Warn().Err(verr).Str("ai_provider", name).Msg("AI provider could not be verified at startup; continuing (fallbacks cover outages)")
		}
		return p
	}
	logger.Warn().Str("ai_provider", name).Msg("unknown AI provider; ignoring it")
	return nil
}

// requireOperatorForWrites lets anyone read the pack catalogue but restricts
// pack activation changes to platform admins/operators.
func requireOperatorForWrites(users domain.UserRepository, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next(w, r)
			return
		}
		userID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		user, err := users.FindByID(r.Context(), userID)
		if err != nil || !(user.HasRole(domain.RoleAdmin) || user.HasRole(domain.RoleOperator)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":"FORBIDDEN","message":"Only platform operators can change pack activation"}}`))
			return
		}
		next(w, r)
	}
}

// allPacksEnabledRepo reports every pack as enabled for every organization.
// Per-organization pack activation is not persisted in the MVP (see
// KNOWN_LIMITATIONS); pack mutations are operator-only and have no effect.
type allPacksEnabledRepo struct{}

func (m *allPacksEnabledRepo) Create(ctx interface{}, orgPack *domain.OrganizationPack) error {
	return nil
}
func (m *allPacksEnabledRepo) FindByOrgAndPack(ctx interface{}, orgID string, code domain.PackCode) (*domain.OrganizationPack, error) {
	return &domain.OrganizationPack{OrganizationID: orgID, PackCode: code, Enabled: true, ActivatedAt: time.Now().Unix()}, nil
}
func (m *allPacksEnabledRepo) ListByOrg(ctx interface{}, orgID string) ([]domain.OrganizationPack, error) {
	return nil, nil
}
func (m *allPacksEnabledRepo) Enable(ctx interface{}, orgID string, packCode domain.PackCode) error {
	return nil
}
func (m *allPacksEnabledRepo) Disable(ctx interface{}, orgID string, packCode domain.PackCode) error {
	return nil
}
func (m *allPacksEnabledRepo) IsPackEnabled(ctx interface{}, orgID string, packCode domain.PackCode) (bool, error) {
	return true, nil
}

// releaseOrphanedCreditReservations periodically returns credits held by
// reservations whose action never completed (process crash mid-request).
func releaseOrphanedCreditReservations(logger zerolog.Logger, repo *postgres.PostgresUsageReservationRepository) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		n, err := repo.ReleaseStaleReservations(ctx, 15*time.Minute, time.Now().Unix())
		cancel()
		if err != nil {
			logger.Warn().Err(err).Msg("stale credit reservation sweep failed")
		} else if n > 0 {
			logger.Info().Int("released", n).Msg("released orphaned credit reservations")
		}
	}
}

type systemClock struct{}

func (systemClock) Now() int64 {
	return time.Now().Unix()
}
