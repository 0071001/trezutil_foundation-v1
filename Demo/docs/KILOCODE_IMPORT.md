# Instructions Kilo Code — Demo Data

Use `data/trezutil_demo_seed.json` as the primary fixture source for frontend demonstrations.

## Rules
- Keep demo records clearly marked (`demo: true`).
- Do not alter production backend contracts just to fit this dataset.
- Map fields to the existing Listing/domain contracts.
- Preserve existing image URL support if present; otherwise add only a frontend fixture mapping layer.
- Do not show `DEMO` as a production label to end users unless the environment is explicitly demo mode.
- Do not pretend the seller/business records are verified.
- Do not hardcode these records directly into React components. Load them from a fixture/data layer.
- Keep real API paths intact. Fixtures are fallback/dev/demo only.
