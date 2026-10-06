# Primary navigation

How the primary navigation is built and why. Rewritten 2026-10-06 on the owner's brief; it replaces the needs-based menu of 2026-10-05 (*Find a place · Get something · Find a service · Go out · Discover*). Frontend only: no route, API or page was added, and no backend logic changed.

```text
Comment ça marche    Découvrir ▾    Organiser ▾    Activités ▾            [Make a request]
How it works         Discover ▾     Organise ▾     Activities ▾
```

Trezutil is presented as **one Commerce OS that helps people discover things, organise objectives and execute commercial actions**, not as "housing + products + services + restaurants":

- **Discover** = what exists.
- **Organise** = what I want to accomplish.
- **Activities** = what I have already started with Trezutil.
- The conversation stays the central interface: the **Make a request** button of the header (and *Ask* in the phone's bottom bar) is unchanged.

No other primary entry exists. *For businesses*, language, theme, account and the bag are the header's actions, not navigation.

## The four entries

**How it works** (direct link) → `/about#how`, the section of the About page that follows the loop **Conversation → Intention → Discovery → Action → Transaction**. Its five stages are now labelled by those names and its heading says Trezutil is not a listing site but one commerce system from the conversation to the transaction (`LoopSection`, shared with the home page).

**Discover ▾**

| Entry (FR) | Line under it | Leads to |
|---|---|---|
| Produits | Trouver ce dont tu as besoin | `/search?vertical=RETAIL` |
| Services | Trouver quelqu’un pour le faire | `/search?vertical=SERVICES` |
| Lieux & sorties | Restaurants, lieux et activités | `/search?vertical=RESTAURANT` |
| Logements | Séjours et hébergements | `/search?vertical=HOTEL` |
| Offres & recommandations | Ce que Trezutil te suggère | `/discover` |

**Organise ▾** — entry points into the existing Organiser/Mission system, not workflows of their own. Each opens the Organiser page with the **beginning of a sentence** in the field, the cursor at its end (`/organize?q=…`, the same starters the page offers):

| Entry | Line | Starts |
|---|---|---|
| Voyage | Transport, logement, activités | « Je veux organiser un voyage à » |
| Événement | Fête, anniversaire, cérémonie | « … un événement pour » |
| Séjour | Logement, transport, activités | « … un séjour à » |
| Déménagement | Logement, transport, aide | « … un déménagement vers » |
| Transport | Déplacement, transfert, chauffeur | « … un transport pour » |
| Autre projet | Dis-moi ce que tu veux organiser | `/organize`, an empty field |

**Activities ▾** — the person's own pages, at the places that already exist:

| Entry | Line | Leads to |
|---|---|---|
| Mes missions | Ce que j’organise | `/organize#missions` (the list of missions on the Organiser page) |
| Mes demandes | Ce que j’ai demandé à Trezutil | `/conversation` |
| Mes commandes | Mes achats en cours | `/orders` |
| Mes réservations | Mes réservations | `/reservations` |
| Mes favoris | Ce que j’ai sauvegardé | `/wishlist` |
| Historique | Mes activités passées | `/notifications` (the activity feed of orders and bookings) |

A guest sees the same entries (each leads to the existing sign-in prompt) and one quiet line in the panel: *Connecte-toi pour retrouver ici ce que tu as commencé.*

## Closest destinations, stated

Nothing was invented for the menu. Where the brief's entry has no page of its own, the nearest existing one is used:

- **Logements** leads to hotels and stays (`vertical=HOTEL`), because search takes one category at a time. Homes and apartments (`REAL_ESTATE`) are no longer in the primary navigation; they remain in the home page index (*Homes and apartments · Hotels*). Change the destination in `src/lib/nav.ts` if homes should come first.
- **Lieux & sorties** leads to restaurants (the only catalogue vertical for going out); **Offres & recommandations** to *Discover*.
- **Mes demandes** leads to the conversation (it keeps the person's requests); there is no separate history of requests. **Historique** leads to the activity feed (orders and reservations), which is what exists.

## How it behaves (`src/components/nav.tsx`)

A menu is a **disclosure**: a button that shows a short list of ordinary links under its word (`aria-expanded`, `aria-controls`). It opens on click, Enter/Space, ArrowDown (first entry) or ArrowUp (last), and, for a mouse, when the pointer rests on it. A menu opened only by hovering closes shortly after the pointer leaves; one that was clicked or opened from the keyboard stays until the word is clicked again, **Escape** (focus returns to the word), a click elsewhere, the focus leaving it, or a link being followed. One menu is open at a time. Inside a panel the arrows move (and wrap), Home/End jump. The panel opens with a 170 ms fade and a 4 px rise, and none under *reduced motion*.

The header word of the menu that holds the current page is marked, except that *Mes missions* (the Organiser page) marks Organise only.

## Look

Light on purpose: a picture before each of the four words (*How it works* a play mark, *Discover* a magnifier, *Organise* layers, *Activities* a pulse), a small caret on the three that open, and a panel that is a single list with a hairline, a soft shadow and, per entry, a small icon tile, a title and one line. No mega-menu, gradient, glow, card, badge or button. The four words are one size (0.97 rem, 0.93 rem in French) and **one box** (2.6 rem high, same padding, rounded on hover), so the link and the three buttons line up; the panels are 21.5 rem wide with 0.98 rem titles. It uses the tokens both frames define, so it follows light and dark mode (`src/styles/nav.css`).

**Alignment.** The navigation sits **next to the brand, in one row, at every width**, with the actions pushed to the right. It used to be centred on the page by absolute positioning, which collided with the actions once the words were larger. The bar shows it from 1201 px up (it needs about 1160 px in French); below that the menu button takes over.

## One header, one phone menu, on every page

The home page used to have its own header (a different width, the navigation on the left, different breakpoints, a dropdown instead of the drawer) and the other pages another one; the owner saw two different menus. There is now **one** `SiteHeader` (`src/components/header.tsx`) used by every product page and by the home frame (`HomeFrame`: home, About, legal), one drawer (`MobileMenu`) and one bottom bar (`MobileNav`). The only difference is that on the home page the bar scrolls with the page (`sticky`) instead of being fixed over it. A test renders the home page and four others and compares the bar, the phone menu and the bottom bar.

The header holds, from the left: the brand, the navigation, then *Kinshasa* (wide screens only), *For businesses* (from 1361 px; always in the drawer and the footer), the bag, theme, language, the account (an avatar, or a *Sign in* button for a guest) and the request button. **The bell is gone**: its page, the activity feed, is *Activities → History*.

**On a phone** (the drawer, and the same drawer on the home page): the request button, *How it works*, then the three groups under their own heading (with its picture) and the line under each entry, then *Bag*, *For businesses*, language and account. *Orders, Reservations, Saved* are no longer separate entries: they are in Activities. The bottom bar (Home, Discover, Ask, Orders, Account) is unchanged.

## No bar across the bottom of a page

The dark "Ask Trezutil anything about this page" bar (`ConversationDock`) is removed from every page, and with it the `hideDock` and `dockPrompt` options of `AppShell`. The header's request button is the one way into the conversation. The pages that have a question of their own already offered it inline (a listing, an order, a booking, the search results); the hotel page, which only had the bar, now has the same quiet inline entry (*Demander à Trezutil à propos de cet hôtel*). The floating chat widget stays unmounted, as decided on 2026-10-05.

## One list

`src/lib/nav.ts` holds the structure once (names, lines, icons, destinations); the header, the home header and the phone menus read it. `src/lib/organise.ts` holds the sentence starters shared with the Organiser page. The needs list (`src/lib/needs.ts`) is now only the index of the home page.

## Changed from before

- The needs-based menu is gone from the header and the drawer, and with it `MENU_NEEDS`, `otherSearches`, `primaryNav` and `PrimaryNavLinks`.
- **"How it works" is back in the primary navigation**, by the owner's new brief. On 2026-10-05 the owner had kept company entries out of it (the footer column *Our company* still links to it, see [OUR_COMPANY.md](OUR_COMPANY.md)); a dropdown for *Our company* had been tried and removed because the French bar overflowed. This bar is much shorter than that one was.

## Not changed

Pages, routes, the backend, the consoles' own menus, the footer and the bottom bar of a phone.

## Changing it

Edit `src/lib/nav.ts` (add the French in `src/i18n/fr.ts`; a test enforces it). `src/components/shell.test.tsx`, `src/pages/home.test.tsx` and `src/lib/i18n.test.tsx` then show what moved.
