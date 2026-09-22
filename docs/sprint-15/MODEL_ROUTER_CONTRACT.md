# Sprint 15 — Model Router Contract

## Location
`internal/application/ai/router.go`

## Purpose

The Model Router maps AI task capabilities to eligible models, then selects the appropriate provider+model using a deterministic routing strategy.

## AI Task Actions

The router operates on `AIActionType` — the AI task category:

```go
const (
    AITaskIntentClassification AIActionType = "INTENT_CLASSIFICATION"
    AITaskParameterExtraction   AIActionType = "PARAMETER_EXTRACTION"
    AITaskResponseGeneration    AIActionType = "RESPONSE_GENERATION"
)
```

## Model Metadata

```go
type AIModel struct {
    ModelID          string         // Unique model identifier (e.g., "gpt-4o")
    ProviderID       string         // Owning provider
    Name             string         // Human-readable name
    Capabilities     []AIModelCapability
    ContextWindow    int            // Max context window in tokens
    SupportsTools    bool           // Whether the model supports tool calling
    SupportsStructured bool         // Whether the model supports structured output
    Status           AIModelStatus  // ACTIVE, INACTIVE
    Priority         int            // Routing priority (higher wins)
}
```

### AIModelCapability

```go
const (
    AIModelCapabilityTextGeneration     AIModelCapability = "TEXT_GENERATION"
    AIModelCapabilityTextClassification AIModelCapability = "TEXT_CLASSIFICATION"
    AIModelCapabilityStructuredOutput   AIModelCapability = "STRUCTURED_OUTPUT"
    AIModelCapabilityVision             AIModelCapability = "VISION"
)
```

### AIModelStatus

```go
const (
    AIModelActive   AIModelStatus = "ACTIVE"
    AIModelInactive AIModelStatus = "INACTIVE"
)
```

## Routing Strategy (Deterministic)

```
AI Task (capability)
    ↓
1. Find all active providers
    ↓
2. Find all active models supporting the required capability
    ↓
3. Filter by tenant eligibility (configurable per-tenant model allowlist)
    ↓
4. Sort by priority (descending)
    ↓
5. Select the first model
```

### Route API

```go
func (r *ModelRouter) Route(ctx context.Context, task AIActionType) (*AIModel, error)
```

### Eligibility Rules

A model is eligible if:
1. `Status == ACTIVE`
2. `Provider.Status == ACTIVE`
3. Model's `Capabilities` includes the capability required by the task
4. If `SupportsStructured == true` and the task requires structured output, the model must support it

## Provider Abstraction

```go
type AIProvider interface {
    ID() string
    Models() []AIModel
    Generate(ctx context.Context, req AIRequest) (AIResponse, error)
}
```

- No vendor SDK is imported into Commerce Core
- All external provider SDKs are wrapped in adapter implementations at the infrastructure layer
- The initial Sprint 15 implementation uses a `MockAIProvider` for determinism in tests

## Tenant Policy

A per-tenant model allowlist can restrict which models are eligible. If no allowlist is configured, all active models are eligible.

## Design Notes

- The router is intentionally deterministic — no load balancing or latency-based selection in Sprint 15
- Provider availability is checked via `Provider.Status`, not via live health checks (future enhancement)
- The router does NOT make AI decisions about what task to perform — it only routes a given task to the right model
