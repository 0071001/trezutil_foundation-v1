# Trezutil mobile app — implementation plan (Phase 0)

> Status: **Phase 0 — discovery. No implementation code has been written and none will be until
> Phase 1 is explicitly approved.** Companion document:
> [FIGMA_MOBILE_IMPLEMENTATION_MAP.md](FIGMA_MOBILE_IMPLEMENTATION_MAP.md) (screens, components,
> tokens, assets, motion, per-element backend mapping).
> Rule used throughout: if something is absent from the Figma or the repository it is reported
> as **ABSENT / MISSING**, never invented.

## 1. Verdict and decisions needed before Phase 1

**Verdict: `NEEDS CLARIFICATION`.** The backend is rich enough to start a Flutter client, but
Phase 1 cannot be *verified* on this machine, and several inputs only the owner can supply.

| # | Decision / input needed | Why it blocks or shapes Phase 1 | Proposal |
|---|---|---|---|
| D1 | **Install the toolchain** (Flutter SDK, Dart, JDK 17, Android SDK + emulator). `flutter`, `dart`, `java`, `javac`, `adb`, `sdkmanager` are **not installed**; `ANDROID_HOME` unset (checked 2026-10-04). Host is **Windows 11** → **iOS cannot be built or run here**. | Phase 1 acceptance requires `flutter analyze`, `flutter test` and an Android build. | Install Flutter stable + Android Studio. iOS: use a Mac or macOS CI runner; until then iOS is **"not verified"**, never "ready". |
| D2 | **Design tokens**: Figma (`#1A69F0`, cream `#F9F8F5`, Instrument Sans + Inter, green `#1A7A4A`) conflicts with `docs/design/BRAND_SYSTEM.md` (`#155EEF`, ice `#F5F8FC`, Manrope + Inter, mint `#35D0BA`) and with the web app (`#176BFF`, DM Sans/Manrope). The task brief names both. | The theme file is the first thing built. | Follow **Figma** values (fidelity rule), isolated in one token file; document the divergence; align later in one file. |
| D3 | **Missing designs** (§2 of the map): listing detail, checkout/payment, order/reservation detail, sign-in/OTP, all error/empty/loading/offline states, notifications, map. | Auth is required for any order/reservation; Phase 2 and 3 cannot be "Figma-faithful" for screens that have no design. | Either supply Figma for these, or approve **token-only minimal screens** explicitly labelled *"not in Figma"*. |
| D4 | **Brand assets**: logo, launcher icon (adaptive Android + iOS), splash. None in the Figma export; web has `favicon.svg` only. | Store builds, splash, native config. | Provide official artwork; until then a text wordmark placeholder. |
| D5 | **Application identity**: app name, Android `applicationId`, iOS bundle id, signing owners, store accounts. **Nothing exists in the repo** (searched). | Project generation (`flutter create --org`). | Suggest `com.trezutil.app` — *a suggestion, not a fact*; needs confirmation. |
| D6 | **Voice**: the Figma shows speech in and out. The backend has **no speech endpoint**. On-device STT (OS service) is possible but may send audio to Apple/Google; TTS is absent. | Privacy/legal and scope for Phase 2. | Phase 2 ships text first; voice input via on-device STT behind an explicit consent screen **if approved**; no TTS in v1. |
| D7 | **French register**: Figma uses *tu*; the web uses *vous*. | All strings. | Follow Figma (*tu*) on mobile unless told otherwise. |
| D8 | **Repo layout**: where the Flutter project lives; and whether the (currently untracked) Figma export folder is committed. | Project generation, CI paths. | `mobile/` at repo root. Commit the Figma folder only on request. |
| D9 | **OTP delivery on staging**: `identity-service` sends OTP through the WhatsApp Cloud API sender; the Meta token expired **2026-09-30**. | Cannot test real sign-in end-to-end until renewed (or a dev/stub path is confirmed). | Renew the Meta token, or confirm a dev OTP mode in Phase 1 contract work. |
| D10 | **"Expériences"** chip (Figma Home) has no vertical of that name. The catalogue has `EVENTS`, `TRAVEL`, `TRANSPORT`, `VEHICLE_RENTAL`, `OTHER`. | Category → vertical mapping. | Map to `EVENTS` + `TRAVEL`, or hide until decided. |

