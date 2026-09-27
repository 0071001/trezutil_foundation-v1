package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application/payment"
	"github.com/trezutil/trezutil/internal/application/provider"
	payverifier "github.com/trezutil/trezutil/internal/infrastructure/payment"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://trezutil:trezutil@localhost:5432/trezutil?sslmode=disable"
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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	paymentRepo := postgres.NewPaymentRepository(db)
	refundRepo := postgres.NewRefundRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	webhookEventStore := postgres.NewPaymentWebhookEventStore(db)
	providerRepo := postgres.NewProviderRepository(db)
	executionRepo := postgres.NewProviderExecutionRepository(db)
	orderRepo := postgres.NewOrderRepository(db)

	providerRegistry := provider.NewProviderRegistry()
	orchestrator := provider.NewProviderOrchestrator(providerRegistry, executionRepo, provider.OrchestratorConfig{
		Logger: logger,
	})

	paymentSvc := payment.NewPaymentService(payment.PaymentServiceConfig{
		Payments:     paymentRepo,
		Refunds:      refundRepo,
		Ledger:       ledgerRepo,
		Orders:       orderRepo,
		Orchestrator: orchestrator,
		Logger:       logger,
	})

	webhookSecrets := map[string]string{
		"stripe": os.Getenv("STRIPE_WEBHOOK_SECRET"),
		"paypal": os.Getenv("PAYPAL_WEBHOOK_SECRET"),
	}
	for k, v := range webhookSecrets {
		if v == "" {
			delete(webhookSecrets, k)
		}
	}
	signatureVerifier := payverifier.NewWebhookSignatureVerifier(payverifier.WebhookVerifierConfig{
		ProviderSecrets: webhookSecrets,
	})

	webhookHandler := payment.NewWebhookHandler(payment.WebhookHandlerConfig{
		Payments:          paymentRepo,
		Refunds:           refundRepo,
		Ledger:            ledgerRepo,
		Events:            webhookEventStore,
		Providers:         providerRepo,
		SignatureVerifier: signatureVerifier,
		Logger:            logger,
	})

	paymentHandler := api.NewPaymentHandler(paymentSvc, webhookHandler)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/payments", paymentHandler.CreatePaymentH)
	mux.HandleFunc("GET /api/v1/payments/{payment_id}", paymentHandler.GetPaymentH)
	mux.HandleFunc("POST /api/v1/payments/{payment_id}/cancel", paymentHandler.CancelPaymentH)
	mux.HandleFunc("POST /api/v1/payments/{payment_id}/capture", paymentHandler.CapturePaymentH)
	mux.HandleFunc("POST /api/v1/payments/{payment_id}/refund", paymentHandler.RefundPaymentH)
	mux.HandleFunc("GET /api/v1/payments/{payment_id}/status", paymentHandler.GetPaymentStatusH)
	mux.HandleFunc("GET /api/v1/payments/{payment_id}/refunds/{refund_id}", paymentHandler.GetRefundH)
	mux.HandleFunc("POST /api/v1/webhooks/payments/{provider_code}", paymentHandler.WebhookPaymentH)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8083"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info().Str("port", port).Msg("Payment service starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down payment service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}

	logger.Info().Msg("Payment service stopped")
}
