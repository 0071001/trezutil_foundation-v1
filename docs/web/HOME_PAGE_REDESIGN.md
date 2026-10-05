# Home page redesign — from a listing site to "just tell Trezutil what you need"

Written 2026-10-04 after the product correction in the brief ("Trezutil n'est pas un site de listing"). **Frontend only**: no backend file, route or API was changed for this redesign, and every business capability (sign-in, search, products, services, listings, reservations, inventory, fulfilment, conversations, WhatsApp, usage credits, providers, seller console) is untouched. Only the *presentation* of the front door changed.

## 1. The product, as the page must say it

NEED → CONVERSATION → UNDERSTANDING → DISCOVERY → MATCHING → ACTION (buy, book, reserve, contact, request a quote), then the conversation and the commerce continue with the business. Listings are the supply layer, never the concept. The acceptance test of the brief: **take every listing card off the page and Trezutil must still be understandable.** (It is: the page no longer needs a single card to make sense; see §6.)

## 2. Audit of the previous home page

The page, top to bottom: navigation with six links, a location pill, a bell, an orders icon, an account icon, a theme switch, a language switch and a WhatsApp/"Ask Trezutil" button · hero · live-figures strip · "Find it near you" (4 product cards) · an 8-tile category mosaic · "Products" (banner + 6 product cards) · "Services" (3 photo cards) · "Homes" (3 property cards + a call to action) · "Book what you need" (a hotel card) · a WhatsApp phone mock-up · "How it works" · a seller section · partner logos · footer. About **25 cards and tiles** before the first explanation of the commerce loop.

### 2.1 What makes it read as a listing marketplace

| Element | Why it hurts |
|---|---|
| Hero composer = one long search bar with a filter button that goes to `/search` | A giant search box is the marketplace pattern; the filter button says "this is a catalogue you query" |
| Hero paragraph: "Shop products, discover trusted services, find places and book what you need" | Enumerates the catalogue instead of promising relief |
| "Find it near you", "Products" (banner + six cards), "Services", "Homes", hotel card | Five sections of supply before any explanation; the hierarchy is *listings, listings, more listings* |
| 8-tile category mosaic (Products, Services, Food, Real Estate, Hotels, Transport, Local Shops, Reservations) | A catalogue index, with photos, shown as the way in |
| Live-figures strip led by "annonces en ligne" | Counts inventory first (the strip stays, now lower, and the figures are chosen by staff) |
| "Transport" tile | Promises a vertical that has no live offers |

### 2.2 What makes it read as a generic AI interface

| Element | Where |
|---|---|
| The sparkle glyph | the logo mark (see §7), the composer, the navigation button, the mobile bar, a property call to action, the booking prompt, a "how it works" step, a seller preview, the floating chat button |
| "Ask Trezutil" as a label on buttons and links | navigation, mobile bar, menu |
| "Try asking" chips with chevrons | a chatbot-suggestion pattern |
| Three overlapping avatar discs (K, G, L, one violet) with a tagline | a stock "social proof" pattern with no real people behind it |
| A live-pulse dot before "Commerce, made simple" | a startup-landing pattern |
| A dotted grid and soft blue glow behind the hero | the "tech" backdrop |

### 2.3 Elements that did not help anyone understand the business (removed)

The location pill and the bell/orders icons in the navigation (they live in the account menu and the footer); the avatar stack; the "Preview" seller dashboard whose figures were dashes and whose badge said "Your store is live" (invented status); the WhatsApp phone mock-up with a scripted conversation (a fixture, not a recording); the 8-tile mosaic; the Products banner; the service, property and hotel card sections; the old "How it works" (four generic verbs).

## 3. New information architecture

| # | Section | Question it answers | Source |
|---|---|---|---|
| 1 | **Arrival**: logo, *Discover*, *For businesses*, language, theme, account | Where am I? | none |
| 2 | **The relief moment** (hero): "What do you need today?", a place to drop the need, six realistic examples | Can I just say what I need? | none |
| 3 | **What can we help you with?** Five problems, not a catalogue | Is this for my problem? | live offers decide whether a domain shows |
| 4 | **Discover what exists around you**: three short editorial lists | Is there a living ecosystem? | live search (hotels, restaurants, newest), 3 rows each |
| 5 | **From need to action**: the loop, and what you can do next | What happens after I ask? | none |
| 6 | **Trezutil in numbers** | Can I trust it? | counted by the platform (managed in Site content) |
| 7 | **Have something to sell or offer?** | Is it for businesses too? | none |
| 8 | **Partners** | Who is behind it? | managed in Site content |
| 9 | Footer | | |

