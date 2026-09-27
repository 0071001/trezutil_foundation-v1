package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/ai"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres:postgres@localhost:5432/trezutil?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	logger.Info().Msg("Connected to database")

	// Initialize stores
	convStore := postgres.NewConversationStore(db)
	contextStore := postgres.NewConversationContextStore(db)
	messageStore := postgres.NewMessageStore(db)
	webhookStore := postgres.NewWebhookEventStore(db)

	// Initialize usage repositories
	creditAccountRepo := postgres.NewCreditAccountRepository(db)
	usagePolicyRepo := postgres.NewUsagePolicyRepository(db)
	usageLedgerRepo := postgres.NewUsageLedgerRepository(db)
	usageReservationRepo := postgres.NewUsageReservationRepository(db)

	// Initialize UsageService
	usageSvc := application.NewUsageService(
		nil, // planRepo - nil for now
		creditAccountRepo,
		usageLedgerRepo,
		usagePolicyRepo,
		usageReservationRepo,
		systemClock{},
	)

	// Initialize action resolver
	actionResolver := conversation.NewActionResolver(conversation.NewDefaultIntentValidator())

	// Initialize vertical pack infrastructure
	packRegistry := vertical.NewPackRegistry(vertical.PackRegistryConfig{Logger: logger})
	for _, pack := range defaultVerticalPacks() {
		if err := packRegistry.Register(pack); err != nil {
			logger.Warn().Err(err).Msg("Failed to register pack")
		}
	}
	policyEngine := vertical.NewPolicyEngine(vertical.PolicyEngineConfig{Logger: logger})
	packResolver := vertical.NewPackResolver(vertical.PackResolverConfig{
		Logger:   logger,
		Registry: packRegistry,
		Policies: policyEngine,
		PackRepo: &noopOrgPackRepo{},
	})

	// Initialize AI classifier (optional)
	classifier := buildIntentClassifier(logger, actionResolver)

	// Initialize commerce runtime
	runtime := buildCommerceRuntime(logger, packRegistry, usageSvc)

	// Initialize conversation processor
	processor := conversation.NewConversationProcessor(conversation.ProcessorConfig{
		Logger:            logger,
		ConversationStore: convStore,
		ContextStore:      contextStore,
		MessageStore:      messageStore,
		WebhookEventStore: webhookStore,
		IntentClassifier:  classifier,
		ActionResolver:    actionResolver,
		Runtime:           runtime,
		PackResolver:      packResolver,
	})

	// Wrap processor in service for HTTP layer
	svc := conversation.NewProcessorService(processor)

	// Wire HTTP handlers
	convHandler := api.NewConversationHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/conversations/message", convHandler.HandleProcessMessage)
	mux.HandleFunc("/api/v1/conversations/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/confirm") && r.Method == http.MethodPost:
			convHandler.HandleConfirmAction(w, r)
		case strings.HasSuffix(path, "/reject") && r.Method == http.MethodPost:
			convHandler.HandleRejectAction(w, r)
		case strings.HasSuffix(path, "/token") && r.Method == http.MethodPost:
			convHandler.HandleGenerateToken(w, r)
		default:
			api.HandleConversationRoutes(convHandler, w, r)
		}
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8091"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      loggingMiddleware(logger)(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info().Str("port", port).Msg("Conversation service starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down conversation service...")
	if err := server.Close(); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}
}

func buildIntentClassifier(logger zerolog.Logger, resolver *conversation.ActionResolver) conversation.IntentClassifier {
	aiProvider := os.Getenv("AI_PROVIDER")
	if aiProvider == "" {
		logger.Info().Msg("AI provider not configured, using keyword-based classification")
		return &conversation.KeywordClassifier{}
	}

	gateway := ai.NewAIGateway(
		ai.NewModelRouter(logger),
		ai.AIGatewayConfig{Logger: logger, Timeout: 30 * time.Second},
	)
	orch := ai.NewAIOrchestrator(ai.AIOrchestratorConfig{
		Logger:       logger,
		Router:       nil,
		Gateway:      gateway,
		Resolver:     resolver,
		UsageService: nil,
	})
	return ai.NewIntentClassifierAdapter(orch)
}

