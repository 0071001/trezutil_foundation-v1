# Figma → Mobile implementation map (Phase 0)

> Status: **Phase 0 — discovery only.** No Flutter code exists yet. This document inventories
> what the Figma export contains, and maps every element to a backend source, a Flutter
> component, a token, an asset and a motion. Where the Figma or the repository is silent, this
> document says **ABSENT** instead of guessing. Companion document:
> [MOBILE_IMPLEMENTATION_PLAN.md](MOBILE_IMPLEMENTATION_PLAN.md).

## 0. Source and method

| Item | Value |
|---|---|
| Figma source | `Design Trezutil Mobile App/` (Figma Make export: React 19 + Vite + Tailwind v4, state-based navigation). **Untracked in git** (`?? "Design Trezutil Mobile App/"`). |
| Read in full | `src/App.tsx`, `src/types.ts`, `src/index.css`, `src/main.tsx`, `components/{BottomNav,VoiceBars,icons}.tsx`, `screens/{Home,Conversation,Discover,Activity,Profile}Screen.tsx`, `AGENTS.md`, `package.json`, `.figma/make/site.json` |
| What the export is | A clickable prototype. Layout, colours, type, spacing, motion are real design decisions. Data, handlers and the voice flow are **simulated** (§9). |
| Frame | One 390 pt-wide phone column (`maxWidth: 390`); the rounded desktop "phone frame", fake status bar (9:41) and `#070E1C` backdrop are presentation only and **not part of the app**. |
| Theme | Light and dark, both fully tokenised in `types.ts › makeTheme(dark)`. |
| Languages in Figma | French only (informal **"tu"**). |

A Figma Make export is not a design file with frames, variants and prototypes. Everything below
is derived from the code of the export, which is the only artefact available. There is **no
exported frame list, no component/variant library, no asset folder and no Figma variables
file** in the folder.

## 1. Complete screen inventory

The export contains **5 screens + 1 persistent shell**. Nothing else exists.

| # | Figma screen | Figma id | Role | Tab |
|---|---|---|---|---|
| 1 | Home — "Accueil" | `home` | Entry point: ask anything, shortcuts, nearby, personalised, resume | Accueil |
| 2 | Conversation — "Demander" | `conversation` | The AI conversation (text + voice) with result carousels | Demander (centre) |
| 3 | Discover — "Découvrir" | `discover` | Search + category browse + trending + nearby + collections | Découvrir |
| 4 | Activity — "Historique" | `activity` | Conversations, reservations, orders history | Activité |
| 5 | Profile — "Compte" | `profile` | Account card, preferences, account, support, sign out | Profil |
| S | Bottom navigation | — | 5-tab bar, centre "Demander" emphasised | all |

### 1.1 Screen 1 — Home

| Element | Detail from Figma | Notes |
|---|---|---|
| Header | Wordmark text "Trezutil" (Instrument Sans 22/700, tracking tight) · city pill "Kinshasa" with map-pin · avatar circle 32 pt with initials "DM" on `blue` | **No logo image**; wordmark is plain text. Pill and avatar have no handler. |
| Greeting | "Bonjour, Diallo" (14, `txt3`) · H1 "Que cherches-tu\naujourd'hui ?" (Instrument Sans 25/700, −0.02em) | Name hard-coded. |
| Composer card | Surface card, radius 16, border 1.5 pt (`bdr` → `blue` on focus), focus ring 3 pt `blue`@~9 %, rest shadow `0 1 6 rgba(0,0,0,.04)`; text field 15 pt; **rotating placeholder** (5 prompts, see §7); bottom bar with "Parler" mic button (left) and char counter `N car.` + send button 32 pt (right) | Mic button navigates to Conversation (it does **not** start recording). Send is disabled-looking (`s2`) when empty, `blue` + shadow when filled. |
| Category quick start | Section label "EXPLORER PAR CATÉGORIE" (11/600 uppercase, tracking .08em); horizontal chips: Immobilier, Restaurants, Produits, Services, Hôtels, Expériences; each chip has its own accent (blue / coral / blue / green / violet / coral) with a tinted background in light mode and neutral `s2` in dark | Every chip goes to Discover with **no filter** passed. |
| "Près de toi" | Title 15/600 + link "Tout voir"; horizontal cards 176 pt wide: image 108 pt high, status tag top-left ("Ouvert"/"Disponible", green), title, subtitle ("Restaurant · 0.4 km"), price tier (`$`/`$$`/`$$$`), star + rating | Card has no handler. |
| "Pour toi" | Title + link "Voir plus"; rows with 60 pt thumbnail, title ("2 ch à Gombe · $720/mois"), reason line ("Car tu as cherché des appartements"), chevron | A **recommendation with a stated reason** — see MISSING §10. |
| Resume card | "Continuer la recherche" / "Appartements à Gombe · 6 résultats" / "Il y a 1h · Conversation en cours"; tinted `bfaint` card, 1 pt blue@19 % border | Goes to Conversation. |

