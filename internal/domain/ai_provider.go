package domain

import (
	"context"
	"time"
)

type AIProviderStatus string

const (
	AIProviderStatusActive   AIProviderStatus = "ACTIVE"
	AIProviderStatusInactive AIProviderStatus = "INACTIVE"
)

type AIModelCapability string

const (
	AIModelCapabilityTextGeneration     AIModelCapability = "TEXT_GENERATION"
	AIModelCapabilityTextClassification AIModelCapability = "TEXT_CLASSIFICATION"
	AIModelCapabilityStructuredOutput   AIModelCapability = "STRUCTURED_OUTPUT"
	AIModelCapabilityVision             AIModelCapability = "VISION"
)

type AIModelStatus string

const (
	AIModelStatusActive   AIModelStatus = "ACTIVE"
	AIModelStatusInactive AIModelStatus = "INACTIVE"
)

type AIModel struct {
	ModelID            string
	ProviderID         string
	Name               string
	Capabilities       []AIModelCapability
	ContextWindow      int
	SupportsTools      bool
	SupportsStructured bool
	Status             AIModelStatus
	Priority           int
}

func (m AIModel) IsActive() bool {
	return m.Status == AIModelStatusActive
}

func (m AIModel) SupportsCapability(cap AIModelCapability) bool {
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

type AIProvider interface {
	ID() string
	Name() string
	Status() AIProviderStatus
	Models() []AIModel
	IsUsable() bool
	Generate(ctx context.Context, request AIRequest) (AIResponse, error)
}

// AITextStreamer is a provider that can stream the text it generates: onText receives each piece, in order, as the
// model writes it, and the returned response holds the whole text. A provider that cannot stream simply does not
// implement it; the gateway then answers in one piece.
type AITextStreamer interface {
	GenerateStream(ctx context.Context, request AIRequest, onText func(string)) (AIResponse, error)
}

type AITaskType string

const (
	AITaskIntentClassification AITaskType = "INTENT_CLASSIFICATION"
	AITaskParameterExtraction  AITaskType = "PARAMETER_EXTRACTION"
	AITaskResponseGeneration   AITaskType = "RESPONSE_GENERATION"
)

func (t AITaskType) RequiredCapability() AIModelCapability {
	switch t {
	case AITaskIntentClassification:
		return AIModelCapabilityTextClassification
	case AITaskParameterExtraction:
		return AIModelCapabilityStructuredOutput
	case AITaskResponseGeneration:
		return AIModelCapabilityTextGeneration
	default:
		return ""
	}
}

func (t AITaskType) RequiresStructuredOutput() bool {
	return t == AITaskIntentClassification || t == AITaskParameterExtraction
}

type AIMessage struct {
	Role    string
	Content string
}

type JSONSchema struct {
	Type       string
	Properties map[string]JSONSchemaProperty
	Required   []string
}

type JSONSchemaProperty struct {
	Type        string
	Description string
	Items       *JSONSchemaProperty
	Enum        []string
	// Properties/Required describe a nested object. Providers enforcing a
	// strict output schema treat an object without Properties as empty.
	Properties map[string]JSONSchemaProperty
	Required   []string
}

type AIRequest struct {
	Task             AITaskType
	Messages         []AIMessage
	SystemPrompt     string
	MaxTokens        int
	Temperature      float64
	StructuredOutput bool
	OutputSchema     *JSONSchema
	Timeout          time.Duration
	RequestID        string
	TraceID          string
	ModelID          string
}

type AIResponse struct {
	Content    string
	Structured map[string]interface{}
	TokensUsed int
	ProviderID string
	ModelID    string
	LatencyMs  int64
}

type AIErrorCode string

const (
	AIErrProviderUnavailable    AIErrorCode = "AI_PROVIDER_UNAVAILABLE"
	AIErrProviderTimeout        AIErrorCode = "AI_PROVIDER_TIMEOUT"
	AIErrProviderRateLimited    AIErrorCode = "AI_PROVIDER_RATE_LIMITED"
	AIErrProviderAuthFailed     AIErrorCode = "AI_PROVIDER_AUTH_FAILED"
	AIErrInvalidResponse        AIErrorCode = "AI_INVALID_RESPONSE"
	AIErrSchemaValidationFailed AIErrorCode = "AI_SCHEMA_VALIDATION_FAILED"
	AIErrUnsupportedTask        AIErrorCode = "AI_UNSUPPORTED_TASK"
	AIErrModelUnavailable       AIErrorCode = "AI_MODEL_UNAVAILABLE"
	AIErrContextTooLarge        AIErrorCode = "AI_CONTEXT_TOO_LARGE"
	AIErrUnknown                AIErrorCode = "UNKNOWN_AI_ERROR"
)

type AIError struct {
	Code    AIErrorCode
	Message string
}

func (e AIError) Error() string {
	return string(e.Code) + ": " + e.Message
}

func NewAIError(code AIErrorCode, message string) AIError {
	return AIError{Code: code, Message: message}
}

func (e AIError) IsRetryable() bool {
	switch e.Code {
	case AIErrProviderUnavailable, AIErrProviderTimeout, AIErrProviderRateLimited, AIErrUnknown:
		return true
	default:
		return false
	}
}

type AIExecutionRecord struct {
	ID             string
	OrganizationID string
	ConversationID string
	RequestID      string
	Task           AITaskType
	ProviderID     string
	ModelID        string
	TokensUsed     int
	LatencyMs      int64
	Success        bool
	ErrorCode      string
	ErrorMessage   string
	CreatedAt      int64
}
