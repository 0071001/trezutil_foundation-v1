package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/config"
	"github.com/trezutil/trezutil/internal/domain"
	searchapp "github.com/trezutil/trezutil/internal/search/application"
	"github.com/trezutil/trezutil/internal/search/infrastructure/opensearch"
	"github.com/trezutil/trezutil/internal/search/projection"
)

func main() {
	// Load configuration
	cfg := loadConfig()

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

	// Initialize listing use cases
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

	// Initialize OpenSearch infrastructure
	osConfig := opensearch.Config{
		URL:          getEnv("OPENSEARCH_URL", "http://localhost:9200"),
		Username:     getEnv("OPENSEARCH_USERNAME", ""),
		Password:     getEnv("OPENSEARCH_PASSWORD", ""),
		Index:        getEnv("OPENSEARCH_INDEX", "trezutil-listings-v1"),
		Alias:        getEnv("OPENSEARCH_ALIAS", "trezutil-listings"),
		Timeout:      parseDuration(getEnv("OPENSEARCH_TIMEOUT", "30s")),
		TLS:          getEnv("OPENSEARCH_TLS", "false") == "true",
		MaxRetries:   parseInt(getEnv("OPENSEARCH_MAX_RETRIES", "3"), 3),
		ReindexBatch: parseInt(getEnv("OPENSEARCH_REINDEX_BATCH", "1000"), 1000),
	}

	opensearchAdapter, err := opensearch.NewOpenSearchAdapter(osConfig)
	if err != nil {
		log.Fatalf("Failed to create OpenSearch adapter: %v", err)
	}

	// Ensure index exists (does NOT destroy existing indexes)
	ctx := context.Background()
	if err := opensearchAdapter.EnsureIndex(ctx); err != nil {
		log.Printf("Warning: failed to ensure OpenSearch index: %v", err)
	}

	// Initialize metrics
	metrics := searchapp.NewMetrics()

	// Initialize dead-letter handler
	deadLetter := searchapp.NewDeadLetter(metrics)

	// Initialize projection components
	projectionSource := projection.NewPostgresListingProjectionSource(
		listingRepo,
		categoryRepo,
		availabilityRepo,
		listingLocationRepo,
		locationRepo,
		mediaRepo,
		attributeValRepo,
		attributeDefRepo,
		sellerRepo,
		storeRepo,
		searchapp.NewSystemClock(),
	)

	mapper := projection.NewSearchListingDocumentMapper()

	// Initialize event processor for projection synchronization
	eventProcessor := searchapp.NewEventProcessor(
		projectionSource,
		mapper,
		opensearchAdapter,
		searchapp.WithMetrics(metrics),
		searchapp.WithDeadLetter(deadLetter),
	)

	// Initialize reindex job
	reindexJob := searchapp.NewReindexJob(
		projectionSource,
		mapper,
		opensearchAdapter,
		osConfig.ReindexBatch,
		searchapp.WithReindexMetrics(metrics),
	)

	// Initialize search use cases with OpenSearch adapter
	searchEngine := opensearchAdapter
	visibilityResolver := searchapp.NewNoopVisibilityResolver()
	authResolver := searchapp.NewAllowAllAuthorizationResolver()
	cursorCodec := searchapp.NewNoopCursorCodec()
	clock := searchapp.NewSystemClock()

	searchUseCases := searchapp.NewSearchService(
		searchEngine,
		visibilityResolver,
		authResolver,
		nil, // listingProjectionReader - foundation
		cursorCodec,
		clock,
	)

	// Initialize handlers
	listingHandler := api.NewListingHandler(listingUseCases)
	searchHandler := api.NewSearchHandler(searchUseCases)

	// Setup routes
	mux := http.NewServeMux()

	// Search routes
	mux.HandleFunc("POST /v1/search", searchHandler.SearchH)
	mux.HandleFunc("GET /v1/search/suggest", searchHandler.SuggestH)

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

	// OpenSearch health check
	mux.HandleFunc("GET /health/opensearch", func(w http.ResponseWriter, r *http.Request) {
		status := opensearchAdapter.Health(ctx)
		if status == domain.SearchHealthAvailable {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OpenSearch available"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("OpenSearch unavailable"))
		}
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
		log.Printf("search-service starting on port %s (env: %s)", cfg.HTTPPort, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Log initialization status
	log.Printf("Search service initialized successfully")
	log.Printf("Event processor initialized: %v", eventProcessor != nil)
	log.Printf("Reindex job initialized: %v", reindexJob != nil)
	log.Printf("OpenSearch adapter health: %s", opensearchAdapter.Health(ctx))
	log.Printf("OpenSearch index: %s", opensearchAdapter.IndexName())
	log.Printf("OpenSearch alias: %s", opensearchAdapter.AliasName())

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down search-service...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("search-service stopped")
}

// Config holds the application configuration.
type appConfig struct {
	AppEnv          string
	LogLevel        string
	HTTPPort        string
	OpenSearchURL   string
	OpenSearchIndex string
}

func loadConfig() *config.Config {
	return &config.Config{
		AppEnv:   getEnv("APP_ENV", "development"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		HTTPPort: getEnv("HTTP_PORT", "8081"),
	}
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return val
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
