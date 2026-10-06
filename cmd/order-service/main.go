// Command order-service is the commerce API: orders, reservations,
// availability, and (for organization staff) inventory and fulfillment.
package main

import (
	"context"
	"log"
	"net/http"
	"net/url"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/availability"
	"github.com/trezutil/trezutil/internal/application/hotel"
	"github.com/trezutil/trezutil/internal/application/reservations"
	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/audit"
	"github.com/trezutil/trezutil/packages/auth"
	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/database"
	"github.com/trezutil/trezutil/packages/observability"
)

func main() {
	logger := common.Logger("order-service")

	authn, err := auth.NewAuthenticatorFromEnv()
	if err != nil {
		log.Fatalf("order-service: %v", err)
	}

	db, err := database.Open(context.Background())
	if err != nil {
		log.Fatalf("order-service: %v", err)
	}
	defer db.Close()

	listingRepo := postgres.NewListingRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	userRepo := postgres.NewUserRepository(db)
	inventoryRepo := postgres.NewInventoryRepository(db)
	capacityRepo := postgres.NewCapacityRepository(db)
	fulfillmentRepo := postgres.NewFulfillmentRepository(db)
	reservationRepo := postgres.NewReservationRepository(db)
	orderRepo := postgres.NewOrderRepository(db)

	// Critical commerce actions leave a structured audit line (internal/infrastructure/audit).
	auditLog := audit.NewLogger(logger).WithStore(postgres.NewAuditRepository(db))
	orderUseCases := application.NewOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo, inventoryRepo, fulfillmentRepo, auditLog)
	availabilityProvider := availability.NewCapacityAvailabilityProvider(inventoryRepo, capacityRepo).WithReservations(reservationRepo)
	availabilityUseCases := availability.NewAvailabilityUseCases(listingRepo, availabilityProvider, nil)
	storeRepo := postgres.NewStoreRepository(db)
	mediaRepo := postgres.NewListingMediaRepository(db)
	listingUseCases := application.NewListingUseCases(
		listingRepo, postgres.NewCategoryRepository(db),
		postgres.NewAttributeDefinitionRepository(db), postgres.NewAttributeValueRepository(db),
		mediaRepo, postgres.NewListingAvailabilityRepository(db),
		postgres.NewLocationRepository(db), postgres.NewListingLocationRepository(db),
		orgRepo, sellerRepo, storeRepo, userRepo, auditLog,
	).WithVerticals(vertical.DefaultCatalog())
	// Hotel Commerce Pack: room types, rooms, nightly capacity and stays on the
	// Commerce Core (docs/architecture/HOTEL_COMMERCE_PACK.md).
	hotels := hotel.NewService(hotel.Config{
		Listings: listingUseCases, ListingRepo: listingRepo, Media: mediaRepo,
		RoomTypes: postgres.NewHotelRoomTypeRepository(db), Inventory: inventoryRepo, Capacity: capacityRepo,
		Availability: availabilityProvider, Reservations: reservationRepo, Orders: orderUseCases,
		Stores: storeRepo, Orgs: orgRepo, Users: userRepo,
	})
	reservationUseCases := reservations.NewReservationUseCases(
		reservationRepo, listingRepo, sellerRepo, userRepo, orgRepo,
		availabilityProvider, capacityRepo, inventoryRepo, fulfillmentRepo, auditLog,
		reservations.ConfirmationPolicy{AutoConfirm: true},
	).WithRules(hotels)

	orders := api.NewOrderHandler(orderUseCases)
	bookings := api.NewReservationHandler(reservationUseCases, availabilityUseCases)
	inventory := api.NewInventoryHandler(application.NewInventoryUseCases(inventoryRepo, listingRepo, sellerRepo, orgRepo, userRepo, auditLog), nil)
	fulfillment := api.NewFulfillmentHandler(application.NewFulfillmentUseCases(fulfillmentRepo, orderRepo, reservationRepo, userRepo, orgRepo, auditLog))
	staff := tenantStaffOnly(orgRepo, userRepo)

	mux := http.NewServeMux()
	// Orders (customer or organization staff; enforced by the use cases)
	mux.HandleFunc("POST /api/v1/orders", orders.CreateOrderH)
	mux.HandleFunc("GET /api/v1/orders", api.ListMyOrdersH(orderRepo))
	mux.HandleFunc("GET /api/v1/orders/incoming", staff(api.IncomingOrdersH(orderRepo)))
	mux.HandleFunc("GET /api/v1/orders/{order_id}", pathToQuery("order_id", orders.GetOrderH))
	mux.HandleFunc("POST /api/v1/orders/{order_id}/cancel", pathToQuery("order_id", orders.CancelOrderH))
	mux.HandleFunc("POST /api/v1/orders/{order_id}/confirm", pathToQuery("order_id", orders.ConfirmOrderH))
	// The seller side after confirmation: start preparing, ready, delivered, complete (never the buyer).
	for _, step := range []string{"process", "ready", "fulfil", "complete"} {
		mux.HandleFunc("POST /api/v1/orders/{order_id}/"+step, pathToQuery("order_id", orders.OrderStepH(step)))
	}
	// Reservations and availability
	mux.HandleFunc("POST /api/v1/reservations", bookings.CreateReservationH)
	mux.HandleFunc("GET /api/v1/reservations", api.ListMyReservationsH(reservationRepo))
	mux.HandleFunc("GET /api/v1/reservations/incoming", staff(api.IncomingReservationsH(reservationRepo)))
	mux.HandleFunc("GET /api/v1/reservations/{reservation_id}", stripAPI(bookings.GetReservationH))
	mux.HandleFunc("POST /api/v1/reservations/{reservation_id}/cancel", stripAPI(bookings.CancelReservationH))
	mux.HandleFunc("GET /api/v1/availability", bookings.CheckAvailabilityH)
	// Hotel Commerce Pack (authorization inside the use cases; property and
	// availability are public like search)
	hotelAPI := api.NewHotelHandler(hotels)
	mux.HandleFunc("POST /api/v1/reservations/{reservation_id}/checkout", hotelAPI.CheckoutStayH)
	mux.HandleFunc("GET /api/v1/hotels/property", hotelAPI.PropertyH)
	mux.HandleFunc("GET /api/v1/hotels/availability", hotelAPI.AvailabilityH)
	mux.HandleFunc("POST /api/v1/hotels/room-types", hotelAPI.CreateRoomTypeH)
	mux.HandleFunc("GET /api/v1/hotels/room-types", hotelAPI.ListRoomTypesH)
	mux.HandleFunc("POST /api/v1/hotels/room-types/{listing_id}", hotelAPI.UpdateRoomTypeH)
	mux.HandleFunc("POST /api/v1/hotels/room-types/{listing_id}/units", hotelAPI.SetUnitsH)
	mux.HandleFunc("POST /api/v1/hotels/room-types/{listing_id}/capacity", hotelAPI.SetCapacityH)
	mux.HandleFunc("GET /api/v1/hotels/room-types/{listing_id}/capacity", hotelAPI.CapacityH)
	// Seller operations (organization staff only)
	mux.HandleFunc("GET /api/v1/inventory", staff(inventory.ListInventory))
	mux.HandleFunc("POST /api/v1/inventory/adjust", staff(inventory.AdjustInventoryH))
	mux.HandleFunc("GET /api/v1/inventory/{inventory_id}", staff(stripAPI(inventory.GetInventory)))
	mux.HandleFunc("GET /api/v1/fulfillments/{fulfillment_id}", staff(stripAPI(fulfillment.GetFulfillment)))
	mux.HandleFunc("POST /api/v1/fulfillments/{fulfillment_id}/transition", staff(stripAPI(fulfillment.TransitionFulfillment)))
	mux.HandleFunc("POST /api/v1/fulfillments/{fulfillment_id}/assign", staff(stripAPI(fulfillment.AssignFulfillment)))
	mux.HandleFunc("POST /api/v1/fulfillments/{fulfillment_id}/tracking", staff(stripAPI(fulfillment.UpdateTracking)))
	mux.Handle("GET /metrics", observability.MetricsHandler())
	mux.HandleFunc("GET /healthz", common.HealthHandler)
	mux.HandleFunc("GET /readyz", common.ReadyHandler(db))

	srv := common.NewServer(":"+common.Getenv("HTTP_PORT", "8085"), common.Observe("order-service", logger, authn.Middleware(mux)))
	if err := common.Run(srv, logger, "order-service"); err != nil {
		log.Fatalf("order-service: %v", err)
	}
}

