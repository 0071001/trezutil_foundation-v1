package conversation

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func listedResults() *domain.ConversationContext {
	return &domain.ConversationContext{ResultItems: []domain.ResultItemRef{
		{RefType: "listing", ID: "l1", Index: 0, Title: "Collier oiseau en or"},
		{RefType: "listing", ID: "l2", Index: 1, Title: "Collier or et turquoise"},
		{RefType: "listing", ID: "l3", Index: 2, Title: "Ciment CPJ 42.5"},
	}}
}

func TestReferencedResultIDUnderstandsFrenchAndNumberedReferences(t *testing.T) {
	cx := listedResults()
	cases := map[string]string{
		"2":                     "l2",
		"3.":                    "l3",
		"Parle-moi du numéro 1": "l1",
		"Parle-moi de l’offre 2 : Collier or et turquoise": "l2",
		"le n° 3":             "l3",
		"et le #2 ?":          "l2",
		"Option 1 stp":        "l1",
		"le premier":          "l1",
		"La deuxième":         "l2",
		"le 3e":               "l3",
		"le dernier":          "l3",
		"the second one":      "l2",
		"Je prends l’offre 1": "l1",
	}
	for text, want := range cases {
		if got := referencedResultID(text, cx); got != want {
			t.Errorf("%q -> %q, want %q", text, got, want)
		}
	}
}

func TestReferencedResultIDIgnoresOrdinaryRequests(t *testing.T) {
	cx := listedResults()
	for _, text := range []string{
		"Parle-moi du numéro 9",                        // out of range
		"Je cherche une maison de 3 chambres à Limete", // a number, not a position
		"Un billet en première classe pour Lubumbashi la semaine prochaine",
		"offre 50000 CDF",
		"Bonjour",
	} {
		if got := referencedResultID(text, cx); got != "" {
			t.Errorf("%q resolved to %q", text, got)
		}
	}
	if got := referencedResultID("le premier", &domain.ConversationContext{}); got != "" {
		t.Errorf("no listed results, got %q", got)
	}
}

func TestKeywordFallbackOpensOrActsOnAReferencedResult(t *testing.T) {
	p := &ConversationProcessor{}
	cx := listedResults()
	run := func(text string) *domain.ConversationIntent {
		return keywordIntent(p, context.Background(), &domain.Conversation{}, cx, &ProcessRequest{MessageText: text})
	}

	open := run("Parle-moi du numéro 1")
	if open == nil || open.ActionType != domain.ActionGetListing || entity(open, "listing_id") != "l1" {
		t.Fatalf("open -> %+v", open)
	}
	buy := run("Je veux acheter le numéro 2")
	if buy == nil || buy.ActionType != domain.ActionCreateOrder || entity(buy, "listing_id") != "l2" || !buy.RequiresConfirmation {
		t.Fatalf("buy -> %+v", buy)
	}
	// The offer's title after the colon does not decide the action.
	titled := keywordIntent(p, context.Background(), &domain.Conversation{}, &domain.ConversationContext{ResultItems: []domain.ResultItemRef{{ID: "x1", Title: "Commande groupée"}}},
		&ProcessRequest{MessageText: "Parle-moi de l’offre 1 : Commande groupée"})
	if titled == nil || titled.ActionType != domain.ActionGetListing {
		t.Fatalf("titled -> %+v", titled)
	}
}

func TestShortReferenceSkipsTheModelOnlyForShortReplies(t *testing.T) {
	if !isShortReference("Parle-moi de l’offre 1 : Collier oiseau en or 14 carats avec chaîne fine") {
		t.Fatal("the card's own prompt is a short reference")
	}
	if isShortReference("Pour l’offre 2, est-ce que la livraison à Limete est possible demain matin ?") {
		t.Fatal("a real question goes to the model")
	}
}

func entity(i *domain.ConversationIntent, name string) string {
	for _, e := range i.Entities {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

func TestOpeningAListedOfferKeepsTheListAndRemembersTheOffer(t *testing.T) {
	p := &ConversationProcessor{}
	cx := listedResults()
	p.storeOpenedListing(context.Background(), "c1", cx, &domain.Listing{ID: "l1", Title: "Collier oiseau en or"})
	if len(cx.ResultItems) != 3 {
		t.Fatalf("the list was replaced: %+v", cx.ResultItems)
	}
	if got := currentListingID(cx); got != "l1" {
		t.Fatalf("current listing = %q", got)
	}
	if got := referencedResultID("et le deuxième ?", cx); got != "l2" {
		t.Fatalf("the next reference = %q", got)
	}

	// An offer that was not in the list becomes the only result.
	p.storeOpenedListing(context.Background(), "c1", cx, &domain.Listing{ID: "other", Title: "Autre"})
	if len(cx.ResultItems) != 1 || currentListingID(cx) != "other" {
		t.Fatalf("outside offer: %+v / %q", cx.ResultItems, currentListingID(cx))
	}

	// A new search forgets the opened offer.
	p.storeSearchResult(context.Background(), "c1", cx, nil)
	if cx.CurrentListing != nil {
		t.Fatal("a new search keeps the old offer")
	}
}