## 2. What was inspected

**Figma export** (`Design Trezutil Mobile App/`): read in full — see the map §0. **5 screens + a
5-tab shell**; light + dark; French only; Unsplash placeholders; simulated data and voice.

**Repository**:
- Brand/design docs: `docs/design/{BRAND_SYSTEM,DESIGN_TOKENS,MOTION_GUIDELINES}.md`, headings of `UI_COMPONENTS.md`, `VERTICAL_COMMERCE_PACKS.md`, and the web `FIGMA_IMPLEMENTATION_MAP.md` (used as the template).
- API: `api/openapi/openapi.yaml` (37 paths — **stale**, see §5.3), gateway routes (`internal/gateway/gateway.go`), service route tables (`cmd/{identity,conversation,search,order,payment}-service/main.go`), handlers (`conversation_handlers.go`, `search_handlers.go`).
- The **real, working client**: `web/frontend/src/api/{client,auth,search,listings,commerce,stripe,conversations,hotels,seller,admin}.ts`, `src/types.ts`, `src/lib/verticals.ts`.
- Live staging checked on 2026-10-04: `GET /api/v1/verticals` (10 verticals), `GET /api/v1/categories`.
- Toolchain probe (§1 D1).

**Not found in the repository (ABSENT):** any existing mobile code; any `mobile/` or Flutter files;
any application id/bundle id; any push-notification, device-token, favourites, ratings, profile-
update, conversation-history or logout endpoint; a current OpenAPI contract.

## 3. Product architecture rule (from the brief)

```
Mobile app ──HTTPS──► Trezutil gateway (/api/v1) ──► identity / search / conversation / order / payment services
                                                        │
                                          Commerce Action Runtime ──► Commerce Core (DB, search index)
```

- The mobile app is **another client of the same REST API as the web app**. It never talks to a service directly, never to an AI provider, never to Stripe with a secret key.
- AI is reached **only** through `POST /api/v1/conversations/message`. No local chatbot, no local ranking, no local intent logic.
- Prices, totals, availability and order state are **server truth**. The app displays them and sends only identifiers/quantities; payment amount is validated server-side against the order.
- No secret of any kind in the app bundle. Stripe uses the **publishable key returned by the API** with the `client_secret` of a server-created PaymentIntent.

## 4. Backend contracts reused

All paths are under the gateway prefix `/api/v1`. "Web impl." = the file whose behaviour the mobile
client must reproduce (it is the only code that exercises the contract end-to-end).

### 4.1 Contracts in use