### 1.2 Screen 2 — Conversation

| Element | Detail from Figma |
|---|---|
| Header | Back button 36 pt (`s2`, arrow-left) → Home · centre "Trezutil" (Instrument Sans 15/600) over a green dot + "Disponible" (11) · "more" button 36 pt (`s2`, three dots, no handler) · 1 pt bottom border |
| Context label | Centred pill "Aujourd'hui · Appartements à Gombe" (11/500, `s2` background) |
| User message | Right-aligned bubble, `navy` background, text `#EEF2F8`, max-width 75 %, radius 16 with **top-right 4**, 14 pt; animation `fadeUp` |
| Assistant message | Left; 24 pt blue circle with white "T" (9/700) as avatar; text 14 pt (no bubble, plain on background), optional **result carousel**, optional **suggestion chips**, timestamp (10, `txt3`, `fr-FR` `HH:mm`); animation `slideInLeft` |
| Result carousel (`CommerceCarousel`) | Horizontal scroll, cards 208 pt wide, radius 16: image 128 pt high; optional badge bottom-left (green, e.g. "Parking"); rank index circle top-right (navy@80 %, 1-based); body: subtitle (11), title (14/600), **price** (Instrument Sans 16/700 in `blue`), location (11), full-width **"Voir"** button (`blue`, 12/600, white) |
| Suggestion chips | Wrapping pills: `bfaint` background, `blue` text, 1 pt `blue`@19 % border, 12/500 — **tap handler is a no-op in the export** |
| Composer (idle) | Container with top border, `surface` background; inner row radius 16, `s2` background, 1 pt border: text field "Continuer la conversation…" · mic button 32 (`surface`) · send button 32 (`blue` when text) |
| Voice hint | "Tu peux aussi dire « le deuxième » ou « organise une visite »" (10, centred) shown only in idle |
| Voice — `listening` | Composer replaced by a card with `blue` 1.5 pt border: cancel (X) 32 pt · 9 animated bars (28 pt high) + "En écoute · m:ss" · stop button 36 pt (`blue`, white stop icon, shadow) |
| Voice — `processing` | Card: three pulsing dots + "Je comprends..." |
| Voice — `speaking` | Card: play/pause 32 pt circle · 9 bars (20 pt high, paused → 40 % opacity) · "Arrêter" text button · thin 2 pt progress track with a **fixed 38 %** fill |
| Auto-scroll | Smooth scroll to the end on new message or voice state change |

### 1.3 Screen 3 — Discover

| Element | Detail |
|---|---|
| Header | Label "DÉCOUVRIR" (11/600 uppercase) |
| Search field | Surface field, radius 16, 1.5 pt border (`blue` on focus + 3 pt ring `blue`@8 %), search icon 17 pt, placeholder "Cherche ou décris ce que tu veux…"; Enter → opens Conversation with the text |
| Hint | While focused: "Appuie sur Entrée pour lancer une conversation" (11) |
| Category pills | Tout · Immobilier · Restaurants · Produits · Services · Hôtels; active pill = `blue` fill + white text; inactive = `surface` + border. **Active state does not filter anything in the export.** |
| "Tendances à Kinshasa" | Trending icon + label; **featured cards** full width, 144 pt high, image + bottom gradient (`rgba(8,15,26,.8)` → transparent at 50 %), title 16/600 white, subtitle 12 white@70 %, badge top-right white@95 % ("Populaire", "Ouvert maintenant") |
| "Autour de toi" | Map-pin + label, link "Carte"; horizontal cards 176 pt wide: image 112 pt, type chip top-right (surface@93 %), title, distance, star + rating |
| "Collections" | 2×2 grid of tinted tiles (title 14/600, count 11, chevron); tap → Conversation with the collection title as query |

