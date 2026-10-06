// Command search-service serves public discovery (/v1/search) and listing
// management (/v1/listings), and keeps the OpenSearch projection in sync with
// PostgreSQL.
package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/audit"
	searchapp "github.com/trezutil/trezutil/internal/search/application"
	"github.com/trezutil/trezutil/internal/search/infrastructure/opensearch"
	"github.com/trezutil/trezutil/internal/search/projection"
	"github.com/trezutil/trezutil/packages/auth"
	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/database"
	"github.com/trezutil/trezutil/packages/observability"
)

func main() {
	logger := common.Logger("search-service")

	authn, err := auth.NewAuthenticatorFromEnv()
	if err != nil {
		log.Fatalf("search-service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.Open(ctx)
	if err != nil {
		log.Fatalf("search-service: %v", err)
	}
	defer db.Close()

	// Repositories
	categoryRepo := postgres.NewCategoryRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	attributeDefRepo := postgres.NewAttributeDefinitionRepository(db)
	attributeValRepo := postgres.NewAttributeValueRepository(db)
	mediaRepo := postgres.NewListingMediaRepository(db)
	availabilityRepo := postgres.NewListingAvailabilityRepository(db)
	locationRepo := postgres.NewLocationRepository(db)
	listingLocationRepo := postgres.NewListingLocationRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	storeRepo := postgres.NewStoreRepository(db)

	listingUseCases := application.NewListingUseCases(
		listingRepo, categoryRepo, attributeDefRepo, attributeValRepo, mediaRepo,
		availabilityRepo, locationRepo, listingLocationRepo,
		postgres.NewOrganizationRepository(db), sellerRepo, storeRepo,
		postgres.NewUserRepository(db),
		// Listing actions (publish, pause, close, suspension) leave an audit line and a stored audit event.
		audit.NewLogger(logger).WithStore(postgres.NewAuditRepository(db)),
	).WithVerticals(vertical.DefaultCatalog())

	// OpenSearch read projection
	osConfig := opensearch.Config{
		URL:          common.Getenv("OPENSEARCH_URL", "http://localhost:9200"),
		Username:     common.Getenv("OPENSEARCH_USERNAME", ""),
		Password:     common.Getenv("OPENSEARCH_PASSWORD", ""),
		Index:        common.Getenv("OPENSEARCH_INDEX", "trezutil-listings-v1"),
		Alias:        common.Getenv("OPENSEARCH_ALIAS", "trezutil-listings"),
		Timeout:      parseDuration(common.Getenv("OPENSEARCH_TIMEOUT", "30s"), 30*time.Second),
		TLS:          common.Getenv("OPENSEARCH_TLS", "false") == "true",
		MaxRetries:   common.GetenvInt("OPENSEARCH_MAX_RETRIES", 3),
		ReindexBatch: common.GetenvInt("OPENSEARCH_REINDEX_BATCH", 1000),
	}
	opensearchAdapter, err := opensearch.NewOpenSearchAdapter(osConfig)
	if err != nil {
		log.Fatalf("search-service: failed to create OpenSearch adapter: %v", err)
	}
	if err := opensearchAdapter.EnsureIndex(ctx); err != nil {
		// The sync loop and readiness probe surface a persistent outage.
		logger.Warn().Err(err).Msg("failed to ensure OpenSearch index")
	}

	metrics := searchapp.NewMetrics()
	projectionSource := projection.NewPostgresListingProjectionSource(
		listingRepo, categoryRepo, availabilityRepo, listingLocationRepo, locationRepo,
		mediaRepo, attributeValRepo, attributeDefRepo, sellerRepo, storeRepo,
		searchapp.NewSystemClock(),
	)
	reindexJob := searchapp.NewReindexJob(
		projectionSource, projection.NewSearchListingDocumentMapper(), opensearchAdapter,
		osConfig.ReindexBatch, searchapp.WithReindexMetrics(metrics),
	)

	// PostgreSQL -> OpenSearch projection sync (publish/pause/close become
	// visible/invisible in search within SEARCH_SYNC_INTERVAL).
	sync := searchapp.NewProjectionSync(listingChanges{listingRepo}, reindexJob, opensearchAdapter, logger)
	go sync.Run(ctx,
		parseDuration(common.Getenv("SEARCH_SYNC_INTERVAL", "5s"), 5*time.Second),
		parseDuration(common.Getenv("SEARCH_FULL_RESYNC_INTERVAL", "1h"), time.Hour))

	searchUseCases := searchapp.NewSearchService(
		opensearchAdapter,
		searchapp.NewNoopVisibilityResolver(),        // public discovery: published listings only
		searchapp.NewAllowAllAuthorizationResolver(), // search is open to guests by design
		nil,
		searchapp.NewNoopCursorCodec(),
		searchapp.NewSystemClock(),
	)

	listingHandler := api.NewListingHandler(listingUseCases)
	searchHandler := api.NewSearchHandler(searchUseCases)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/search", searchHandler.SearchH)
	mux.HandleFunc("GET /v1/search/suggest", searchHandler.SuggestH)
	mux.HandleFunc("POST /v1/listings", listingHandler.CreateListingH)
	mux.HandleFunc("GET /v1/listings", listingHandler.GetListingH)
	mux.HandleFunc("POST /v1/listings/publish", listingHandler.PublishListingH)
	mux.HandleFunc("POST /v1/listings/pause", listingHandler.PauseListingH)
	mux.HandleFunc("POST /v1/listings/resume", listingHandler.ResumeListingH)
	mux.HandleFunc("POST /v1/listings/close", api.CloseListingH(listingUseCases))
	mux.HandleFunc("POST /v1/listings/suspend", api.SuspendListingH(listingUseCases))
	mux.HandleFunc("POST /v1/listings/reinstate", api.ReinstateListingH(listingUseCases))
	mux.HandleFunc("POST /v1/listings/update", api.UpdateListingH(listingUseCases))
	mux.HandleFunc("POST /v1/listings/promotion", api.SetPromotionH(listingUseCases))
	mux.HandleFunc("POST /v1/listings/promotion/end", api.EndPromotionH(listingUseCases))
	mux.HandleFunc("GET /v1/listings/manage", organizationMemberOnly(postgres.NewOrganizationRepository(db), postgres.NewUserRepository(db), api.ManageListingsH(listingRepo)))
	mux.HandleFunc("GET /v1/categories", api.ListCategoriesH(categoryRepo))
	mux.HandleFunc("GET /v1/verticals", api.ListVerticalsH(vertical.DefaultCatalog()))
	mux.HandleFunc("POST /v1/categories", api.CreateCategoryH(categoryRepo, postgres.NewUserRepository(db)))
	mux.Handle("GET /metrics", observability.MetricsHandler())
	mux.HandleFunc("GET /healthz", common.HealthHandler)
	mux.HandleFunc("GET /health", common.HealthHandler)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if opensearchAdapter.Health(r.Context()) != domain.SearchHealthAvailable {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"opensearch_unavailable"}`))
			return
		}
		common.ReadyHandler(db)(w, r)
	})

	srv := common.NewServer(":"+common.Getenv("HTTP_PORT", "8081"), common.Observe("search-service", logger, authn.Middleware(mux)))
	if err := common.Run(srv, logger, "search-service"); err != nil {
		log.Fatalf("search-service: %v", err)
	}
}

// listingChanges adapts the PostgreSQL listing repository to the sync source.
type listingChanges struct{ repo *postgres.ListingRepository }

func (l listingChanges) ListChangedSince(ctx context.Context, ts time.Time, id string, limit int) ([]searchapp.ListingChange, error) {
	rows, err := l.repo.ListChangedSince(ctx, ts, id, limit)
	if err != nil {
		return nil, err
	}
	out := make([]searchapp.ListingChange, len(rows))
	for i, r := range rows {
		out[i] = searchapp.ListingChange{ID: r.ID, Status: r.Status, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}

func parseDuration(s string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return d
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return def
}

// organizationMemberOnly restricts organization-wide listing management reads
// to members of ?organization_id= (any role) or platform admins/operators.
func organizationMemberOnly(orgs domain.OrganizationRepository, users domain.UserRepository, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		allowed := false
		if u, err := users.FindByID(r.Context(), userID); err == nil && !u.IsSuspended() && (u.HasRole(domain.RoleAdmin) || u.HasRole(domain.RoleOperator)) {
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