| Domain | Method + path | Request | Response (as used by web) | Auth | Web impl. |
|---|---|---|---|---|---|
| Auth | `POST /auth/request-otp` | `{phone}` | `{request_id, expires_at}` | none | `api/auth.ts` |
| Auth | `POST /auth/verify-otp` | `{request_id, code}` | `Session {user, tokens{access_token, refresh_token, expires_at}}` | none | `api/auth.ts` |
| Auth | `POST /auth/refresh` | `{refresh_token}` | `Session` (partial: tokens) | none | `api/client.ts` (single-flight) |
| Auth | `GET /auth/users/me` | — | `User {id, phone, email?, display_name, status, roles[], created_at, verified_at?}` | Bearer | `api/auth.ts` |
| Catalogue | `GET /verticals` | — | `items[]` `{code, name, description, flows[BOOK|BUY], capabilities[], listing_types[]}` | web: anonymous* | `api/search.ts`, `lib/verticals.ts` |
| Catalogue | `GET /categories` | — | category tree | web: anonymous* | `api/search.ts` |
| Search | `POST /search` | `{text, category_id?, vertical?, location?, price_min?, price_max?, page_size, cursor?, sort_field?, sort_direction?}` | `{items[SearchResult], total, has_next, next_cursor}` | web: anonymous* | `api/search.ts` |
| Search | `GET /search/suggest?text&limit` | — | suggestions | web: anonymous* | `api/search.ts` |
| Listing | `GET /listings?listing_id=` | — | listing (+`media[]`) | web: anonymous* | `api/listings.ts` |
| Store | `GET /stores?store_id=` | — | store | web: anonymous* | `api/listings.ts` |
| Availability | `GET /availability?listing_id&start_at&end_at&quantity` | — | availability | web: anonymous* | `api/listings.ts` |
| Hotels | `GET /hotels/property?store_id`, `GET /hotels/availability?store_id&check_in&check_out&guests&rooms` | — | property / availability | web: anonymous* | `api/hotels.ts` |
| Conversation | `POST /conversations/message` | `{message_text, channel:"mobile", sender_external_id, message_idempotency_key, locale}` | `ConversationReply {conversation_id, text, cards[], result_items[], suggestions[], confirmation?, missing_params[], is_clarification, requires_user_input, refs{order_id,reservation_id,listing_id}, action_type, source}` | Bearer if signed in; web also works anonymously via a per-session `sender_external_id` | `api/conversations.ts` |
| Conversation | `POST /conversations/{id}/confirm` · `/reject` | `{token}` | `ConversationReply` | as above | `api/conversations.ts` |
| Orders | `POST /orders` | `{listing_id, quantity}` or `{items:[…]}` + header `Idempotency-Key` | order (server price snapshot) | Bearer | `api/commerce.ts` |
| Orders | `GET /orders`, `GET /orders/{id}`, `POST /orders/{id}/cancel` | — | order(s) | Bearer | `api/commerce.ts` |
| Reservations | `POST /reservations`, `GET /reservations`, `GET /reservations/{id}`, `POST /reservations/{id}/cancel`, `POST /reservations/{id}/checkout` (hotel) | see web | reservation(s) | Bearer | `api/commerce.ts`, `api/hotels.ts` |
| Payments | `POST /payments?organization_id=` | `{order_id, amount_minor, currency, payment_method_type:"CARD", payment_method_id:"card", idempotency_key:"pay-<orderId>"}` | `PaymentStart` incl. `client_secret`, `publishable_key`, `provider_code` | Bearer | `api/commerce.ts`, `api/stripe.ts` |
| Payments | `GET /payments/{id}` · `/status` | — | payment status | Bearer | (poll after confirmation) |

\* "anonymous" = the web client lets visitors browse without signing in; the exact per-endpoint
auth requirement is **to be pinned down by a contract test in Phase 1** rather than assumed.

Not used by mobile: `/auth/staff/login`, `/admin/*`, `/sellers/*`, `/organizations/*`,
`/listings/manage|update|publish|pause|resume`, `/orders/incoming`, `/reservations/incoming`,
`/inventory`, `/fulfillments`, `/hotels/room-types` (seller/staff/admin surfaces — out of scope
for the customer app).

### 4.2 Wire conventions the mobile client must implement

| Topic | Contract |
|---|---|
| Base URL | `<origin>/api/v1`, injected by environment config — **never hard-coded**. |
| Errors | `{error:{code, message}}` with HTTP status; client maps to a typed `ApiFailure{status, code, message, requestId}`. Codes confirmed in code/tests: `LISTING_UNAVAILABLE`, `INVALID_CHANNEL`, `INVALID_REQUEST`, `INVALID_PHONE`, `RATE_LIMITED`, `OTP_DELIVERY_FAILED`, `INVALID_CREDENTIALS`, `ACCOUNT_SUSPENDED`, `AUTH_FAILED`; the full catalogue is not documented anywhere, so unknown codes must degrade to a generic localised message. The server never returns internal detail (hardening commit `14dd013`). |
| Request id | The gateway sets `X-Request-ID` (`internal/gateway/gateway.go`); read it from responses for diagnostics. |
| Idempotency | Header `Idempotency-Key` on `POST /orders` (and reservations); `message_idempotency_key` in the conversation body; `idempotency_key` in the payment body. Generate one **per user intent** and reuse it on retry. |
| Money | Integer **minor units** (`total_minor`, `amount_minor`) + ISO currency on orders/payments; search results use `price_amount` (number) + `price_currency`. Format with `intl`; never compute payable totals on the client. |
| Field casing | Some Go aggregates serialise **PascalCase**, DTOs **snake_case**; the web uses tolerant mappers (`read(raw,'snake_case','PascalCase')`). Mobile needs the same → **hand-written tolerant mappers**, not blind `json_serializable`. |
| Pagination | Cursor (`next_cursor`, `has_next`, `page_size`). |
| Auth refresh | On `401` for a non-`/auth/*` request: refresh once (**single flight** across concurrent requests), retry once, else sign out. |
| Timeout | Web uses a request timeout constant; mobile defines its own (proposal 15 s) plus cancel-on-dispose. |
| Locale | `locale` is sent per message; no per-user language store server-side. |
| Channel | `channel:"mobile"` is a **valid** `domain.Channel` (`internal/domain/channel.go`: whatsapp, web, mobile, ussd, sms, voice, other). |
| Transport security | HTTPS in staging/production; cleartext only for the local dev emulator host. CORS does not apply to native clients (no browser origin); the gateway's `CORS_ALLOWED_ORIGINS` requirement (enforced in non-development environments) concerns the web app only. |

