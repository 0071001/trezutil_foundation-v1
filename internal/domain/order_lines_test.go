package domain

import (
	"errors"
	"testing"
)

func TestNormalizeOrderLines(t *testing.T) {
	t.Run("merges repeated listings in first-appearance order", func(t *testing.T) {
		got, err := NormalizeOrderLines([]OrderLine{{"a", 1}, {"b", 2}, {"a", 3}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0] != (OrderLine{"a", 4}) || got[1] != (OrderLine{"b", 2}) {
			t.Fatalf("got %+v", got)
		}
	})
	bad := map[string][]OrderLine{
		"empty":               {},
		"no listing":          {{"", 1}},
		"zero quantity":       {{"a", 0}},
		"negative":            {{"a", -1}},
		"too many of one":     {{"a", MaxLineQuantity + 1}},
		"merged over the cap": {{"a", MaxLineQuantity}, {"a", 1}},
	}
	for name, lines := range bad {
		if _, err := NormalizeOrderLines(lines); !errors.Is(err, ErrInvalidOrderLines) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	many := make([]OrderLine, MaxOrderLines+1)
	for i := range many {
		many[i] = OrderLine{ListingID: string(rune('a' + i)), Quantity: 1}
	}
	if _, err := NormalizeOrderLines(many); !errors.Is(err, ErrInvalidOrderLines) {
		t.Errorf("too many lines: err = %v", err)
	}
}

func TestOrderSameLines(t *testing.T) {
	o := &Order{Items: []*OrderItem{{ListingID: "a", Quantity: 2}, {ListingID: "b", Quantity: 1}}}
	if !o.SameLines([]OrderLine{{"b", 1}, {"a", 2}}) {
		t.Error("same items in another order should match")
	}
	for name, lines := range map[string][]OrderLine{
		"other quantity": {{"a", 3}, {"b", 1}},
		"missing item":   {{"a", 2}},
		"other listing":  {{"a", 2}, {"c", 1}},
	} {
		if o.SameLines(lines) {
			t.Errorf("%s should not match", name)
		}
	}
}
