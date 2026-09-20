package opensearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	opensearch "github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

// Client wraps the OpenSearch client.
type Client struct {
	client *opensearch.Client
	config Config
}

// NewClient creates a new OpenSearch client.
func NewClient(config Config) (*Client, error) {
	clientConfig := opensearch.Config{
		Addresses:  []string{config.URL},
		Username:   config.Username,
		Password:   config.Password,
		MaxRetries: config.MaxRetries,
	}

	if config.TLS {
		clientConfig.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	client, err := opensearch.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenSearch client: %w", err)
	}

	return &Client{
		client: client,
		config: config,
	}, nil
}

// Ping checks if OpenSearch is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req := opensearchapi.PingRequest{}
	resp, err := req.Do(ctx, c.client)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opensearch ping returned status %d", resp.StatusCode)
	}

	_, _ = io.ReadAll(resp.Body)
	return nil
}

// CreateIndex creates an index with the given mapping.
func (c *Client) CreateIndex(ctx context.Context, indexName string, mapping map[string]interface{}) error {
	bodyMap := map[string]interface{}{
		"mappings": mapping,
	}

	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return fmt.Errorf("failed to marshal mapping: %w", err)
	}

	req := opensearchapi.IndicesCreateRequest{
		Index: indexName,
		Body:  bytes.NewReader(bodyBytes),
	}

	resp, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("opensearch create index returned status %d", resp.StatusCode)
	}

	_, _ = io.ReadAll(resp.Body)
	return nil
}

// IndexExists checks if an index exists.
func (c *Client) IndexExists(ctx context.Context, indexName string) (bool, error) {
	req := opensearchapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	resp, err := req.Do(ctx, c.client)
	if err != nil {
		return false, fmt.Errorf("failed to check index existence: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// DeleteIndex deletes an index.
func (c *Client) DeleteIndex(ctx context.Context, indexName string) error {
	req := opensearchapi.IndicesDeleteRequest{
		Index: []string{indexName},
	}

	resp, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to delete index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("opensearch delete index returned status %d", resp.StatusCode)
	}

	_, _ = io.ReadAll(resp.Body)
	return nil
}

// PutAlias creates or updates an alias.
func (c *Client) PutAlias(ctx context.Context, indexName, aliasName string) error {
	req := opensearchapi.IndicesPutAliasRequest{
		Index: []string{indexName},
		Name:  aliasName,
	}

	resp, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to put alias: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("opensearch put alias returned status %d", resp.StatusCode)
	}

	_, _ = io.ReadAll(resp.Body)
	return nil
}

// GetAlias retrieves indices for an alias.
func (c *Client) GetAlias(ctx context.Context, aliasName string) ([]string, error) {
	req := opensearchapi.IndicesGetAliasRequest{
		Name: []string{aliasName},
	}

	resp, err := req.Do(ctx, c.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get alias: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opensearch get alias returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode alias response: %w", err)
	}

	var indices []string
	for idx := range result {
		indices = append(indices, idx)
	}

	return indices, nil
}

// SwitchAlias switches an alias from one index to another atomically.
func (c *Client) SwitchAlias(ctx context.Context, oldIndex, newIndex, aliasName string) error {
	actions := []map[string]interface{}{
		{"remove": map[string]interface{}{"index": oldIndex, "alias": aliasName}},
		{"add": map[string]interface{}{"index": newIndex, "alias": aliasName}},
	}

	bodyMap := map[string]interface{}{
		"actions": actions,
	}

	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return fmt.Errorf("failed to marshal alias switch body: %w", err)
	}

	req := opensearchapi.IndicesUpdateAliasesRequest{
		Body: bytes.NewReader(bodyBytes),
	}

	resp, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to switch alias: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("opensearch switch alias returned status %d", resp.StatusCode)
	}

	_, _ = io.ReadAll(resp.Body)
	return nil
}
