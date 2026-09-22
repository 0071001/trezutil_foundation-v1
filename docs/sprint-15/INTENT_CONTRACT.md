# Sprint 15 — Intent Contract

## Location
`internal/domain/ai_intent.go`

## Purpose

The Intent Contract defines the structured output that AI providers produce. This output is the boundary between the untrusted AI layer and the trusted Commerce layer.

## AI Output Schema

```json
{
    "intent": "string (CommerceActionType enum value)",
    "confidence": "float64 (0.0 to 1.0)",
    "parameters": {
        "key": "value"
    },
    "explanation": "string (optional, for debugging)"
}
```

## Intent Resolution

The `AIOrchestrator` validates and converts the AI output to a `domain.ConversationIntent`:

```go
type ConversationIntent struct {
    Name       string                 // "AI_CLASSIFIED_INTENT"
    Confidence float64                // from AI output
    Source     string                 // "AI"
    Entities   []ConversationEntity   // extracted from parameters
    ActionType CommerceActionType     // mapped from "intent" field
}
```

## CommerceActionType Mapping

The AI output `intent` field must match a valid `CommerceActionType`. Valid mappings:

| AI Intent         | CommerceActionType       |
|-------------------|--------------------------|
| SEARCH_LISTINGS   | ActionSearchListings     |
| GET_LISTING       | ActionGetListing         |
| COMPARE_LISTINGS  | ActionCompareListings    |
| SUGGEST           | ActionSuggest            |
| CREATE_LISTING    | ActionCreateListing      |
| UPDATE_LISTING    | ActionUpdateListing      |
| PUBLISH_LISTING   | ActionPublishListing     |
| PAUSE_LISTING     | ActionPauseListing       |
| RESUME_LISTING    | ActionResumeListing      |
| CREATE_ORDER      | ActionCreateOrder        |
| GET_ORDER         | ActionGetOrder           |
| CANCEL_ORDER      | ActionCancelOrder        |
| CHECK_AVAILABILITY| ActionCheckAvailability  |
| CREATE_RESERVATION| ActionCreateReservation  |
| GET_RESERVATION   | ActionGetReservation     |
| CANCEL_RESERVATION| ActionCancelReservation  |
| MODIFY_RESERVATION| ActionModifyReservation  |
| CHECK_INVENTORY   | ActionCheckInventory     |
| ADJUST_INVENTORY  | ActionAdjustInventory    |

## Allowlist Enforcement

Only action types registered in the `CommerceActionRuntime.HandlerRegistry` and explicitly allowlisted for AI execution are eligible.

### AI-Allowlisted Actions

```
SEARCH_LISTINGS     (READ — no confirmation)
GET_LISTING         (READ — no confirmation)
COMPARE_LISTINGS    (READ — no confirmation)
CHECK_AVAILABILITY  (READ — no confirmation)
GET_ORDER           (READ — no confirmation)
GET_RESERVATION     (READ — no confirmation)
```

### Non-Allowlisted (AI cannot suggest)

```
CREATE_LISTING      → AI can suggest params only
CREATE_ORDER        → AI can suggest params only
CREATE_RESERVATION  → AI can suggest params only
CANCEL_RESERVATION  → AI can suggest params only
PUBLISH_LISTING     → AI can suggest params only
PAYMENT             → Never via AI
```

If the AI suggests a non-allowlisted action, the orchestrator returns a confirmation-required response or a safe fallback message.

## Confidence Threshold

Actions below the confidence threshold (default: 0.7) are NOT executed. Instead, the system returns a clarification request to the user.

## Parameter Extraction

AI-extracted parameters that map to intent entities:

| Entity Name      | Source from Parameters    |
|------------------|---------------------------|
| query            | parameters.query          |
| location         | parameters.location       |
| listing_id       | parameters.listing_id     |
| order_id         | parameters.order_id       |
| reservation_id   | parameters.reservation_id |
| date_range       | parameters.start_date, parameters.end_date |
| guest_count      | parameters.guests         |

All AI-extracted parameters are treated as **untrusted** and must pass:
1. Schema validation (JSON schema)
2. CommerceActionRuntime parameter validation
3. Domain-level validation in handlers

## Untrusted Fields

The AI output must NEVER contain:
- `tenant_id`
- `organization_id`
- `user_id`
- `actor_id`
- `role`
- `permissions`
- `confirmation_state`

These are always resolved server-side from the authenticated actor context.
