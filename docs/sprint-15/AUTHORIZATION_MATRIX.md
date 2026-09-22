# Sprint 15 — Authorization Matrix

## Overview

This document defines the authorization matrix for AI-related operations. All authorization is server-side and enforced by the existing CommerceActionRuntime.

## AI Component Roles

| Component              | Trust Level     | Can Execute Actions | Can Set Actor | Can Set Tenant | Can Bypass RBAC |
|------------------------|-----------------|---------------------|---------------|----------------|-----------------|
| AI Provider (external) | Untrusted       | No                  | No            | No             | No              |
| AI Gateway             | Untrusted       | No                  | No            | No             | No              |
| Model Router           | Internal        | No                  | No            | No             | No              |
| AI Orchestrator        | Internal        | No                  | No            | No             | No              |
| CommerceActionRuntime  | Trusted         | Yes (via override)  | Yes (override)| Yes (override)| Enforces RBAC   |

## Action Authorization

All commerce actions suggested by AI must pass RBAC authorization. The actor is always the authenticated user — never the AI.

### RBAC Rules (Existing — Sprint 2)

| Actor Type              | Can Perform                          |
|-------------------------|--------------------------------------|
| Customer                | Search listings, create/manage own orders, create/manage own reservations |
| Seller                  | All customer actions + create/update own listings, manage own orders |
| OrganizationAdmin       | All seller actions + manage all listings in org |
| PlatformOperator        | All actions across all tenants       |

## AI Task Authorization

| AI Task                   | Permission Required       |
|---------------------------|---------------------------|
| INTENT_CLASSIFICATION     | Conversation access       |
| PARAMETER_EXTRACTION      | Conversation access       |
| RESPONSE_GENERATION       | Conversation access       |

The AI task itself (model inference) is not gated by RBAC — it is always available to authenticated conversation participants. Only the commerce action execution that follows is RBAC-gated.

## Tenant-Based Authorization

| Operation              | Actor Must Belong To |
|------------------------|----------------------|
| AI on conversation     | Same org as conversation |
| Commerce action result | Same org as resource  |

### Cross-Tenant Protection

- AI requests are always scoped to the conversation's tenant
- Model selection is tenant-scoped via provider allowlists
- Data returned by AI is filtered to the tenant scope
- The AI provider never receives tenant IDs

## Provider Authorization

| Operation              | Role Required          |
|------------------------|------------------------|
| List AI providers      | PlatformAdmin, OrgAdmin |
| List AI models         | PlatformAdmin, OrgAdmin |
| Check AI health        | PlatformAdmin          |
| Invoke AI interpretation | Authenticated user   |

Internal AI endpoints (`/internal/api/v1/ai/*`) require a service-to-service token.

## Confirmation Authorization

| Action Type            | Who Can Confirm       |
|------------------------|-----------------------|
| LOW_RISK_MUTATION      | Any authenticated actor in the tenant |
| HIGH_RISK_MUTATION     | Seller/OrgAdmin only  |
| FINANCIAL              | OrgAdmin/PlatformOp   |

**AI cannot confirm actions** — confirmation is always requested from or performed by the authenticated user.

## Usage Authorization

| Operation              | Role Required          |
|------------------------|------------------------|
| AI interpretation      | Any authenticated user |
| AI with credit cost    | Must have sufficient credits |

AI interpretations that consume credits are gated by the existing `UsageService.Check`.

## Summary

- AI has no authorization — it cannot authorize anything
- The authenticated actor is always used for authorization
- Tenant isolation is enforced at every layer
- RBAC is never bypassed
- Confirmation requirements are server-side