The page is ordered by the user's experience: **YOU → TREZUTIL → DISCOVERY → COMMERCE → BUSINESSES**.

### 3.1 The hero

- Title "What do you need today?" (*De quoi avez-vous besoin aujourd'hui ?*); sub-line "Tell us what you need. We'll help you find it."
- A **place to leave a need**, not a search bar: a labelled, multi-line field ("Your need"), a warm card, **Dictate** (voice) and **Send**. Enter sends, Shift+Enter adds a line. No icon, no "assistant", no gradient.
- Six realistic examples written as the sentence a person would say ("I'm looking for a furnished apartment in Gombe.", "I need a mechanic nearby.", "Find me a hotel for this weekend.", "I'm looking for an iPhone 15 in Kinshasa.", "I'm looking for someone to install a camera.", "I want to book a restaurant tonight."), translated into French; one tap opens the conversation with that sentence.
- Underneath: "You don't have to search everywhere. Just tell Trezutil what you need."
- **Not shipped, because the platform cannot do it yet:** *Photo* (the conversation accepts text only; attachments are unsupported) and *Location* (no geolocation parameter exists in the conversation or search APIs). A button that did nothing would be a lie; both are listed in §7.

### 3.2 "What can we help you with?"

Five problem-led groups, each with its domains (links into search or Discover, so browsing is still one click away, it is just not the front door) and one or two example needs that start a conversation: *Find a place* (homes, apartments, hotels) · *Get something* (products, local shops) · *Find a service* (services and professionals) · *Go out* (restaurants, places to go) · *Get around* (cars, transport). **The first four always show**, because a conversation starter needs no stock. **Get around appears only when vehicle-rental or transport offers are live** (the page asks the search API once): today it is hidden, and it returns by itself the day a seller publishes one.

### 3.3 Discovery

Three short lists (three rows each, a small picture, the place and the price): *To sleep* (hotels), *To eat* (restaurants), *New on Trezutil*. No claim of popularity, availability today or "people are talking to" (none of that is measured). A list with no offers is omitted; with none at all the section disappears.

### 3.4 From need to action (the loop)

You say what you need → Trezutil understands (budget, place, date) → relevant offers appear → you act → the business confirms. Presented as **two columns**: the title and, at the foot of that column, what you can do (**buy, book, pay online** live; **contact the seller, request a quote** marked *Soon*, because the seller inbox is not built), and on the right one quiet list of the five steps tied by a single vertical line (what you do in ink, what Trezutil and the business do in blue). One column on phones: title, steps, then what you can do. The page never claims what the platform cannot do.

### 3.5 Businesses

"Have something to sell or offer?" · "Bring your products, services and availability to customers who are already looking for them." Six capabilities, all real: products, services, orders, bookings, availability, statistics. Secondary by design.

## 4. Visual direction

Premium, modern, warm, clean, human, trustworthy, editorial. A warm paper background (`#FBF8F3`; a deep blue-black in dark mode) instead of cool blue-grey, ink-coloured text, the brand blue only for actions and links; Manrope for headings, DM Sans for text (the Figma export's fonts); generous space; one idea per screen. The styles are in `web/frontend/src/styles/home.css`, scoped under `.app.home`, and `figma.css` is untouched. **None of:** violet or blue-purple gradients, glowing orbs, robot or sparkle glyphs, "AI-powered", glass panels, floating blobs, a ChatGPT-like frame. The words "AI", "IA", "assistant" and "intelligence" do not appear on the page (a test checks it).

### 4.1 Second pass: how the sections are presented (2026-10-05)

The first pass fixed the structure but still looked like a template: a grid of identical white cards, each with an icon in a circle, pill buttons, large pale step numbers. Those are the marks of a generated page, so each section was re-set with a layout that carries meaning instead:

| Section | Presented as | Why |
|---|---|---|
| Welcome | Title left, the sentence that completes it right, level with the title's last line; the field is as tall as the list of example sentences beside it | The place to say a need is the biggest thing on the page; examples read as sentences you can tap, not as pills |
| What can we help you with? | An **index**: a strong rule, then one line per problem (name and domains on the left, the two example needs on the right), hairlines between | A problem is a sentence, not a card; no icons |
| Discover | Three short lists under the same strong rule | Rows (picture, name, place, price), never cards |
| From need to action | Two columns: title and what you can do on the left, **one list of five steps** on the right, tied by a vertical line (zigzag above and below a horizontal line in the first pass, replaced on 2026-10-05: it left half the cells empty and read as cluttered) | One reading direction, one idea per row, the same shape on phones |
| Then you can | A plain list under a strong rule; what works today in ink, what is coming in grey with *Soon* | The honesty about what is live is visible, not only written |
| Live figures | A white band: the figure as a large numeral with its words and a small drawing of what it counts (conversations, customers and businesses, transactions, merchants) | They say what Trezutil enables; they were *listings online, listings on promotion, cities covered*, which read as a listing site |
| For businesses | The only dark band of the page, photograph on the right, capabilities as a plain two-column list | A change of rhythm, and it keeps businesses secondary but unmistakable |

**Third pass: pictures (2026-10-05).** The welcome sits on a photograph of a shopping centre seen from above, supplied by the owner, under one flat navy veil with white type measured at 4.8:1 or better (it first used a CC0 photograph of a Kinshasa avenue, then the owner's aerial film, both replaced at the owner's request). Its elements were reduced by about a quarter (title 96 → 69 px, field and examples tighter; the whole welcome is about 630 px tall instead of about 800). The five needs in the index each have an original vector drawing; the business photograph is bundled instead of loaded from a third party. Sources, licences, what to know about the photograph (it is Spanish, with third-party brands) and how to replace it: [HOME_IMAGES.md](HOME_IMAGES.md).

**Fourth pass (2026-10-05).** *What can we help you with?*, *Discover what exists around you* and the live figures sit on **white bands** across the page (the card surface in dark mode), which gives the cream page a rhythm (the first two follow each other, so they share a single rule between them); a small **WhatsApp invitation** sits after *From need to action* (a placeholder until the number exists, see [OUR_COMPANY.md](OUR_COMPANY.md)); the figures band also meets the dark business band without the sliver of page that used to separate them. The figures are now **conversations started, connections between customers and businesses, transactions and merchants** (definitions: [DASHBOARDS_AND_SITE_CONTENT.md](DASHBOARDS_AND_SITE_CONTENT.md)); users and partners stay available to the administrators, and none of them is typed by hand. *Our company* (About, How it works, Contact us) lives **in the footer only**, by the owner's decision ([OUR_COMPANY.md](OUR_COMPANY.md)). In French the top bar needs about 1366 px, so below 1365 px the menu button takes over (it overflowed between 1180 and 1365 px before).

Everything stays typographic: Manrope for headings, DM Sans for text, hairlines, one accent colour for actions. French has its own title composition (two full-width lines) because the sentence is longer. Verified at 1440, 820 and 390 px, light and dark, in English and French.

## 5. What happens to the managed home content (Site content)

The home page's *order* is now the product narrative (§3), so administrators control **what shows, not the order**: Site content → Home layout toggles six sections (*What can we help you with*, *Discover*, *From need to action*, *Trezutil in numbers*, *For businesses*, *Partners*). The live figures and the partners stay fully managed. The five obsolete section keys (`showcase`, `services`, `properties`, `reservations`, `whatsapp`) remain in the stored document, invisible and unused, so **no backend change was needed**.

## 6. Acceptance

- *"I can come here and explain what I need."* The hero is a place to leave a need, with six examples.
- *"Trezutil can help me find a product, a service, a place or a solution."* Section 3, in problems.
- *"Then I can buy, book, contact or ask."* §3.4: the loop and the actions under it (buy, book and pay are live; contacting a seller and requesting a quote are honestly marked *Soon*).
- *"If I remove every listing card, is Trezutil still understandable?"* Yes: with no offers at all the page keeps the hero, the four problems that always show, the loop, the businesses section and the partners, and the Discover block simply disappears; `src/pages/home.test.tsx` removes every offer and checks exactly that (and that no listing card ever appears, even when offers exist).

## 7. Open points for the owner

1. **The logo mark is a sparkle** (the Figma export's mark, used on every page and console). It is the one remaining "AI" glyph; replacing it is a brand decision, so it was not changed.
2. **Photo and Location** actions need platform support (attachments in the conversation; a geolocation parameter) before they can be offered.
3. **Contact the seller / request a quote** are shown as *Soon* until the seller inbox exists (gap G-B8).
4. **Get around** returns by itself once vehicle or transport offers are live.
5. Seller section photo: kept as a plain, human photograph without invented status badges; swap it for a real merchant photo when available.
