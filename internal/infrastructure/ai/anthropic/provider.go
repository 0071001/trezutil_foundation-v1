// Package anthropic implements domain.AIProvider on the Anthropic Messages
// API. It is one interchangeable provider behind the AI Gateway: the gateway,
// model router, capability matrix and action allowlist decide what the model
// is used for, and nothing here executes commerce actions.
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/trezutil/trezutil/internal/domain"
)

// ProviderID is the router identifier and the AI_PROVIDER value selecting it.
const ProviderID = "anthropic"

// DefaultModel is used when AI_MODEL is not set. It is the ONLY place the
// default lives: every service reads AI_MODEL and falls back to this constant.
// "claude-opus-5" is a documented Anthropic API model id; set AI_MODEL to move
// to another model (for example "claude-opus-5-5") without a code change.
const DefaultModel = "claude-opus-5"

var (
	// ErrInvalidModel means the configured model id is malformed or unknown to
	// the API. It is a configuration error: the service refuses to start.
	ErrInvalidModel = errors.New("anthropic: invalid model")
	// ErrKeyRejected means the API rejected the configured key.
	ErrKeyRejected = errors.New("anthropic: API key rejected")

	modelIDPattern = regexp.MustCompile(`^claude-[a-z0-9]+(-[a-z0-9.]+)*$`)
)

// ValidateModelID checks the shape of a model id (claude-<family>-<version>,
// optionally date-suffixed). Whether the API serves it is checked by Verify.
func ValidateModelID(id string) error {
	if !modelIDPattern.MatchString(id) || len(id) > 64 {
		return fmt.Errorf("%w: %q is not a Claude model id (expected e.g. claude-opus-5-5)", ErrInvalidModel, id)
	}
	return nil
}

// Config configures the provider. APIKey must come from the environment or a
// secret store; it is never logged or returned in errors.
type Config struct {
	APIKey  string
	Model   string
	BaseURL string // optional, for proxies and tests
	// Effort trades depth for latency/cost: low | medium | high. Intent
	// classification is a short task, so low is the default.
	Effort string
	// RequestTimeout bounds each HTTP attempt; the gateway's context deadline
	// bounds the whole call including retries.
	RequestTimeout time.Duration
	// MaxRetries for 408/409/429/5xx and connection errors (SDK backoff).
	MaxRetries int
	HTTPClient *http.Client
}

// Provider is a domain.AIProvider backed by the Anthropic Messages API.
type Provider struct {
	client sdk.Client
	model  string
	effort sdk.OutputConfigEffort
}

// New builds a provider. It fails without an API key rather than starting in
// a state where every call fails.
func New(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("anthropic: API key is not configured")
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
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithMaxRetries(cfg.MaxRetries),
		option.WithRequestTimeout(cfg.RequestTimeout),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if cfg.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(cfg.HTTPClient))
	}
	effort := sdk.OutputConfigEffort(strings.ToLower(cfg.Effort))
	switch effort {
	case sdk.OutputConfigEffortLow, sdk.OutputConfigEffortMedium, sdk.OutputConfigEffortHigh:
	default:
		effort = sdk.OutputConfigEffortLow
	}
	return &Provider{client: sdk.NewClient(opts...), model: cfg.Model, effort: effort}, nil
}

// Verify asks the API whether it serves the configured model (GET
// /v1/models/{id}) so a wrong AI_MODEL or key is found at startup, not on the
// first customer request. Only a definite answer is an error: ErrInvalidModel
// for 400/404, ErrKeyRejected for 401/403. A network failure or a 5xx returns
// the raw error and the caller decides (the service starts and keeps its
// keyword fallback).
func (p *Provider) Verify(ctx context.Context) error {
	_, err := p.client.Models.Get(ctx, p.model, sdk.ModelGetParams{})
	if err == nil {
		return nil
	}
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadRequest, http.StatusNotFound:
			return fmt.Errorf("%w: the API does not serve %q", ErrInvalidModel, p.model)
		case http.StatusUnauthorized, http.StatusForbidden:
			return ErrKeyRejected
		}
	}
	return err
}

func (p *Provider) ID() string                      { return ProviderID }
func (p *Provider) Name() string                    { return "Anthropic" }
func (p *Provider) Status() domain.AIProviderStatus { return domain.AIProviderStatusActive }
func (p *Provider) IsUsable() bool                  { return true }

