# ADR 0004 — Event-Driven Integration with Kafka

## Decision
Trezutil will use Kafka-compatible event streaming for durable asynchronous domain events and integration workflows.

## Rationale
Commerce workflows span search indexing, notifications, analytics, marketing, logistics and external automation. Event-driven boundaries reduce synchronous coupling.

## Constraints
Database transactions must use an outbox pattern before publishing events. Consumers must be idempotent. Events are facts, not commands.