### 1.4 Screen 4 — Activity

| Element | Detail |
|---|---|
| Header | Label "MON ACTIVITÉ" + H2 "Historique" (Instrument Sans 22/700) |
| Conversations | Rows (radius 16, border): 36 pt icon tile (`bfaint`, clock icon in `blue`) · status dot (active `#1A69F0`, paused `#D97706`, completed `#1A7A4A`) + title (14/600, truncated) · preview (12) · "time · N messages" (10) · chevron. Row → Conversation (no id passed) |
| Reservations | Cards: 64 pt thumbnail · title · detail ("Vendredi 18 oct · 20h00 · 2 personnes") · status **pill** tinted with its colour at 12.5 % ("Confirmée" green, "En attente" amber) |
| Orders | Same card; status shown as **dot + text** ("En route", blue) — different from reservations (pill vs dot) |
| Empty/"all caught up" footer | Card with check icon, "Tu es à jour", "Aucune activité en attente de ton attention." — shown **always** below the lists in the export (not a real empty state) |

### 1.5 Screen 5 — Profile

| Element | Detail |
|---|---|
| Header | Label "COMPTE" |
| Profile card | 64 pt rounded-square avatar (`blue`, initials "DM", Instrument Sans 20/700) · name "Diallo Mbeki" (19/700) · map-pin + "Kinshasa, RDC" · email · divider · **3 stats**: Conversations 14 · Réservations 4 · Achats 2 (Instrument Sans 20/700 over a 10 pt label) |
| Group "PRÉFÉRENCES" | Localisation (value "Gombe, Kinshasa") · Langue (value "Français") · Notifications (value "Activées") · Mode sombre/clair (**switch** 44×24, thumb 20, 200 ms) |
| Group "COMPTE" | Éléments sauvegardés · Paiements · Confidentialité & Sécurité |
| Group "SUPPORT" | Centre d'aide |
| Sign-out | Danger row "Se déconnecter" (`#E8634A` on `#FFF0ED` tile) |
| Footer | "Trezutil · Version 1.0.0" (10, centred) |
| Behaviour | Only the dark-mode switch is wired in the export; every other row has no destination. |

### 1.6 Shell — bottom navigation

Five equal-width tabs, `minHeight 56`, top border, `surface` background, bottom safe-area inset
respected (`env(safe-area-inset-bottom)`):

| Tab | Label | Icon | Active style |
|---|---|---|---|
| `home` | Accueil | house | icon `blue` (stroke 2 vs 1.75) + 4 pt dot under the icon + `blue` label |
| `discover` | Découvrir | compass | same |
| `conversation` | **Demander** | message | **Centre button**: 40 pt rounded-16 tile; active = `blue` fill + white icon + shadow `0 4 12 blue@25 %`, inactive = `s2` fill + `txt3` icon; label 10/600 |
| `activity` | Activité | clock | same as Home |
| `profile` | Profil | user | same |

The bar is visible on **every** screen, including Conversation (it keeps the bar below the
composer in the export — see §11, keyboard/safe-area behaviour is undefined in Figma).

## 2. Navigation map

```
                          ┌───────────── BottomNav (persistent, 5 tabs) ─────────────┐
   Accueil ─────────┐     │                                                          │
   Découvrir ───────┤     │  Tabs are peers; tab switch = instant swap (no transition│
   Demander ────────┼──►  │  defined in Figma). No back stack between tabs.          │
   Activité ────────┤     └──────────────────────────────────────────────────────────┘
   Profil ──────────┘

Edges present in the export (all are "go to tab X", never deeper):
  Home  ─ composer submit / "Parler" / resume card ───────────► Conversation
  Home  ─ category chip ──────────────────────────────────────► Discover   (no filter)
  Discover ─ search Enter / collection tile ──────────────────► Conversation (query is stored but never read)
  Activity ─ conversation row ────────────────────────────────► Conversation (always the same thread)
  Conversation ─ back arrow ──────────────────────────────────► Home
```