### 4.3 The OpenAPI document is not a contract

`api/openapi/openapi.yaml` has 37 paths and omits most of what the web client calls (e.g. staff
login, admin, verticals, hotel endpoints, the conversation response shape). **Plan:** treat the
web client + handlers as the contract; in Phase 1 write mobile *contract tests* against staging
that pin the shapes actually used, and propose (separately, not in this task) refreshing the
OpenAPI file.

## 5. Authentication mapping

| Aspect | Reality in the repo | Mobile design |
|---|---|---|
| Method | Phone number + OTP (`request-otp` → `verify-otp`) → JWT access + refresh. **Only** customer method. Staff email/password exists for the web admin only. | Same flow. No email/password, no social login (none exists). |
| OTP channel | WhatsApp Cloud API sender (`buildOTPSender`); Meta token expired 2026-09-30 → real delivery blocked on staging (D9). | UI says "code sent" only on a successful `request-otp` response. |
| OTP rules (server) | 6 digits, valid **5 min** (`DefaultCodeTTL`), max **5 verification attempts** per challenge, max **5 code requests per phone per hour**, plus a per-IP limiter (`internal/application/identity/service.go`, `internal/api/identity_handlers.go`). | Show the real limits to the user; resend cooldown is a UI choice. Never loop-retry `request-otp`. |
| Auth error codes | `request-otp`: `INVALID_REQUEST` 400, `INVALID_PHONE` 400, `RATE_LIMITED` 429, `OTP_DELIVERY_FAILED` 503. `verify-otp`: `INVALID_REQUEST` 400, `INVALID_CREDENTIALS` **401** ("invalid or expired code"), `ACCOUNT_SUSPENDED` 403, `AUTH_FAILED` 500. | Map each to a localised message. **A 401 from `/auth/*` is a wrong code, not an expired session — the interceptor must not try to refresh on `/auth/*`** (the web client has the same exclusion). |
| Tokens | `access_token`, `refresh_token`, `expires_at` | **Keystore / Keychain** via `flutter_secure_storage` (never SharedPreferences, never logs). Keep access token in memory; persist refresh token only. |
| Refresh | `POST /auth/refresh` `{refresh_token}` | Single-flight interceptor; on failure → clear secure storage → signed-out state. |
| Roles | `User.roles[]` | Customer app ignores seller/admin roles; if a staff account signs in, show the customer experience only. |
| Sign-out | **No logout/revoke endpoint exists.** | Local wipe only; the refresh token stays valid server-side until expiry → **MISSING contract** (§6). |
| Anonymous use | Web chat/search work without login; conversation uses a generated `sender_external_id`. | Allow anonymous browsing + chat; require sign-in at order/reservation/payment. Persist the anonymous id in secure storage (so a thread survives restarts). |
| Account creation | Implicit: verifying an OTP for a new number creates the user (as in the web flow; seed behaviour to be re-confirmed in Phase 1). | — |
| Biometrics / PIN | Not in repo, not in Figma. | Out of scope. |

## 6. Missing backend contracts

Ranked by impact on the Figma screens. **None is being implemented in the mobile task** (no
backend rebuild); each needs an owner decision.

