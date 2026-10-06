# Trezutil — Console modules, pass 2

*Written 2026-10-05.* What was built to make the seller workspace and the administration console more than a shell, what each piece may and may not do, and what is still missing. The audit that planned it is in `docs/web/PORTALS_BACKEND_GAP_ANALYSIS.md` (each item below is annotated there); the counts at the end are the honest ones.

## 1. The order lifecycle (seller)

Before: a paid order could be confirmed, and that was the end: no order ever reached "fulfilled" or "completed", so the site's *transactions* figure counted only bookings.

Now an order moves along **paid → confirmed → processing → ready → fulfilled → completed**, one step at a time:

| Step | Endpoint | Starts from | Status shown to the customer |
|---|---|---|---|
| Confirm | `POST /api/v1/orders/{id}/confirm` | paid | Confirmed |
| Start preparing | `POST /api/v1/orders/{id}/process` | confirmed | Processing |
| Mark ready | `POST /api/v1/orders/{id}/ready` | processing | Ready |
| Mark delivered | `POST /api/v1/orders/{id}/fulfil` | ready | Delivered |
| Complete | `POST /api/v1/orders/{id}/complete` | fulfilled | Completed |

Rules, enforced by the server (`internal/application/orders.go`):

- Only the seller side may take a step: the seller profile's owner, the owner/admin/manager of the business, or platform staff. **Never the buyer**, whatever the status.
- Each step starts from exactly one status. A step asked of an order in another status is a **409 `INVALID_TRANSITION`** (it used to be a 500 for the confirmation).
- Two clicks racing: the order's version is checked on save; the loser gets a **409 `ORDER_CHANGED`** and changes nothing. A fulfilment record is created when the order becomes ready, **after** the order is saved, so only the winner creates it, once.
- Every step is audited (`order.processing`, `order.ready_for_fulfillment`, `order.fulfilled`, `order.completed`).
- The seller Orders page offers each order the one step its status allows (a delivered order stays under *In progress* until it is completed), reloads after every step, and shows the server's answer if the order was changed elsewhere.

**Not built:** the order does not expose its fulfilment record, and there is no courier assignment or tracking screen; the seller *Fulfilment* module stays "not available yet". There is no customer notification (nothing tells a customer an order moved except their own Orders page).

A defect found on the way: `NewFulfillment` sliced the first 8 characters of an id and would panic on a shorter one (real ids are UUIDs, so only tests tripped it). It no longer can.

## 2. The audit trail (administration)

Before: an audit entry was one log line. Now it is also a row.

- **Table** `audit_events` (migration `000025`). **Append-only**: a trigger refuses `UPDATE` and `DELETE`. This stops a bug or a compromised service account from rewriting history; it is **not a tamper-proof ledger**, because a database superuser can still change it, and the log line is written regardless.
- **Writing**: `audit.Logger.WithStore` is attached in order-service, conversation-service and identity-service. A failed write is logged (`event=audit_store_failed`) and **never blocks the action**; the write survives a cancelled request. Metadata over 8 KB is replaced by a marker.
- **What it holds**: only what the services already audited: order, reservation, listing, inventory, capacity, fulfilment and organization actions, plus the new account suspensions. Staff sign-ins and site-content saves are still log lines only.
- **Reading**: `GET /api/v1/admin/audit?action=&actor_id=&target_type=&target_id=&from=&to=&page=&page_size=` for administrators and operators. `action` matches the start of a name (`order.`), filters are validated, newest first, 50 per page (200 at most), the actor's display name resolved when read.
- **Console**: *Audit* filters by kind of activity, dates, target id and actor (a click on a name), opens an entry beside the list with its request id and every detail, and exports the page on screen as CSV. The order, booking and business pages link to the history of their record (`/admin/audit?target_id=…`).
- **Backups and retention**: the table is in PostgreSQL, so it is in the nightly backup. Nothing prunes it; at the current volume that is a decision for later (see `RUNBOOK.md`).

## 3. Account moderation (administration)

`POST /api/v1/admin/users/{id}/suspend {"reason": "…"}` and `POST /api/v1/admin/users/{id}/reactivate`, **administrators only**.

- A suspension needs a reason of 3 to 300 characters; it goes to the audit trail with who, whom and why.
- Nobody moderates **themselves**, and **staff accounts** (admin, operator, moderator) are not moderated from the console: they are managed with `trezutil-admin`, so one hijacked session cannot lock the team out.
- The change is one conditional statement (`WHERE status = <what the caller saw>`): two administrators acting at once cannot both succeed.
- A suspended account cannot sign in or refresh its session. **A session already open may keep working for up to 15 minutes** (`JWT_ACCESS_TTL`); the console says so before the administrator confirms. The account's listings and orders are not touched.
- The Users page offers *Suspend* / *Reactivate* only where the server would accept them.

**Not built:** the user detail page, and granting or removing roles from the console.

## 4. Platform lists (administration)

`GET /api/v1/admin/orders`, `/reservations` and `/organizations` (`?q=&status=&page=&page_size=`), for administrators and operators.

- Newest first, paged by the server (25 by default, 100 at most), searchable by id and by the names involved (business, customer, and listing for a booking), filterable by one status. An unknown status is a 400; the search word is only ever a bound parameter.
- **Names and states, never contact data**: no phone number or email address in any of the three responses (a test checks it). The Users list, which does show them, stays administrators-only.
- Read-only. Nothing in these pages changes an order, a booking or a business.

**Not built:** payments across businesses, the exceptions feed (failed payments, webhooks, orders), filters by business, vertical and date, a business detail page, and business verification or onboarding actions.

## 5. Counts

| Console | Ready before | Ready now | Modules now ready |
|---|---|---|---|
| Seller workspace | 8 of 20 | **8 of 20** | Dashboard, Listings, Orders (now the full lifecycle), Reservations, Inventory, Hotel rooms, Analytics, Business profile |
| Administration | 3 of 20 | **7 of 20** | Overview, Users (now with moderation), Site content, **Orders, Reservations, Organizations, Audit** |

The seller count does not move: the order lifecycle completed a module that was already counted. The twelve seller modules and thirteen administration modules still unavailable or planned are named, each with the gap that blocks it, in the console itself and in `PORTALS_BACKEND_GAP_ANALYSIS.md`: inbox, media, availability, fulfilment, marketing, customers, finance, usage, verticals, team, integrations, settings (seller); verticals, categories, listings, payments, providers, AI, channels, monetization, search, incidents, security, health, settings (administration).

## 6. How it was checked

- Go: unit tests for the order steps (sequence, refusals, buyer refused, operator allowed, racing click creates nothing), the moderation rules and the HTTP mapping of every refusal; **integration tests on the real schema** for the full lifecycle (including the fulfilment record count and the *transactions* figure), the append-only table (a rewrite and an erase are refused by the database), the audit filters and paging, a store that fails, `SetStatus` under a race, and the three platform lists (search, status, paging, an injection attempt that stays data).
- Front end: the new pages and flows are covered by tests (lifecycle buttons per status, the 409 refresh, audit filters, drawer, CSV, moderation dialog, no-contact-data, role gating); French coverage is complete.
- **Not yet checked live**: the new endpoints and pages against the staging stack with a browser (see the staging record in `HARDENING_PASS_1.md` §8 once done).
