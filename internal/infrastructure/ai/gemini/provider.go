// Package gemini implements domain.AIProvider on the Google Gemini
// generateContent REST API. Like the Anthropic provider it is one
// interchangeable provider behind the AI Gateway: it interprets text into a
// structured intent and nothing here executes commerce actions.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// ProviderID is the router identifier and the AI_PROVIDER value selecting it.
const ProviderID = "gemini"

// DefaultModel is used when GEMINI_MODEL is not set. A "flash-lite" model is
// fast and inexpensive, supports structured JSON output, and in tests answered
// reliably where the "flash-latest" alias was often overloaded (503/429) and
// slow. Google retires pinned models quickly: set GEMINI_MODEL to move on
// without a code change.
const DefaultModel = "gemini-3.1-flash-lite"

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

var (
	// ErrInvalidModel means the configured model id is malformed or unknown to the API.
	ErrInvalidModel = errors.New("gemini: invalid model")
	// ErrKeyRejected means the API rejected the configured key.
	ErrKeyRejected = errors.New("gemini: API key rejected")

	modelIDPattern = regexp.MustCompile(`^(gemini|gemma)-[a-z0-9]+([.-][a-z0-9]+)*$`)
)

// ValidateModelID checks the shape of a model id (gemini-<version>-<tier>).
// Whether the API serves it is checked by Verify.
func ValidateModelID(id string) error {
	if !modelIDPattern.MatchString(id) || len(id) > 64 {
		return fmt.Errorf("%w: %q is not a Gemini model id (expected e.g. gemini-3.1-flash-lite)", ErrInvalidModel, id)
	}
	return nil
}

// Config configures the provider. APIKey must come from the environment or a
// secret store; it is sent in a header and never logged or returned in errors.
type Config struct {
	APIKey         string
	Model          string
	BaseURL        string // optional, for proxies and tests
	RequestTimeout time.Duration
	HTTPClient     *http.Client
}

// Provider is a domain.AIProvider backed by the Gemini API.
type Provider struct {
	key        string
	model      string
	baseURL    string
	client     *http.Client
	retryDelay time.Duration
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusInternalServerError ||
		status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

// New builds a provider. It fails without an API key rather than starting in a
// state where every call fails.
func New(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("gemini: API key is not configured")
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if err := ValidateModelID(cfg.Model); err != nil {
		return nil, err
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 20 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.RequestTimeout}
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = defaultBaseURL
	}
	return &Provider{key: cfg.APIKey, model: cfg.Model, baseURL: base, client: client, retryDelay: 700 * time.Millisecond}, nil
}

func (p *Provider) ID() string                      { return ProviderID }
func (p *Provider) Name() string                    { return "Google Gemini" }
func (p *Provider) Status() domain.AIProviderStatus { return domain.AIProviderStatusActive }
func (p *Provider) IsUsable() bool                  { return true }

func (p *Provider) Models() []domain.AIModel {
	return []domain.AIModel{{
		ModelID:    p.model,
		ProviderID: ProviderID,
		Name:       p.model,
		Capabilities: []domain.AIModelCapability{
			domain.AIModelCapabilityTextClassification,
			domain.AIModelCapabilityTextGeneration,
			domain.AIModelCapabilityStructuredOutput,
		},
		SupportsStructured: true,
		Status:             domain.AIModelStatusActive,
		Priority:           100,
	}}
}

func (p *Provider) do(ctx context.Context, method, path string, body interface{}) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("x-goog-api-key", p.key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return resp.StatusCode, data, err
}

// Verify asks the API whether it serves the configured model (GET
// /models/{id}) so a wrong GEMINI_MODEL or key is found at startup. Only a
// definite answer is an error: ErrInvalidModel for 404, ErrKeyRejected for
// 400/401/403 (Google answers 400 for an invalid key). A network failure or a
// 5xx returns the raw error and the caller decides.
func (p *Provider) Verify(ctx context.Context) error {
	status, _, err := p.do(ctx, http.MethodGet, "/models/"+url.PathEscape(p.model), nil)
	if err != nil {
		return err
	}
	switch {
	case status == http.StatusOK:
		return nil
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: the API does not serve %q", ErrInvalidModel, p.model)
	case status == http.StatusBadRequest || status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ErrKeyRejected
	}
	return fmt.Errorf("gemini: verify answered HTTP %d", status)
}

