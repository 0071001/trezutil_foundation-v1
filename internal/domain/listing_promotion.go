package domain

import (
	"fmt"
	"math"
	"time"
)

// A promotion is a sale. The listing keeps selling at its current PriceAmount
// (so orders, payments and search sorting all agree with what customers see);
// the regular price it replaced is remembered under Metadata["promotion"] so
// the discount can be shown and the original price restored when the sale ends.
//
// The key is written only by StartPromotion / EndPromotion. Client-supplied
// metadata can never create, change or hide a promotion (see StripPromotion).

// PromotionMetadataKey is the Listing.Metadata key holding an active promotion.
const PromotionMetadataKey = "promotion"

const promotionCompareAtKey = "compare_at_amount"

// MaxListingPrice bounds any price a seller can set.
const MaxListingPrice = 1e9

// ListingPromotion is the public view of an active sale.
type ListingPromotion struct {
	// CompareAtAmount is the regular price the sale price replaced.
	CompareAtAmount float64
	// DiscountPercent is the whole-number saving, rounded down so a discount is
	// never overstated.
	DiscountPercent int
}

// DiscountPercent returns the saving from compareAt to price as a whole
// percentage, rounded down; 0 when there is no saving or the inputs are invalid.
func DiscountPercent(compareAt, price float64) int {
	if compareAt <= 0 || price < 0 || price >= compareAt {
		return 0
	}
	// The tiny epsilon keeps exact results such as 20% from floating-point
	// noise (28.999999999999996) rounding one point down.
	return int(math.Floor((compareAt-price)*100/compareAt + 1e-9))
}

// roundMoney rounds to two decimals, the precision prices are stored with.
func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

// Promotion returns the listing's active sale, or nil when it has none. A
// stored promotion that is not a real saving of at least one percent (for
// example after a price edit) is treated as absent.
func (l *Listing) Promotion() *ListingPromotion {
	raw, ok := l.Metadata[PromotionMetadataKey].(map[string]interface{})
	if !ok {
		return nil
	}
	compareAt, ok := raw[promotionCompareAtKey].(float64)
	if !ok {
		return nil
	}
	percent := DiscountPercent(compareAt, l.PriceAmount)
	if percent < 1 {
		return nil
	}
	return &ListingPromotion{CompareAtAmount: compareAt, DiscountPercent: percent}
}

// StartPromotion puts the listing on sale at salePrice. The first call records
// the current price as the regular price; calling it again changes the sale
// price while the regular price stays what it was before the sale began.
func (l *Listing) StartPromotion(salePrice float64) error {
	sale := roundMoney(salePrice)
	if math.IsNaN(sale) || math.IsInf(sale, 0) || sale <= 0 || sale > MaxListingPrice {
		return fmt.Errorf("%w: sale price must be a positive amount", ErrInvalidParameter)
	}
	compareAt := l.PriceAmount
	if current := l.Promotion(); current != nil {
		compareAt = current.CompareAtAmount
	}
	if sale >= compareAt {
		return fmt.Errorf("%w: sale price must be below the regular price (%.2f)", ErrInvalidParameter, compareAt)
	}
	if DiscountPercent(compareAt, sale) < 1 {
		return fmt.Errorf("%w: a sale must save at least 1%% off the regular price (%.2f)", ErrInvalidParameter, compareAt)
	}
	if l.Metadata == nil {
		l.Metadata = make(map[string]interface{})
	}
	l.Metadata[PromotionMetadataKey] = map[string]interface{}{promotionCompareAtKey: compareAt}
	l.PriceAmount = sale
	l.UpdatedAt = time.Now().Unix()
	return nil
}

// EndPromotion restores the regular price and clears the sale. It reports
// whether there was a promotion to end.
func (l *Listing) EndPromotion() bool {
	current := l.Promotion()
	if current == nil {
		// A stale marker (no real saving) is cleaned up without touching the price.
		if _, had := l.Metadata[PromotionMetadataKey]; had {
			delete(l.Metadata, PromotionMetadataKey)
			l.UpdatedAt = time.Now().Unix()
		}
		return false
	}
	l.PriceAmount = current.CompareAtAmount
	delete(l.Metadata, PromotionMetadataKey)
	l.UpdatedAt = time.Now().Unix()
	return true
}

// StripPromotion returns a copy of client-supplied metadata without the
// promotion key, so a seller cannot forge a discount through a metadata edit.
func StripPromotion(metadata map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(metadata))
	for k, v := range metadata {
		if k != PromotionMetadataKey {
			out[k] = v
		}
	}
	return out
}