| # | Missing contract | Figma element affected | Severity |
|---|---|---|---|
| B1 | **Conversation history**: list my conversations; get messages of one conversation | Activity › Conversations, Home › "Continuer la recherche", reopening a thread, Profile stat "Conversations" | **High** (core screen) |
| B2 | **Geo search**: `lat`, `lng`, `radius` (or `near`) request parameters. Results already have `distance_meters` and `location.latitude/longitude`, and `sort_field: distance` exists, but `SearchRequest` has no point to measure from | Home "Près de toi" (0.4 km), Discover "Autour de toi", map | **High** |
| B3 | **Ratings/reviews** (score + count). Only `seller.trust_level` exists | stars on every card | Medium (can hide) |
| B4 | **Opening hours / "open now"** and price tier (`$$`) | Home tags "Ouvert", price tier | Medium (can hide) |
| B5 | **Recommendations** ("Pour toi" + reason), **trending**, **collections** (+counts) | Home "Pour toi", Discover Trending/Collections | Medium (can hide, or compose from search) |
| B6 | **Push notifications**: device-token registration, preference, delivery | Profile › Notifications, notifications screen | Medium |
| B7 | **Profile update** (name, email, city/location, language) and **avatar** | Profile card, Localisation, Langue | Medium |
| B8 | **Logout / token revocation** | Se déconnecter | Medium (security) |
| B9 | **Favourites / saved items** | "Éléments sauvegardés", heart icon | Low |
| B10 | **Speech**: STT/TTS endpoints (or a decision to use on-device only) | Voice composer | Medium |
| B11 | **Mobile-money payments** — only Stripe CARD is wired (`payment_method_type:"CARD"`); the Congolese market's primary instrument is **not connected** (also noted in the business plan) | "Paiements", checkout | **High for the market**, outside Figma scope |
| B12 | **Saved payment methods / payment history UI contract** | "Paiements" row | Low |
| B13 | **Current OpenAPI** (see §4.3) | — | Medium (tooling) |
| B14 | **Help centre content** | "Centre d'aide" | Low (could be a URL) |
| B15 | **User-level language/locale store** | "Langue" | Low (local pref works) |

## 7. Vertical and capability mapping

`GET /verticals` (live, 2026-10-04) returns 10 verticals. The customer CTA is decided from
**flows**, never hard-coded per vertical (the web rule in `lib/verticals.ts`; fallback to
`listing_type`).

| Vertical | Flows | Key capabilities | Listing types | Figma entry point | Customer action (from flows) |
|---|---|---|---|---|---|
| `RETAIL` | BUY | CATALOG, SEARCH, ORDERING, INVENTORY, PAYMENT, FULFILLMENT, DELIVERY | physical_product, vehicle, food, other | chip "Produits" | Buy → order + pay |
| `SERVICES` | BOOK | …, AVAILABILITY, SCHEDULING, CAPACITY, RESERVATION, ORDERING, PAYMENT | service, other | chip "Services" | Book |
| `HOTEL` | BOOK | …, AVAILABILITY, RESERVATION, CAPACITY, ORDERING, PAYMENT | real_estate, service, other | chip "Hôtels" | Book a stay (dates/guests via `/hotels/availability`) |
| `RESTAURANT` | BOOK + BUY | …, RESERVATION, CAPACITY, ORDERING, PAYMENT, DELIVERY | food, service, other | chip "Restaurants" | Book a table **or** order |
| `REAL_ESTATE` | BOOK | CATALOG, SEARCH, AVAILABILITY, RESERVATION, PAYMENT, FULFILLMENT | real_estate | chip "Immobilier" | Book a visit |
| `VEHICLE_RENTAL` | BOOK | …, RESERVATION, CAPACITY, PAYMENT | vehicle | none in Figma | Book |
| `TRANSPORT` | BOOK | …, TRACKING | service, vehicle, other | none | Book |
| `EVENTS` | BUY | …, ORDERING, PAYMENT | service, other | possibly "Expériences" (D10) | Buy tickets |
| `TRAVEL` | BOOK | …, RESERVATION, PAYMENT | service, real_estate, other | possibly "Expériences" (D10) | Book |
| `OTHER` | BUY | CATALOG, SEARCH, ORDERING, PAYMENT | all | none | Buy |

Notes: only five verticals have seeded data on staging (RETAIL, SERVICES, HOTEL, RESTAURANT,
REAL_ESTATE); `/verticals` exposes all ten. The Figma shows **no** vertical-specific UI (no hotel
date picker, restaurant table picker, real-estate visit scheduler, product variants). Per-vertical
screens are a gap (D3), not something to improvise. The app must load verticals at runtime so a
newly enabled vertical appears without a release; unknown future verticals fall back to the
generic result card + the conversation.

