# OpenAPI contracts

`openapi.yaml` is the external REST contract baseline.

Rules:

1. Update the contract before implementing a new external endpoint.
2. Keep request/response schemas explicit.
3. Document authorization and idempotency.
4. Avoid exposing internal database models directly.
5. Validate OpenAPI in CI using an OpenAPI linter/validator.
