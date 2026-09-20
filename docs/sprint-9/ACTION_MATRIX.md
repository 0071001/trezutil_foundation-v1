# Sprint 9 — Usage & Credits — Action Matrix

## Commerce Action Type Changes

No new CommerceActionTypes added. Usage metering is applied transparently via CommerceActionRuntime middleware, not via new action types.

## Usage Policy Actions

The following action types can have usage policies configured via UsagePolicyRepository:

| Action Type | Typical Cost | Status |
|-------------|-------------|--------|
| SEARCH_LISTINGS | 2 credits | Ready for metering |
| GET_LISTING | 1 credit | Ready for metering |
| CREATE_ORDER | 10 credits | Ready for metering |
| PUBLISH_LISTING | 5 credits | Ready for metering |
| Others | TBD | Configurable |

Costs are defined per policy in UsagePolicyRepository, not hardcoded.