// Models advertises the single configured model to the router.
func (p *Provider) Models() []domain.AIModel {
	return []domain.AIModel{{
		ModelID:    p.model,
		ProviderID: ProviderID,
		Name:       p.model,
		Capabilities: []domain.AIModelCapability{
			domain.AIModelCapabilityTextGeneration,
			domain.AIModelCapabilityTextClassification,
			domain.AIModelCapabilityStructuredOutput,
		},
		SupportsTools:      true,
		SupportsStructured: true,
		Status:             domain.AIModelStatusActive,
		Priority:           100,
	}}
}

// Generate performs one Messages API call. Structured requests are
// constrained to the request's schema; the decoded object is returned in
// Structured and the raw JSON in Content.
func (p *Provider) Generate(ctx context.Context, req domain.AIRequest) (domain.AIResponse, error) {
	system, messages := convertMessages(req)
	if len(messages) == 0 {
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "no user message to interpret")
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4000
	}
	model := req.ModelID
	if model == "" {
		model = p.model
	}

	params := sdk.MessageNewParams{
		Model:        sdk.Model(model),
		MaxTokens:    int64(maxTokens),
		Messages:     messages,
		OutputConfig: sdk.OutputConfigParam{Effort: p.effort},
	}
	if system != "" {
		params.System = []sdk.TextBlockParam{{Text: system}}
	}
	structured := req.StructuredOutput && req.OutputSchema != nil
	if structured {
		params.OutputConfig.Format = sdk.JSONOutputFormatParam{Schema: schemaToMap(req.OutputSchema)}
	}
	// Temperature is intentionally not sent: current models reject sampling
	// parameters, and the output schema already constrains the answer.

	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return domain.AIResponse{}, normalizeError(ctx, err)
	}

	switch msg.StopReason {
	case sdk.StopReasonRefusal:
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model declined the request")
	case sdk.StopReasonMaxTokens:
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model output was truncated")
	}

	var text strings.Builder
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(sdk.TextBlock); ok {
			text.WriteString(tb.Text)
		}
	}
	resp := domain.AIResponse{
		Content:    strings.TrimSpace(text.String()),
		TokensUsed: int(msg.Usage.InputTokens + msg.Usage.OutputTokens),
		ProviderID: ProviderID,
		ModelID:    string(msg.Model),
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

// GenerateStream performs one streamed Messages API call (domain.AITextStreamer): each text delta goes to onText as
// the model writes it. Used for writing answers, never for structured output.
func (p *Provider) GenerateStream(ctx context.Context, req domain.AIRequest, onText func(string)) (domain.AIResponse, error) {
	system, messages := convertMessages(req)
	if len(messages) == 0 {
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "no user message to answer")
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1500
	}
	model := req.ModelID
	if model == "" {
		model = p.model
	}
	params := sdk.MessageNewParams{
		Model:        sdk.Model(model),
		MaxTokens:    int64(maxTokens),
		Messages:     messages,
		OutputConfig: sdk.OutputConfigParam{Effort: p.effort},
	}
	if system != "" {
		params.System = []sdk.TextBlockParam{{Text: system}}
	}

	stream := p.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()
	msg := sdk.Message{}
	for stream.Next() {
		event := stream.Current()
		if err := msg.Accumulate(event); err != nil {
			return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "AI stream could not be read")
		}
		if delta, ok := event.AsAny().(sdk.ContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(sdk.TextDelta); ok && text.Text != "" && onText != nil {
				onText(text.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return domain.AIResponse{}, normalizeError(ctx, err)
	}
	switch msg.StopReason {
	case sdk.StopReasonRefusal:
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model declined the request")
	case sdk.StopReasonMaxTokens:
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model output was truncated")
	}
	var text strings.Builder
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(sdk.TextBlock); ok {
			text.WriteString(tb.Text)
		}
	}
	resp := domain.AIResponse{
		Content:    strings.TrimSpace(text.String()),
		TokensUsed: int(msg.Usage.InputTokens + msg.Usage.OutputTokens),
		ProviderID: ProviderID,
		ModelID:    string(msg.Model),
	}
	if resp.Content == "" {
		return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "model returned no text")
	}
	return resp, nil
}

