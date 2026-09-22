# Sprint 15 — AI Gateway Contract

## Location
`internal/application/ai/gateway.go`

## Responsibilities

- Authentication context propagation (request ID, trace ID)
- Request normalization (prompt + schema → `AIRequest`)
- Model/provider selection via the Model Router
- Request validation and size limits
- Timeout management (default: 30s; configurable per task)
- Tracing (propagates trace IDs)
- Normalized provider invocation
- Error normalization to generic error codes
- Response validation against expected schema
- Safety boundary enforcement (no credential leakage, no PII in logs)

## AIRequest

```go
type AIRequest struct {
    Task             AIActionType     // The AI task (e.g., INTENT_CLASSIFICATION, PARAMETER_EXTRACTION)
    Messages         []AIMessage      // Conversation context (sanitized)
    SystemPrompt     string           // System instruction for this task
    MaxTokens        int              // Maximum output tokens
    Temperature      float64          // Sampling temperature
    StructuredOutput bool             // Whether the response should be structured via schema
    OutputSchema     *JSONSchema      // JSON schema for structured output (if applicable)
    RequestID        string           // Correlation ID
    TraceID          string           // Trace ID for observability
}
```

## AIMessage

```go
type AIMessage struct {
    Role    string `json:"role"`    // "system", "user", "assistant"
    Content string `json:"content"` // Message content
}
```

## AIResponse

```go
type AIResponse struct {
    Content      string                 // The assistant's response content
    Structured   map[string]interface{} // Structured output (if StructuredOutput was requested)
    TokensUsed   int                    // Tokens used by this request
    ProviderID   string                 // Which provider responded
    ModelID      string                 // Which model responded
    LatencyMs    int64                  // Response latency
}
```

## Generate API

```go
func (g *AIGateway) Generate(ctx context.Context, req AIRequest) (*AIResponse, error)
```

### Steps

1. **Validate Request** — ensure required fields are present, message count is within limits
2. **Resolve Model** — delegate to `ModelRouter.Route(ctx, req.Task)`
3. **Normalize Request** — convert `AIRequest` to provider-specific format via adapter
4. **Apply Timeout** — wrap context with deadline from `req.Timeout` or default
5. **Invoke Provider** — call `provider.Generate(ctx, normalizedReq)`
6. **Validate Response** — check response is well-formed, schema matches (if structured)
7. **Normalize Errors** — map provider-specific errors to generic codes
8. **Log Execution** — record provider, model, latency, token usage, success/failure

## Error Normalization

External provider errors are mapped to:

```
AI_PROVIDER_UNAVAILABLE
AI_PROVIDER_TIMEOUT
AI_PROVIDER_RATE_LIMITED
AI_PROVIDER_AUTH_FAILED
AI_INVALID_RESPONSE
AI_SCHEMA_VALIDATION_FAILED
AI_UNSUPPORTED_TASK
AI_MODEL_UNAVAILABLE
AI_CONTEXT_TOO_LARGE
UNKNOWN_AI_ERROR
```

Raw provider errors are logged internally but never returned to end users.

## Constraints

- The AI Gateway must NOT modify tenant, actor, or authentication information
- The AI Gateway must NOT make decisions about commerce authorization
- All prompts and responses containing sensitive content must be redacted in logs
- The AI Gateway must enforce maximum context length (configurable, default 16K tokens worth)
