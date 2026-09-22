# Sprint 15 — Provider Integration

## Overview

The AI layer in Sprint 15 must NOT couple Trezutil to any specific AI vendor SDK (OpenAI, Gemini, Anthropic, etc.). All external AI providers are integrated through a clean abstraction.

## AI Provider Interface

```go
type AIProvider interface {
    ID() string
    Models() []AIModel
    Generate(ctx context.Context, request AIRequest) (AIResponse, error)
}
```

## Adapter Pattern

Each external AI provider is wrapped in an adapter:

```
AI Gateway
    ↓
AIProvider interface
    ↓
Adapter (OpenAIAdapter, GeminiAdapter, AnthropicAdapter, etc.)
    ↓
Vendor SDK
```

### Constraints

- The `AIProvider` interface lives in `internal/domain` (or `internal/application/ai`)
- Vendor SDK imports are ONLY in the adapter implementations at `infrastructure/ai/`
- Commerce Core never imports vendor SDKs directly

## Initial Provider Strategy

Sprint 15 ships with:

1. **MockAIProvider** — in-memory, deterministic provider for tests and as the default
   - Returns canned responses for known intent classification scenarios
   - Configurable to simulate errors, latency, and specific structured outputs
   - No external dependencies

2. **Provider Registration** — providers are registered with the Model Router:
   ```go
   router := NewModelRouter()
   router.RegisterProvider(&MockAIProvider{})
   ```

## Future Provider Integrations

Future providers (not in Sprint 15) will be added as adapters:

### OpenAI Adapter
- Implements `AIProvider`
- Wraps `github.com/sashabaran.../openai` or equivalent
- Maps `AIRequest` → OpenAI ChatCompletion request
- Maps OpenAI response → `AIResponse`

### Gemini Adapter
- Implements `AIProvider`
- Maps to Google's generative AI SDK

### Anthropic Adapter
- Implements `AIProvider`
- Maps to Anthropic's SDK

### Self-hosted Provider
- Any provider implementing the interface
- Can be registered the same way

## Environment Configuration

Providers are configured via environment variables or config files:

```yaml
ai:
  providers:
    - id: mock
      enabled: true
      type: mock
    - id: openai
      enabled: false
      type: openai
      api_key_ref: openai-api-key
      base_url: https://api.openai.com/v1
    - id: anthropic
      enabled: false
      type: anthropic
      api_key_ref: anthropic-api-key
```

Secrets are always referenced by name (`api_key_ref`), never stored in plaintext configuration.

## No External API Calls in Sprint 15

The default and only provider in Sprint 15 is the `MockAIProvider`. External provider adapters may be implemented, but they are NOT enabled by default. The sprint does not depend on external AI API availability.

## Provider Availability

Provider availability is checked via the `Provider.Status` field (`ACTIVE`/`INACTIVE`), not via live health checks. A provider must be explicitly enabled to be eligible for routing.

## Integration with CommerceActionRuntime

The AI layer NEVER calls external providers directly. All external capability calls (hotels, inventory, etc.) go through:

```
AI → Intent → CommerceActionRuntime → Commerce Action → Provider Orchestrator (Sprint 14) → External Provider
```

The AI layer only interacts with AI providers for language model inference — never for commerce operations.
