# ADR 0007 — Multi-Provider AI Gateway

## Decision
Trezutil will access language, vision and speech models through a provider-neutral AI gateway.

## Rationale
Model capabilities, pricing, availability and regional constraints evolve. Provider independence reduces product and operational lock-in.

## Policy
The gateway exposes explicit capabilities such as chat, structured extraction, vision and speech-to-text. Business services never depend directly on a model SDK.
