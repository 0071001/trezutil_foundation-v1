package commerce

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
)

// Search criteria beyond vertical and place: a budget, an ordering and a
// category. They come from the conversation (a model or the keyword router),
// so every value is validated and rewritten in canonical form before it
// reaches the search engine.

// maxSearchPrice bounds a budget; a larger number is a misreading, not a price.
const maxSearchPrice = 1_000_000_000

var categoryIDShape = regexp.MustCompile(`^[0-9a-fA-F-]{16,64}$`)

// searchSorts are the orderings a conversation may ask for.
var searchSorts = map[string]domain.SearchSort{
	"relevance":  {Field: domain.SearchSortFieldRelevance, Direction: domain.SearchSortDirectionDesc},
	"price_asc":  {Field: domain.SearchSortFieldPriceAsc, Direction: domain.SearchSortDirectionAsc},
	"price_desc": {Field: domain.SearchSortFieldPriceDesc, Direction: domain.SearchSortDirectionDesc},
	"newest":     {Field: domain.SearchSortFieldNewest, Direction: domain.SearchSortDirectionDesc},
}

// floatParam reads a number that may arrive as a number or as text ("600",
// "600.50", "$600").
func floatParam(parameters map[string]interface{}, key string) (float64, bool) {
	raw, ok := parameters[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return v, !math.IsNaN(v) && !math.IsInf(v, 0)
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		s := strings.TrimSpace(strings.NewReplacer("$", "", "€", "", "USD", "", "usd", "", " ", "", ",", ".").Replace(v))
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
	}
	return 0, false
}

// normalizeSearchFilters validates price_min, price_max, sort and category_id
// when present and rewrites them canonically (prices as float64, sort as one
// of searchSorts, an empty value removed).
func normalizeSearchFilters(p map[string]interface{}) error {
	for _, k := range []string{"price_min", "price_max"} {
		raw, ok := p[k]
		if !ok || raw == nil {
			continue
		}
		if s, isString := raw.(string); isString && strings.TrimSpace(s) == "" {
			delete(p, k)
			continue
		}
		f, valid := floatParam(p, k)
		if !valid || f < 0 || f > maxSearchPrice {
			return &InvalidParameterError{Param: k, Reason: "must be an amount of zero or more"}
		}
		p[k] = f
	}
	min, hasMin := p["price_min"].(float64)
	max, hasMax := p["price_max"].(float64)
	if hasMin && hasMax && min > max {
		return &InvalidParameterError{Param: "price_min", Reason: "must not be above price_max"}
	}
	if raw, ok := p["sort"]; ok && raw != nil {
		s, isString := raw.(string)
		s = strings.ToLower(strings.TrimSpace(s))
		if isString && s == "" {
			delete(p, "sort")
		} else if _, known := searchSorts[s]; !isString || !known {
			return &InvalidParameterError{Param: "sort", Reason: fmt.Sprintf("must be one of relevance, price_asc, price_desc, newest (got %v)", raw)}
		} else {
			p["sort"] = s
		}
	}
	if raw, ok := p["category_id"]; ok && raw != nil {
		s, isString := raw.(string)
		s = strings.TrimSpace(s)
		switch {
		case isString && s == "":
			delete(p, "category_id")
		case !isString || !categoryIDShape.MatchString(s):
			return &InvalidParameterError{Param: "category_id", Reason: "must be a category identifier"}
		default:
			p["category_id"] = s
		}
	}
	return nil
}

// applySearchFilters copies the validated criteria into the search query.
func applySearchFilters(query *domain.DiscoveryQuery, parameters map[string]interface{}) {
	min, hasMin := floatParam(parameters, "price_min")
	max, hasMax := floatParam(parameters, "price_max")
	if hasMin || hasMax {
		query.PriceRange = &domain.PriceRange{}
		if hasMin {
			query.PriceRange.Min = &min
		}
		if hasMax {
			query.PriceRange.Max = &max
		}
	}
	if s, ok := parameters["sort"].(string); ok {
		if sort, known := searchSorts[strings.ToLower(s)]; known && sort.Field != domain.SearchSortFieldRelevance {
			query.Sort = &sort
		}
	}
	if id, ok := parameters["category_id"].(string); ok && id != "" {
		query.Category = &domain.CategoryFilter{CategoryID: id, IncludeChildren: true}
	}
}
