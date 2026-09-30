package opensearch

import (
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// buildSearchQuery translates a DiscoveryQuery into OpenSearch DSL.
func buildSearchQuery(query domain.DiscoveryQuery) map[string]interface{} {
	boolQuery := map[string]interface{}{
		"must":                 []interface{}{},
		"filter":               []interface{}{},
		"should":               []interface{}{},
		"minimum_should_match": 1,
	}

	// Full-text search (must)
	if query.Text != "" {
		boolQuery["must"] = append(boolQuery["must"].([]interface{}), map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  query.Text,
				"fields": []string{"title^3", "description", "category_path^2", "seller_display_name", "store_name", "attributes.key^2", "attributes.value_text^2", "attributes.value_select^2"},
				"type":   "best_fields",
			},
		})
	}

	// Visibility: public search only sees published
	boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
		"term": map[string]interface{}{
			"status": "published",
		},
	})

	// Category filter
	if query.Category != nil {
		if query.Category.IncludeChildren {
			boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
				"terms": map[string]interface{}{
					"category_path": []string{query.Category.CategoryID},
				},
			})
		} else {
			boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
				"term": map[string]interface{}{
					"category_id": query.Category.CategoryID,
				},
			})
		}
	}

	// Listing type filter
	if query.ListingType != nil {
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
			"term": map[string]interface{}{
				"listing_type": string(*query.ListingType),
			},
		})
	}

	// Vertical filter
	if query.Vertical != "" {
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
			"term": map[string]interface{}{
				"vertical": string(query.Vertical),
			},
		})
	}

	// Place filter: every word must appear in the place names or the listing text.
	if query.Place != "" {
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":    query.Place,
				"fields":   []string{"place", "title", "description", "store_name"},
				"type":     "cross_fields",
				"operator": "and",
			},
		})
	}

	// Price range filter
	if query.PriceRange != nil {
		priceRange := map[string]interface{}{}
		if query.PriceRange.Min != nil {
			priceRange["gte"] = *query.PriceRange.Min
		}
		if query.PriceRange.Max != nil {
			priceRange["lte"] = *query.PriceRange.Max
		}
		if len(priceRange) > 0 {
			boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
				"range": map[string]interface{}{
					"price_amount": priceRange,
				},
			})
		}
	}

	// Currency filter
	if query.Currency != "" {
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
			"term": map[string]interface{}{
				"price_currency": query.Currency,
			},
		})
	}

	// Availability filter
	if query.Availability != nil {
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
			"term": map[string]interface{}{
				"availability_status": string(*query.Availability),
			},
		})
	}

	// Seller filter
	if query.SellerID != "" {
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
			"term": map[string]interface{}{
				"seller_id": query.SellerID,
			},
		})
	}

	// Store filter
	if query.StoreID != "" {
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), map[string]interface{}{
			"term": map[string]interface{}{
				"store_id": query.StoreID,
			},
		})
	}

	// Attribute filters
	for _, attr := range query.Attributes {
		attrFilter := buildAttributeFilter(attr)
		if attrFilter != nil {
			boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), attrFilter)
		}
	}

	// Geo filter
	if query.Location != nil && query.RadiusMeters != nil {
		geoFilter := map[string]interface{}{
			"geo_distance": map[string]interface{}{
				"distance": fmt.Sprintf("%fm", *query.RadiusMeters),
				"location": map[string]interface{}{
					"lat": *query.Location.Latitude,
					"lon": *query.Location.Longitude,
				},
			},
		}
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), geoFilter)
	}

	// Score aggregation for ranking
	shouldClauses := buildRankingSignals(query)
	boolQuery["should"] = shouldClauses

	// Only require minimum_should_match when there are actual should clauses.
	// When should is empty, minimum_should_match=1 returns zero results in real OpenSearch.
	if len(shouldClauses) > 0 {
		boolQuery["minimum_should_match"] = 1
	} else {
		boolQuery["minimum_should_match"] = 0
	}

	// Sort
	sort := buildSort(query)

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
		},
		"sort":             sort,
		"size":             query.Pagination.PageSize,
		"from":             0,
		"track_total_hits": true,
	}

	return body
}

