package config

type Config struct {
	AppEnv   string
	LogLevel string
	HTTPPort string

	OpenSearchURL          string
	OpenSearchUsername     string
	OpenSearchPassword     string
	OpenSearchIndex        string
	OpenSearchAlias        string
	OpenSearchTimeout      int
	OpenSearchTLS          bool
	OpenSearchMaxRetries   int
	OpenSearchReindexBatch int
}
