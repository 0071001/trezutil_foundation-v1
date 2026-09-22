package ai

import (
	"context"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type MockAIProvider struct {
	id        string
	name      string
	status    domain.AIProviderStatus
	models    []domain.AIModel
	latency   time.Duration
	responses map[string]domain.AIResponse
	errors    map[string]domain.AIError

	lastMessages  []domain.AIMessage
	lastRequestID string
}

func (p *MockAIProvider) LastMessages() []domain.AIMessage {
	return p.lastMessages
}

func (p *MockAIProvider) LastRequestID() string {
	return p.lastRequestID
}

func NewMockAIProvider() *MockAIProvider {
	model := domain.AIModel{
		ModelID:            "mock-intent-classifier",
		ProviderID:         "mock",
		Name:               "Mock Intent Classifier",
		Capabilities:       []domain.AIModelCapability{domain.AIModelCapabilityTextClassification, domain.AIModelCapabilityStructuredOutput},
		ContextWindow:      8192,
		SupportsTools:      false,
		SupportsStructured: true,
		Status:             domain.AIModelStatusActive,
		Priority:           100,
	}

	return &MockAIProvider{
		id:        "mock",
		name:      "Mock AI Provider",
		status:    domain.AIProviderStatusActive,
		models:    []domain.AIModel{model},
		latency:   2 * time.Millisecond,
		responses: make(map[string]domain.AIResponse),
		errors:    make(map[string]domain.AIError),
	}
}

func (p *MockAIProvider) ID() string                      { return p.id }
func (p *MockAIProvider) Name() string                    { return p.name }
func (p *MockAIProvider) Status() domain.AIProviderStatus { return p.status }
func (p *MockAIProvider) Models() []domain.AIModel        { return p.models }

func (p *MockAIProvider) SetLatency(d time.Duration) {
	p.latency = d
}

func (p *MockAIProvider) SetResponse(intent string, resp domain.AIResponse) {
	p.responses[strings.ToLower(intent)] = resp
}

func (p *MockAIProvider) SetError(intent string, err domain.AIError) {
	p.errors[strings.ToLower(intent)] = err
}

func (p *MockAIProvider) IsUsable() bool {
	return p.status == domain.AIProviderStatusActive
}

func (p *MockAIProvider) Generate(ctx context.Context, request domain.AIRequest) (domain.AIResponse, error) {
	if p.latency > 0 {
		select {
		case <-time.After(p.latency):
		case <-ctx.Done():
			return domain.AIResponse{}, domain.NewAIError(domain.AIErrProviderTimeout, "context cancelled")
		}
	}

	p.lastMessages = request.Messages
	p.lastRequestID = request.RequestID

	query := extractQuery(request.Messages)
	key := strings.ToLower(query)

	if err, ok := p.errors[key]; ok {
		return domain.AIResponse{}, err
	}

	if resp, ok := p.responses[key]; ok {
		resp.ProviderID = p.id
		resp.ModelID = p.models[0].ModelID
		resp.LatencyMs = p.latency.Milliseconds()
		return resp, nil
	}

	return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "no mock response configured for query")
}

func extractQuery(messages []domain.AIMessage) string {
	for _, msg := range messages {
		if msg.Role == "user" {
			return msg.Content
		}
	}
	return ""
}
