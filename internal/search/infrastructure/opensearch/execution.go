package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"

	"github.com/trezutil/trezutil/internal/domain"
)

// SearchRequest represents a translated OpenSearch search request.
type SearchRequest struct {
	Index   string
	Body    map[string]interface{}
	Timeout time.Duration
}

// SearchResponse represents a translated OpenSearch search response.
type SearchResponse struct {
	Total    int64       `json:"total"`
	MaxScore float64     `json:"max_score"`
	Hits     []SearchHit `json:"hits"`
	Took     int         `json:"took"`
}

// SearchHit represents a single search result hit.
type SearchHit struct {
	ID     string                 `json:"_id"`
	Score  float64                `json:"_score"`
	Source map[string]interface{} `json:"_source"`
}

// ExecuteSearch executes a search query against OpenSearch.
func (a *OpenSearchAdapter) ExecuteSearch(ctx context.Context, query domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	if err := validateSearchQuery(query); err != nil {
		return domain.SearchResultPage{}, fmt.Errorf("invalid search query: %w", err)
	}

	pageSize := query.Pagination.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	searchBody := buildSearchQuery(query)
	searchBody["size"] = pageSize

	req := opensearchapi.SearchRequest{
		Index:          []string{a.config.AliasName()},
		Body:           bytesReader(searchBody),
		Timeout:        a.config.Timeout,
		TrackTotalHits: true,
	}

	if query.Pagination.Mode == domain.PaginationModeCursor && query.Pagination.Cursor != "" {
		if from, err := decodeCursor(query.Pagination.Cursor); err == nil {
			searchBody = addSearchAfter(searchBody, from)
			req.Body = bytesReader(searchBody)
		}
	}

	resp, err := req.Do(ctx, a.client.client)
	if err != nil {
		return domain.SearchResultPage{}, fmt.Errorf("opensearch search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return domain.SearchResultPage{}, fmt.Errorf("opensearch search returned status %d: %s", resp.StatusCode, string(body))
	}

	var rawResponse struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			MaxScore float64     `json:"max_score"`
			Hits     []SearchHit `json:"hits"`
		} `json:"hits"`
		Took int `json:"took"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return domain.SearchResultPage{}, fmt.Errorf("failed to decode search response: %w", err)
	}

	results := make([]domain.SearchResult, 0, len(rawResponse.Hits.Hits))
	for _, hit := range rawResponse.Hits.Hits {
		result, err := mapHitToSearchResult(hit)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	if len(results) > pageSize {
		results = results[:pageSize]
	}

	hasNext := len(results) == pageSize

	var nextCursor *string
	if hasNext {
		sortField := domain.SearchSortFieldRelevance
		if query.Sort != nil {
			sortField = query.Sort.Field
		}
		cursor, err := CursorFromResult(results[len(results)-1], sortField)
		if err != nil {
			return domain.SearchResultPage{}, fmt.Errorf("failed to encode cursor: %w", err)
		}
		nextCursor = &cursor
	}

	return domain.SearchResultPage{
		Items:      results,
		Total:      int(rawResponse.Hits.Total.Value),
		HasNext:    hasNext,
		NextCursor: nextCursor,
	}, nil
}

// Suggest executes a suggestion query against OpenSearch.
func (a *OpenSearchAdapter) Suggest(ctx context.Context, text string, categoryID *string, limit int) ([]domain.Suggestion, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	// Autocomplete: titles of published listings that start with the typed
	// phrase. (A term suggester only proposes spelling corrections.)
	filters := []interface{}{map[string]interface{}{"term": map[string]interface{}{"status": "published"}}}
	if categoryID != nil && *categoryID != "" {
		filters = append(filters, map[string]interface{}{"term": map[string]interface{}{"category_id": *categoryID}})
	}
	body := map[string]interface{}{
		"size":    limit,
		"_source": []string{"title"},
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{"match_phrase_prefix": map[string]interface{}{"title": map[string]interface{}{"query": text}}},
				},
				"filter": filters,
			},
		},
	}

	req := opensearchapi.SearchRequest{
		Index: []string{a.config.AliasName()},
		Body:  bytesReader(body),
	}

	resp, err := req.Do(ctx, a.client.client)
	if err != nil {
		return nil, fmt.Errorf("opensearch suggest failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensearch suggest returned status %d", resp.StatusCode)
	}

	var rawResponse struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Title string `json:"title"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, fmt.Errorf("failed to decode suggest response: %w", err)
	}

	seen := map[string]bool{}
	suggestions := make([]domain.Suggestion, 0, len(rawResponse.Hits.Hits))
	for _, h := range rawResponse.Hits.Hits {
		title := h.Source.Title
		if title == "" || seen[strings.ToLower(title)] {
			continue
		}
		seen[strings.ToLower(title)] = true
		suggestions = append(suggestions, domain.Suggestion{Text: title, Type: "listing"})
	}

	return suggestions, nil
}

func bytesReader(body map[string]interface{}) io.Reader {
	data, _ := json.Marshal(body)
	return bytes.NewReader(data)
}

func addSearchAfter(body map[string]interface{}, from map[string]interface{}) map[string]interface{} {
	body["search_after"] = from
	return body
}
