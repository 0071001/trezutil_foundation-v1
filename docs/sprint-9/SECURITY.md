# Sprint 9 — Usage & Credits — Security

## Identity Protection

System actors (ActorTypeSystem) skip usage policy checking entirely. This prevents internal system actions from consuming credits.

```go
// In CommerceActionRuntime.Execute():
if r.usageService != nil && authenticatedActor.IsAuthenticated() {
    // Only check usage for authenticated (non-system) actors
}
```

## Tenant Isolation

Usage accounts are keyed by actor ID, which is resolved from authenticated identity (never from AI/client input). Tenant context flows through the same resolution as commerce actions.

## Authorization

No additional authorization layers needed for usage operations. UsageService operates on actor ID, which is already authenticated and protected by CommerceActionRuntime's identity protection.

## Credit Security

- Credits cannot go negative (CanAfford check before Reserve)
- Reservation state machine: active → consumed/released (no reversal)
- All usage events recorded in ledger for audit trail
