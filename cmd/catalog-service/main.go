// Command catalog-service exposes the listing management API on its own.
// The public gateway routes /api/v1/listings to search-service, which serves
// the same handlers; catalog-service remains available for internal use.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/infrastructure/audit"
	"github.com/trezutil/trezutil/packages/auth"
	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/database"
	"github.com/trezutil/trezutil/packages/observability"
)

func main() {
	logger := common.Logger("catalog-service")

	authn, err := auth.NewAuthenticatorFromEnv()
	if err != nil {
		log.Fatalf("catalog-service: %v", err)
	}

	db, err := database.Open(context.Background())
	if err != nil {
		log.Fatalf("catalog-service: %v", err)
	}
	defer db.Close()

	listingUseCases := application.NewListingUseCases(
		postgres.NewListingRepository(db),
		postgres.NewCategoryRepository(db),
		postgres.NewAttributeDefinitionRepository(db),
		postgres.NewAttributeValueRepository(db),
		postgres.NewListingMediaRepository(db),
		postgres.NewListingAvailabilityRepository(db),
		postgres.NewLocationRepository(db),
		postgres.NewListingLocationRepository(db),
		postgres.NewOrganizationRepository(db),
		postgres.NewSellerRepository(db),
		postgres.NewStoreRepository(db),
		postgres.NewUserRepository(db),
		audit.NewLogger(logger),
	).WithVerticals(vertical.DefaultCatalog())

	listingHandler := api.NewListingHandler(listingUseCases)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/listings", listingHandler.CreateListingH)
	mux.HandleFunc("GET /v1/listings", listingHandler.GetListingH)
	mux.HandleFunc("POST /v1/listings/publish", listingHandler.PublishListingH)
	mux.HandleFunc("POST /v1/listings/pause", listingHandler.PauseListingH)
	mux.HandleFunc("POST /v1/listings/resume", listingHandler.ResumeListingH)
	mux.Handle("GET /metrics", observability.MetricsHandler())
	mux.HandleFunc("GET /healthz", common.HealthHandler)
	mux.HandleFunc("GET /health", common.HealthHandler)
	mux.HandleFunc("GET /readyz", common.ReadyHandler(db))

	srv := common.NewServer(":"+common.Getenv("HTTP_PORT", "8082"), common.Observe("catalog-service", logger, authn.Middleware(mux)))
	if err := common.Run(srv, logger, "catalog-service"); err != nil {
		log.Fatalf("catalog-service: %v", err)
	}
}