## 8. Flutter architecture proposal

### 8.1 Principles
Feature-first modules with clean layers (presentation → application → domain ← data); no business
rules in widgets; the **domain layer holds only mappings of server truth** (no pricing, ranking or
intent logic); all I/O behind repository interfaces so UI tests use fixtures and production uses
the API; every string localised; every colour/size from tokens.

### 8.2 Project location and structure (proposal — D8)

```
mobile/                              (new; Flutter project, Android + iOS)
  lib/
    main_development.dart  main_staging.dart  main_production.dart   # entry points per environment
    app/            # App widget, router, bootstrap, DI composition root
    core/
      config/       # Environment (apiBaseUrl, env name, stripe off/on…) from --dart-define; no hosts in code
      network/      # ApiClient (Dio), AuthInterceptor (single-flight refresh), ErrorMapper, Idempotency
      storage/      # SecureTokenStore (Keystore/Keychain), PrefsStore (non-sensitive: theme, locale)
      errors/       # ApiFailure, Failure hierarchy, user-message mapping
      theme/        # tokens (light/dark), TzTheme, typography, radii, spacing, motion durations/curves
      l10n/         # app_fr.arb, app_en.arb (+ generated)
      widgets/      # Tz* design-system widgets (C1–C30 of the map)
      utils/        # money/date formatting, tolerant JSON readers (read(raw,'a','A'))
    features/
      shell/        # BottomNav + tab host
      home/  discover/  conversation/  activity/  profile/
      auth/         # phone, OTP, session
      listing/ checkout/ orders/ reservations/   # only when designs exist (D3)
      <feature>/{presentation, application, domain, data}
  test/             # unit, widget, golden
  integration_test/ # critical journeys vs staging or local emulators
  assets/fonts/  assets/icons/  assets/images/
  android/  ios/
```

### 8.3 Technology choices (versions pinned in Phase 1 against pub.dev with the installed SDK — not asserted here)

| Concern | Choice | Rationale |
|---|---|---|
| State / DI | `flutter_riverpod` | Testable, no `BuildContext` coupling, scoped overrides for fixtures |
| Navigation | `go_router` with a `StatefulShellRoute` (5 tabs, preserved state) + auth redirect | Matches the persistent-bar + per-tab stack pattern; deep links later |
| HTTP | `dio` + interceptors (auth, request-id, logging **without** tokens/PII) | Cancel tokens, timeouts, retry policy control |
| Models | Hand-written immutable classes + tolerant mappers | PascalCase/snake_case mix (§4.2); mirrors the web's proven mappers |
| Secure storage | `flutter_secure_storage` (Keystore / Keychain) | Brief requirement |
| Prefs | `shared_preferences` (non-sensitive only) | theme mode, language, onboarding flags |
| i18n | `flutter_localizations` + `intl` + ARB (`fr`, `en`) | Lingala/Swahili later — **no placeholder translations** |
| Images | `cached_network_image` | Real media URLs; placeholders from tokens, not Unsplash |
| SVG/icons | `flutter_svg` or `CustomPainter` | Re-draw the 33 icons exactly |
| Payments | `flutter_stripe` (publishable key from the API + server `client_secret`) | Stripe card path exists; no secret key in the app |
| Location | `geolocator` (+ permission UX) | needed for B2 once the backend accepts geo |
| Speech (if D6 approved) | `speech_to_text` (on-device) | no backend speech API |
| Testing | `flutter_test`, `mocktail`, golden tests, `integration_test` | brief |
| Lint | `flutter_lints` + project rules | CI gate |

### 8.4 Environments

Three entry points / `--dart-define` sets: `development` (emulator → host machine; cleartext
allowed **only** here), `staging`, `production`. Each defines `API_BASE_URL`, `ENV_NAME`; Stripe
publishable key is **not** configured in the app (it arrives in `PaymentStart`). No host appears
in code; a unit test fails the build if a literal URL host is found outside `core/config`.

### 8.5 Theming
`TzTokens` (light + dark, §4 of the map) → `ThemeData` + a `ThemeExtension<TzColors>` for tokens
Material has no slot for (`s2`, `bfaint`, `txt2/txt3`, accent tints). Fonts bundled. Follows
system theme by default with a Profile switch (Figma has a manual switch). Hard-coded Figma
colours become named tokens (success, coral, violet, amber, star, scrim).

