package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"

	"github.com/trezutil/trezutil/internal/search/projection"
)

// DocumentStore handles OpenSearch document operations.
type DocumentStore struct {
	client *Client
	config Config
}

// NewDocumentStore creates a new DocumentStore.
func NewDocumentStore(client *Client, config Config) *DocumentStore {
	return &DocumentStore{
		client: client,
		config: config,
	}
}

// IndexDocument indexes or updates a document.
func (s *DocumentStore) IndexDocument(ctx context.Context, doc *projection.SearchListingDocument) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	req := opensearchapi.IndexRequest{
		Index:      s.config.Index,
		DocumentID: doc.ListingID,
		Body:       bytes.NewReader(body),
		Refresh:    "wait_for",
	}

	resp, err := req.Do(ctx, s.client.client)
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("opensearch index returned status %d", resp.StatusCode)
	}

	_, _ = io.ReadAll(resp.Body)
	return nil
}

// DeleteDocument removes a document from the index.
func (s *DocumentStore) DeleteDocument(ctx context.Context, listingID string) error {
	req := opensearchapi.DeleteRequest{
		Index:      s.config.Index,
		DocumentID: listingID,
		Refresh:    "wait_for",
	}

	resp, err := req.Do(ctx, s.client.client)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		return fmt.Errorf("opensearch delete returned status %d", resp.StatusCode)
	}

	_, _ = io.ReadAll(resp.Body)
	return nil
}

// BulkIndexDocuments indexes multiple documents in a single request.
func (s *DocumentStore) BulkIndexDocuments(ctx context.Context, docs []*projection.SearchListingDocument) error {
	if len(docs) == 0 {
		return nil
	}

	var body bytes.Buffer
	for _, doc := range docs {
		docBody, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("failed to marshal document: %w", err)
		}

		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": s.config.Index,
				"_id":    doc.ListingID,
			},
		}
		metaBody, _ := json.Marshal(meta)
		body.Write(metaBody)
		body.WriteByte('\n')
		body.Write(docBody)
		body.WriteByte('\n')
	}

	req := opensearchapi.BulkRequest{
		Index: s.config.Index,
		Body:  &body,
	}

	resp, err := req.Do(ctx, s.client.client)
	if err != nil {
		return fmt.Errorf("failed to bulk index documents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("opensearch bulk index returned status %d", resp.StatusCode)
	}

	_, _ = io.ReadAll(resp.Body)
	return nil
}
