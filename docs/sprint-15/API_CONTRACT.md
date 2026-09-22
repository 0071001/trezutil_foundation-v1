# Sprint 15 — API Contract

## Overview

Sprint 15 does not introduce new public API endpoints for AI execution. Instead, the AI orchestrator integrates with the existing Conversation API flow, maintaining channel neutrality.

## Existing API Integration

### WhatsApp Flow

```
WhatsApp Message
    ↓
WhatsApp Handler (existing)
    ↓
ConversationService.AddMessage
    ↓
AIOrchestrator.Interpret (NEW — internal)
    ↓
CommerceActionRuntime.Execute
    ↓
Conversation Response → WhatsApp
```

The AI orchestrator is called internally by the conversation service — no new external API endpoints are needed for WhatsApp.

### Web/Mobile Flow

```
REST API Request
    ↓
Existing API Handler
    ↓
AIOrchestrator.Interpret (if AI processing is needed)
    ↓
CommerceActionRuntime.Execute
    ↓
API Response
```

## Internal AI Endpoints (Administrative)

### List AI Providers

```
GET /api/v1/admin/ai/providers
```

**Auth:** Platform Admin / Organization Admin

**Returns:**
```json
{
    "data": [
        {
            "id": "mock",
            "name": "Mock Provider",
            "type": "mock",
            "status": "active",
            "models": ["mock-gpt-4"]
        }
    ]
}
```

### List AI Models

```
GET /api/v1/admin/ai/models
```

**Auth:** Platform Admin / Organization Admin

**Returns:**
```json
{
    "data": [
        {
            "model_id": "mock-gpt-4",
            "provider_id": "mock",
            "name": "Mock GPT-4",
            "capabilities": ["TEXT_GENERATION", "STRUCTURED_OUTPUT"],
            "context_window": 8192,
            "supports_structured_output": true,
            "priority": 100,
            "status": "active"
        }
    ]
}
```

### AI Health Check

```
GET /api/v1/admin/ai/health
```

**Auth:** Platform Admin

**Returns:**
```json
{
    "providers": [
        {
            "provider_id": "mock",
            "status": "healthy",
            "latency_ms": 2
        }
    ],
    "models": [
        {
            "model_id": "mock-gpt-4",
            "status": "available"
        }
    ]
}
```

## AI Interpretation Endpoint (Internal)

Used by conversation services to invoke AI interpretation:

```
POST /internal/api/v1/ai/interpret
```

**Note:** This endpoint is NOT publicly exposed. It is only available on the internal service network.

**Request:**
```json
{
    "conversation_id": "uuid",
    "message": "I want to book a hotel in Lubumbashi"
}
```

**Response:**
```json
{
    "intent": "SEARCH_LISTINGS",
    "confidence": 0.91,
    "parameters": {
        "query": "hotel in Lubumbashi"
    },
    "action_result": {
        "status": "SUCCESS",
        "data": { ... }
    },
    "response": {
        "text": "Here are hotels in Lubumbashi...",
        "cards": [ ... ]
    }
}
```

## Authentication & Authorization

All AI endpoints require:
1. Valid authentication token (same as all other APIs)
2. Platform Admin or Organization Admin role for administrative endpoints
3. Internal service token for `/internal/api/v1/ai/*` endpoints

## Security

- No secrets are returned in any AI API response
- `SecretRef` is excluded via `json:"-"` tags
- All responses are filtered through explicit response structs
- Cross-tenant access is rejected with 404