### 8.6 Motion
A single `TzMotion` (durations, the Figma curve `Cubic(.22,.61,.36,1)`) and a `reduceMotion`
helper reading `MediaQuery.disableAnimations`; every animation consults it (map §6).

### 8.7 Error, offline, loading
A typed `ApiFailure` → `UserMessage` mapper (localised); a connectivity banner; per-list
loading/empty/error widgets built from tokens. **These states are not in Figma** — they are
implemented minimally and listed in the screen map as *not in Figma* (D3).

### 8.8 Security checklist (from the brief)
Tokens only in Keystore/Keychain · no secrets or provider keys in the bundle · no logging of
tokens, OTP, phone, card data · HTTPS-only outside dev · certificate pinning **not planned**
(operational risk; revisit) · `FLAG_SECURE`/screenshot blocking on payment screens (platform
channel, Phase 3) · obfuscation + `--split-debug-info` for release · server-authoritative prices
· idempotency keys for every mutation · no PII in crash reports.

## 9. Implementation phases

| Phase | Name | Content | Exit criteria |
|---|---|---|---|
| **0** | Discovery | This document + the Figma map | Reviewed; decisions D1–D10 answered or consciously deferred |
| **1** | Foundation | See §10 | `flutter analyze` clean, tests green, Android debug build runs, tab shell matches Figma |
| **2** | Core screens (read paths) | Home, Discover, Conversation (text), Activity (reservations + orders), Profile — wired to real endpoints, Figma-faithful where designed; hide elements whose contract is missing (B2–B5) rather than fake them; voice only if D6 | Journeys pass against staging; each screen has a fidelity check vs Figma |
| **3** | Transactions | Sign-in/OTP UX, listing detail, order (BUY) + Stripe card payment, reservation (BOOK) incl. hotel dates, cancel flows; needs the D3 designs | E2E: search → order → pay → status; search → reserve → confirm; idempotent retries proven |
| **4** | Hardening | Offline/error/empty/loading states, reduced motion, accessibility (TalkBack/VoiceOver labels, 48 dp targets, text scale), performance budget, security checklist | A11y + perf + security checklists signed off |
| **5** | Platform features | Location (needs B2), push (needs B6), voice (D6), deep links, localisation review FR/EN | Per-feature acceptance |
| **6** | Release | Signing, flavors, store metadata, CI/CD, beta tracks, iOS build on macOS | Store-ready artefacts; **iOS claimed only if built and tested on iOS** |

Phases are strictly sequential: DISCOVER → CONTRACT → IMPLEMENT → TEST → VERIFY → DOCUMENT inside
each phase. No phase starts without explicit instruction.

## 10. Exact Phase 1 scope

**Goal:** a verified, empty-but-correct foundation that every later screen plugs into. *No real
product screens yet.*

### In scope
1. **Project creation** in `mobile/` (`flutter create`, org/app id from D5), Android + iOS targets, `flutter_lints`, `analysis_options.yaml`, `.gitignore`, README with run instructions.
2. **Environments:** three entry points + `Environment` config from `--dart-define`; host-literal guard test.
3. **Design system base:** token file (light/dark, from Figma), `ThemeData` + `ThemeExtension`, bundled fonts (Instrument Sans, Inter — licence check), `TzMotion` + reduce-motion helper, spacing/radius constants; theme **follow-system + manual override** persisted in prefs.
4. **Localisation:** ARB `fr` + `en`, generated delegates, locale override persisted; strings for the shell only. English strings authored by us for review — not machine-translated filler.
5. **Navigation shell:** `go_router` `StatefulShellRoute` with the Figma **5-tab bar** (`TzBottomNav`, centre "Demander" emphasised, icons drawn from the Figma set, safe-area aware) and **placeholder tab bodies** that show only the screen's label (no fake content).
6. **Networking:** `ApiClient` (Dio) with base URL from config, timeouts, `X-Request-ID` read-through, typed `ApiFailure`/error-envelope mapping, idempotency-key helper, tolerant JSON readers.
7. **Auth foundation (logic only, no OTP screens):** `SecureTokenStore`, `AuthRepository` (`request-otp`, `verify-otp`, `refresh`, `me`), `AuthInterceptor` with single-flight refresh + one retry + sign-out on failure, session state provider (signed-out / signed-in), local sign-out.
8. **Tests:** unit (mappers, error mapper, interceptor incl. concurrent-401 single-flight, token store with a fake backend, config guard), widget (bottom nav, theme switch, locale), golden for the bottom nav in light/dark.
9. **Contract tests** (opt-in, against local staging): `verticals`, `categories`, `search`, `auth/request-otp` error path — pin the shapes the web client relies on.
10. **Docs:** `docs/mobile/PHASE_1_REPORT.md` (what was built, how verified, what was not).

