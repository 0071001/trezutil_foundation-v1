# Sprint 14 — OpenAPI Specification

## Provider Endpoints

Add these endpoints to the existing OpenAPI spec under `/api/v1/providers`:

### ListProviders

```yaml
get:
  path: /api/v1/providers
  summary: List providers for the authenticated organization
  parameters:
    - name: offset
      in: query
      schema: { type: integer, minimum: 0, default: 0 }
    - name: limit
      in: query
      schema: { type: integer, minimum: 1, maximum: 100, default: 25 }
  responses:
    200:
      description: Provider list
      content:
        application/json:
          schema:
            type: object
            properties:
              data:
                type: array
                items: { $ref: '#/components/schemas/ProviderResponse' }
              total:
                type: integer
    401:
      description: Missing or invalid authentication
    403:
      description: Unauthorized
```

### GetProvider

```yaml
get:
  path: /api/v1/providers/{id}
  summary: Get a single provider
  parameters:
    - name: id
      in: path
      required: true
      schema: { type: string, format: uuid }
  responses:
    200:
      description: Provider
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ProviderResponse' }
    404:
      description: Provider not found
```

### CreateProvider

```yaml
post:
  path: /api/v1/providers
  summary: Create a new provider
  requestBody:
    required: true
    content:
      application/json:
        schema: { $ref: '#/components/schemas/CreateProviderRequest' }
  responses:
    201:
      description: Created provider
      content:
        application/json:
          schema: { $ref: '#/components/schemas/ProviderResponse' }
    400:
      description: Invalid request
    403:
      description: Forbidden
```

### EnableProvider

```yaml
post:
  path: /api/v1/providers/{id}/enable
  summary: Enable a provider
  parameters:
    - name: id in: path required: true schema: { type: string, format: uuid }
  responses:
    200:
      description: Enabled provider
      content: { application/json: { schema: { $ref: '#/components/schemas/ProviderResponse' } } }
    404:
      description: Provider not found
```

### DisableProvider

```yaml
post:
  path: /api/v1/providers/{id}/disable
  summary: Disable a provider
  responses:
    200:
      description: Disabled provider
      content: { application/json: { schema: { $ref: '#/components/schemas/ProviderResponse' } } }
    404:
      description: Provider not found
```

### GetProviderCapabilities

```yaml
get:
  path: /api/v1/providers/{id}/capabilities
  summary: List provider capabilities
  responses:
    200:
      description: Capabilities list
      content:
        application/json:
          schema:
            type: object
            properties:
              provider_id: { type: string, format: uuid }
              capabilities: { type: array, items: { type: string } }
    404:
      description: Provider not found
```

## Schemas

```yaml
components:
  schemas:
    ProviderResponse:
      type: object
      properties:
          id:             { type: string, format: uuid }
          organization_id: { type: string, format: uuid }
          code:           { type: string }
          name:           { type: string }
          type:           { type: string }
          status:         { type: string, enum: [ACTIVE, INACTIVE, SUSPENDED, DEGRADED] }
          capabilities:   { type: array, items: { type: string } }
          configuration:  { type: object }
          priority:       { type: integer, default: 0 }
          health_status:  { type: string, enum: [UNKNOWN, HEALTHY, DEGRADED, UNAVAILABLE] }
          created_at:     { type: integer }
          updated_at:     { type: integer }

    CreateProviderRequest:
      type: object
      required: [code, name, type, capabilities]
      properties:
        code:           { type: string, maxLength: 100 }
        name:           { type: string, maxLength: 255 }
        type:           { type: string, maxLength: 100 }
        capabilities:   { type: array, items: { type: string }, minItems: 1 }
        configuration:  { type: object, description: "Provider configuration (no secrets)" }
        secret_ref:     { type: string, description: "Reference to secret in secret management" }
        priority:       { type: integer, default: 0 }
```
