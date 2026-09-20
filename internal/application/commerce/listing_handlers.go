package commerce

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// CreateListingHandler handles CREATE_LISTING actions.
type CreateListingHandler struct {
	listingUseCases ListingUseCases
}

// NewCreateListingHandler creates a new CreateListingHandler.
func NewCreateListingHandler(listingUseCases ListingUseCases) *CreateListingHandler {
	return &CreateListingHandler{listingUseCases: listingUseCases}
}

// Handle executes a create listing action.
func (h *CreateListingHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	listing, ok := parameters["listing"].(*domain.Listing)
	if !ok || listing == nil {
		return nil, domain.ErrListingNotFound
	}

	var attributes []*domain.AttributeValue
	if v, ok := parameters["attributes"].([]*domain.AttributeValue); ok {
		attributes = v
	}

	var media []*domain.ListingMedia
	if v, ok := parameters["media"].([]*domain.ListingMedia); ok {
		media = v
	}

	var availability *domain.ListingAvailability
	if v, ok := parameters["availability"].(*domain.ListingAvailability); ok {
		availability = v
	}

	var locations []*domain.ListingLocation
	if v, ok := parameters["locations"].([]*domain.ListingLocation); ok {
		locations = v
	}

	if err := h.listingUseCases.CreateListing(ctx, actor.ID, listing, attributes, media, availability, locations); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"listing_id": listing.ID,
		"status":     "created",
	}, nil
}

// ActionType returns the action type this handler handles.
func (h *CreateListingHandler) ActionType() domain.CommerceActionType {
	return domain.ActionCreateListing
}

// PublishListingHandler handles PUBLISH_LISTING actions.
type PublishListingHandler struct {
	listingUseCases ListingUseCases
}

// NewPublishListingHandler creates a new PublishListingHandler.
func NewPublishListingHandler(listingUseCases ListingUseCases) *PublishListingHandler {
	return &PublishListingHandler{listingUseCases: listingUseCases}
}

// Handle executes a publish listing action.
func (h *PublishListingHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	listingID, ok := parameters["listing_id"].(string)
	if !ok || listingID == "" {
		return nil, domain.ErrListingNotFound
	}

	if err := h.listingUseCases.PublishListing(ctx, actor.ID, listingID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"listing_id": listingID,
		"status":     "published",
	}, nil
}

// ActionType returns the action type this handler handles.
func (h *PublishListingHandler) ActionType() domain.CommerceActionType {
	return domain.ActionPublishListing
}

// PauseListingHandler handles PAUSE_LISTING actions.
type PauseListingHandler struct {
	listingUseCases ListingUseCases
}

// NewPauseListingHandler creates a new PauseListingHandler.
func NewPauseListingHandler(listingUseCases ListingUseCases) *PauseListingHandler {
	return &PauseListingHandler{listingUseCases: listingUseCases}
}

// Handle executes a pause listing action.
func (h *PauseListingHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	listingID, ok := parameters["listing_id"].(string)
	if !ok || listingID == "" {
		return nil, domain.ErrListingNotFound
	}

	if err := h.listingUseCases.PauseListing(ctx, actor.ID, listingID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"listing_id": listingID,
		"status":     "paused",
	}, nil
}

// ActionType returns the action type this handler handles.
func (h *PauseListingHandler) ActionType() domain.CommerceActionType {
	return domain.ActionPauseListing
}

// ResumeListingHandler handles RESUME_LISTING actions.
type ResumeListingHandler struct {
	listingUseCases ListingUseCases
}

// NewResumeListingHandler creates a new ResumeListingHandler.
func NewResumeListingHandler(listingUseCases ListingUseCases) *ResumeListingHandler {
	return &ResumeListingHandler{listingUseCases: listingUseCases}
}

// Handle executes a resume listing action.
func (h *ResumeListingHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	listingID, ok := parameters["listing_id"].(string)
	if !ok || listingID == "" {
		return nil, domain.ErrListingNotFound
	}

	if err := h.listingUseCases.ResumeListing(ctx, actor.ID, listingID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"listing_id": listingID,
		"status":     "published",
	}, nil
}

// ActionType returns the action type this handler handles.
func (h *ResumeListingHandler) ActionType() domain.CommerceActionType {
	return domain.ActionResumeListing
}