**ABSENT from Figma (no screen, no edge):** listing detail, store/merchant page, compare,
cart, checkout/payment, order detail, reservation/booking form, reservation detail,
sign-in/OTP, onboarding/permissions, notifications inbox, map view (the "Carte" link has no
target), filters sheet, "more" menu of the conversation, location picker, language picker,
saved items, payments, privacy & security, help centre, settings sub-pages, any dialog/sheet/
toast, any error/loading/empty/offline state.

## 3. Component inventory

### 3.1 Present in Figma (to be built to match)

| # | Component | Source | Variants / states in Figma | Flutter widget (proposed) |
|---|---|---|---|---|
| C1 | Bottom navigation | `BottomNav` | 5 tabs; centre emphasised; active/inactive | `TzBottomNav` |
| C2 | Voice bars | `VoiceBars` | `playing` true/false; colour, height, 9 bars with per-bar duration/delay/max height (table in §7) | `TzVoiceBars` (`AnimationController` + `CustomPainter`) |
| C3 | Icon set | `icons.tsx` | 33 stroke icons; `size`, `color`, `strokeWidth` (nav only); `StarIcon` has `fill` | `TzIcons` (custom painter or SVG assets, see §6) |
| C4 | Composer card (Home) | `HomeScreen` | idle / focused; empty / filled; animated placeholder | `TzHomeComposer` |
| C5 | Conversation composer | `ConversationScreen` | idle · listening · processing · speaking | `TzChatComposer` + `TzVoiceComposer` |
| C6 | User bubble | `MessageRow` | single | `TzUserBubble` |
| C7 | Assistant message row | `MessageRow` | text · text+carousel · text+suggestions · timestamp | `TzAssistantMessage` |
| C8 | Result card (carousel) | `CommerceCarousel` | with/without badge; rank index | `TzResultCard` |
| C9 | Suggestion chips | `SuggestionChips` | single | `TzSuggestionChip` |
| C10 | Context pill | Conversation | single | `TzContextPill` |
| C11 | Section label | all screens | uppercase 11/600 | `TzSectionLabel` |
| C12 | Category chip (accent tinted) | Home | 4 accents; light tinted / dark neutral | `TzCategoryChip` |
| C13 | Category pill (filter) | Discover | active / inactive | `TzFilterPill` |
| C14 | Place/offer card (176 pt) | Home "Près de toi", Discover "Autour de toi" | tag top-left (Home) vs type chip top-right (Discover); price tier vs distance | `TzPlaceCard` (two layouts) |
| C15 | Featured trend card | Discover | image + gradient + badge | `TzFeaturedCard` |
| C16 | "For you" row | Home | thumbnail + title + reason + chevron | `TzSuggestionRow` |
| C17 | Resume card | Home | single | `TzResumeCard` |
| C18 | Collection tile | Discover | 4 accents (tinted/neutral) | `TzCollectionTile` |
| C19 | Search field | Discover | idle / focused (+hint) | `TzSearchField` |
| C20 | Conversation row | Activity | status dot active/paused/completed | `TzConversationRow` |
| C21 | Reservation card | Activity | status pill | `TzReservationCard` |
| C22 | Order card | Activity | status dot+text | `TzOrderCard` |
| C23 | "All caught up" card | Activity | single | `TzCaughtUp` |
| C24 | Profile card + stats | Profile | single | `TzProfileCard` |
| C25 | Setting group / row | Profile | normal · with value · danger · trailing switch | `TzSettingGroup`, `TzSettingRow` |
| C26 | Switch | Profile | on/off | `TzSwitch` (match 44×24/20, **not** Material default) |
| C27 | Avatar | Home (32 pt circle), Profile (64 pt rounded square), Conversation (24 pt "T" circle) | 3 sizes/shapes | `TzAvatar` |
| C28 | Status dot | Conversation header, Activity | green / blue / amber / grey | `TzStatusDot` |
| C29 | Round icon button | back/more (36 pt, `s2`), mic (32), cancel (32), stop (36) | single | `TzIconButton` |
| C30 | Primary button ("Voir") | carousel card | single | `TzButton.primary` (only one variant exists) |

### 3.2 Needed by the product but **ABSENT** from Figma