func buildCommerceRuntime(logger zerolog.Logger, packRegistry *vertical.PackRegistry, usageSvc domain.UsageService) *commerce.CommerceActionRuntime {
	handlerRegistry := commerce.NewHandlerRegistry()
	runtime := commerce.NewCommerceActionRuntime(commerce.RuntimeConfig{
		Logger:             logger,
		HandlerRegistry:    handlerRegistry,
		IdempotencyStore:   nil,
		ConfirmationPolicy: nil,
		ParameterValidator: nil,
		UsageService:       usageSvc,
	})
	_ = packRegistry
	return runtime
}

func defaultVerticalPacks() []*domain.VerticalPack {
	eCommerce := domain.NewVerticalPack(
		domain.PackECommerce,
		"E-Commerce Pack",
		[]domain.PackCapability{
			domain.CapabilitySearch, domain.CapabilityOrdering,
			domain.CapabilityInventory, domain.CapabilityFulfillment,
			domain.CapabilityCustomerSupport, domain.CapabilityMerchantOps,
		},
		domain.AISpecialistCommerce,
	)
	eCommerce.SupportedActions = []domain.CommerceActionType{
		domain.ActionSearchListings, domain.ActionGetListing,
		domain.ActionCompareListings, domain.ActionSuggest,
		domain.ActionCreateOrder, domain.ActionGetOrder,
		domain.ActionCancelOrder, domain.ActionInitiatePayment,
		domain.ActionGetPaymentStatus, domain.ActionGetDeliveryStatus,
		domain.ActionGetFulfillmentStatus,
	}

	booking := domain.NewVerticalPack(
		domain.PackBooking,
		"Booking & Hospitality Pack",
		[]domain.PackCapability{
			domain.CapabilitySearch, domain.CapabilityAvailability,
			domain.CapabilityReservation, domain.CapabilityCapacity,
			domain.CapabilityScheduling, domain.CapabilityCustomerSupport,
		},
		domain.AISpecialistBooking,
	)
	booking.SupportedActions = []domain.CommerceActionType{
		domain.ActionSearchListings, domain.ActionGetListing,
		domain.ActionSuggest, domain.ActionCheckAvailability,
		domain.ActionCreateReservation, domain.ActionGetReservation,
		domain.ActionCancelReservation, domain.ActionModifyReservation,
	}

	return []*domain.VerticalPack{eCommerce, booking}
}

type noopOrgPackRepo struct{}

func (m *noopOrgPackRepo) Create(ctx interface{}, orgPack *domain.OrganizationPack) error {
	return nil
}
func (m *noopOrgPackRepo) FindByOrgAndPack(ctx interface{}, orgID string, code domain.PackCode) (*domain.OrganizationPack, error) {
	return &domain.OrganizationPack{OrganizationID: orgID, PackCode: code, Enabled: true, ActivatedAt: time.Now().Unix()}, nil
}
func (m *noopOrgPackRepo) ListByOrg(ctx interface{}, orgID string) ([]domain.OrganizationPack, error) {
	return nil, nil
}
func (m *noopOrgPackRepo) Enable(ctx interface{}, orgID string, packCode domain.PackCode) error {
	return nil
}
func (m *noopOrgPackRepo) Disable(ctx interface{}, orgID string, packCode domain.PackCode) error {
	return nil
}
func (m *noopOrgPackRepo) IsPackEnabled(ctx interface{}, orgID string, packCode domain.PackCode) (bool, error) {
	return true, nil
}

func loggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r)
			logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.statusCode).
				Dur("duration", time.Since(start)).
				Msg("Request processed")
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

type systemClock struct{}

func (systemClock) Now() int64 {
	return time.Now().Unix()
}
