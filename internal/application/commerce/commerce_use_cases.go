package commerce

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

type SearchUseCases interface {
	ExecuteSearch(ctx context.Context, actorID string, query domain.DiscoveryQuery) (domain.SearchResultPage, error)
	Suggest(ctx context.Context, actorID string, req domain.SuggestRequest) (domain.SuggestResponse, error)
}

type ListingUseCases interface {
	CreateListing(ctx context.Context, actorID string, listing *domain.Listing, attributes []*domain.AttributeValue, media []*domain.ListingMedia, availability *domain.ListingAvailability, locations []*domain.ListingLocation) error
	GetListing(ctx context.Context, actorID string, listingID string) (*domain.Listing, error)
	PublishListing(ctx context.Context, actorID string, listingID string) error
	PauseListing(ctx context.Context, actorID string, listingID string) error
	ResumeListing(ctx context.Context, actorID string, listingID string) error
}

type OrderUseCases interface {
	// CreateOrderIdempotent returns the order already created with the same key instead of creating another.
	CreateOrderIdempotent(ctx context.Context, actorID string, listingID string, quantity int, idempotencyKey string) (*domain.Order, error)
	GetOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error)
	CancelOrder(ctx context.Context, actorID string, orderID string) error
}

type ReservationUseCases interface {
	CreateReservation(ctx context.Context, actorID string, listingID string, startAt int64, endAt int64, quantity int, partySize int, idempotencyKey string) (*domain.Reservation, error)
	GetReservation(ctx context.Context, actorID string, reservationID string) (*domain.Reservation, error)
	CancelReservation(ctx context.Context, actorID string, reservationID string) error
}

type AvailabilityUseCases interface {
	CheckAvailability(ctx context.Context, actorID string, query domain.AvailabilityQuery) (domain.AvailabilityResult, error)
}
