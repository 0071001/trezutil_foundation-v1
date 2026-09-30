# Figma → Web implementation map

**Source of truth:** `Premium Homepage Design.zip` (repository root), a Figma Make export. Its unmodified contents are also committed under `trezutil_demo_catalog/` for reference.

**Implementation:** `web/frontend` (React 19, React Router 7, Vite 8, Tailwind 4), served by the API gateway.

## 1. What the export contains

| Item | Content |
|---|---|
| Format | A runnable Figma Make React project (Vite + Tailwind v4, `react-router`), not static HTML or images |
| Screens | `src/pages/Home.tsx` (homepage, 11 sections) and `src/pages/Pages.tsx` (19 app screens), with routes in `src/app/routes.tsx` |
| Styles | `src/index.css`, 393 lines / 84 KB: design tokens, every component, 5 breakpoints, dark theme, reduced motion |
| Assets | No bundled images: Unsplash URLs; inline SVG icon set (24 glyphs) |
| Data | Hard-coded demo arrays (products, orders, reservations, notifications) |
| Behaviour | Local UI state only: a chat "state switcher", a fake checkout stepper, a fake login |

There are **20 screens**:
- **Home.**
- **Customer:** Search, Discover, Conversation (six states), Detail (five variants), Cart, Wishlist, Checkout (four steps), Sign in, Register/OTP, Orders, Order detail, Reservations, Reservation detail, Account, Notifications.
- **Seller:** Seller overview, Seller listings, Create listing (six steps), Seller orders/reservations.

About **45 components** are reused throughout: Navbar, Brand, conversation composer, inline composer, suggestion chips, floating cards, product/result/property/service/booking cards, category tiles, rails, filter panel, tabs, fields, choice cards, checkout progress, timeline, operation card, metric, seller shell/nav, conversation dock, message notice, empty state, mobile bottom nav, WhatsApp phone mock and so on.

## 2. Screen mapping

| Figma screen | Route | Status | Deviations / missing elements | Backend integration |
|---|---|---|---|---|
| Home (`Home.tsx`) | `/` | Implemented | "FR" language switch removed (no translations); location tool removed from composer (no geo search); WhatsApp CTA shown only when `VITE_WHATSAPP_NUMBER` is set, otherwise "Ask Trezutil"; hero cards, WhatsApp phone and seller dashboard kept as illustrations (hidden from assistive tech, no fake figures) | Composer → conversation; "Find it near you" = newest listings; properties = `apartment` search; booking card = best `hotel` match |
| Search | `/search` | Implemented | Distance slider omitted (no geo filter in the search contract); filters are Type, Category, Availability; mobile filters open as a bottom sheet (Figma hides them) | `POST /search` (text, category, sort, cursor), `GET /search/suggest`, `GET /categories` |
| Discover | `/discover` | Implemented | Rails: "New around you", "Homes & stays"; collage shows the live listing count | `POST /search` |
| Conversation (six states) | `/conversation` | Implemented | The design's state switcher is replaced by real states (thinking, clarification, confirmation, error + retry, empty); the sidebar lists this conversation's requests (there is no history API) | `POST /conversations/message`, `/{id}/confirm`, `/{id}/reject` |
| Detail: product / service / restaurant / hotel / property | `/products/:id`, `/services/:id`, `/restaurants/:id`, `/hotels/:id`, `/properties/:id` | Implemented (one page adapts to the listing type) | Tabs: Overview / Details (no reviews or menu/rooms data in the API); location card becomes a booking/delivery card | `GET /listings` (now returns `Media`), `GET /stores`, `POST /search` (similar) |
| Cart | `/cart` | Implemented as the device-local "Bag" | Each item is ordered separately (the order API is single-listing); no delivery fee (not modelled) | Checkout per item |
| Wishlist | `/wishlist` | Implemented (device-local "Saved") | No wishlist API | — |
| Checkout (4 steps) | `/checkout` | Implemented as Review → Payment → Confirmation | No address or contact steps (not part of the order contract; arranged with the seller) | `POST /orders` (idempotent) → `GET /orders/{id}` → `POST /payments` → Stripe Payment Element → webhook → polling |
| — (not in Figma) | `/book/:id` | Added in the checkout visual language | — | `GET /availability`, `POST /reservations` (idempotent) |
| Sign in | `/login` | Implemented | Password, "remember me" and Google removed: the platform uses phone OTP only | `POST /auth/request-otp`, `POST /auth/verify-otp`, refresh |
| Register (+ OTP) | `/register` | Implemented (same phone-code flow; first sign-in creates the account) | Name, email and password fields removed (not in the identity contract) | as above |
| Orders | `/orders` | Implemented | Tabs: Active / Completed / Cancelled with live counts | `GET /orders` |
| Order detail | `/orders/:id` | Implemented | Timeline derived from order status; pickup card shows the item, payment and cancel actions | `GET /orders/{id}`, `POST /payments`, `POST /orders/{id}/cancel` |
| Reservations | `/reservations` | Implemented | Tabs: Upcoming / Pending / Past / Cancelled | `GET /reservations`, `GET /listings` |
| Reservation detail | `/reservations/:id` | Implemented | Cancel uses an inline confirmation (no browser dialog) | `GET /reservations/{id}`, `POST /reservations/{id}/cancel` |
| Account | `/account` | Implemented | "Edit profile" omitted (no profile-update API); theme toggle and sign-out added | `GET /auth/users/me` |
| Notifications | `/notifications` ("Activity") | Implemented as account activity | No notification service: shows real order and reservation changes | `GET /orders`, `GET /reservations` |
| Seller overview | `/seller` | Implemented | Metrics are computed from real data (sales this week, orders to confirm, upcoming bookings, live listings); "discovery views" omitted (no analytics API) | `GET /sellers/me`, `/listings/manage`, `/orders/incoming`, `/reservations/incoming` |
| Seller listings | `/seller/listings`, `/seller/listings/:id` | Implemented | Row actions: Publish / Pause / Resume / Edit; listing detail with stock | `GET /listings/manage`, `POST /listings/{publish,pause,resume}`, `GET /inventory` |
| Create listing (6 steps) | `/seller/listings/new` | Implemented | The Images step takes https image links: there is no upload API | `POST /listings` (with `media`, `availability`), then optional publish |
| — | `/seller/listings/:id/edit` | Added | Only draft or paused listings are editable (server rule) | `POST /listings/update` |
| Seller orders / reservations | `/seller/orders`, `/seller/reservations` | Implemented | Confirm is only offered for paid orders (server rule); bookings auto-confirm | `GET /orders/incoming`, `POST /orders/{id}/confirm`, `GET /reservations/incoming` |
| — | `/seller/inventory`, `/seller/business`, `/seller/onboarding` | Added in the seller visual language | Stock is read-only (no adjust API); onboarding is operator-assisted | `GET /inventory`, `GET /sellers/me` |
| — | `*`, 403, 401, 500, offline | Added (Figma empty-state style) | — | — |

