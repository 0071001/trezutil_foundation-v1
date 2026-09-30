# Dispatch: food delivery and ride-hailing

Status: 2026-09-29. **Built and tested:** multi-item orders (`POST /orders` with `items`), and the dispatch core (migration 000022, `internal/domain/dispatch.go`, `infrastructure/postgres/dispatch.go`, `internal/application/dispatch`, 9 PostgreSQL tests including the accept race). **Not built yet:** restaurant settings and delivery fee, the food order flow, dispatch and food HTTP endpoints, cash reconciliation into Payment Core, the web screens, rides, and E2E journeys.

Design notes added while building:
- The delivery fee is carried in the order pricing snapshot and added to the order total at creation, because a payment must equal the order total exactly.
- A restaurant preparation state uses the existing fulfillment states (PENDING, ACCEPTED, IN_PROGRESS, READY, COMPLETED); no new order status is needed.
- A cash order stays PENDING_PAYMENT until the courier hands over the food and confirms the cash; then the order is paid through the order use case with provider `cash`.
- Confirmation code: a delivery is confirmed at hand-over, a ride when the trip starts. Five wrong attempts lock it.

## Decision
- RESTAURANT becomes **online food ordering and delivery** (Uber Eats / Glovo style).
- TRANSPORT becomes **on-demand rides** (Uber / Bolt style).
- Both need one new capability, **dispatch**: an agent (courier or driver) is matched to a job, moves, and is tracked. It is built once and shared.
- Order: food delivery first (it proves dispatch on top of existing orders and payments), rides second.
- Payment: card through Payment Core, plus **cash** on delivery or ride (`PaymentMethodCash` already exists). An agent confirms cash collection.
- Supply: couriers and drivers are **native** (they sign up and go online). Yango and other providers are a future integration; only a supply port is prepared, no provider contract is written now (earlier rule: no speculative provider contracts).

## What stays the same
Commerce Core (Store, Listing, Order, Payment, Fulfillment), CommerceActionRuntime, usage/credits, authorization, `/api/v1/packs`. A restaurant is a Store; a menu item is a FOOD Listing; a food order is an Order paid through Payment Core; delivery is a Fulfillment of type DELIVERY; a ride is a Fulfillment of type TRAVEL_TICKET (tracked).

## New core: dispatch
| Concept | Purpose |
|---|---|
| `dispatch_agents` | A user acting as COURIER or DRIVER: vehicle type, ONLINE/OFFLINE/BUSY, last position and time. |
| `dispatch_jobs` | One unit of work: kind FOOD_DELIVERY or RIDE, pickup and dropoff points, status, assigned agent, fee or fare, proof code. Linked to an Order or a ride request. |
| `dispatch_offers` | A job offered to one agent with an expiry; accept, decline or expire. |
| `dispatch_positions` | Recent agent positions for live tracking, pruned. |

Job lifecycle: REQUESTED → OFFERED → ASSIGNED → AT_PICKUP → IN_TRANSIT → COMPLETED, or CANCELLED / NO_AGENT.

Rules:
- Assignment is one conditional update (`WHERE status = 'OFFERED' AND agent free`), so two agents can never take one job, and one agent never holds two.
- Matching picks the nearest ONLINE, free agent within a radius (great-circle distance; no map provider needed), one offer at a time with a time limit, then the next.
- Only the assigned agent can advance a job; only the customer and the store's members can read it; positions of an agent are visible to a customer only while that agent holds the customer's active job.
- Supply port: `AgentSource` (find candidates near a point). The native source reads `dispatch_agents`. A Yango source would implement the same port later.
- Every state change goes through the existing runtime, authorization and audit paths.

## Phase 1: food
1. Restaurant: opening hours, prep time, delivery radius, minimum order, delivery fee rule (base + per km).
2. Menu: FOOD listings grouped by category, availability toggle (sold out).
3. **Multi-item orders** (currently one item per order, limitation L5): needed for a cart.
4. Order flow: place → pay (card, or cash on delivery) → restaurant accepts → preparing → ready → dispatch job → pickup → delivered with a **delivery code** the customer reads to the courier.
5. Customer: browse, cart, address, track order and courier position, reorder.
6. Restaurant console: incoming orders, accept/reject with reason, mark ready.
7. Courier: go online/offline, accept offers, status buttons, earnings and **cash float** (cash collected vs owed).
8. Chat: "order a chicken meal to Gombe" through the existing intent path, with explicit confirmation.

## Phase 2: rides
1. Ride types (Moto, Economy, Comfort) as TRANSPORT listings with base fare, per-km and per-minute rates.
2. Fare estimate before requesting (distance estimated from coordinates with a road factor; replaceable later by a routing provider).
3. Request → match → driver arrives → start → complete → fare → pay (card or cash).
4. Safety: share-trip link, trip code to confirm the right car, cancel rules with fee after a grace period.
5. Two-way ratings, driver earnings and cash float.
6. Scheduled bus/shuttle seats keep the existing capacity model.

## Unique differentiators (after both phases)
- One conversation for everything: the same chat can order food, book a ride and reserve a hotel.
- Delivery/trip proof codes that work without smartphones' data on the courier's side (SMS/WhatsApp fallback).
- Agent cash-float ledger: cash collected is reconciled against what is owed to the platform and to merchants.
- Native + future external supply behind one port, so Yango capacity can be added without changing customer flows.

## Not in scope now
Yango or any external provider, route optimisation and live traffic, surge pricing, batching several orders on one courier, real card or mobile-money credentials.
