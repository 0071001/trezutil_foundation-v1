# ADR 0001 — Trezutil Foundation Architecture

## Decision
Start with a well-structured modular Go backend and explicit domain boundaries. Extract independently deployable services when scaling, ownership or reliability requires it.

## Rationale
Avoid premature distributed-system complexity while preserving clean boundaries for future decomposition.