// convertMessages maps gateway messages to the Messages API: "system" turns
// go to the system prompt, consecutive same-role turns are merged, and the
// list is trimmed so it starts and ends with a user turn.
func convertMessages(req domain.AIRequest) (string, []sdk.MessageParam) {
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
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		role := strings.ToLower(m.Role)
		if role == "system" {
			if !contains(systemParts, content) {
				systemParts = append(systemParts, content)
			}
			continue
		}
		if role != "assistant" {
			role = "user"
		}
		if n := len(turns); n > 0 && turns[n-1].role == role {
			turns[n-1].text = append(turns[n-1].text, content)
			continue
		}
		turns = append(turns, turn{role: role, text: []string{content}})
	}
	for len(turns) > 0 && turns[0].role != "user" {
		turns = turns[1:]
	}
	for len(turns) > 0 && turns[len(turns)-1].role != "user" {
		turns = turns[:len(turns)-1]
	}
	messages := make([]sdk.MessageParam, 0, len(turns))
	for _, t := range turns {
		block := sdk.NewTextBlock(strings.Join(t.text, "\n\n"))
		if t.role == "assistant" {
			messages = append(messages, sdk.NewAssistantMessage(block))
		} else {
			messages = append(messages, sdk.NewUserMessage(block))
		}
	}
	return strings.Join(systemParts, "\n\n"), messages
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// schemaToMap renders the provider-neutral schema as JSON Schema. Every object
// is closed (additionalProperties: false) as strict structured output needs.
func schemaToMap(s *domain.JSONSchema) map[string]any {
	return objectSchema(s.Properties, s.Required, "")
}

func objectSchema(props map[string]domain.JSONSchemaProperty, required []string, description string) map[string]any {
	out := map[string]any{"type": "object", "additionalProperties": false}
	properties := map[string]any{}
	for name, prop := range props {
		properties[name] = propertySchema(prop)
	}
	out["properties"] = properties
	if len(required) > 0 {
		out["required"] = required
	}
	if description != "" {
		out["description"] = description
	}
	return out
}

func propertySchema(p domain.JSONSchemaProperty) map[string]any {
	if p.Type == "object" {
		return objectSchema(p.Properties, p.Required, p.Description)
	}
	out := map[string]any{"type": p.Type}
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

// normalizeError maps SDK/transport failures to gateway error codes. Provider
// messages are not forwarded: they can echo request content.
func normalizeError(ctx context.Context, err error) error {
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
		return domain.NewAIError(domain.AIErrProviderTimeout, "AI provider call timed out")
	}
	var apiErr *sdk.Error
	if !errors.As(err, &apiErr) {
		return domain.NewAIError(domain.AIErrProviderUnavailable, "AI provider unreachable")
	}
	switch code := apiErr.StatusCode; {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return domain.NewAIError(domain.AIErrProviderAuthFailed, "AI provider rejected the credentials")
	case code == http.StatusTooManyRequests:
		return domain.NewAIError(domain.AIErrProviderRateLimited, "AI provider rate limit reached")
	case code == http.StatusNotFound:
		return domain.NewAIError(domain.AIErrModelUnavailable, "configured AI model is not available")
	case code == http.StatusRequestEntityTooLarge:
		return domain.NewAIError(domain.AIErrContextTooLarge, "AI request too large")
	case code == http.StatusRequestTimeout:
		return domain.NewAIError(domain.AIErrProviderTimeout, "AI provider call timed out")
	case code >= 500:
		return domain.NewAIError(domain.AIErrProviderUnavailable, "AI provider temporarily unavailable")
	case code == http.StatusBadRequest && strings.Contains(strings.ToLower(apiErr.Error()), "credit balance"):
		// The Anthropic account has run out of credit: unavailable until credit is added, not a bad request.
		return domain.NewAIError(domain.AIErrProviderUnavailable, "AI provider account is out of credit")
	case code == http.StatusBadRequest && strings.Contains(strings.ToLower(apiErr.Error()), "prompt is too long"):
		return domain.NewAIError(domain.AIErrContextTooLarge, "conversation too long for the model")
	default:
		return domain.NewAIError(domain.AIErrInvalidResponse, "AI provider rejected the request")
	}
}

var (
	_ domain.AIProvider     = (*Provider)(nil)
	_ domain.AITextStreamer = (*Provider)(nil)
)
