package opensearch

import "time"

// Config holds OpenSearch configuration.
type Config struct {
	URL          string
	Username     string
	Password     string
	Index        string
	Alias        string
	IndexVersion string
	Timeout      time.Duration
	TLS          bool
	MaxRetries   int
	ReindexBatch int
}

// DefaultConfig returns default OpenSearch configuration.
func DefaultConfig() Config {
	return Config{
		URL:          "http://localhost:9200",
		Username:     "",
		Password:     "",
		Index:        "trezutil-listings-v1",
		Alias:        "trezutil-listings",
		IndexVersion: "v1",
		Timeout:      30 * time.Second,
		TLS:          false,
		MaxRetries:   3,
		ReindexBatch: 1000,
	}
}

// AliasName returns the alias name for the index.
func (c Config) AliasName() string {
	if c.Alias != "" {
		return c.Alias
	}
	return c.Index
}
