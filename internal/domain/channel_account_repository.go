package domain

import "context"

// ChannelAccountRepository defines the interface for channel account persistence.
type ChannelAccountRepository interface {
	Create(ctx context.Context, account *ChannelAccount) error
	FindByID(ctx context.Context, id string) (*ChannelAccount, error)
	FindByChannelExternalID(ctx context.Context, channel Channel, externalID string) (*ChannelAccount, error)
	Update(ctx context.Context, account *ChannelAccount) error
}