type part struct {
	Text string `json:"text"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type generateRequest struct {
	SystemInstruction *content         `json:"systemInstruction,omitempty"`
	Contents          []content        `json:"contents"`
	GenerationConfig  generationConfig `json:"generationConfig"`
}

type thinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type generationConfig struct {
	// ThinkingConfig turns the model's private reasoning off: classifying a
	// shopper's sentence does not need it, and it multiplies latency and cost.
	ThinkingConfig   *thinkingConfig        `json:"thinkingConfig,omitempty"`
	MaxOutputTokens  int                    `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string                 `json:"responseMimeType,omitempty"`
	ResponseSchema   map[string]interface{} `json:"responseSchema,omitempty"`
}

type generateResponse struct {
	Candidates []struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	UsageMetadata struct {
		TotalTokenCount int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
}

// Generate performs one generateContent call. Structured requests are
// constrained to the request's schema; the decoded object is returned in
// Structured and the raw JSON in Content.
func (p *Provider) Generate(ctx context.Context, req domain.AIRequest) (domain.AIResponse, error) {
	system, contents := convertMessages(req)
	if len(contents) == 0 {
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "no user message to interpret")
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4000
	}
	model := p.model
	// The router stamps the request with the model it chose from Models(); a
	// request carrying another provider's model id uses this provider's own.
	if req.ModelID != "" && ValidateModelID(req.ModelID) == nil {
		model = req.ModelID
	}
	body := generateRequest{Contents: contents, GenerationConfig: generationConfig{MaxOutputTokens: maxTokens, ThinkingConfig: &thinkingConfig{ThinkingBudget: 0}}}
	if system != "" {
		body.SystemInstruction = &content{Parts: []part{{Text: system}}}
	}
	structured := req.StructuredOutput && req.OutputSchema != nil
	if structured {
		body.GenerationConfig.ResponseMimeType = "application/json"
		body.GenerationConfig.ResponseSchema = objectSchema(req.OutputSchema.Properties, req.OutputSchema.Required, "")
	}

	status, data, err := p.do(ctx, http.MethodPost, "/models/"+url.PathEscape(model)+":generateContent", body)
	// A model that cannot run without reasoning answers 400 to a zero thinking
	// budget: ask again with its default.
	if err == nil && status == http.StatusBadRequest && bytes.Contains(bytes.ToLower(data), []byte("thinking")) {
		body.GenerationConfig.ThinkingConfig = nil
		status, data, err = p.do(ctx, http.MethodPost, "/models/"+url.PathEscape(model)+":generateContent", body)
	}
	// Google answers 503 "high demand" in short spikes: one more attempt
	// usually succeeds, and is cheaper than failing over to another provider.
	if err == nil && retryable(status) {
		select {
		case <-time.After(p.retryDelay):
			status, data, err = p.do(ctx, http.MethodPost, "/models/"+url.PathEscape(model)+":generateContent", body)
		case <-ctx.Done():
		}
	}
	if err != nil {
		return domain.AIResponse{}, normalizeTransport(ctx, err)
	}
	if status != http.StatusOK {
		return domain.AIResponse{}, normalizeStatus(status, data)
	}
	var out generateResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model returned an unreadable answer")
	}
	if out.PromptFeedback.BlockReason != "" {
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model declined the request")
	}
	if len(out.Candidates) == 0 {
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model returned no candidate")
	}
	cand := out.Candidates[0]
	switch cand.FinishReason {
	case "MAX_TOKENS":
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model output was truncated")
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT":
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model declined the request")
	}
	var text strings.Builder
	for _, pt := range cand.Content.Parts {
		text.WriteString(pt.Text)
	}
	resp := domain.AIResponse{
		Content:    strings.TrimSpace(text.String()),
		TokensUsed: out.UsageMetadata.TotalTokenCount,
		ProviderID: ProviderID,
		ModelID:    model,
	}
	if out.ModelVersion != "" {
		resp.ModelID = out.ModelVersion
	}
	if resp.Content == "" {
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model returned no text")
	}
	if structured {
		if err := json.Unmarshal([]byte(resp.Content), &resp.Structured); err != nil {
			return domain.AIResponse{}, domain.NewAIError(domain.AIErrSchemaValidationFailed, "model output is not valid JSON")
		}
	}
	return resp, nil
}

