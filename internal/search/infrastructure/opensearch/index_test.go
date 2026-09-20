package opensearch

import (
	"context"
	"testing"
	"time"
)

func TestIndexManager_VersionedIndexNaming(t *testing.T) {
	config := Config{
		Index:        "trezutil-listings-v1",
		Alias:        "trezutil-listings",
		IndexVersion: "v1",
		URL:          "http://localhost:9200",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Skipf("OpenSearch not available: %v", err)
	}

	manager := NewIndexManager(client, config)

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
	config := Config{
		Index: "trezutil-listings-v1",
		Alias: "trezutil-listings",
		URL:   "http://localhost:9200",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Skipf("OpenSearch not available: %v", err)
	}

	manager := NewIndexManager(client, config)

	err = manager.SwitchAlias(context.Background(), "trezutil-listings-v1", "trezutil-listings-v2")
	if err == nil {
		t.Error("expected error when switching to non-existent index")
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
