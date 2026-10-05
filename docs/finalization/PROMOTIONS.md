# Promotions and the sale price

What was added to the web app and the APIs behind it. Written 2026-10-04. The homepage banner and product-listing blocks described in the first version of this note no longer exist: the home page was redesigned the same day around "tell us what you need" (see `docs/web/HOME_PAGE_REDESIGN.md`).

## What customers see

| Where | What |
|---|---|
| Homepage | **No promotions section.** One was built (a bento of the products on sale on its own coral stage) and **removed at the owner's request on 2026-10-04**; its code, styles and tests are gone. The live-figures strip can still show how many listings are on promotion ("annonces en promotion"), and administrators can switch that figure off in Site content. |
| Cards, search results, listing page | A **−N% badge** on the image, the selling price, and the regular price struck through. |
| Search | `/search?promo=1&sort=discount`: an **On promotion** filter and a **Biggest discount** sort in the filter panel (this is now the only place customers browse deals; the homepage no longer links to it). |

There is **no slideshow, no banner and no tinted "stage"** on the homepage (all were built, then removed at the owner's request).

A sale is only ever shown when the regular price is really above the selling price. The percentage is computed from the two prices and rounded **down** (a discount is never overstated) — on the server (`domain.DiscountPercent`) and in the browser (`lib/promo.ts`), with the same test cases. The deals request is filtered again in the browser so a regular-price product can never appear as a deal, whatever the server returned.

## What sellers do

Seller console → a listing → **Promotion** panel: enter a sale price, *Start promotion*; *Change sale price*; *End promotion*. It works on a **live** listing (no pause needed — unlike a plain price edit). Draft, pending, published and paused listings can run a sale; sold, closed, expired and suspended ones cannot. Only the seller's owner/admin/manager (or a platform admin/operator) can do it; staff see it read-only.

## How it works

* **The listing's price *is* the sale price.** Orders, payments and search sorting read `price_amount`, so what the page shows is exactly what is charged. The regular price is remembered under `Listing.Metadata["promotion"] = {"compare_at_amount": N}` (no migration — the metadata column already exists).
* **`StartPromotion(salePrice)`** records the current price as the regular price (or keeps the original one when the sale price is changed), then sets the price to the sale price. It rejects a price that is not positive, not below the regular price, or less than 1% off. Prices carry two decimals.
* **`EndPromotion()`** restores the regular price and removes the marker. Ending when there is no sale is a harmless no-op.
* A **price edit** on a listing already on sale: below the regular price it becomes the new sale price; at or above it the sale ends.
* The `promotion` metadata key can be written **only** through those methods. Client-supplied metadata on create or update has the key stripped, and an update that replaces metadata keeps the real promotion — a seller cannot forge, alter or erase a discount through a metadata edit.
* Existing orders keep their pricing snapshot; a sale never changes what a customer already agreed to.
* **No end date.** A sale runs until the seller ends it. (A scheduled end would need a job that restores prices; leaving a price silently at its sale value after a date would be worse than a manual end.)

## API

| Call | Notes |
|---|---|
| `POST /api/v1/listings/promotion?listing_id=` `{"sale_price": 79.5}` | Sign-in required. Returns the updated listing. `400 INVALID_PARAMETER`, `403 FORBIDDEN`, `404 LISTING_NOT_FOUND`, `409 LISTING_NOT_EDITABLE` (sold/closed/expired/suspended). |
| `POST /api/v1/listings/promotion/end?listing_id=` | Restores the regular price. |
| `POST /api/v1/search` `{"on_promotion": true, "sort_field": "discount"}` | `on_promotion` keeps only listings currently on sale; `discount` sorts the biggest saving first (listings without a sale last, ties newest first). Results carry `CompareAtAmount` and `DiscountPercent`. |

`api/openapi/openapi.yaml` is not maintained for these (it is already out of date; see `docs/mobile/MOBILE_IMPLEMENTATION_PLAN.md` §4.3).

## Search index

`compare_at_amount` (`scaled_float`) and `discount_percent` (`short`) are added to the OpenSearch mapping — in the new-index mapping and as an **additive** mapping applied to an existing index at startup. The incremental sync (every `SEARCH_SYNC_INTERVAL`, default 5 s, driven by `updated_at`) projects a changed listing; measured on the local stack a sale becomes visible in search after a few seconds, but **right after the search service restarts the startup full resync indexes every listing first (one `refresh=wait_for` each), so changes can take a minute or more to appear**. The result mapper derives the discount from the two prices rather than trusting the stored percentage.

## Front-end pieces

| File | Role |
|---|---|
| `components/cards.tsx` | `DiscountBadge`, `PriceTag` (screen readers hear "Now …" and "Regular price …"). |
| `lib/promo.ts` | `discountPercent`, `saleOf`, `listingSale`. |
| `pages/seller/SellerListings.tsx` | The seller's **Promotion** panel. |
| `styles/app.css` | Styles for the badge, the price tag and the seller panel (`figma.css` stays the unmodified export). |

## Verification

* Go: `domain` (discount maths, start/change/end, forged metadata), `search` (filter, sort, validation, result mapping, index mapping), `api` (handlers, auth, error mapping, `on_promotion`), and a **real-PostgreSQL integration test** (`infrastructure/postgres/integration/listing_promotion_test.go`) covering ownership, live-listing sales, metadata forgery, repricing and closed listings. The whole Go suite passes (`go test -count=1 -p 1 ./...`, including the 8-minute database suite).
* Web: tests for the discount maths shared with the server, search with the promotion filter and sort, the listing page and the seller panel, plus a regression test that the home page has no slideshow and no promotions section and never asks the server for sales (`src/pages/promotions.test.tsx`).
* Local staging was rebuilt with the new backend. Demo sales on seven retail listings were set directly in the **local** staging database (not through the API); they show as the price badge on cards and as the "listings on promotion" figure.

## Not done / follow-ups

* No end date or schedule for a sale (see above).
* No promotion on the AI/WhatsApp result cards yet (the data is in the results).
* Hotel and booking flows show the nightly price as before; sales are shown wherever a listing price is shown from a search result or the listing page.
