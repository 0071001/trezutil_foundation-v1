package domain

import "time"

// ChannelAccountType represents the type of channel account.
type ChannelAccountType string

const (
	ChannelAccountTypeWhatsApp ChannelAccountType = "whatsapp"
	ChannelAccountTypeWeb      ChannelAccountType = "web"
	ChannelAccountTypeMobile   ChannelAccountType = "mobile"
)

// ChannelAccountStatus represents the status of a channel account.
type ChannelAccountStatus string

const (
	ChannelAccountStatusActive   ChannelAccountStatus = "active"
	ChannelAccountStatusInactive ChannelAccountStatus = "inactive"
)

// ChannelAccount represents a provider channel account mapped to an organization.
type ChannelAccount struct {
	ID             string
	OrganizationID string
	Channel        Channel
	ExternalID     string
	Status         ChannelAccountStatus
	CreatedAt      int64
	UpdatedAt      int64
}

// NewChannelAccount creates a new ChannelAccount.
func NewChannelAccount(channel Channel, externalID string, organizationID string) *ChannelAccount {
	now := time.Now().Unix()
	return &ChannelAccount{
		ID:             NewID(),
		OrganizationID: organizationID,
		Channel:        channel,
		ExternalID:     externalID,
		Status:         ChannelAccountStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
