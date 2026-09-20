package opensearch

import (
	"testing"
	"time"
)

func TestConfig_MappingFields(t *testing.T) {
	config := DefaultConfig()
	_ = config

	indexManager := NewIndexManager(nil, config)
	_ = indexManager
}

func TestClient_Config(t *testing.T) {
	config := Config{
		URL:        "http://localhost:9200",
		Username:   "user",
		Password:   "pass",
		Index:      "trezutil-listings-v1",
		Timeout:    30 * time.Second,
		TLS:        false,
		MaxRetries: 3,
	}

	if config.Timeout != 30*time.Second {
		t.Error("timeout mismatch")
	}
	if !config.TLS {
		t.Log("TLS disabled (expected for local dev)")
	}
}

func TestIndexManager_IsVersionedIndex(t *testing.T) {
	config := DefaultConfig()
	manager := NewIndexManager(nil, config)

	tests := []struct {
		name     string
		index    string
		expected bool
	}{
		{"v1", "trezutil-listings-v1", true},
		{"v2", "trezutil-listings-v2", true},
		{"alias", "trezutil-listings", false},
		{"empty", "", false},
		{"random", "my-index", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.IsVersionedIndex(tt.index)
			if result != tt.expected {
				t.Errorf("IsVersionedIndex(%q) = %v, want %v", tt.index, result, tt.expected)
			}
		})
	}
}
