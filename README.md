# Trezutil

**Trezutil = Très utile**

Trezutil is a conversation-first **Commerce OS** designed for the DRC and, progressively, Africa.

## Product principle

- **WhatsApp is the primary interface** for customers and sellers.
- Customers discover, compare, buy, pay, book and track through WhatsApp.
- Sellers can publish, manage catalog/stock/orders, run marketing and receive reports through WhatsApp.
- Web interfaces are for advanced administration and operational tooling, not a prerequisite for everyday commerce.
- The backend is channel-independent: WhatsApp is a channel, not the domain core.

## Initial vertical

Real estate is the first high-value vertical, especially:
- verified listings
- availability freshness
- rich media
- virtual tours
- visit scheduling
- agent trust

The architecture is generic enough to add retail, vehicles, food, services, construction and other verticals.

## Development principles

1. **API contracts before business code.**
2. Domain logic must not depend on WhatsApp, n8n, a specific LLM or a payment provider.
3. AI may interpret and orchestrate actions, but **business services enforce authorization and business rules**.
4. Events are first-class contracts.
5. Start as a well-structured modular backend; extract services when scale or ownership requires it.
6. Security, observability, idempotency and auditability are mandatory from the foundation.

## Initial technology direction

- Go
- PostgreSQL
- Redis
- Kafka
- OpenSearch
- MinIO/S3-compatible object storage
- Docker Compose for local development
- OpenAPI for external APIs
- Protobuf/gRPC for internal contracts where needed
- n8n for external automation/integration workflows
- WhatsApp Business Platform / Cloud API for the primary conversational channel
- Multi-provider AI gateway

## Repository status

This repository is a **foundation scaffold**. It intentionally contains production-oriented structure, contracts/placeholders, local infrastructure and engineering conventions, but does not pretend that the commerce domain is fully implemented.

Next documents to finalize before serious domain implementation:
- `docs/PRODUCT_SPEC.md`
- `docs/ARCHITECTURE.md`
- `docs/DOMAIN_MODEL.md`
- `docs/API_CONTRACT.md`
- `docs/EVENTS.md`
- `docs/SECURITY.md`