### Explicitly out of scope for Phase 1
Any real screen content (Home/Discover/Conversation/Activity/Profile bodies), OTP screens,
listing/checkout/order/reservation, Stripe, location, push, voice, maps, any backend change, any
OpenAPI refresh, release signing.

### Acceptance (all required, evidence attached to the report)
- `flutter analyze` with zero issues; `flutter test` green; golden files committed.
- App launches on an **Android emulator/device** showing the 5-tab shell in light and dark; tab labels in FR and EN; reduce-motion respected.
- No secret/host literal outside `core/config`; token never printed (test).
- iOS: **"not built / not verified"** unless built on macOS — stated in the report.

### Preconditions
D1 (toolchain) is mandatory. D2 (tokens), D5 (app id), D8 (location) must be answered. D3/D4/D6/D9/D10 are
**not** needed for Phase 1.

## 11. Risks

| # | Risk | Impact | Mitigation |
|---|---|---|---|
| R1 | Toolchain absent; Windows host cannot build iOS | Cannot verify; false "iOS ready" claims | Install SDK now (D1); macOS CI/Mac for iOS; report iOS status honestly |
| R2 | Figma is 5 prototype screens; most journeys have no design | Fidelity vs completeness conflict; improvised UI | D3 decision; mark every non-Figma screen; no invented visuals |
| R3 | Token conflict (Figma vs brand docs vs web) | Visual inconsistency across clients | D2; single token file; documented divergence |
| R4 | Stale OpenAPI; PascalCase/snake_case mix | Wrong parsing, silent nulls | Tolerant mappers + contract tests against staging |
| R5 | Missing contracts B1–B5 under real Figma elements | Screens look empty or tempt fake data | Hide elements without a contract; **never** ship fixtures in production; report each gap |
| R6 | OTP sender depends on an expired Meta token | Cannot test sign-in on staging | D9; confirm dev OTP path early (Phase 1 contract test) |
| R7 | Only card payments exist; mobile money is the real market instrument | Product viability in DRC | Raise B11 with the owner; not solvable in the mobile client |
| R8 | Voice flow in Figma has no backend; STT may send audio to third parties | Privacy/compliance, scope creep | D6; consent screen; text first |
| R9 | Photo/media availability for seeded listings unverified | Empty cards | Check in Phase 2; token-based placeholder |
| R10 | Search quality/AI behaviour is server-side (and was recently tuned) | Mobile "bad answers" are backend issues | Keep the client thin; surface `request id` for triage |
| R11 | Token refresh races, duplicate orders/payments on retry | Double charge / duplicate order | Single-flight refresh; idempotency keys per intent (stable, persisted until success) |
| R12 | Brand "tu" vs "vous"; FR/EN copy authored by engineers | Tone inconsistencies | D7; copy review before release |
| R13 | Figma folder is untracked; large uncommitted change set exists in the repo (dispatch, migration 000022, `Demo/` intentionally uncommitted) | Accidental commit of unrelated work | Never `git add -A`; stage explicit paths; inspect `git status`/diff first |
| R14 | Scope: 6 phases for one agent | Slippage | Strict phase gates; this plan's Phase 1 is deliberately small |

## 12. Git and working agreements

- Never `git add -A`; stage explicit paths; inspect status/diff first; no secrets; no unrelated files.
- Nothing is committed unless asked. The two Phase 0 documents are **uncommitted**.
- Dispatch work, migration `000022`, `Demo/`, and the large pending web/backend changes remain untouched.
- Phase 1 starts only on explicit instruction.
