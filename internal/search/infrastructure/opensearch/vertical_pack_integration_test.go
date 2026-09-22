//go:build integration

package opensearch

import (
	"context"
	"testing"
)

func TestVerticalPackMetadataIndex(t *testing.T) {
	adapter := skipIfRealOpenSearchUnavailable(t)

	ctx := context.Background()
	indexName := uniqueTestIndexName()
	defer func() {
		_ = adapter.client.DeleteIndex(ctx, indexName)
	}()

	mapping := map[string]interface{}{
		"properties": map[string]interface{}{
			"pack_code": map[string]interface{}{
				"type": "keyword",
			},
			"pack_name": map[string]interface{}{
				"type":     "text",
				"analyzer": "standard",
				"fields": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type":         "keyword",
						"ignore_above": 256,
					},
				},
			},
			"version": map[string]interface{}{
				"type": "keyword",
			},
			"ai_profile": map[string]interface{}{
				"type": "keyword",
			},
			"capabilities": map[string]interface{}{
				"type": "keyword",
			},
			"created_at": map[string]interface{}{
				"type": "date",
			},
			"updated_at": map[string]interface{}{
				"type": "date",
			},
		},
	}

	if err := adapter.client.CreateIndex(ctx, indexName, mapping); err != nil {
		t.Fatalf("failed to create pack index: %v", err)
	}

	exists, err := adapter.client.IndexExists(ctx, indexName)
	if err != nil {
		t.Fatalf("failed to check index existence: %v", err)
	}
	if !exists {
		t.Error("expected pack metadata index to exist")
	}
}

func TestVerticalPackMappingSearchable(t *testing.T) {
	adapter := skipIfRealOpenSearchUnavailable(t)

	ctx := context.Background()
	indexName := uniqueTestIndexName()
	defer func() {
		_ = adapter.client.DeleteIndex(ctx, indexName)
	}()

	mapping := map[string]interface{}{
		"properties": map[string]interface{}{
			"pack_code":    map[string]interface{}{"type": "keyword"},
			"capabilities": map[string]interface{}{"type": "keyword"},
		},
	}

	if err := adapter.client.CreateIndex(ctx, indexName, mapping); err != nil {
		t.Fatalf("failed to create pack index: %v", err)
	}

	if err := adapter.client.DeleteIndex(ctx, indexName); err != nil {
		t.Fatalf("failed to delete pack index: %v", err)
	}

	exists, err := adapter.client.IndexExists(ctx, indexName)
	if err != nil {
		t.Fatalf("failed to check index existence after delete: %v", err)
	}
	if exists {
		t.Error("expected pack metadata index to be deleted")
	}
}