## 3. Visual system

`src/styles/figma.css` is the export's `index.css`, **byte-for-byte**. `src/styles/app.css` only adds behaviour the static design does not specify, using the same tokens.

| Aspect | Figma | Web |
|---|---|---|
| Typography | Manrope (headings, 600/700/800, tracking −0.045em), DM Sans (body 400–700), Google Fonts | Same families, weights, sizes (`clamp()` scales) and letter spacing; headings are real `h1`–`h3` elements with Figma's `.heading-*` classes |
| Colors | `--navy #071a2f`, `--navy-2 #0b2440`, `--blue #176bff`, `--blue-2 #4c8dff`, `--blue-soft #eaf2ff`, `--ice #f4f8fb`, `--surface #fff`, `--surface-2 #edf3f7`, `--text #102238`, `--muted #647488`, `--mint #9be5c2`, `--mint-soft #e7faf1`, `--coral #ff8064`, `--line #dfe8ee`, `--wa #1ba667` | Same tokens; added `--danger`, `--warning`, `--success` for state messages (derived from the badge colors already in the design) |
| Radii | `--radius-sm .75rem`, `-md 1.15rem`, `-lg 1.75rem`, `-xl 2.5rem`; pills `2rem` | Same |
| Shadows | `--shadow-sm 0 .5rem 1.8rem rgba(10,35,62,.08)`, `--shadow-lg 0 2rem 5rem rgba(8,31,55,.13)` | Same |
| Spacing | Sections `7.5rem 2rem` (desktop), `5rem 1.15rem` (phone); app pages `.page-section`; containers 82rem / 90rem | Same classes |
| Breakpoints | 1200, 1100, 900, 760, 640 px | Same; the production additions use 760 and 900 |
| Dark theme | `.app.dark` token swap | Same, persisted per device and defaulting to the OS preference. **Fix:** Figma never sets `.app { color }`, so uncolored text stayed dark on dark; app.css sets it |
| Motion | `rise` entrance animations, 180–300 ms transitions, `--ease cubic-bezier(.2,.75,.25,1)`, `prefers-reduced-motion` reset | Same, plus 260 ms page fade, skeleton shimmer, drawer/sheet slides and toasts, all disabled under reduced motion |
| Interaction states | hover colors, `:focus-within` composer lift, active tabs/nav | Same, plus `:focus-visible` rings, disabled/busy buttons, `aria-pressed` saves, `aria-current` steps |

## 4. Deliberate deviations (accessibility and product truth)

1. **Minimum text sizes.** Figma sets some labels at 0.5–0.62rem (8–10 px). Badges, field labels, tab labels, table heads and timestamps are raised to 0.68–0.82rem, and tap targets to at least 40 px.
2. **Mobile replacements.** Below 760 px, Figma hides the filter options, the conversation sidebar and the seller navigation with no alternative. The app adds a filter bottom sheet, a toggleable conversation panel and seller tabs, and keeps seller row actions visible.
3. **No fabricated data.** Every figure (prices, counts, ratings, distances) comes from the API or is omitted. Ratings, reviews, distances and "discovery views" have no backend source and are not shown.
4. **Illustrations stay illustrations.** The hero's floating cards, the WhatsApp phone and the seller dashboard on the homepage are product illustrations (`aria-hidden`), with neutral copy instead of fake numbers.
5. **Authentication.** Figma shows password and Google sign-in; the platform uses phone OTP only (an approved decision), so the sign-in screens keep the Figma layout with the phone-code flow.