// pathToQuery exposes a REST path parameter as the query parameter the
// shared handlers read, so both /orders/{id} and ?order_id= work.
func pathToQuery(name string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if v := r.PathValue(name); v != "" {
			q := r.URL.Query()
			q.Set(name, v)
			r.URL.RawQuery = q.Encode()
		}
		next(w, r)
	}
}

// stripAPI rewrites /api/v1/... to /v1/... for handlers that parse IDs from
// the internal path layout.
func stripAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := *r.URL
		if len(u.Path) > 4 && u.Path[:4] == "/api" {
			u.Path = u.Path[4:]
			u.RawPath = ""
		}
		r2 := r.Clone(r.Context())
		r2.URL = &url.URL{Path: u.Path, RawQuery: u.RawQuery}
		next(w, r2)
	}
}

// tenantStaffOnly requires an authenticated owner/admin/manager of the
// organization named by ?tenant_id= (or a platform admin/operator). Inventory
// and fulfillment handlers trust tenant_id, so it must be authorized here.
func tenantStaffOnly(orgs domain.OrganizationRepository, users domain.UserRepository) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.RequireUser(w, r)
			if !ok {
				return
			}
			tenantID := r.URL.Query().Get("tenant_id")
			allowed := false
			if u, err := users.FindByID(r.Context(), userID); err == nil && !u.IsSuspended() &&
				(u.HasRole(domain.RoleAdmin) || u.HasRole(domain.RoleOperator)) {
				allowed = true
			}
			if !allowed && tenantID != "" {
				if role, err := orgs.GetMemberRole(r.Context(), tenantID, userID); err == nil {
					allowed = role == domain.OrganizationRoleOwner || role == domain.OrganizationRoleAdmin || role == domain.OrganizationRoleManager
				}
			}
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":{"code":"FORBIDDEN","message":"Organization staff access required"}}`))
				return
			}
			next(w, r)
		}
	}
}
