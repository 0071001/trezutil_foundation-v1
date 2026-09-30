package domain

import (
	"fmt"
	"time"
)

// ListingType represents the type of listing.
type ListingType string

const (
	ListingTypePhysicalProduct ListingType = "physical_product"
	ListingTypeRealEstate      ListingType = "real_estate"
	ListingTypeVehicle         ListingType = "vehicle"
	ListingTypeFood            ListingType = "food"
	ListingTypeService         ListingType = "service"
	ListingTypeOther           ListingType = "other"
)

// PricingModel represents the pricing model of a listing.
type PricingModel string

const (
	PricingModelFixed      PricingModel = "fixed"
	PricingModelNegotiable PricingModel = "negotiable"
	PricingModelAuction    PricingModel = "auction"
)

// ListingStatus represents the status of a listing.
type ListingStatus string

const (
	ListingStatusDraft         ListingStatus = "draft"
	ListingStatusPendingReview ListingStatus = "pending_review"
	ListingStatusPublished     ListingStatus = "published"
	ListingStatusPaused        ListingStatus = "paused"
	ListingStatusSold          ListingStatus = "sold"
	ListingStatusClosed        ListingStatus = "closed"
	ListingStatusExpired       ListingStatus = "expired"
	ListingStatusSuspended     ListingStatus = "suspended"
)

// Listing represents a listing in the system.
type Listing struct {
	ID             string
	OrganizationID string
	SellerID       string
	StoreID        string
	CategoryID     string
	Title          string
	Description    *string
	ListingType    ListingType
	// Vertical is the commerce vertical the listing belongs to (HOTEL,
	// RESTAURANT…). It is set when the listing is created and never changes.
	Vertical      Vertical
	PricingModel  PricingModel
	PriceAmount   float64
	PriceCurrency string
	Status        ListingStatus
	Metadata      map[string]interface{}
	PublishedAt   *int64
	CreatedAt     int64
	UpdatedAt     int64
}