`TzButton` secondary/ghost/disabled/loading, text field variants + error, form (phone/OTP),
OTP code input, modal bottom sheet, dialog, toast/snackbar, skeleton, empty state, error
state, offline banner, pull-to-refresh, quantity stepper, date/time/guest pickers, price
summary, payment sheet, map pin/cluster, listing gallery, rating display with count,
notification row, badge counter. **None will be invented in the Figma-faithful screens.**
Phase 3+ must either receive Figma designs for these or implement the minimum
platform-native/token-only version, clearly labelled as *not in Figma* in the screen map.

## 4. Design-token mapping

### 4.1 Figma tokens (authoritative for fidelity — from `types.ts › makeTheme`)

| Figma token | Light | Dark | Role |
|---|---|---|---|
| `bg` | `#F9F8F5` | `#080F1A` | Screen background |
| `surface` | `#FFFFFF` | `#0E1E33` | Cards, bars, inputs |
| `s2` | `#EEF2F8` | `#162640` | Chips, tiles, secondary surfaces |
| `s3` | `#F5F7FB` | `#1A3050` | Defined; **not referenced** by any screen |
| `txt` | `#0B1929` | `#EEF2F8` | Primary text |
| `txt2` | `#4A6080` | `#8BA4BC` | Secondary text |
| `txt3` | `#8898AA` | `#5C7290` | Tertiary / placeholders / inactive icons |
| `bdr` | `#DCE5F2` | `#1A3050` | Borders, dividers |
| `blue` | `#1A69F0` | `#4B8DF5` | Primary / accent |
| `bfaint` | `#EBF4FF` | `#0F2040` | Tinted primary background |
| `navy` | `#0B1929` | `#0B1929` | User bubble, rank badge |

**Hard-coded colours used outside the theme object** (must become tokens, none is theme-aware
in the export):

| Value | Used for |
|---|---|
| `#1A7A4A` / `#C8EDD9` | "success" green + its tint (Services chip/collection, "Ouvert"/"Disponible" tags, confirmed status, completed dot, "Parking" badge) |
| `#E8634A` / `#FFF0ED` | coral accent + tint (Restaurants/Expériences chips, danger row) |
| `#7B4FD8` / `#F0EBFF` | violet accent + tint (Hôtels chip, Tech collection) |
| `#EBF4FF` | blue tint (Immobilier/Produits chips) |
| `#D97706` | warning/amber (paused, "En attente") |
| `#F59E0B` | star rating |
| Tailwind `green-500` (`#22C55E`) | "Disponible" presence dot |
| `rgba(8,15,26,.8)` → transparent | image gradient overlay |
| `rgba(255,255,255,.95)`, text `#0B1929` | badge on image |
| `#EEF2F8` | text on `navy` bubbles (equals light `s2`/dark `txt`) |

### 4.2 Typography (from `index.css` and screens)

| Role | Font | Sizes used |
|---|---|---|
| Display (wordmark, H1/H2, prices, avatar initials, stats) | **Instrument Sans** 400/500/600/700 | 25 (H1, −0.02em), 22, 20, 19 (−0.01em), 16 (price), 15 |
| Body / UI | **Inter** 300–700 | 15, 14, 13, 12, 11, 10, 9 |
| Section label | Inter 600 uppercase | 11, tracking 0.08em |

Fonts are loaded from Google Fonts in the export. A native app must **bundle** them (see
plan, assets).

### 4.3 Radius, spacing, elevation

| Token | Value |
|---|---|
| Radius | 16 (cards, composers, bubbles, groups), 12 (small buttons, thumbnails, tiles), full (chips, avatars, dots), 4 (user bubble top-right corner) |
| Page gutter | 20 (Home, Discover, Activity, Profile); 16 (Conversation) |
| Card padding | 12–16 (`p-3`/`p-3.5`/`p-4`/`p-5`) |
| List gap | 8 (activity rows), 10 (for-you/collections), 12 (carousels) |
| Elevation | none by default; composer rest shadow `0 1 6 rgba(0,0,0,.04)`; primary buttons `0 3–4 10–12 blue@21–25 %`; no card shadows |
| Focus ring | 3 pt `blue` at ≈9 % (Home) / ≈8 % (Discover) |

### 4.4 Conflict between Figma and repository brand documents — **needs a decision**

