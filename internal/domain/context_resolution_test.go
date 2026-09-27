package domain

import (
	"testing"
)

func TestResolveContextualReference_Ordinal(t *testing.T) {
	ctx := &ConversationContext{
		ResultItems: []ResultItemRef{
			{RefType: "listing", ID: "list-1", Index: 0, Title: "iPhone"},
			{RefType: "listing", ID: "list-2", Index: 1, Title: "Samsung"},
			{RefType: "listing", ID: "list-3", Index: 2, Title: "Pixel"},
		},
	}

	tests := []struct {
		input        string
		wantID       string
		wantStrategy ReferenceResolutionStrategy
	}{
		{"the first one", "list-1", StrategyOrdinal},
		{"second listing", "list-2", StrategyOrdinal},
		{"the third item", "list-3", StrategyOrdinal},
		{"last one", "list-3", StrategyOrdinal},
		{"this one", "", StrategyCurrent},
		{"it", "", StrategyCurrent},
		{"some random text", "", StrategyNone},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			ref := ResolveContextualReference(tc.input, ctx)
			if ref == nil {
				if tc.wantStrategy != StrategyNone {
					t.Fatalf("expected non-nil ref, got nil")
				}
				return
			}
			if ref.Strategy != tc.wantStrategy {
				t.Errorf("strategy: got %s, want %s", ref.Strategy, tc.wantStrategy)
			}
			if ref.ResolvedItemID != tc.wantID {
				t.Errorf("resolved ID: got %s, want %s", ref.ResolvedItemID, tc.wantID)
			}
		})
	}
}

func TestResolveContextualReference_CurrentItem(t *testing.T) {
	ctx := &ConversationContext{
		CurrentListing: &ListingRef{ID: "current-listing", Title: "Current Product"},
		ResultItems: []ResultItemRef{
			{RefType: "listing", ID: "list-1", Index: 0, Title: "First"},
		},
	}

	ref := ResolveContextualReference("this one", ctx)
	if ref == nil {
		t.Fatal("expected non-nil ref")
	}
	if ref.Strategy != StrategyCurrent {
		t.Errorf("expected StrategyCurrent, got %s", ref.Strategy)
	}
}

func TestResolveContextualReference_NilContext(t *testing.T) {
	ref := ResolveContextualReference("the first one", nil)
	if ref == nil {
		t.Fatal("expected non-nil ref even with nil context")
	}
	if ref.ResolvedItemID != "" {
		t.Errorf("expected empty resolved ID with nil context, got %s", ref.ResolvedItemID)
	}
}

func TestResolveParameterValue(t *testing.T) {
	ctx := &ConversationContext{
		ResultItems: []ResultItemRef{
			{RefType: "listing", ID: "list-99", Index: 0, Title: "Product A"},
			{RefType: "listing", ID: "list-88", Index: 1, Title: "Product B"},
		},
	}

	// Test contextual reference resolution
	resolved, ok := ResolveParameterValue("listing_id", "the first one", ctx)
	if !ok {
		t.Fatal("expected ok=true for contextual reference")
	}
	if resolved != "list-99" {
		t.Errorf("expected list-99, got %s", resolved)
	}

	// Test non-reference value (no resolution)
	resolved, ok = ResolveParameterValue("listing_id", "list-123", ctx)
	if ok {
		t.Error("expected ok=false for non-reference value")
	}
	if resolved != "list-123" {
		t.Errorf("expected original value, got %s", resolved)
	}
}

func TestParseOrdinal(t *testing.T) {
	tests := []struct {
		input string
		want  int
		ok    bool
	}{
		{"first", 1, true},
		{"second", 2, true},
		{"third", 3, true},
		{"fifth", 5, true},
		{"last", -1, true},
		{"latest", -1, true},
		{"previous", -2, true},
		{"1st", 1, true},
		{"2nd", 2, true},
		{"3rd", 3, true},
		{"4th", 4, true},
		{"unknown", 0, false},
		{"", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, ok := ParseOrdinal(tc.input)
			if ok != tc.ok {
				t.Errorf("ok: got %v, want %v", ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("value: got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestResolveOrdinalReference(t *testing.T) {
	ctx := &ConversationContext{
		ResultItems: []ResultItemRef{
			{RefType: "listing", ID: "list-1", Index: 0, Title: "First"},
			{RefType: "listing", ID: "list-2", Index: 1, Title: "Second"},
			{RefType: "listing", ID: "list-3", Index: 2, Title: "Third"},
		},
	}

	// First item
	item, err := ResolveOrdinalReference("first", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "list-1" {
		t.Errorf("expected list-1, got %s", item.ID)
	}

	// Last item
	item, err = ResolveOrdinalReference("last", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "list-3" {
		t.Errorf("expected list-3, got %s", item.ID)
	}

	// Out of range
	_, err = ResolveOrdinalReference("tenth", ctx)
	if err == nil {
		t.Error("expected error for out of range ordinal")
	}

	// Nil context
	_, err = ResolveOrdinalReference("first", nil)
	if err == nil {
		t.Error("expected error for nil context")
	}
}

func TestDetectEntityType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"the first listing", "listing"},
		{"second product", "listing"},
		{"my order", "order"},
		{"reservation", "reservation"},
		{"book my booking", "reservation"},
		{"something else", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := detectEntityType(tc.input)
			if got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}
