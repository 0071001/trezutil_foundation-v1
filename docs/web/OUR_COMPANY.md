# Our company

*Added 2026-10-05.* Where a visitor finds out what Trezutil is and how to reach it.

## Decision

**"Our company" is in the footer only.** It is not in the primary menu, nor in the mobile menus (the owner's decision, after a dropdown in the top bar was tried: the French bar overflowed by about 170 px at 1440 px, and the menu is meant for what a person came to do). The footer has a column, *Our company* / *Notre entreprise*, between *Discover* and *For businesses*.

| Entry | Leads to |
|---|---|
| About Trezutil / À propos de Trezutil | `/about` |
| How it works / Comment ça marche | `/about#how` (the five steps) |
| Contact us / Nous contacter | `/about#contact` |

The list is `COMPANY_LINKS` in `web/frontend/src/lib/company.ts`; the footer (`components/home/chrome.tsx`) reads it.

## The About page (`/about`)

Built in the home page's own frame (`HomeFrame`: top bar, footer, phone bar) and look, from what the site already says:

1. **Title and one paragraph**: *Commerce through conversation.* and what Trezutil helps people do (find a place to live, something to buy, someone to call, a table for tonight). It says Trezutil *currently serves Kinshasa* (the same sentence as the location tooltip).
2. **Three statements**: you start from a need, not a catalogue; offers come from real businesses (with their price and, where it applies, availability); you can act on it (buy, book, pay online; contacting a seller and quotes are said to be on their way, as on the home page).
3. **How it works**: the home page's own block (`LoopSection`), so both say the same thing.
4. **Live figures and partners**: the home page's own blocks, which hide themselves when there is nothing to show. If an administrator hides *Live figures* or *Partners* from the home page (Site content), they are hidden here too.
5. **Contact us**: *Make a request* (the conversation), WhatsApp when `VITE_WHATSAPP_NUMBER` is set (nothing otherwise), and a line for businesses (*Start selling on Trezutil*).

**Nothing is invented**: no team, no founding date, no address, no email, no phone, no "careers" or "press". A test checks that the page contains none of those words. The French is complete (a test enforces it).

## WhatsApp, PlayStore and AppStore: placeholders first, real values later

Right after the tagline, the footer says **"Disponible sur WhatsApp, PlayStore et AppStore."** (English: "Available on WhatsApp, PlayStore and AppStore.") with a **QR code** for the WhatsApp chat, and the home page has a small **WhatsApp block** (*Trezutil, directement dans votre WhatsApp*), between *From need to action* and the live figures. None of the three channels is connected yet (no WhatsApp Business number configured, the mobile app is not published), so, **at the owner's request (2026-10-05), they are shown as placeholders now and connected one by one when the real values exist.**

| Where | Placeholder (now, on staging) | Real (when the value is set) |
|---|---|---|
| Footer line | the three names as plain text (not links) | each name becomes a link as its value is set |
| Footer QR | a grey frame with no data in it (three finder squares and a speech bubble), caption "Code QR bientôt disponible" | the real QR code of `https://wa.me/<number>`, caption "Scannez pour discuter avec Trezutil sur WhatsApp" |
| WhatsApp block | the same text, a **disabled** button "Écrire à Trezutil sur WhatsApp" with "Le numéro WhatsApp arrive bientôt.", a QR frame | an active button to the chat (with a greeting) and the real QR code |
| *Find us on WhatsApp* (footer button) | not shown (it needs a number) | shown |

**How it is switched.** Placeholders appear only when the build variable `VITE_CHANNEL_PLACEHOLDERS` is exactly `true`. It is set in `deployments/docker/.env.staging` (git-ignored) and is **empty in production**: a production build without real values mentions no channel at all, so the site never says "available on the App Store" about an app that is not there. A placeholder is never a dead control: names are text, the button is disabled with a visible reason, and the QR frame holds no data (it cannot be scanned, so it can never send anyone to a number nobody set).

**To connect a channel** set its variable and rebuild the image (`docker compose ... build migrate`, then recreate `api-gateway`); nothing else changes:

| Channel | Variable (build argument) | Value |
|---|---|---|
| WhatsApp (QR code, button, footer link) | `VITE_WHATSAPP_NUMBER` | digits only, with the country code |
| PlayStore | `VITE_PLAY_STORE_URL` | the full https address of the app's page |
| AppStore | `VITE_APP_STORE_URL` | the full https address of the app's page |

The footer sentence is built from the channels that are shown (real ones, plus the placeholders when those are on), in the language's own list rules; a store address that is not https is ignored. **Before launch**, set the three real values and leave `VITE_CHANNEL_PLACEHOLDERS` empty in the production environment.

**The WhatsApp block** says only what the conversation does (understands a need written in a message, finds and compares offers, checks availability; nothing to install; no sign-up form, the number is enough; in French or English) and nothing about speed, price, support, delivery or payment: a test checks the words. It is one of the home blocks an administrator can hide (Site content → Home layout: the server already knew the key `whatsapp`, so there is no backend change). The QR code is not shown on a phone (a QR code is for another phone's camera), only the button.

**The QR code** is drawn in the browser as one SVG path (`components/QrCode.tsx`, with the MIT-licensed `qrcode-generator`, 14 KB): nothing is loaded from a third party and the address is not sent anywhere. It encodes the short address `https://wa.me/<number>`, so a small code with large modules, easy to scan, always dark on a white tile with the four-module quiet zone in every theme. Checked by decoding the rendered code with an independent reader (jsQR) in light, dark and phone layouts. The placeholder frame is `QrPlaceholder` in `components/channels.tsx`.

## The floating chat is gone

The floating **"Ask Trezutil"** chat (and its WhatsApp button) was removed on 2026-10-05 at the owner's request: `App.tsx` no longer mounts it. The component and its tests are kept, unmounted (see `FRONTEND_PRODUCTION_READINESS.md`).

## Scrolling to a place on a page

`/about#how` and `/about#contact` need the app to scroll to a section after a page change (it only scrolled to the top). `RouteFocus` in `App.tsx` now scrolls to the element whose `id` is in the address once the page has drawn it (pages load on demand, so it retries for a second), and to the top when there is no such element. Sections that can be linked to leave room for the sticky bar (`scroll-margin-top` in `home.css`).

## What is missing, and needs a decision from the owner

These would normally sit under "Our company", and cannot be written without facts only the owner has:

- **A real contact channel**: an email address, a phone number or a postal address. Today *Contact us* leads to the conversation with Trezutil (and WhatsApp once a number is configured), which is not a human customer service. Say so before promising answers.
- **Team, careers, press, legal pages** (terms, privacy): none exists. Add them as pages, then as entries in `COMPANY_LINKS` (and their French).
- **Mission and values**: the three statements above restate what the product does; a mission statement is the owner's to write.

## Tests

`src/pages/about.test.tsx`: the page (title, statements, five steps, contact, WhatsApp only when configured, hidden blocks, French, nothing invented), the footer column (English and French), the absence of any company entry in the primary menus, and scrolling to a place.
