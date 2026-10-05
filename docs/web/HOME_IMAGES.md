# Home page images

What pictures the home page uses, where they come from, under which licence, and how to replace them. Written 2026-10-05; the welcome background changed three times that day (see "History").

**Nothing the page chooses itself is loaded from a third party.** The welcome picture, the drawings and the business photograph are bundled with the site (`web/frontend/public/hero`, `public/images`) or drawn in code, so they cannot break when another site changes and send no visitor data elsewhere. The only other pictures are the listing thumbnails in *Discover*, which come from the listing data: real sellers upload their own, while the **local demo data** points at photos on `api.openverse.org` (checked on the staging stack on 2026-10-05). A test (`src/pages/home.test.tsx`, "bundles every picture it uses") checks that every address points into the two bundled folders and that every file exists.

## What is used

| Where | Files | What it is |
|---|---|---|
| Welcome (background) | `public/hero/mall-{900,1600,2400}.webp` (desktop, 2.2:1) and `mall-portrait-{700,1100}.webp` (phones, 4:5) | **The inside of a shopping centre seen from above**: three floors of shops, glass walkways, yellow steel columns, people walking. Supplied by the owner (file `welovebarcelona-de-gOe4rbB8K6U-unsplash.jpg`, 4000 × 2250): an **Unsplash** photograph by *We Love Barcelona* (photo id `gOe4rbB8K6U`), used under the **Unsplash Licence** (free for commercial use, no permission or credit required) |
| "What can we help you with?" | `src/components/home/art.tsx` | **Five original vector drawings** (a building with a pin, a market stall, a toolbox with a tick, a lamp over a table, a taxi), drawn for Trezutil in SVG. No stock art and no generated bitmap. Colours come from the page's tokens, so they follow light and dark mode |
| "Have something to sell or offer?" | `public/images/merchant-{640,1100}.webp` | A shopkeeper serving a customer. From Unsplash (photo id `1687422808384-c896d0efd4ab`), same licence. The photographer was not recorded when it was first added. **Replace it with a photograph of a real Trezutil merchant when you have one** |
| "Discover what exists around you" | listing photos from the API | Real photos uploaded by sellers, shown as small thumbnails; nothing here is bundled |

### What to know about the welcome photograph

- **It is not Kinshasa.** The signs and prices are Spanish (*Mid season sale -50%*, *Solvisión*), and the file name says Barcelona. On a platform that says "Kinshasa" this may read as an imported stock image.
- **It shows third-party brands**: Mango, Calzedonia, Solvisión, Tous, Coca-Cola, Dolce & Gabbana, Majorica, Agatha and Gelati. The Unsplash Licence covers the photograph, not those trademarks, and the brands may not like to appear on a commerce site that is not theirs. The veil makes them less prominent, not invisible.
- A few visitors of the centre are visible from above, small and not recognisable.

None of this blocks the use; it is the owner's call. If it matters, the earlier choices below are drop-in replacements.

## History of the welcome background

1. **A CC0 photograph of Boulevard Lumumba, Kinshasa** (Wikimedia Commons, by Lingabo, 2024). Replaced at the owner's request.
2. **The owner's aerial film of Kinshasa** (`bg/hero.mp4`, 1922 × 1080, 30 s, 20.5 MB; author and licence not recorded). It was re-encoded for the web and ran for a few hours; replaced by the current photograph at the owner's request. **The original file is still in `bg/` (untracked, not used by the site).** To bring the film back: crop the 1922 × 1080 source to `crop=1920:872:0:100` (desktop) and `crop=540:1080:691:0` (phones), dissolve the last 1.2 s into the first for a seamless loop, encode H.264 High with `-preset slow -crf 27` capped at 3.2 Mb/s (1.8 Mb/s for phones), `-movflags +faststart`, no audio (SSIM 0.985 against the source; 7.7 MB and 2.9 MB), and play it muted, looping, `playsInline`, with a still frame as `poster` and a still image instead when the person prefers reduced motion or saves data. A first 3 MB encode looked soft and was rejected: keep the native pixels. WebM (VP9) came out larger than the MP4 for the same quality, so only the MP4 is worth shipping.

## How the files were made

The 4000 × 2250 JPEG was cropped to 2.2:1 for desktop (full width, top offset 173 px) and to 4:5 for phones (the middle 1800 px), resized, and saved as WebP at quality 0.7: **57 / 135 / 291 KB** for desktop and **67 / 147 KB** for phones. A `<picture>` sends phones the portrait crop, and `srcset` lets each screen download only the width it needs; the image is requested with high priority because it is the first thing on the page. No image tool was installed on the build machine, so the resizing was done with Chromium's canvas; any tool that produces the same sizes does the job. A test keeps the weight in check (no file over 400 KB, the smallest phone file under 150 KB).

## Legibility

The white type sits on the photograph under one flat navy veil (`rgba(8, 22, 40, 0.6)` on `.home-hero::before`; no gradient). Measured on the rendered page at 1440, 820 and 390 px, in English and French, against the brightest 2% of the pixels behind each text, the **worst case is 4.8:1**, above the 4.5:1 that WCAG AA asks of small text. The photograph is bright and busy (lit shopfronts, white floors), so the margin is small: if you raise the brightness of the picture or lighten the veil, measure again.

## Replacing the picture

1. Put the new files in `web/frontend/public/hero` (same names keeps the code untouched; different names means editing `src/components/home/HeroBackdrop.tsx`).
2. Keep the descriptor widths in `srcset` and the portrait `<source>`, and keep the picture decorative (`alt=""`, `aria-hidden`).
3. Measure the contrast again and check the weight test.
4. Record the source and the licence in the table above.

## Not done, and why

- **No bitmap generated by a model.** There is no image model in this environment. The drawings are written by hand in SVG; the photographs are real.
- **No photographs for the five needs** (housing, shopping, trades, going out, getting around). Freely licensed photographs of these in Kinshasa are rare, so drawings keep the page consistent until you have your own.
