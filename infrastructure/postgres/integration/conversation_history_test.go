package integration

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// GET /api/v1/conversations reads ConversationStore.ListByCustomer: only the customer's own conversations on the
// channel asked for, the most recently active first, titled by the first message the customer wrote.
func TestConversationStore_ListByCustomer(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174120"
	me := "123e4567-e89b-12d3-a456-426614174121"
	other := "123e4567-e89b-12d3-a456-426614174122"
	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, me)
	// createTestUser always uses the same phone: the second customer gets its own.
	if err := postgres.NewUserRepository(db.DB).Create(ctx, &domain.User{
		ID: other, Phone: "+243812345679", DisplayName: "Other", Status: domain.UserStatusActive,
		CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(), Roles: []domain.Role{},
	}); err != nil {
		t.Fatalf("create second user: %v", err)
	}

	convs := postgres.NewConversationStore(db.DB)
	msgs := postgres.NewMessageStore(db.DB)
	now := time.Now().Unix()

	newConv := func(id string, channel domain.Channel, external, customer string) {
		c := domain.NewConversation(channel, external, customer, orgID)
		c.ID, c.CreatedAt, c.UpdatedAt = id, now-3600, now-3600
		if err := convs.Create(ctx, c); err != nil {
			t.Fatalf("create conversation %s: %v", id, err)
		}
	}
	say := func(convID string, dir domain.MessageDirection, text, key string, at int64) {
		m := domain.NewMessage(convID, dir, domain.MessageTypeText, "x", &domain.MessageContent{Text: text})
		m.IdempotencyKey, m.ExternalRef, m.CreatedAt = key, key, at
		if err := msgs.Create(ctx, m); err != nil {
			t.Fatalf("create message: %v", err)
		}
	}

	older := "123e4567-e89b-12d3-a456-426614174130"
	recent := "123e4567-e89b-12d3-a456-426614174131"
	newConv(older, domain.ChannelWeb, "web-older", me)
	newConv(recent, domain.ChannelWeb, "web-recent", me)
	newConv("123e4567-e89b-12d3-a456-426614174132", domain.ChannelWhatsApp, "wa-mine", me)
	newConv("123e4567-e89b-12d3-a456-426614174133", domain.ChannelWeb, "web-theirs", other)

	say(older, domain.MessageDirectionInbound, "Je cherche 50 ordinateurs", "o1", now-600)
	say(older, domain.MessageDirectionOutbound, "J’ai trouvé 2 offres.", "o2", now-600)
	say(recent, domain.MessageDirectionInbound, "Un traiteur pour 80 personnes", "r1", now-60)
	say(recent, domain.MessageDirectionInbound, "Plutôt à Gombe", "r2", now-30)

	list, err := convs.ListByCustomer(ctx, me, domain.ChannelWeb, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want the 2 web conversations of the customer, got %d: %+v", len(list), list)
	}
	if list[0].ID != recent || list[1].ID != older {
		t.Fatalf("want the most recently active first, got %s then %s", list[0].ID, list[1].ID)
	}
	if list[0].Title != "Un traiteur pour 80 personnes" || list[0].MessageCount != 2 || list[0].ExternalID != "web-recent" {
		t.Fatalf("unexpected summary: %+v", list[0])
	}
	if list[0].LastActivityAt < now-60 {
		t.Fatalf("last activity should come from the latest message, got %d", list[0].LastActivityAt)
	}

	// Two messages in the same second: the title is the one inserted first, and the transcript keeps that order.
	sameSecond := "123e4567-e89b-12d3-a456-426614174134"
	newConv(sameSecond, domain.ChannelWeb, "web-same-second", me)
	say(sameSecond, domain.MessageDirectionInbound, "Premier besoin", "s1", now)
	say(sameSecond, domain.MessageDirectionInbound, "Deuxième précision", "s2", now)
	say(sameSecond, domain.MessageDirectionInbound, "Troisième question", "s3", now)
	again, err := convs.ListByCustomer(ctx, me, domain.ChannelWeb, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, c := range again {
		if c.ID == sameSecond && c.Title != "Premier besoin" {
			t.Fatalf("the title must be the first message inserted, got %q", c.Title)
		}
	}
	transcript, err := msgs.FindByConversationID(ctx, sameSecond)
	if err != nil || len(transcript) != 3 || transcript[0].Content.Text != "Premier besoin" || transcript[2].Content.Text != "Troisième question" {
		t.Fatalf("the transcript must keep the insertion order within a second: %v %v", transcript, err)
	}

	none, err := convs.ListByCustomer(ctx, "123e4567-e89b-12d3-a456-426614174199", domain.ChannelWeb, 50)
	if err != nil || len(none) != 0 {
		t.Fatalf("a customer without conversations gets an empty list, got %v %v", none, err)
	}
}