func buildAttributeFilter(attr domain.AttributeFilter) map[string]interface{} {
	key := attr.Key
	operator := string(attr.Operator)

	nestedPath := "attributes"

	switch operator {
	case "eq":
		return map[string]interface{}{
			"nested": map[string]interface{}{
				"path": nestedPath,
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{
							map[string]interface{}{"term": map[string]interface{}{nestedPath + ".key": key}},
							map[string]interface{}{"term": map[string]interface{}{nestedPath + ".value_select": fmt.Sprintf("%v", attr.Value)}},
						},
					},
				},
			},
		}
	case "contains":
		return map[string]interface{}{
			"nested": map[string]interface{}{
				"path": nestedPath,
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{
							map[string]interface{}{"term": map[string]interface{}{nestedPath + ".key": key}},
							map[string]interface{}{"wildcard": map[string]interface{}{nestedPath + ".value_text": "*" + fmt.Sprintf("%v", attr.Value) + "*"}},
						},
					},
				},
			},
		}
	case "search":
		return map[string]interface{}{
			"nested": map[string]interface{}{
				"path": nestedPath,
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{
							map[string]interface{}{"term": map[string]interface{}{nestedPath + ".key": key}},
							map[string]interface{}{"match": map[string]interface{}{nestedPath + ".value_text": fmt.Sprintf("%v", attr.Value)}},
						},
					},
				},
			},
		}
	default:
		// gt, gte, lt, lte for numeric values
		if attr.Value != nil {
			rangeOp := ""
			switch operator {
			case "gt":
				rangeOp = "gt"
			case "gte":
				rangeOp = "gte"
			case "lt":
				rangeOp = "lt"
			case "lte":
				rangeOp = "lte"
			}
			if rangeOp != "" {
				return map[string]interface{}{
					"nested": map[string]interface{}{
						"path": nestedPath,
						"query": map[string]interface{}{
							"bool": map[string]interface{}{
								"must": []interface{}{
									map[string]interface{}{"term": map[string]interface{}{nestedPath + ".key": key}},
									map[string]interface{}{"range": map[string]interface{}{
										nestedPath + ".value_number": map[string]interface{}{
											rangeOp: attr.Value,
										},
									}},
								},
							},
						},
					},
				}
			}
		}
	}

	return nil
}

func buildRankingSignals(query domain.DiscoveryQuery) []interface{} {
	signals := []interface{}{}

	// Geo-distance signal: closer locations are better (only when location is specified)
	if query.Location != nil && query.RadiusMeters != nil && *query.RadiusMeters > 0 {
		signals = append(signals, map[string]interface{}{
			"geo_distance": map[string]interface{}{
				"distance": fmt.Sprintf("%fm", *query.RadiusMeters),
				"location": map[string]interface{}{
					"lat": *query.Location.Latitude,
					"lon": *query.Location.Longitude,
				},
				"boost": 1.2,
			},
		})
	}

	// Category match boost
	if query.Category != nil {
		signals = append(signals, map[string]interface{}{
			"term": map[string]interface{}{
				"category_path": map[string]interface{}{
					"value": query.Category.CategoryID,
					"boost": 1.1,
				},
			},
		})
	}

	return signals
}

func buildSort(query domain.DiscoveryQuery) []interface{} {
	if query.Sort == nil {
		return []interface{}{
			map[string]interface{}{
				"_score": map[string]interface{}{
					"order": "desc",
				},
			},
			map[string]interface{}{
				"published_at": map[string]interface{}{
					"order": "desc",
				},
			},
		}
	}

	switch query.Sort.Field {
	case domain.SearchSortFieldRelevance:
		return []interface{}{
			map[string]interface{}{
				"_score": map[string]interface{}{"order": "desc"},
			},
			map[string]interface{}{
				"published_at": map[string]interface{}{"order": "desc"},
			},
		}
	case domain.SearchSortFieldDistance:
		if query.Location != nil {
			return []interface{}{
				map[string]interface{}{
					"_geo_distance": map[string]interface{}{
						"location": map[string]interface{}{
							"lat": *query.Location.Latitude,
							"lon": *query.Location.Longitude,
						},
						"order": "asc",
						"unit":  "m",
					},
				},
			}
		}
	case domain.SearchSortFieldPriceAsc:
		return []interface{}{
			map[string]interface{}{
				"price_amount": map[string]interface{}{"order": "asc"},
			},
		}
	case domain.SearchSortFieldPriceDesc:
		return []interface{}{
			map[string]interface{}{
				"price_amount": map[string]interface{}{"order": "desc"},
			},
		}
	case domain.SearchSortFieldNewest:
		return []interface{}{
			map[string]interface{}{
				"published_at": map[string]interface{}{"order": "desc"},
			},
		}
	case domain.SearchSortFieldOldest:
		return []interface{}{
			map[string]interface{}{
				"published_at": map[string]interface{}{"order": "asc"},
			},
		}
	}

	return []interface{}{map[string]interface{}{"_score": map[string]interface{}{"order": "desc"}}}
}
