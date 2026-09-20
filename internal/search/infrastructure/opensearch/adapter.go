package opensearch

import (
	"context"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/projection"
)

// OpenSearchAdapter implements both domain.SearchEnginePort and domain.SearchIndexPort.
type OpenSearchAdapter struct {
	client        *Client
	indexManager  *IndexManager
	documentStore *DocumentStore
	healthChecker *HealthChecker
	config        Config
}

// NewOpenSearchAdapter creates a new OpenSearchAdapter.
func NewOpenSearchAdapter(config Config) (*OpenSearchAdapter, error) {
	client, err := NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenSearch client: %w", err)
	}

	indexManager := NewIndexManager(client, config)
	documentStore := NewDocumentStore(client, config)
	healthChecker := NewHealthChecker(client)

	return &OpenSearchAdapter{
		client:        client,
		indexManager:  indexManager,
		documentStore: documentStore,
		healthChecker: healthChecker,
		config:        config,
	}, nil
}

// IndexListingDocument implements domain.SearchIndexPort.
func (a *OpenSearchAdapter) IndexListingDocument(ctx context.Context, document interface{}) error {
	doc, ok := document.(*projection.SearchListingDocument)
	if !ok {
		return fmt.Errorf("invalid document type: expected *projection.SearchListingDocument")
	}

	return a.documentStore.IndexDocument(ctx, doc)
}

// DeleteListingDocument implements domain.SearchIndexPort.
func (a *OpenSearchAdapter) DeleteListingDocument(ctx context.Context, listingID string) error {
	return a.documentStore.DeleteDocument(ctx, listingID)
}

// EnsureIndex creates the index if it does not exist.
func (a *OpenSearchAdapter) EnsureIndex(ctx context.Context) error {
	return a.indexManager.EnsureIndex(ctx)
}

// Health returns the current health status.
func (a *OpenSearchAdapter) Health(ctx context.Context) domain.SearchHealth {
	available := a.healthChecker.Check(ctx).Available
	if available {
		return domain.SearchHealthAvailable
	}
	return domain.SearchHealthUnavailable
}

// IndexName returns the current index name (for testing/observability).
func (a *OpenSearchAdapter) IndexName() string {
	return a.config.Index
}

// AliasName returns the current alias name (for testing/observability).
func (a *OpenSearchAdapter) AliasName() string {
	return a.config.AliasName()
}
