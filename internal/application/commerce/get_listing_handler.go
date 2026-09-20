package commerce

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// GetListingHandler handles GET_LISTING actions.
type GetListingHandler struct {
	listingUseCases ListingUseCases
}

// NewGetListingHandler creates a new GetListingHandler.
func NewGetListingHandler(listingUseCases ListingUseCases) *GetListingHandler {
	return &GetListingHandler{listingUseCases: listingUseCases}
}

// Handle executes a get listing action.
func (h *GetListingHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	listingID, ok := parameters["listing_id"].(string)
	if !ok || listingID == "" {
		return nil, domain.ErrListingNotFound
	}

	listing, err := h.listingUseCases.GetListing(ctx, actor.ID, listingID)
	if err != nil {
		return nil, err
	}

	return listing, nil
}

// ActionType returns the action type this handler handles.
func (h *GetListingHandler) ActionType() domain.CommerceActionType {
	return domain.ActionGetListing
}
