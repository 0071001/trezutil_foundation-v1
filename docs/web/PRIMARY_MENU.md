# Primary menu

How the primary menu is built and why. Written 2026-10-05, after the home page was redesigned around the person's need (see [HOME_PAGE_REDESIGN.md](HOME_PAGE_REDESIGN.md)). Frontend only: no route, API or page was added or removed.

## What changed

The menu used to be the list of **catalogue categories**: *Discover · Products · Services · Real Estate · Reservations*. That is the shape of a listing site, and *Reservations* is not a way to start anything: it is the person's own page. The menu is now the **needs Trezutil helps with**, in the order of the home page, then *Discover* to browse:

| Entry (EN) | Entry (FR) | Leads to | Icon |
|---|---|---|---|
| Find a place | Trouver un logement | `/search?vertical=REAL_ESTATE` | home |
| Get something | Trouver un produit | `/search?vertical=RETAIL` | shop |
| Find a service | Trouver un service | `/search?vertical=SERVICES` | service |
| Go out | Sortir | `/search?vertical=RESTAURANT` | food |
| Discover | Découvrir | `/discover` | search |

The first action is not in the list but beside it: the **Make a request** button (a speech bubble) opens the conversation, and so does *Ask* in the bottom bar of a phone.

## One list for every place

`src/lib/needs.ts` holds the needs once: name, icon, where each is met, and the example sentences. The **home page index**, the **bar** (home and every other page) and the **mobile drawer** all read it, so a need cannot be named or routed differently from one to another. *Get around* is left out of the menus because it exists only when vehicle or transport offers are live (the home page asks the search API for that; a menu cannot do it on every page).

## Where it shows

- **Home page bar** (`pages/Home.tsx`): the five entries as text, *For businesses* as a separate, quieter link on the right, then language, theme and sign-in. Below 1180 px (1365 px in French, whose labels are longer) the bar becomes a menu button.
- **Bar of every other page** (`components/shell.tsx`): the five entries with their icons, from 1401 px up; below that the drawer takes over. The call to action keeps its icon only at most widths, so it carries an accessible name (*Make a request*).
- **Drawer** (phones and small laptops): *Make a request*; the needs, with the other searches of a need under it (*Hotels* under *Find a place*, because search takes one category at a time); *Discover*; then *Orders, Reservations, Saved, Bag*; then *For businesses*; language and account.

## Decisions

- **Reservations leaves the primary menu** and sits with Orders and Saved (the drawer, the account page, the home footer). It is still one tap away; it simply no longer presents itself as a thing to browse.
- **"Find a place" leads to homes**, not hotels, because the search accepts a single category. Hotels are one link below it in the drawer, in the home index (*Homes and apartments · Hotels*) and in the categories of *Discover*. If hotels should come first, change the first domain of that need in `needs.ts`.
- **Short French labels.** The French names are the longest strings of the bar. They were kept short enough for one line at 1440 px (checked), with slightly less spacing on the home page in French.
- **"For businesses" is not a need**, so it is not in the list: it is the second audience and stays secondary.

## Fixed along the way

- **The mobile drawer of every inner page was squeezed into the height of the header** and so was unusable on phones. It already was before this change (checked on the previous build). The header blurs what is behind it, which makes it the frame of every fixed child; the drawer now sits beside the header, and a test (`shell.test.tsx`) pins it.
- **The call to action had no accessible name** (its text is hidden at most widths and its icon is decorative).
- **The sparkle glyph** on the call to action, in the drawer, in the bottom bar and on the page-level conversation bar is now a speech bubble. The logo mark is unchanged (see the open point in the redesign note).

## Not changed

- The **seller and administration consoles** have their own menus (module registry in `src/portal/modules.ts`).
- The search composer of *Discover* and *Search* still shows its sparkle; those pages were not part of this pass.
- No page was added: an entry is a link to a page that already existed.
- **"Our company" is not in the primary menu**, on purpose (owner's decision, 2026-10-05): the menu is for what a person came to do. It is a column of the footer, see [OUR_COMPANY.md](OUR_COMPANY.md). A dropdown for it was tried first and removed: with it the French bar overflowed by about 170 px at 1440 px.

## Changing it

Edit `src/lib/needs.ts` (name, icon, destination, examples; add the French in `src/i18n/fr.ts`, a test enforces it). The bar, the drawer and the home page follow. `src/components/shell.test.tsx` and `src/pages/home.test.tsx` then show what moved.