// convertMessages maps gateway messages to Gemini contents: "system" turns go
// to the system instruction, consecutive same-role turns are merged, and the
// list is trimmed so it starts and ends with a user turn.
func convertMessages(req domain.AIRequest) (string, []content) {
	systemParts := []string{}
	if s := strings.TrimSpace(req.SystemPrompt); s != "" {
		systemParts = append(systemParts, s)
	}
	type turn struct {
		role string
		text []string
	}
	var turns []turn
	for _, m := range req.Messages {
		text := strings.TrimSpace(m.Content)
		if text == "" {
			continue
		}
		role := strings.ToLower(m.Role)
		if role == "system" {
			if !contains(systemParts, text) {
				systemParts = append(systemParts, text)
			}
			continue
		}
		if role == "assistant" {
			role = "model"
		} else {
			role = "user"
		}
		if n := len(turns); n > 0 && turns[n-1].role == role {
			turns[n-1].text = append(turns[n-1].text, text)
			continue
		}
		turns = append(turns, turn{role: role, text: []string{text}})
	}
	for len(turns) > 0 && turns[0].role != "user" {
		turns = turns[1:]
	}
	for len(turns) > 0 && turns[len(turns)-1].role != "user" {
		turns = turns[:len(turns)-1]
	}
	out := make([]content, 0, len(turns))
	for _, t := range turns {
		out = append(out, content{Role: t.role, Parts: []part{{Text: strings.Join(t.text, "\n\n")}}})
	}
	return strings.Join(systemParts, "\n\n"), out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// objectSchema renders the provider-neutral schema in Gemini's OpenAPI-style
// dialect (upper-case type names).
func objectSchema(props map[string]domain.JSONSchemaProperty, required []string, description string) map[string]interface{} {
	out := map[string]interface{}{"type": "OBJECT"}
	properties := map[string]interface{}{}
	for name, prop := range props {
		properties[name] = propertySchema(prop)
	}
	if len(properties) > 0 {
		out["properties"] = properties
	}
	if len(required) > 0 {
		out["required"] = required
	}
	if description != "" {
		out["description"] = description
	}
	return out
}

func propertySchema(p domain.JSONSchemaProperty) map[string]interface{} {
	if p.Type == "object" {
		return objectSchema(p.Properties, p.Required, p.Description)
	}
	out := map[string]interface{}{"type": strings.ToUpper(p.Type)}
	if p.Description != "" {
		out["description"] = p.Description
	}
	if len(p.Enum) > 0 {
		out["enum"] = p.Enum
	}
	if p.Items != nil {
		out["items"] = propertySchema(*p.Items)
	}
	return out
}

// normalizeTransport maps transport failures to gateway error codes.
func normalizeTransport(ctx context.Context, err error) error {
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
		return domain.NewAIError(domain.AIErrProviderTimeout, "AI provider call timed out")
	}
	var ne interface{ Timeout() bool }
	if errors.As(err, &ne) && ne.Timeout() {
		return domain.NewAIError(domain.AIErrProviderTimeout, "AI provider call timed out")
	}
	return domain.NewAIError(domain.AIErrProviderUnavailable, "AI provider unreachable")
}

// normalizeStatus maps HTTP failures to gateway error codes. Provider messages
// are not forwarded: they can echo request content.
func normalizeStatus(status int, body []byte) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return domain.NewAIError(domain.AIErrProviderAuthFailed, "AI provider rejected the credentials")
	case status == http.StatusTooManyRequests:
		return domain.NewAIError(domain.AIErrProviderRateLimited, "AI provider rate limit reached")
	case status == http.StatusNotFound:
		return domain.NewAIError(domain.AIErrModelUnavailable, "configured AI model is not available")
	case status == http.StatusRequestEntityTooLarge:
		return domain.NewAIError(domain.AIErrContextTooLarge, "AI request too large")
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return domain.NewAIError(domain.AIErrProviderTimeout, "AI provider call timed out")
	case status >= 500:
		return domain.NewAIError(domain.AIErrProviderUnavailable, "AI provider temporarily unavailable")
	case status == http.StatusBadRequest && bytes.Contains(bytes.ToLower(body), []byte("api key")):
		return domain.NewAIError(domain.AIErrProviderAuthFailed, "AI provider rejected the credentials")
	default:
		return domain.NewAIError(domain.AIErrInvalidResponse, "AI provider rejected the request")
	}
}

var _ domain.AIProvider = (*Provider)(nil)