| Token | Figma export | `docs/design/BRAND_SYSTEM.md` + `DESIGN_TOKENS.md` | Web app (`web/frontend`) |
|---|---|---|---|
| Primary blue | `#1A69F0` | `#155EEF` | `#176BFF` |
| Dark ink | `#0B1929` | `#0B1220` | — |
| Light surface/background | `#F9F8F5` (warm cream) | `#F5F8FC` (ice) | — |
| Success/accent | `#1A7A4A` green | mint `#35D0BA` | — |
| Coral | `#E8634A` | `#FF7A66` | — |
| Display font | Instrument Sans | Manrope | DM Sans / Manrope |
| Tone | "tu" | not specified | "vous" |

The task brief says both "the Figma is the visual source of truth" and lists the brand-doc
colours. **They cannot both be honoured.** Proposal (pending confirmation): follow the Figma
values for the mobile app (it is the newest, most specific, and the brief's fidelity rule), keep
all of them as **named tokens** in one file so a later alignment is a one-file change, and record
the divergence from web in the plan. **No token is silently "corrected".**

## 5. Asset mapping

| Asset | In Figma export | Mobile source |
|---|---|---|
| Logo / app icon | **ABSENT.** Wordmark is text. | The web app has `web/frontend/public/favicon.svg` (blue rounded square, white sparkle) — origin/intent unconfirmed; **need an official logo + launcher icon + splash from the user.** |
| Images (cards, trending, thumbnails) | Unsplash URLs (`images.unsplash.com`) used as **placeholders** | Real images come from the backend: the web model carries `primary_media_url` (search result card) and `media[]` (listing detail) — `web/frontend/src/types.ts`. Whether every seeded listing actually has media is not verified. Do not ship Unsplash. |
| Icons | 33 custom stroke icons in `icons.tsx` (house, compass, message, clock, user, send, mic, arrow-left, chevron-right/down, more, map-pin, star(+fill), heart, search, x, stop, play, pause, building, utensils, bag, wrench, bed, sun, moon, bell, help, shield, credit-card, log-out, check, trending, globe) | Re-draw as vectors (SVG → `flutter_svg`, or `CustomPainter`) at the same stroke widths. Icons `building, utensils, bag, wrench, bed, chevron-down` are defined but **not used by any of the 5 screens** (probably intended for vertical icons — unconfirmed). |
| Fonts | Instrument Sans, Inter (Google Fonts CDN) | Bundle the TTFs (licences: both SIL OFL — verify before bundling). |
| Illustrations / empty-state art / onboarding | **ABSENT** | — |
| Sounds, haptics | **ABSENT** | — |

## 6. Animation and motion inventory

| # | Motion | Where | Spec from CSS | Flutter mapping |
|---|---|---|---|---|
| M1 | `fadeUp` | user bubble | 280 ms, `cubic-bezier(.22,.61,.36,1)`, opacity 0→1, translateY 10→0 | `TweenAnimationBuilder` / `FadeTransition`+`SlideTransition`, same curve (`Cubic(.22,.61,.36,1)`) |
| M2 | `slideInLeft` | assistant row | 250 ms, same curve, opacity 0→1, translateX −8→0 | same |
| M3 | `voiceBar` | voice bars | 9 bars, 3 pt wide, 3 pt gap, `scaleY .18→1→.18`, `ease-in-out`, infinite, origin bottom-centre; per-bar (duration s / delay s / max height pt): (0.85/0/18) (1.05/.12/30) (0.78/.25/22) (1.15/.08/34) (0.92/.33/26) (0.80/.18/20) (1.10/.40/28) (0.88/.05/16) (1.00/.28/32); paused → animation paused + opacity .4 | `CustomPainter` driven by one `AnimationController`, per-bar phase; **in `listening`, drive from the real microphone amplitude** when STT is available (the Figma loop is only the visual) |
| M4 | `dotPulse` | processing | 1.2 s ease-in-out infinite, opacity .25→1, scale .75→1, delays 0/.2/.4 s | three `ScaleTransition`/`FadeTransition` with staggered intervals |
| M5 | `breathe` | — | 2.4 s, opacity .6→1, scale 1→1.06 | **Defined in CSS, used nowhere.** Do not implement unless a screen needs it. |
| M6 | Rotating placeholder | Home composer | every **3 800 ms**: fade out + translateY −4 over ~300 ms (`duration-300`), swap text after **280 ms**, fade in | `AnimatedSwitcher` with timer; stop when the field has text or the app is backgrounded |
| M7 | Focus ring | Home/Discover composer | border + ring transition, 200 ms (Home) / 150 ms (Discover) | `AnimatedContainer` |
| M8 | Switch | Profile | 200 ms, thumb translateX 0→20 on a 44×24 track | custom `TzSwitch` |
| M9 | Send button state | both composers | colour/shadow transition on empty ↔ filled | `AnimatedContainer` |
| M10 | Auto-scroll | Conversation | `scrollIntoView({behavior:'smooth'})` on new message/voice change | `ScrollController.animateTo(maxScrollExtent)` |
| M11 | Progress fill | speaking | `transition-all 300 ms`, width **static 38 %** | bind to real TTS/audio progress or hide (see MISSING: no TTS) |

**Not defined in Figma:** tab/route transitions (the export swaps screens instantly), carousel
snap/parallax, press/ripple feedback, haptics, skeleton shimmer, sheet transitions, hero
transitions. Plan: no invented animations — use a plain cross-fade/none for tab swaps and flag it.

**Reduced motion.** The export has **no** reduced-motion handling. Mobile requirement (brief):
when `MediaQuery.disableAnimations` / platform reduce-motion is on → M1, M2, M6, M8, M10 become
instant, M3/M4 become a static state (bars at a fixed mid height, dots at full opacity).

## 7. States — what Figma defines vs. what the app needs

| State | Figma | Needed |
|---|---|---|
| Idle/focused/filled inputs | ✅ | ✅ |
| Voice idle/listening/processing/speaking | ✅ (simulated) | ✅ + permission-denied, no-speech, error — **ABSENT** |
| Light / dark | ✅ | ✅ (+ follow system) |
| Loading (skeleton/spinner) | **ABSENT** | every list and the conversation (waiting for the assistant is only represented by the `processing` voice state; text-mode "waiting for the assistant" has **no design**) |
| Empty | partially ("Tu es à jour" footer, always shown) | every list |
| Error / retry | **ABSENT** | every network call |
| Offline | **ABSENT** | brief requires it |
| Signed-out | **ABSENT** (Profile always shows "Diallo Mbeki") | required |
| Keyboard open / safe areas / small phones | **ABSENT** (bar is always shown under the composer) | required |

## 8. Prototype-only behaviour in the export (do **not** port)

1. `INITIAL_MESSAGES` (a canned apartment conversation) and the canned reply *"Je cherche les meilleures options pour toi. Un moment..."* after 1.1 s — fake assistant.
2. Voice: `setTimeout`-driven `listening → processing (1.4 s) → speaking (4 s)` with a canned reply — no microphone, no STT, no TTS.
3. Static data: names, counts ("14 / 4 / 2"), distances, prices, ratings, statuses, "6 résultats", "Il y a 1h".
4. `initialQuery` is stored on navigation but **never read** by the Conversation screen (queries from Discover/Home do not reach the thread except Home's `handleSendMessage`).
5. `SuggestionChips.onSelect` is a no-op; "Voir", "Tout voir", "Voir plus", "Carte", the "more" button, and all Profile rows except the theme switch have no behaviour.
6. Discover's active pill, the Home city pill and avatar do nothing.
7. Desktop phone frame + fake status bar + `#070E1C` backdrop.

## 9. Backend mapping per screen

Legend — **REUSE** = endpoint exists and is used by the web client today · **DERIVE** = computable
client-side from REUSE data · **MISSING** = no endpoint/field found (see plan §8) · **n/a** = purely
local.

| Screen / element | Source | Status |
|---|---|---|
| **Home** greeting name | `GET /auth/users/me` → `display_name` (first token) | REUSE (fallback when signed out: no name — Figma silent) |
| Home avatar initials | `display_name` | DERIVE |
| Home city pill | no user/location field; device location + search `location` param is a *place name* | **MISSING** (location source/contract) |
| Home composer submit | `POST /conversations/message` (`channel:"mobile"`, valid in `internal/domain/channel.go`) | REUSE |
| Home category chips | `GET /verticals` + `GET /categories` | REUSE (mapping in plan §7) |
| Home "Près de toi" (distance, open status, price tier, rating) | `POST /search` accepts only `location` as a *place name* (`internal/api/search_handlers.go`: text, category_id, vertical, location, price_min/max, sort, cursor, page_size). Results already carry `distance_meters`, `location.latitude/longitude`, `price_amount/currency`, `availability_status`, `primary_media_url`, `seller.trust_level` | **MISSING**: lat/lng/radius **request** parameters (so `distance_meters` cannot be requested; `sort_field: distance` exists but has no point to sort from), opening hours / "open now", price tier (`$$`), **ratings** (only `trust_level` exists, not a review score) |
| Home "Pour toi" + reason | no recommendation endpoint | **MISSING** |
| Home resume card | no conversation list/history endpoint | **MISSING** (local cache of last thread id is possible but is not backend truth) |
| **Conversation** send/receive | `POST /conversations/message` (`message_text`, `channel`, `sender_external_id`, `message_idempotency_key`, `locale`) | REUSE |
| Conversation confirm/reject action cards | `POST /conversations/{id}/confirm|reject` `{token}` | REUSE (the Figma has **no UI for confirm/reject** — absent design) |
| Conversation result cards | reply `cards[]` + `result_items[]` (`mapReply` in `api/conversations.ts`) | REUSE (image, title, price, location available; Figma's `badge` and `rating` have no confirmed source) |
| Conversation suggestion chips | reply `suggestions: string[]` ("tappable quick replies") — sending the chip text as the next message | REUSE (the Figma no-op handler becomes "send as message") |
| Conversation assistant action cards (not in Figma) | reply `confirmation {summary, token, action_type}`, `missing_params[]`, `is_clarification`, `refs {order_id, reservation_id, listing_id}` | REUSE data, **no Figma design** |
| Conversation history on reopen | no list/get-messages endpoint | **MISSING** |
| Conversation voice STT/TTS | no speech endpoint | **MISSING** (on-device STT is a decision; TTS absent) |
| "Voir" on a result → listing detail | `GET /listings?listing_id=` + `GET /stores?store_id=` | REUSE data, **no Figma screen** |
| **Discover** search | `POST /search`, `GET /search/suggest` | REUSE |
| Discover pills | `/verticals` / `/categories` | REUSE |
| Trending, Collections, "X annonces" counts | no endpoint (web has none) | **MISSING** |
| "Autour de toi" + "Carte" | see Home "Près de toi" | **MISSING** + no map design |
| **Activity** conversations | no list endpoint | **MISSING** |
| Activity reservations | `GET /reservations` (+ `/{id}`, `/{id}/cancel`) | REUSE |
| Activity orders | `GET /orders` (+ `/{id}`, `/{id}/cancel`, pay: `POST /payments`) | REUSE |
| **Profile** name/phone/email | `GET /auth/users/me` | REUSE (no location, avatar or city on the user) |
| Profile stats | reservations/orders: `.length` of the lists (DERIVE); conversations: **MISSING** | partial |
| Profile location / language / notifications | local prefs; location & push registration have **no backend contract** | local / **MISSING** |
| Profile language | `locale` field is sent on each message; no user-level language store | local |
| Dark switch | local | n/a |
| Saved items / Payments / Privacy / Help | no endpoint, no Figma screen | **MISSING** |
| Sign out | no logout/revoke endpoint | local only (**MISSING** server revoke) |
| Sign-in / OTP (all of Figma is signed-in) | `POST /auth/request-otp`, `/auth/verify-otp`, `/auth/refresh` | REUSE data, **no Figma screen** |

## 10. Explicit gaps summary (nothing here is invented)

1. 5 screens only; the 20+ screens/sheets listed in §2 and §3.2 do not exist in Figma.
2. No logo, launcher icon, splash, illustration or real imagery.
3. No design for loading, error, empty (real), offline, signed-out, permission, or keyboard states.
4. No tab/route transition spec and no reduced-motion spec.
5. Token conflict with `docs/design` brand documents and the web app (§4.4).
6. French only; informal "tu"; no English strings (English is required by the brief and must be authored, not machine-guessed).
7. No Figma variables/variants/prototype file — only the generated React export.
8. Figma folder is untracked in git.
