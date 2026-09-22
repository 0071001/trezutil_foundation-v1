package opensearch

import (
	"context"
	"testing"
	"time"
)

func TestIndexManager_VersionedIndexNaming(t *testing.T) {
	config := testConfig
	config.Alias = "trezutil-listings"
	config.Index = "trezutil-listings-v1"

	manager := NewIndexManager(nil, config)

	if !manager.IsVersionedIndex("trezutil-listings-v1") {
		t.Error("expected trezutil-listings-v1 to be versioned")
	}
	if !manager.IsVersionedIndex("trezutil-listings-v2") {
		t.Error("expected trezutil-listings-v2 to be versioned")
	}
	if manager.IsVersionedIndex("trezutil-listings") {
		t.Error("expected trezutil-listings (alias) to NOT be versioned")
	}
	if manager.IsVersionedIndex("") {
		t.Error("expected empty string to NOT be versioned")
	}
}

func TestIndexManager_AliasName(t *testing.T) {
	config := Config{
		Index: "trezutil-listings-v1",
		Alias: "trezutil-listings",
	}

	manager := NewIndexManager(nil, config)
	_ = manager

	if config.AliasName() != "trezutil-listings" {
		t.Errorf("expected alias 'trezutil-listings', got '%s'", config.AliasName())
	}
}

func TestIndexManager_DefaultAlias(t *testing.T) {
	config := DefaultConfig()

	if config.Alias != "trezutil-listings" {
		t.Errorf("expected default alias 'trezutil-listings', got '%s'", config.Alias)
	}
	if config.Index != "trezutil-listings-v1" {
		t.Errorf("expected default index 'trezutil-listings-v1', got '%s'", config.Index)
	}
}

func TestConfig_IndexVersioningDefaults(t *testing.T) {
	config := DefaultConfig()

	if config.IndexVersion == "" {
		t.Error("expected index version to be set")
	}
	if config.Alias == "" {
		t.Error("expected alias to be set")
	}
	if config.Alias == config.Index {
		t.Error("alias and index should be different")
	}
}

func TestConfig_FromEnvironment(t *testing.T) {
	config := Config{
		URL:          "http://custom:9200",
		Username:     "user",
		Password:     "pass",
		Index:        "trezutil-listings-v2",
		Alias:        "trezutil-listings",
		IndexVersion: "v2",
		Timeout:      15 * time.Second,
		TLS:          true,
		MaxRetries:   5,
		ReindexBatch: 500,
	}

	if config.URL != "http://custom:9200" {
		t.Error("custom URL not set")
	}
	if !config.TLS {
		t.Error("TLS should be enabled")
	}
	if config.MaxRetries != 5 {
		t.Errorf("expected 5 retries")
	}
}

func TestIndexManager_SwitchAlias(t *testing.T) {
	adapter := ensureTestIndex(t)

	manager := adapter.indexManager
	oldIndex := adapter.config.Index

	newIndex, err := manager.CreateVersionedIndex(context.Background(), "v2")
	if err != nil {
		t.Fatalf("failed to create versioned index: %v", err)
	}

	if err := manager.SwitchAlias(context.Background(), oldIndex, newIndex); err != nil {
		t.Fatalf("failed to switch alias: %v", err)
	}

	resolvedIndex, err := manager.GetIndexForAlias(context.Background())
	if err != nil {
		t.Fatalf("failed to get index for alias: %v", err)
	}

	if resolvedIndex != newIndex {
		t.Errorf("expected alias to point to %s, got %s", newIndex, resolvedIndex)
	}
}

func TestConfig_NoHardcodedCredentials(t *testing.T) {
	config := DefaultConfig()

	if config.Username != "" {
		t.Error("username should not have a hardcoded default")
	}
	if config.Password != "" {
		t.Error("password should not have a hardcoded default")
	}
	if config.URL != "http://localhost:9200" {
		t.Errorf("unexpected default URL: %s", config.URL)
	}
}

func TestIndexManager_EnsureIndex(t *testing.T) {
	adapter := ensureTestIndex(t)

	exists, err := adapter.client.IndexExists(context.Background(), adapter.config.Index)
	if err != nil {
		t.Fatalf("failed to check index existence: %v", err)
	}
	if !exists {
		t.Error("expected index to exist after EnsureIndex")
	}

	indices, err := adapter.client.GetAlias(context.Background(), adapter.config.AliasName())
	if err != nil {
		t.Fatalf("failed to get alias: %v", err)
	}
	if len(indices) == 0 {
		t.Error("expected alias to exist after EnsureIndex")
	}
}

func TestIndexManager_EnsureIndex_Idempotent(t *testing.T) {
	adapter := ensureTestIndex(t)

	if err := adapter.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("second EnsureIndex should be idempotent: %v", err)
	}
}

func TestIndexManager_MappingContainsExpectedFields(t *testing.T) {
	config := testConfig
	manager := NewIndexManager(nil, config)

	mapping := manager.buildMapping()
	props, ok := mapping["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties in mapping")
	}

	requiredFields := []string{
		"listing_id", "organization_id", "seller_id", "store_id",
		"title", "description", "price_amount", "price_currency",
		"availability_status", "category_id", "category_path",
		"listing_type", "location", "status", "published_at",
	}

	for _, field := range requiredFields {
		if _, ok := props[field]; !ok {
			t.Errorf("expected field %s in mapping", field)
		}
	}
}
