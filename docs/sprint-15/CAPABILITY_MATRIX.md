# Sprint 15 — Capability Matrix

## Overview

This document maps AI capabilities to existing platform capabilities, models, and action types.

## AI Task Capabilities

| AI Task                  | Description                        | Required Model Capability  |
|--------------------------|------------------------------------|---------------------------|
| INTENT_CLASSIFICATION    | Classify user message to action    | TEXT_CLASSIFICATION       |
| PARAMETER_EXTRACTION     | Extract parameters from message    | STRUCTURED_OUTPUT         |
| RESPONSE_GENERATION      | Generate assistant response        | TEXT_GENERATION           |

## Model Capabilities

| Capability              | Description                          |
|-------------------------|--------------------------------------|
| TEXT_GENERATION         | Generate text responses              |
| TEXT_CLASSIFICATION     | Classify text into categories        |
| STRUCTURED_OUTPUT       | Return structured JSON data          |
| VISION                  | Process image inputs                 |

## AI Model Metadata

| Field                  | Type      | Description                              |
|------------------------|-----------|------------------------------------------|
| `ModelID`             | string    | Unique model identifier (e.g., "gpt-4o") |
| `ProviderID`          | string    | Owning provider ID                       |
| `Name`                | string    | Human-readable display name              |
| `Capabilities`        | []string  | What the model can do                    |
| `ContextWindow`       | int       | Max context window in tokens             |
| `SupportsTools`       | bool      | Whether model supports tool/function calling |
| `SupportsStructured`  | bool      | Whether model supports JSON schema output |
| `Status`              | string    | ACTIVE, INACTIVE                         |
| `Priority`            | int       | Routing priority (higher wins)           |

## Action Allowlist (AI-Eligible)

Only actions in this list can be produced by AI intent classification:

| CommerceActionType           | AI Allowlist | Requires Confirmation | Risk Category    |
|------------------------------|--------------|-----------------------|------------------|
| SEARCH_LISTINGS              | Yes          | No                    | READ_ONLY        |
| GET_LISTING                  | Yes          | No                    | READ_ONLY        |
| COMPARE_LISTINGS             | Yes          | No                    | READ_ONLY        |
| SUGGEST                      | Yes          | No                    | READ_ONLY        |
| CHECK_AVAILABILITY           | Yes          | No                    | READ_ONLY        |
| GET_ORDER                    | Yes          | No                    | READ_ONLY        |
| GET_RESERVATION              | Yes          | No                    | READ_ONLY        |
| GET_FULFILLMENT_STATUS       | Yes          | No                    | READ_ONLY        |
| GET_DELIVERY_STATUS          | Yes          | No                    | READ_ONLY        |
| CREATE_ORDER                 | Yes          | Yes                   | LOW_RISK_MUTATION|
| CREATE_RESERVATION           | Yes          | Yes                   | LOW_RISK_MUTATION|
| CANCEL_ORDER                 | Yes          | Yes                   | LOW_RISK_MUTATION|
| CANCEL_RESERVATION           | Yes          | Yes                   | LOW_RISK_MUTATION|
| MODIFY_RESERVATION           | Yes          | Yes                   | LOW_RISK_MUTATION|
| CREATE_LISTING               | No*          | Yes                   | LOW_RISK_MUTATION|
| UPDATE_LISTING               | No*          | Yes                   | LOW_RISK_MUTATION|
| PUBLISH_LISTING              | No*          | Yes                   | LOW_RISK_MUTATION|
| PAUSE_LISTING                | No*          | Yes                   | LOW_RISK_MUTATION|
| RESUME_LISTING               | No*          | Yes                   | LOW_RISK_MUTATION|
| ADJUST_INVENTORY             | No*          | Yes                   | LOW_RISK_MUTATION|
| CREATE_MARKETING_CAMPAIGN    | No*          | Yes                   | LOW_RISK_MUTATION|
| INITIATE_PAYMENT             | No           | Yes                   | FINANCIAL        |
| GET_PAYMENT_STATUS           | Yes          | No                    | READ_ONLY        |
| CHECK_INVENTORY              | Yes          | No                    | READ_ONLY        |

* — AI can suggest parameters for these actions, but the intent classification must be explicitly invoked (e.g., via a "suggest listing" flow). The AI does not independently classify to these action types.

## AI Tasks vs. Model Capabilities

| AI Task                | Model Capability Needed | Structured Output Required |
|------------------------|-------------------------|---------------------------|
| INTENT_CLASSIFICATION  | TEXT_CLASSIFICATION     | Yes                       |
| PARAMETER_EXTRACTION   | STRUCTURED_OUTPUT       | Yes                       |
| RESPONSE_GENERATION    | TEXT_GENERATION         | No (free text)            |

## Provider Eligibility

| Provider Status | Eligible for Routing |
|-----------------|----------------------|
| ACTIVE          | Yes                  |
| INACTIVE        | No                   |
| SUSPENDED       | No                   |
| DEGRADED        | Yes (with warning)   |

## Model Eligibility

| Model Status | Eligible for Routing |
|--------------|----------------------|
| ACTIVE       | Yes                  |
| INACTIVE     | No                   |

## Routing Priority

When multiple models are eligible:

1. Tenant model allowlist (if configured)
2. Model priority (descending)
3. Provider priority (descending)
4. Alphabetical by model ID (deterministic tiebreaker)
