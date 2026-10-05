# Dashboards and site content

Two features, both built on the platform's own data and neither on invented numbers:

1. **Dashboards** — real charts and indicators for the administration (the whole platform) and for each business (its own orders, bookings and listings).
2. **Site content** — what visitors see on the home page (the live-figures strip, the partner logos, which sections show) is managed by administrators from the console instead of being written into the code.

Written 2026-10-04. Backend: migration `000024_site_content.sql`, `internal/domain/{analytics,site}.go`, `infrastructure/postgres/{analytics,site}.go`, `internal/api/{analytics,site}_handlers.go`, routes in `cmd/identity-service`. Frontend: `web/frontend/src/{api,components/charts,components/dashboard,pages/admin,pages/seller}`.

## 1. Rules both features follow

- **Every figure is counted by the server from its own tables.** Nothing is estimated, extrapolated, smoothed or stored. A day with no activity is a real `0`; a period with nothing says so in a sentence instead of drawing a flat line.
- **No figure can be typed.** The site-content editor stores *which* metrics to show, in what order, from what value and in what words. The number behind a metric is recomputed on every read (cached for 60 seconds).
- **The server is the only authority.** Roles and memberships are re-read from the database on every request (a suspended administrator is shut out at once). The screens hide what a role cannot use; they never grant anything.
- **Currencies are never added together.** Revenue is reported per currency in minor units; the dashboards show the main currency (USD first) and name the others beside it.

## 2. Definitions

| Term | Meaning |
|---|---|
| Day | A calendar day in `Africa/Kinshasa` (UTC+1, no daylight saving). Every series is filled for every day of the window. |
| Window | The last 7, 30 or 90 days (the API accepts 1 to 180). The **previous period** is the window of the same length just before it; every headline figure is compared with it. |
| Order placed | An order that is not a `draft` (a draft is a cart). Cancelled and expired orders are placed orders and appear in the status breakdown. |
| Revenue | The total of orders in status `paid`, `confirmed`, `processing`, `ready_for_fulfillment`, `fulfilled` or `completed`, per currency. Cancelled, refunded, disputed and awaiting-payment orders are not revenue. |
| Booking | A reservation created in the window, any status (the status breakdown shows how many were cancelled). |
| Buyer | A distinct customer with at least one placed order in the window. *Returning* buyers had already placed an order, in the same scope, before the window. |
| Best seller | A listing ranked by the paid orders that contain it (orders, then units). Orders without itemised lines are not attributed to a listing. |

## 3. Analytics API

| Endpoint | Who | Scope |
|---|---|---|
| `GET /api/v1/admin/analytics?days=30` | administrators, operators | the whole platform |
| `GET /api/v1/sellers/analytics?organization_id=…&days=30` | owner, admin or manager of that business; platform administrators and operators | one business |

`401` anonymous · `403` anyone else (a customer, a staff or accountant member of the business, the owner of **another** business, a suspended account) · `400` malformed `organization_id` · `days` outside 1–180 is clamped, never trusted. Responses are `Cache-Control: no-store`.

Common fields: `days`, `from`, `to` (inclusive), `timezone`, `revenue_currencies` (USD first, then alphabetical), `current` and `previous` (`orders`, `reservations`, `conversations`, `new_users`, `new_listings`, `customers`, `returning_customers`, `revenue` per currency), per-day series `orders`, `revenue` (`amounts` per currency), `reservations`, `new_listings`, and the breakdowns `orders_by_status`, `reservations_by_status` (both within the window), `listings_by_status`, `published_listings_by_vertical` (as of now).

Platform only: series `new_users`, `conversations`, `users_by_role`, `top_organizations` (five, by orders in the window, each with its revenue per currency). Business only: `top_listings` (five). Parts that do not apply to a scope are **absent**, so a seller can never receive a platform-wide list.

## 4. Site content API

| Endpoint | Who |
|---|---|
| `GET /api/v1/site/content` | public. `{trust:[{metric,value,label_fr?,label_en?}], partners:[{id,name,kind,logo_url,link_url?}], sections:[{key,visible}]}`; cacheable for 60 s |
| `GET /api/v1/site/assets/{id}` | public. An uploaded logo; immutable, `nosniff`, `Content-Security-Policy: sandbox` |
| `GET /api/v1/admin/site/content` | administrators. The whole editable document, `version`, who saved last, and the **live figure behind every metric** |
| `PUT /api/v1/admin/site/content` | administrators. Publishes the document; must name `expected_version` |
| `POST /api/v1/admin/site/assets` | administrators. Multipart field `file`: one PNG, JPEG or WebP logo, at most 512 KiB |

**The document.**

- `trust` — one entry per metric: `enabled`, `min_value` (hidden until the figure reaches it), optional `label_fr` / `label_en` (at most 40 characters of plain text). The order of the list is the order on the page. Metrics (13), in the order of the editor. **Shown by default**, because they say what Trezutil enables rather than how much it lists: `conversations` (conversations people started to say what they need; every row of `conversations`), `connections` (customer and business pairs that went through with something: a distinct `(customer, business)` pair among orders fulfilled or completed and reservations confirmed or completed, so a pair counts once whatever it holds), `transactions` (orders fulfilled or completed plus reservations confirmed or completed) and `merchants` (active businesses with at least one published listing). **Available, off by default**: `registered_users` (active accounts with the role customer or seller; staff are never counted, a person with two roles counts once), `partners` (the partners the band shows, counted from the managed list itself, never from the database), then the older figures about the catalogue: `live_listings`, `verified_sellers`, `deals_live` (listings whose "compare at" price really is above the selling price), `cities_covered`, `bookings_confirmed`, `orders_completed`, `verticals_live`. Listing counts were the default until 2026-10-05; they reinforce the idea of a listing site, so they are off.
- `partners` — at most 40: `id` (slug), `name` (1–60, plain text), `kind` (bank, mobile_money, shop, hotel, hub, tech, other), `logo_url`, optional `link_url` (https only), `visible`.
- `sections` — every home page section exactly once (`trust`, `nearby`, `categories`, `showcase`, `services`, `properties`, `reservations`, `whatsapp`, `how`, `seller`, `partners`); the stored position is **no longer used by the page**: since the home page redesign (2026-10-04) the front end shows six of them in a fixed order that tells the product story (`categories`, `nearby`, `how`, `trust`, `seller`, `partners`) and only honours `visible`. `showcase`, `services`, `properties` and `reservations` are still stored and validated (no backend change) but nothing renders them. `whatsapp` came back on 2026-10-05 as a small block after `how` (the order in code is now `categories`, `nearby`, `how`, `whatsapp`, `trust`, `seller`, `partners`); the server already knew the key, so nothing changed there. The hero is always first and cannot be hidden. (A `deals` section existed and was removed on 2026-10-04; a document saved before that is cleaned on read, so it keeps working.)

