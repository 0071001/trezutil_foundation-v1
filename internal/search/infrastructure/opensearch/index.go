package opensearch

import (
	"context"
	"fmt"
	"strings"
)

// IndexManager manages OpenSearch index lifecycle.
type IndexManager struct {
	client *Client
	config Config
}

// NewIndexManager creates a new IndexManager.
func NewIndexManager(client *Client, config Config) *IndexManager {
	return &IndexManager{
		client: client,
		config: config,
	}
}

// EnsureIndex creates the index if it does not exist and ensures the alias.
func (m *IndexManager) EnsureIndex(ctx context.Context) error {
	alias := m.config.AliasName()

	exists, err := m.client.IndexExists(ctx, alias)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}

	if exists {
		return nil
	}

	indexName := m.config.Index
	mapping := m.buildMapping()
	if err := m.client.CreateIndex(ctx, indexName, mapping); err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	if err := m.client.PutAlias(ctx, indexName, alias); err != nil {
		return fmt.Errorf("failed to ensure alias: %w", err)
	}

	return nil
}

// EnsureAlias ensures the alias points to the given index.
func (m *IndexManager) EnsureAlias(ctx context.Context, indexName string) error {
	alias := m.config.AliasName()
	if err := m.client.PutAlias(ctx, indexName, alias); err != nil {
		return fmt.Errorf("failed to ensure alias: %w", err)
	}
	return nil
}

// CreateVersionedIndex creates a new versioned index (e.g., trezutil-listings-v2).
func (m *IndexManager) CreateVersionedIndex(ctx context.Context, version string) (string, error) {
	indexName := fmt.Sprintf("trezutil-listings-%s", version)
	mapping := m.buildMapping()
	if err := m.client.CreateIndex(ctx, indexName, mapping); err != nil {
		return "", fmt.Errorf("failed to create versioned index %s: %w", indexName, err)
	}
	return indexName, nil
}

// SwitchAlias atomically switches an alias from one index to another.
func (m *IndexManager) SwitchAlias(ctx context.Context, oldIndex, newIndex string) error {
	return m.client.SwitchAlias(ctx, oldIndex, newIndex, m.config.AliasName())
}

// GetIndexForAlias returns the index currently pointed to by the alias.
func (m *IndexManager) GetIndexForAlias(ctx context.Context) (string, error) {
	indices, err := m.client.GetAlias(ctx, m.config.AliasName())
	if err != nil {
		return "", err
	}
	if len(indices) == 0 {
		return "", fmt.Errorf("no index found for alias: %s", m.config.AliasName())
	}
	return indices[0], nil
}

// IsVersionedIndex returns true if the name follows the trezutil-listings-vN pattern.
func (m *IndexManager) IsVersionedIndex(name string) bool {
	return strings.HasPrefix(name, "trezutil-listings-")
}

// buildMapping returns the OpenSearch mapping for the search index.
func (m *IndexManager) buildMapping() map[string]interface{} {
	return map[string]interface{}{
		"properties": map[string]interface{}{
			"listing_id": map[string]interface{}{
				"type": "keyword",
			},
			"organization_id": map[string]interface{}{
				"type": "keyword",
			},
			"seller_id": map[string]interface{}{
				"type": "keyword",
			},
			"store_id": map[string]interface{}{
				"type": "keyword",
			},
			"title": map[string]interface{}{
				"type":     "text",
				"analyzer": "standard",
				"fields": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"description": map[string]interface{}{
				"type":     "text",
				"analyzer": "standard",
			},
			"category_id": map[string]interface{}{
				"type": "keyword",
			},
			"category_path": map[string]interface{}{
				"type": "keyword",
			},
			"listing_type": map[string]interface{}{
				"type": "keyword",
			},
			"pricing_model": map[string]interface{}{
				"type": "keyword",
			},
			"price_amount": map[string]interface{}{
				"type":           "scaled_float",
				"scaling_factor": 100,
			},
			"price_currency": map[string]interface{}{
				"type": "keyword",
			},
			"availability_status": map[string]interface{}{
				"type": "keyword",
			},
			"quantity": map[string]interface{}{
				"type": "integer",
			},
			"location": map[string]interface{}{
				"type": "geo_point",
			},
			"location_ids": map[string]interface{}{
				"type": "keyword",
			},
			"attributes": map[string]interface{}{
				"type": "nested",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type": "keyword",
					},
					"name": map[string]interface{}{
						"type": "text",
					},
					"type": map[string]interface{}{
						"type": "keyword",
					},
					"value_text": map[string]interface{}{
						"type": "text",
					},
					"value_number": map[string]interface{}{
						"type":           "scaled_float",
						"scaling_factor": 100,
					},
					"value_boolean": map[string]interface{}{
						"type": "boolean",
					},
					"value_date": map[string]interface{}{
						"type": "date",
					},
					"value_select": map[string]interface{}{
						"type": "keyword",
					},
					"searchable": map[string]interface{}{
						"type": "boolean",
					},
				},
			},
			"media_count": map[string]interface{}{
				"type": "integer",
			},
			"primary_media_url": map[string]interface{}{
				"type":  "keyword",
				"index": false,
			},
			"seller_display_name": map[string]interface{}{
				"type":     "text",
				"analyzer": "standard",
				"fields": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"seller_trust_level": map[string]interface{}{
				"type": "byte",
			},
			"store_name": map[string]interface{}{
				"type":     "text",
				"analyzer": "standard",
				"fields": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"store_status": map[string]interface{}{
				"type": "keyword",
			},
			"status": map[string]interface{}{
				"type": "keyword",
			},
			"published_at": map[string]interface{}{
				"type": "date",
			},
			"updated_at": map[string]interface{}{
				"type": "date",
			},
			"freshness_score": map[string]interface{}{
				"type":           "scaled_float",
				"scaling_factor": 1000,
			},
			"metadata": map[string]interface{}{
				"type":    "object",
				"enabled": false,
			},
		},
	}
}
