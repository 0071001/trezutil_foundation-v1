package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func newPromoListing(price float64) *Listing {
	return NewListing("org-1", "seller-1", "store-1", "cat-1", "Phone", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", price)
}

func TestDiscountPercent(t *testing.T) {
	cases := []struct {
		compareAt, price float64
		want             int
	}{
		{100, 80, 20},        // exact, no floating-point shortfall
		{120, 90, 25},        // 25%
		{100, 66.5, 33},      // 33.5% rounds down: never overstated
		{100, 99.5, 0},       // 0.5% is not a discount
		{100, 99, 1},         // smallest displayed discount
		{100, 100, 0},        // no saving
		{100, 120, 0},        // price above the regular price
		{0, 0, 0},            // invalid
		{-5, 1, 0},           // invalid
		{100, -1, 0},         // invalid
		{1000000, 1, 99},     // very deep cut stays within range
		{19.99, 14.99, 25},   // typical retail pair (25.01%)
		{29.99, 20.99, 30},   // 30.01%
		{299.99, 209.99, 30}, // 30.00%: 0.3000100… must not drop to 29
	}
	for _, c := range cases {
		if got := DiscountPercent(c.compareAt, c.price); got != c.want {
			t.Errorf("DiscountPercent(%v, %v) = %d, want %d", c.compareAt, c.price, got, c.want)
		}
	}
}

func TestStartPromotionRecordsRegularPrice(t *testing.T) {
	l := newPromoListing(100)
	if l.Promotion() != nil {
		t.Fatal("a new listing must not be on promotion")
	}
	if err := l.StartPromotion(80); err != nil {
		t.Fatalf("StartPromotion: %v", err)
	}
	if l.PriceAmount != 80 {
		t.Errorf("price = %v, want the sale price 80 (orders must charge it)", l.PriceAmount)
	}
	p := l.Promotion()
	if p == nil || p.CompareAtAmount != 100 || p.DiscountPercent != 20 {
		t.Fatalf("promotion = %+v, want compare-at 100 and 20%%", p)
	}
}

func TestStartPromotionAgainKeepsOriginalRegularPrice(t *testing.T) {
	l := newPromoListing(100)
	_ = l.StartPromotion(80)
	if err := l.StartPromotion(70); err != nil {
		t.Fatalf("changing the sale price: %v", err)
	}
	p := l.Promotion()
	if l.PriceAmount != 70 || p == nil || p.CompareAtAmount != 100 || p.DiscountPercent != 30 {
		t.Fatalf("price %v promotion %+v: the regular price must stay 100 across sale-price changes", l.PriceAmount, p)
	}
}

func TestStartPromotionRejectsInvalidPrices(t *testing.T) {
	for name, sale := range map[string]float64{"zero": 0, "negative": -5, "equal": 100, "above": 120, "under 1 percent": 99.7, "too large": 2e9} {
		l := newPromoListing(100)
		err := l.StartPromotion(sale)
		if !errors.Is(err, ErrInvalidParameter) {
			t.Errorf("%s: err = %v, want ErrInvalidParameter", name, err)
		}
		if l.PriceAmount != 100 || l.Promotion() != nil {
			t.Errorf("%s: a rejected sale must leave the listing untouched (price %v)", name, l.PriceAmount)
		}
	}
}

func TestStartPromotionRoundsToCents(t *testing.T) {
	l := newPromoListing(100)
	if err := l.StartPromotion(79.999); err != nil {
		t.Fatal(err)
	}
	if l.PriceAmount != 80 {
		t.Errorf("price = %v, want 80.00 (prices carry two decimals)", l.PriceAmount)
	}
}

func TestEndPromotionRestoresRegularPrice(t *testing.T) {
	l := newPromoListing(100)
	_ = l.StartPromotion(60)
	if !l.EndPromotion() {
		t.Fatal("EndPromotion must report that a sale ended")
	}
	if l.PriceAmount != 100 || l.Promotion() != nil {
		t.Errorf("price %v promotion %+v, want 100 and no promotion", l.PriceAmount, l.Promotion())
	}
	if _, kept := l.Metadata[PromotionMetadataKey]; kept {
		t.Error("the promotion key must be removed from metadata")
	}
	if l.EndPromotion() {
		t.Error("ending twice must report that there was nothing to end")
	}
}

func TestPromotionIgnoredWhenPriceNoLongerBelowRegular(t *testing.T) {
	l := newPromoListing(100)
	_ = l.StartPromotion(80)
	l.PriceAmount = 100 // repriced to the regular price
	if l.Promotion() != nil {
		t.Error("a sale with no saving must not be reported")
	}
	if l.EndPromotion() {
		t.Error("nothing to restore")
	}
	if l.PriceAmount != 100 {
		t.Errorf("price = %v: cleaning a stale marker must not change the price", l.PriceAmount)
	}
	if _, kept := l.Metadata[PromotionMetadataKey]; kept {
		t.Error("stale marker must be cleaned up")
	}
}

func TestPromotionSurvivesJSONRoundTrip(t *testing.T) {
	// The metadata column is JSONB: numbers come back as float64.
	l := newPromoListing(100)
	_ = l.StartPromotion(75)
	raw, err := json.Marshal(l.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]interface{}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	l.Metadata = back
	if p := l.Promotion(); p == nil || p.CompareAtAmount != 100 || p.DiscountPercent != 25 {
		t.Fatalf("promotion after a JSON round trip = %+v", p)
	}
}

func TestStripPromotionRemovesForgedKey(t *testing.T) {
	in := map[string]interface{}{"color": "red", PromotionMetadataKey: map[string]interface{}{"compare_at_amount": 1e9}}
	out := StripPromotion(in)
	if _, ok := out[PromotionMetadataKey]; ok {
		t.Error("promotion key must be stripped")
	}
	if out["color"] != "red" {
		t.Error("other metadata must be preserved")
	}
	if _, ok := in[PromotionMetadataKey]; !ok {
		t.Error("the input map must not be mutated")
	}
}
