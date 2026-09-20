# ADR 0006 — Channel-Independent Commerce Core

## Decision
WhatsApp-specific payloads and provider logic remain in adapters. Domain and application services consume normalized commands/events.

## Rationale
Prevents vendor lock-in and makes future web/mobile/Telegram/partner channels possible without rewriting business rules.
