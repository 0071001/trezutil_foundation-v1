# Partner logos (home page band)

The home page shows a scrolling band of partner logos at its foot. **Administrators
manage it from Administration → Site content → Partners** (add, edit, remove,
reorder, show or hide, upload a logo, optional website); see
[DASHBOARDS_AND_SITE_CONTENT.md](DASHBOARDS_AND_SITE_CONTENT.md). Until someone
publishes, the site shows the built-in list (`DefaultSiteContent` in
[internal/domain/site.go](../../internal/domain/site.go)), whose logo files live in
`web/frontend/public/partners/` and are served by the app itself; a Go test checks
that each file exists. Logos added in the console are stored by the platform
(`site_assets`) and served from `/api/v1/site/assets/{id}`. Nothing is ever loaded
from a partner's website, and the server refuses any other logo address.

## Before launch

- **Each logo is a trademark.** The band says "Nos partenaires". Keep only
  organisations Trezutil really works with, and check each one's brand rules
  (some require written permission, a minimum size or a clear zone).
- The list was assembled from the categories asked for (banks, mobile money,
  shops, hotels, Silikin Village) and from well-known organisations in
  Kinshasa. **The repository holds no record of these partnerships**, so none of
  them is confirmed by the code or the other documents.
- Remove, add or reorder partners in the console. A partner removed there
  disappears from the site when you publish; the bundled files stay in the app.
  Uploads accept PNG, JPEG or WebP up to 512 KB (SVG is refused: it can carry
  scripts).

## Where each file came from

Retrieved on 2026-10-04.

| File | Organisation | Source | Terms shown at source |
| --- | --- | --- | --- |
| rawbank.png | Rawbank | Wikimedia Commons, "File:Rawbank DRC logo.png" | Public domain (text logo), marked trademarked |
| mpesa.svg | M-Pesa | Wikimedia Commons, "File:M-PESA LOGO-01.svg" | Public domain (text logo), marked trademarked |
| orange-money.svg | Orange Money | Wikimedia Commons, "File:Logo Orange Money.svg" | Public domain (text logo), marked trademarked |
| airtel.svg | Airtel Money | Wikimedia Commons, "File:Airtel Africa logo.svg" (Airtel Africa mark) | Public domain (text logo), marked trademarked |
| ecobank.svg | Ecobank | Wikimedia Commons, "File:Ecobank Logo.svg" | Public domain (text logo), marked trademarked |
| access-bank.png | Access Bank | Wikimedia Commons, "File:Access Bank Logo.png" | Public domain (text logo) |
| pullman.svg | Pullman Kinshasa Grand Hotel | Wikimedia Commons, "File:Pullman logo 2013.svg" (Pullman brand mark) | Public domain (text logo), marked trademarked |
| stripe.svg | Stripe | Wikimedia Commons, "File:Stripe Logo, revised 2016.svg" | Public domain (text logo), marked trademarked |
| whatsapp.svg | WhatsApp | Wikimedia Commons, "File:WhatsApp.svg" | Public domain (text logo), marked trademarked |
| silikin-village.png | Silikin Village | silikinvillage.com (site logo, landscape) | The organisation's own logo; no licence stated |
| memling.png | Hôtel Memling | memling.net (site logo) | The organisation's own logo; no licence stated |
| tmb.png | TMB (Trust Merchant Bank) | tmb.cd (site logo) | The organisation's own logo; no licence stated |
| hyper-psaro.svg | Hyper Psaro | psaro.com (site logo) | The organisation's own logo; no licence stated |

"Public domain" on Commons means the logo is too simple to be copyrighted. It
does **not** mean anyone may use it as a sign of partnership: trademark law
still applies.

## Not included, and why

- **Equity BCDC** and **Africell**: the only Commons copies are licensed
  CC BY-SA 3.0, which requires attribution and share-alike on the file. Add them
  from the organisations' own brand kits instead.
- **Kin Marché**: its site refuses to serve the logo file to anything but its
  own pages.
- No logo was redrawn or recoloured; they are shown as published.
