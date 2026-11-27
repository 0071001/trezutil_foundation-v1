package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type unreadablePrices struct{ MockUsageService }

func (*unreadablePrices) ResolvePolicy(context.Context, domain.CommerceActionType, domain.Channel, domain.ActorType) (*domain.UsagePolicy, error) {
	return nil, errors.New("database unavailable")
}

// A price that cannot be read is not a price of zero: the model is not used.
func TestMeteringFailsClosed(t *testing.T) {
	_, ok := meterUsage(context.Background(), &unreadablePrices{}, "AI_INTERPRETATION", domain.ChannelWeb,
		domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), "org", "req")
	if ok {
		t.Fatal("the model would be used for free")
	}
}
