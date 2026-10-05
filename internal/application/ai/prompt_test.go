package ai

import (
	"context"
	"strings"
	"testing"
)

func TestInstructionsGiveTheModelTheContextOfAShopAssistant(t *testing.T) {
	p := buildStaticPrompt()
	for _, want := range []string{
		"Kinshasa", "US dollars", "French", // where and who
		"price_max", "price_asc", "category_id", "party_size", // what it can extract
		"carries ALL the previous criteria", // follow-ups merge with the last search
		"numbered results",                  // "le deuxième"
		"just asked",                        // answers to the assistant's own question
		"GET_ORDER", "NONE",                 // tracking and small talk
		"Examples",
		"HOTEL (Hotels & accommodation", // the vertical catalogue with the words people use
		"hôtel",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
	if strings.Contains(p, "%VERTICALS%") {
		t.Error("the vertical list was not filled in")
	}
}

func TestSchemaAllowsTheNewSearchCriteria(t *testing.T) {
	props := intentOutputSchema().Properties["parameters"].Properties
	for _, k := range []string{"price_max", "price_min", "sort", "category_id", "nights", "party_size", "location", "vertical", "text"} {
		if _, ok := props[k]; !ok {
			t.Errorf("schema lacks %s", k)
		}
	}
	if got := props["sort"].Enum; len(got) != 3 {
		t.Errorf("sort enum = %v", got)
	}
}

func TestRequestPromptAddsCategoriesAndConversationState(t *testing.T) {
	a := NewIntentClassifierAdapter(&AIOrchestrator{}, WithCategories(func(context.Context) ([]CategoryOption, error) {
		return []CategoryOption{{ID: "cat-1", Name: "Produits"}, {ID: "cat-2", Name: "Smartphones", Parent: "Produits"}}, nil
	}))
	got := a.requestPrompt(context.Background(), "- Last search: vertical HOTEL, place Gombe.\n")
	for _, want := range []string{"Smartphones (in Produits) [category_id cat-2]", "Conversation state:", "Last search: vertical HOTEL"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt lacks %q:\n%s", want, got)
		}
	}

	// Without categories or state the prompt is just the instructions.
	bare := NewIntentClassifierAdapter(&AIOrchestrator{}).requestPrompt(context.Background(), "")
	if strings.Contains(bare, "Conversation state") || strings.Contains(bare, "Categories (") {
		t.Errorf("no categories and no state, yet: %q", bare[len(bare)-120:])
	}
}

func TestCategoryListIsCachedAndAFailureDoesNotBreakThePrompt(t *testing.T) {
	calls := 0
	c := newCategoryCache(func(context.Context) ([]CategoryOption, error) {
		calls++
		return []CategoryOption{{ID: "1", Name: "A"}}, nil
	})
	for i := 0; i < 5; i++ {
		c.Section(context.Background())
	}
	if calls != 1 {
		t.Fatalf("category list loaded %d times", calls)
	}
	failing := newCategoryCache(func(context.Context) ([]CategoryOption, error) { return nil, context.DeadlineExceeded })
	if failing.Section(context.Background()) != "" {
		t.Fatal("a failing category source yields no section")
	}
}
