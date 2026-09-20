package commerce

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trezutil/trezutil/internal/domain"
)

type mockReservationUseCases struct {
	createErr   error
	getErr      error
	cancelErr   error
	reservation *domain.Reservation
}

func (m *mockReservationUseCases) CreateReservation(ctx context.Context, actorID string, listingID string, startAt int64, endAt int64, quantity int, partySize int, idempotencyKey string) (*domain.Reservation, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.reservation, nil
}

func (m *mockReservationUseCases) GetReservation(ctx context.Context, actorID string, reservationID string) (*domain.Reservation, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.reservation, nil
}

func (m *mockReservationUseCases) CancelReservation(ctx context.Context, actorID string, reservationID string) error {
	return m.cancelErr
}

type mockAvailabilityUseCases struct {
	checkErr error
	result   domain.AvailabilityResult
}

func (m *mockAvailabilityUseCases) CheckAvailability(ctx context.Context, actorID string, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	if m.checkErr != nil {
		return domain.AvailabilityResult{}, m.checkErr
	}
	return m.result, nil
}

func TestCreateReservationHandler_Handle_Success(t *testing.T) {
	reservation := &domain.Reservation{
		ID:             "res-1",
		OrganizationID: "org-1",
		CustomerID:     "user-1",
		ListingID:      "listing-1",
		Status:         domain.ReservationStatusConfirmed,
		StartAt:        1760227200,
		EndAt:          1760486400,
		Quantity:       1,
		PartySize:      2,
		CreatedAt:      1760000000,
		UpdatedAt:      1760000000,
	}

	uc := &mockReservationUseCases{reservation: reservation}
	handler := NewCreateReservationHandler(uc)

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	parameters := map[string]interface{}{
		"listing_id": "listing-1",
		"start_at":   1760227200,
		"end_at":     1760486400,
		"quantity":   1,
		"party_size": 2,
	}

	result, err := handler.Handle(context.Background(), actor, parameters)
	require.NoError(t, err)

	data := result.(map[string]interface{})
	assert.Equal(t, "res-1", data["reservation_id"])
	assert.Equal(t, "confirmed", data["status"])
	assert.Equal(t, int64(1760227200), data["start_at"])
	assert.Equal(t, int64(1760486400), data["end_at"])
	assert.Equal(t, 1, data["quantity"])
	assert.Equal(t, 2, data["party_size"])
	assert.Equal(t, "user-1", data["customer_id"])
	assert.Equal(t, "listing-1", data["listing_id"])
}

func TestCreateReservationHandler_Handle_MissingListing(t *testing.T) {
	uc := &mockReservationUseCases{}
	handler := NewCreateReservationHandler(uc)

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	parameters := map[string]interface{}{
		"start_at": 1760227200,
		"end_at":   1760486400,
	}

	_, err := handler.Handle(context.Background(), actor, parameters)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing not found")
}

func TestCreateReservationHandler_Handle_InvalidDates(t *testing.T) {
	uc := &mockReservationUseCases{createErr: fmt.Errorf("end_at must be after start_at")}
	handler := NewCreateReservationHandler(uc)

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	parameters := map[string]interface{}{
		"listing_id": "listing-1",
		"start_at":   1760486400,
		"end_at":     1760227200, // end before start
	}

	_, err := handler.Handle(context.Background(), actor, parameters)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "end_at must be after start_at")
}

func TestGetReservationHandler_Handle_Success(t *testing.T) {
	reservation := &domain.Reservation{
		ID:             "res-1",
		OrganizationID: "org-1",
		CustomerID:     "user-1",
		ListingID:      "listing-1",
		Status:         domain.ReservationStatusConfirmed,
		StartAt:        1760227200,
		EndAt:          1760486400,
		Quantity:       1,
		PartySize:      2,
		CreatedAt:      1760000000,
		UpdatedAt:      1760000000,
	}

	uc := &mockReservationUseCases{reservation: reservation}
	handler := NewGetReservationHandler(uc)

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	parameters := map[string]interface{}{
		"reservation_id": "res-1",
	}

	result, err := handler.Handle(context.Background(), actor, parameters)
	require.NoError(t, err)

	data := result.(map[string]interface{})
	assert.Equal(t, "res-1", data["reservation_id"])
	assert.Equal(t, "confirmed", data["status"])
}

func TestGetReservationHandler_Handle_MissingID(t *testing.T) {
	uc := &mockReservationUseCases{}
	handler := NewGetReservationHandler(uc)

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	parameters := map[string]interface{}{}

	_, err := handler.Handle(context.Background(), actor, parameters)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reservation not found")
}

func TestCancelReservationHandler_Handle_Success(t *testing.T) {
	uc := &mockReservationUseCases{cancelErr: nil}
	handler := NewCancelReservationHandler(uc)

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	parameters := map[string]interface{}{
		"reservation_id": "res-1",
	}

	result, err := handler.Handle(context.Background(), actor, parameters)
	require.NoError(t, err)

	data := result.(map[string]interface{})
	assert.Equal(t, "res-1", data["reservation_id"])
	assert.Equal(t, "cancelled", data["status"])
}

func TestCheckAvailabilityHandler_Handle_Success(t *testing.T) {
	availUC := &mockAvailabilityUseCases{
		result: domain.AvailabilityResult{
			Available: true,
			Capacity:  10,
			Remaining: 7,
			Reason:    "",
			StartAt:   1760227200,
			EndAt:     1760486400,
		},
	}
	handler := NewCheckAvailabilityHandler(availUC)

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	parameters := map[string]interface{}{
		"listing_id": "listing-1",
		"start_at":   1760227200,
		"end_at":     1760486400,
		"quantity":   1,
		"party_size": 2,
	}

	result, err := handler.Handle(context.Background(), actor, parameters)
	require.NoError(t, err)

	data := result.(map[string]interface{})
	assert.Equal(t, true, data["available"])
	assert.Equal(t, 10, data["capacity"])
	assert.Equal(t, 7, data["remaining"])
}
