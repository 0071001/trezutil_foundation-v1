// Command payment-service serves the Payment Core (/api/v1/payments) and
// provider webhooks (/api/v1/webhooks/payments/{provider_code}).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application/payment"
	"github.com/trezutil/trezutil/internal/application/provider"
	payverifier "github.com/trezutil/trezutil/internal/infrastructure/payment"
	"github.com/trezutil/trezutil/internal/infrastructure/payment/saspay"
	"github.com/trezutil/trezutil/internal/infrastructure/payment/stripe"
	"github.com/trezutil/trezutil/packages/auth"
	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/database"
	"github.com/trezutil/trezutil/packages/observability"
)

func main() {
	logger := common.Logger("payment-service")

	authn, err := auth.NewAuthenticatorFromEnv()
	if err != nil {
		log.Fatalf("payment-service: %v", err)
	}

	db, err := database.Open(context.Background())
	if err != nil {
		log.Fatalf("payment-service: %v", err)
	}
	defer db.Close()

	paymentRepo := postgres.NewPaymentRepository(db)
	refundRepo := postgres.NewRefundRepository(db)
	// Captures and refunds are written together with their ledger entries, in one transaction.
	accounting := postgres.NewPaymentAccounting(db)
	locker := postgres.NewAdvisoryLocker(db)
	webhookEventStore := postgres.NewPaymentWebhookEventStore(db)
	providerRepo := postgres.NewProviderRepository(db)
	executionRepo := postgres.NewProviderExecutionRepository(db)
	orderRepo := postgres.NewOrderRepository(db)

	// Payment provider adapters are registered from each organization's
	// providers rows (code 'stripe'); API keys are resolved from environment
	// references, never stored in the database.
	providerRegistry := provider.NewProviderRegistry()
	stripeLoader := &stripe.Loader{Providers: providerRepo, Registry: providerRegistry, Logger: logger}
	if n, err := stripeLoader.Load(context.Background()); err != nil {
		logger.Error().Err(err).Msg("failed to load Stripe providers")
	} else if n == 0 {
		logger.Warn().Msg("no payment provider configured: payment creation returns PAYMENT_PROVIDER_UNAVAILABLE until a 'stripe' provider row and its secret are configured")
	} else {
		logger.Info().Int("providers", n).Msg("Stripe payment providers registered")
	}
	// Trezutil's own SasPay account (mobile money in DR Congo, cards in USD): a providers row with scope "platform"
	// serves every organisation without a payment provider of its own.
	saspayLoader := &saspay.Loader{Providers: providerRepo, Registry: providerRegistry, Logger: logger}
	if n, err := saspayLoader.Load(context.Background()); err != nil {
		logger.Error().Err(err).Msg("failed to load SasPay providers")
	} else if n > 0 {
		logger.Info().Int("providers", n).Msg("SasPay payment providers registered")
	}
	// New or changed provider rows are picked up without a restart.
	refresh := time.Duration(common.GetenvInt("PAYMENT_PROVIDER_REFRESH_SECONDS", 60)) * time.Second
	go stripeLoader.Run(context.Background(), refresh)
	go saspayLoader.Run(context.Background(), refresh)

	orchestrator := provider.NewProviderOrchestrator(providerRegistry, executionRepo, provider.OrchestratorConfig{
		Logger: logger,
	})

	paymentSvc := payment.NewPaymentService(payment.PaymentServiceConfig{
		Payments:     paymentRepo,
		Refunds:      refundRepo,
		Accounting:   accounting,
		Orders:       orderRepo,
		Orchestrator: orchestrator,
		Logger:       logger,
		Locker:       locker,
	})

	// Reads recent unanswered payments back from their provider and logs records that disagree. It moves no money and
	// writes no correction on its own (see trezutil-admin repair-ledger). 0 turns it off.
	reconciler := &payment.Reconciler{
		Source:     postgres.NewPaymentReconciliation(db),
		Payments:   paymentRepo,
		Refunds:    refundRepo,
		Accounting: accounting,
		Locker:     locker,
		Audit:      postgres.NewAuditRepository(db),
		Logger:     logger,
	}
	go reconciler.Run(context.Background(), paymentSvc, time.Duration(common.GetenvInt("PAYMENT_RECONCILE_SECONDS", 900))*time.Second)

	webhookSecrets := map[string]string{}
	for code, env := range map[string]string{"stripe": "STRIPE_WEBHOOK_SECRET", "paypal": "PAYPAL_WEBHOOK_SECRET"} {
		if v := os.Getenv(env); v != "" {
			webhookSecrets[code] = v
		}
	}
	signatureVerifier := payverifier.NewWebhookSignatureVerifier(payverifier.WebhookVerifierConfig{
		ProviderSecrets: webhookSecrets,
	})

	webhookHandler := payment.NewWebhookHandler(payment.WebhookHandlerConfig{
		Payments:          paymentRepo,
		Refunds:           refundRepo,
		Accounting:        accounting,
		Locker:            locker,
		Events:            webhookEventStore,
		Providers:         providerRepo,
		SignatureVerifier: signatureVerifier,
		Orders:            orderRepo,
		Normalizers: map[string]payment.WebhookNormalizer{
			stripe.ProviderCode: &stripe.WebhookNormalizer{SecretFor: stripeLoader.WebhookSecret},
			saspay.ProviderCode: &saspay.WebhookNormalizer{SecretFor: saspayLoader.WebhookSecret},
		},
		Logger: logger,
	})

	access := payment.NewAccessPolicy(orderRepo, paymentRepo, postgres.NewOrganizationRepository(db), postgres.NewUserRepository(db))
	paymentHandler := api.NewPaymentHandler(paymentSvc, webhookHandler).WithAccessPolicy(access)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/payments", paymentHandler.CreatePaymentH)
	mux.HandleFunc("GET /api/v1/payments/{payment_id}", paymentHandler.GetPaymentH)
	mux.HandleFunc("POST /api/v1/payments/{payment_id}/cancel", paymentHandler.CancelPaymentH)
	mux.HandleFunc("POST /api/v1/payments/{payment_id}/capture", paymentHandler.CapturePaymentH)
	mux.HandleFunc("POST /api/v1/payments/{payment_id}/refund", paymentHandler.RefundPaymentH)
	mux.HandleFunc("GET /api/v1/payments/{payment_id}/status", paymentHandler.GetPaymentStatusH)
	mux.HandleFunc("GET /api/v1/payments/{payment_id}/refunds/{refund_id}", paymentHandler.GetRefundH)
	// Webhooks authenticate by provider signature, not by user token.
	mux.HandleFunc("POST /api/v1/webhooks/payments/{provider_code}", paymentHandler.WebhookPaymentH)
	mux.Handle("GET /metrics", observability.MetricsHandler())
	mux.HandleFunc("GET /healthz", common.HealthHandler)
	mux.HandleFunc("GET /readyz", common.ReadyHandler(db))

	srv := common.NewServer(":"+common.Getenv("HTTP_PORT", "8083"), common.Observe("payment-service", logger, authn.Middleware(mux)))
	if err := common.Run(srv, logger, "payment-service"); err != nil {
		log.Fatalf("payment-service: %v", err)
	}
}
