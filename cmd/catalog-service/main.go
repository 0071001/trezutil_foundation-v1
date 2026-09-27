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

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/config"
)

func main() {
	// Load configuration
	cfg := &config.Config{
		AppEnv:   getEnv("APP_ENV", "development"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		HTTPPort: getEnv("HTTP_PORT", "8082"),
	}

	// Initialize database connection
	db, err := sql.Open("postgres", getEnv("DATABASE_URL", "postgres://trezutil:trezutil@localhost:5432/trezutil?sslmode=disable"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Initialize repositories
	categoryRepo := postgres.NewCategoryRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	attributeDefRepo := postgres.NewAttributeDefinitionRepository(db)
	attributeValRepo := postgres.NewAttributeValueRepository(db)
	mediaRepo := postgres.NewListingMediaRepository(db)
	availabilityRepo := postgres.NewListingAvailabilityRepository(db)
	locationRepo := postgres.NewLocationRepository(db)
	listingLocationRepo := postgres.NewListingLocationRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	storeRepo := postgres.NewStoreRepository(db)
	userRepo := postgres.NewUserRepository(db)

	// Initialize use cases
	listingUseCases := application.NewListingUseCases(
		listingRepo,
		categoryRepo,
		attributeDefRepo,
		attributeValRepo,
		mediaRepo,
		availabilityRepo,
		locationRepo,
		listingLocationRepo,
		orgRepo,
		sellerRepo,
		storeRepo,
		userRepo,
		nil, // auditLogger - can be initialized later
	)

	// Initialize handlers
	listingHandler := api.NewListingHandler(listingUseCases)

	// Setup routes
	mux := http.NewServeMux()

	// Listing routes
	mux.HandleFunc("POST /v1/listings", listingHandler.CreateListingH)
	mux.HandleFunc("GET /v1/listings", listingHandler.GetListingH)
	mux.HandleFunc("POST /v1/listings/publish", listingHandler.PublishListingH)
	mux.HandleFunc("POST /v1/listings/pause", listingHandler.PauseListingH)
	mux.HandleFunc("POST /v1/listings/resume", listingHandler.ResumeListingH)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start server
	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("catalog-service starting on port %s (env: %s)", cfg.HTTPPort, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down catalog-service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("catalog-service stopped")
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