**What the server refuses (`422`, with the field named):** unknown or repeated metrics, sections or partner ids; markup or control characters in names and wording; a minimum outside 0–1,000,000; a link that is not a full `https://` address. **A logo may only be one bundled with the site (`/partners/…`) or one uploaded here (`/api/v1/site/assets/<id>`)**: a remote address, a `data:` or `javascript:` URL or a path trick is refused, so the public site never loads an image from a third party. An unknown field in the request is a `400`: a client can never smuggle a figure in.

**Concurrent editors.** A save names the version it read. If someone saved in between it is refused with `409 SITE_CONTENT_CONFLICT`; the editor tells the person and asks them to reload. The check is one conditional `UPDATE`, so two simultaneous saves cannot both win (tested with eight at once).

**Uploads.** The type is decided from the bytes, never from the file name or header. SVG is refused (it can carry scripts). PNG and JPEG are decoded in full (16–2048 px a side); WebP is checked by its signature. The same bytes are stored once. The database itself also refuses other types and sizes.

**Defaults.** Until an administrator publishes, the site shows the built-in content (`DefaultSiteContent` in `internal/domain/site.go`): four metrics on (`conversations`, `connections`, `transactions`, `merchants`), each hidden until it reaches 1, so a figure that is still zero is never advertised (on a young platform the strip may show only some of the four, or none, and then it is not drawn), and the logos shipped in `web/frontend/public/partners/` (a Go test checks the files exist). Metrics added by a later release are added to an older saved document **switched off**, and new sections **visible**.

**Audit.** Every publish and upload is logged with the actor (`site_content_saved`, `site_asset_uploaded`). There is no queryable audit store yet (gap G-C1).

## 5. Screens

- **Administration → Overview.** *Platform today* (the existing totals) and *Activity over the last N days*: six KPI cards with the change against the previous period, revenue per day (with a currency switch), orders, bookings, sign-ups, conversations and new listings per day, orders and bookings by status, live listings by sector, and the businesses with the most orders.
- **Seller → Dashboard** (managers): the same figures for the business, then what needs attention and recent activity (read from the latest orders). Other members see the catalogue only and no analytics request is made.
- **Seller → Analytics** (managers): everything above plus new listings, bookings by status, listings by status and sector.
- **Administration → Site content** (administrators): *Live figures* (switch, minimum, wording in French and English, order, with today's real number and whether it shows), *Partners* (add, edit, remove, reorder, show/hide, upload a logo, optional website), *Home layout* (show or hide each of the six blocks; the order is the product narrative and is kept in code, so there is nothing to move). A **preview** shows the strip or band with today's real figures. Changes are a draft until **Publish changes**; publishing asks for confirmation, goes live at once, and a stale draft is refused.
- **Public home page.** The strip (*Trezutil en chiffres*), the partner band and which sections show all come from `GET /site/content`. If it cannot be loaded the strip and the band are simply not shown and the default layout is used: nothing is ever filled in with placeholders.

**Charts** are drawn in SVG with no chart library: the axis always starts at zero, nothing is smoothed or interpolated, the axis uses whole numbers for counts, and a tooltip reads the exact value. Each chart is a named group with a one-sentence summary, can be read day by day with the arrow keys (the day is announced), and every card has **View as a table** showing the same numbers as text. Chart colours reach 3:1 against their surfaces in both themes (checked by `src/portal/tokens.test.ts`); status colours are never the only cue (labels and numbers are always printed).

## 6. Not included

- No per-day *previous* series (only the previous period's totals), no forecasts, no cohort or funnel views, no revenue by vertical or by category.
- Customers appear as counts only (new and returning); a seller still cannot list or read customers (gap G-B5).
- Payments, refunds, settlements and platform AI usage are not part of these figures (gaps G-B4, G-B10).
- The hero text, banners and promotions copy are not managed from the console.
- The headline figures are not exported (CSV) yet.

## 7. Tests

Go: validation of every editable field (including logo origins), handler authorization matrices for both features, upload hardening, version conflicts, the 60-second figure cache and its stale fallback, and **integration tests on real PostgreSQL** that build a known platform and check every figure: Kinshasa day boundaries, drafts excluded, cancelled orders not counted as revenue, two currencies kept apart, two businesses never mixed, the previous period, real zeros for a quiet business, the trust metrics (including a promotion that is not a real saving), concurrent publishes, uploads, and the full HTTP path with real roles and memberships. Frontend: the chart toolkit (scales, keyboard reading, tooltips, tables, deltas), both dashboards (window requests, per-business scoping, isolated failures, quiet periods), the Site content editor (permissions, draft and publish, validation, conflicts, upload, reorder, preview) and the public strip, band and layout.