// NewListing creates a new Listing.
func NewListing(organizationID, sellerID, storeID, categoryID, title, listingType, pricingModel, priceCurrency string, priceAmount float64) *Listing {
	now := time.Now().Unix()
	return &Listing{
		OrganizationID: organizationID,
		SellerID:       sellerID,
		StoreID:        storeID,
		CategoryID:     categoryID,
		Title:          title,
		ListingType:    ListingType(listingType),
		Vertical:       DefaultVerticalForListingType(ListingType(listingType)),
		PricingModel:   PricingModel(pricingModel),
		PriceAmount:    priceAmount,
		PriceCurrency:  priceCurrency,
		Status:         ListingStatusDraft,
		Metadata:       make(map[string]interface{}),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Publish transitions the listing to published status.
// Valid from: DRAFT, PENDING_REVIEW, PAUSED
func (l *Listing) Publish() {
	if l.Status == ListingStatusDraft || l.Status == ListingStatusPendingReview || l.Status == ListingStatusPaused {
		now := time.Now().Unix()
		l.Status = ListingStatusPublished
		if l.PublishedAt == nil {
			l.PublishedAt = &now
		}
		l.UpdatedAt = now
	}
}

// Pause transitions the listing to paused status.
// Valid from: PUBLISHED
func (l *Listing) Pause() {
	if l.Status == ListingStatusPublished {
		l.Status = ListingStatusPaused
		l.UpdatedAt = time.Now().Unix()
	}
}

// Resume transitions a paused listing back to published.
// Valid from: PAUSED
func (l *Listing) Resume() {
	if l.Status == ListingStatusPaused {
		l.Status = ListingStatusPublished
		l.UpdatedAt = time.Now().Unix()
	}
}

// Suspend transitions the listing to suspended status.
// Valid from: PUBLISHED
func (l *Listing) Suspend() {
	if l.Status == ListingStatusPublished {
		l.Status = ListingStatusSuspended
		l.UpdatedAt = time.Now().Unix()
	}
}

// Reject transitions a pending review listing back to draft.
// Valid from: PENDING_REVIEW
func (l *Listing) Reject() {
	if l.Status == ListingStatusPendingReview {
		l.Status = ListingStatusDraft
		l.UpdatedAt = time.Now().Unix()
	}
}

// MarkAsSold transitions the listing to sold status.
// Valid from: PUBLISHED, PAUSED
func (l *Listing) MarkAsSold() {
	if l.Status == ListingStatusPublished || l.Status == ListingStatusPaused {
		l.Status = ListingStatusSold
		l.UpdatedAt = time.Now().Unix()
	}
}

// MarkAsClosed transitions the listing to closed status.
// Valid from: PUBLISHED, PAUSED, PENDING_REVIEW
func (l *Listing) MarkAsClosed() {
	if l.Status == ListingStatusPublished || l.Status == ListingStatusPaused || l.Status == ListingStatusPendingReview {
		l.Status = ListingStatusClosed
		l.UpdatedAt = time.Now().Unix()
	}
}

// MarkAsExpired transitions the listing to expired status.
// Valid from: PUBLISHED, PAUSED
func (l *Listing) MarkAsExpired() {
	if l.Status == ListingStatusPublished || l.Status == ListingStatusPaused {
		l.Status = ListingStatusExpired
		l.UpdatedAt = time.Now().Unix()
	}
}

// IsPublished checks if the listing is published.
func (l *Listing) IsPublished() bool {
	return l.Status == ListingStatusPublished
}

// IsDraft checks if the listing is in draft status.
func (l *Listing) IsDraft() bool {
	return l.Status == ListingStatusDraft
}

// IsTerminal checks if the listing is in a terminal state.
func (l *Listing) IsTerminal() bool {
	return l.Status == ListingStatusSold ||
		l.Status == ListingStatusClosed ||
		l.Status == ListingStatusExpired ||
		l.Status == ListingStatusSuspended
}

// CanBeModified checks if the listing can be modified.
func (l *Listing) CanBeModified() bool {
	return l.Status == ListingStatusDraft ||
		l.Status == ListingStatusPendingReview ||
		l.Status == ListingStatusPaused
}

// CanBePublished checks if the listing can be published.
func (l *Listing) CanBePublished() bool {
	return l.Status == ListingStatusDraft ||
		l.Status == ListingStatusPendingReview ||
		l.Status == ListingStatusPaused
}

// CanBePaused checks if the listing can be paused.
func (l *Listing) CanBePaused() bool {
	return l.Status == ListingStatusPublished
}

// ValidateTransition validates a status transition.
func (l *Listing) ValidateTransition(newStatus ListingStatus) error {
	current := l.Status

	// Allow any transition to the same status
	if current == newStatus {
		return nil
	}

	switch newStatus {
	case ListingStatusPublished:
		if current != ListingStatusDraft && current != ListingStatusPendingReview && current != ListingStatusPaused {
			return ErrInvalidStatusTransition
		}
	case ListingStatusPaused:
		if current != ListingStatusPublished {
			return ErrInvalidStatusTransition
		}
	case ListingStatusDraft:
		if current != ListingStatusPendingReview {
			return ErrInvalidStatusTransition
		}
	case ListingStatusSuspended:
		if current != ListingStatusPublished {
			return ErrInvalidStatusTransition
		}
	case ListingStatusSold:
		if current != ListingStatusPublished && current != ListingStatusPaused {
			return ErrInvalidStatusTransition
		}
	case ListingStatusClosed:
		if current != ListingStatusPublished && current != ListingStatusPaused && current != ListingStatusPendingReview {
			return ErrInvalidStatusTransition
		}
	case ListingStatusExpired:
		if current != ListingStatusPublished && current != ListingStatusPaused {
			return ErrInvalidStatusTransition
		}
	default:
		return ErrInvalidStatusTransition
	}

	return nil
}

// ErrInvalidStatusTransition is returned when a status transition is not allowed.
var ErrInvalidStatusTransition = fmt.Errorf("invalid status transition")
