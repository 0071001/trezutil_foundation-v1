# Legal pages: what is done, what only the company can supply

*Written 2026-10-05.* Four pages exist on the site, in French and English: **Terms of use**, **Privacy policy**, **Cancellations and refunds** and **Legal notice** (`/legal/terms`, `/legal/privacy`, `/legal/refunds`, `/legal/notice`; `src/pages/Legal.tsx`, texts in `src/pages/legalContent.tsx`).

They are **drafts**. They describe what the product really does (what it collects, who receives it, how orders, bookings and refunds work) and leave a **highlighted marker** for every fact the repository cannot know. Nothing was invented: no company name, address, registration number, retention period, refund delay or governing law.

## How they stay honest until they are reviewed

| Mechanism | Effect |
|---|---|
| `LEGAL.reviewed` is `false` (`src/lib/legal.ts`) | each page shows a banner "this text is a draft…"; the footer links to the pages **only on staging** (placeholders on); the sign-in form does **not** say people accept the terms |
| A missing fact | shown as `[To complete: …]` / `[À compléter : …]` on a yellow dashed highlight, never as a made-up value |
| `LEGAL.reviewed` set to `true` | the banner goes, the footer links the pages on the live site, and the sign-in form adds "By continuing you accept the Terms of use and the Privacy policy" with links |

**To publish:** fill the facts below in `src/lib/legal.ts`, have a lawyer review both languages, set `reviewed: true`, rebuild. A test (`src/pages/legal.test.tsx`) pins this behaviour.

## Facts to fill in (`LEGAL` in `src/lib/legal.ts`)

| Field | What it is |
|---|---|
| `entity.name`, `entity.form` | the company that operates Trezutil and its legal form |
| `entity.address`, `entity.registration` | registered address; trade register, national and tax numbers |
| `entity.director` | the person responsible for the publication of the site |
| `entity.email`, `entity.phone` | where people write about anything legal or about their data |
| `entity.hosting` | the hosting company and country |
| `entity.jurisdiction` | the governing law and the competent courts |
| `minimumAge` | the minimum age to hold an account |
| `retention` | how long accounts, conversations and orders are kept after the last activity |
| `authority` | the data-protection authority people may complain to |
| `refundWindow`, `refundDelay` | the time to ask for a refund, and the time before the money arrives |
| `prohibitedItems` | what may not be sold on Trezutil |

Also left as markers inside the texts, because they are legal judgements: the **legal basis** of each processing, the **countries** where data is hosted and the **safeguards** for transfers, the **extent of the limitation of liability**, and the **complaint procedure**.

## Questions for the lawyer

1. Which data-protection and consumer-protection rules apply to an operator in the DRC serving buyers in Kinshasa, and are there registrations to make (for example with a data-protection authority)?
2. Is Trezutil a **marketplace operator** (platform) or an **agent** in the sellers' sales? The terms say it is **not the seller** and that the payment is made to the seller's own payment account; confirm that is the right framing, and what it means for refunds and liability.
3. Are the **AI model providers** (Anthropic, Google) and the **messaging provider** (Meta) acceptable recipients of message text and phone numbers, and what must the policy say about transfers abroad?
4. What **retention** periods are required or advisable (accounts, conversations, orders and payment records, logs)?
5. The terms promise that **no order or booking is made without an express confirmation**. This is how the product works (the assistant asks for a confirmation); confirm the wording is enough to count as consent for a distance contract.
6. A **self-service deletion** of an account does not exist yet: the policy tells people to write to the company. Is that acceptable at launch?
7. What must the **seller agreement** contain (commission, payout, KYC), given that sellers are onboarded by the team today? A seller contract is **not** drafted here.

## What the texts say, and where it comes from in the product

| Statement | Basis in the product |
|---|---|
| sign-in code on WhatsApp, kept only as a fingerprint, few attempts | `identity` service (OTP, HMAC-stored, 5 attempts) |
| no order or booking without express confirmation | conversation runtime: confirmations for orders and reservations |
| card numbers never reach Trezutil | Stripe Payment Element; the platform stores a status and a reference |
| a refund cannot exceed what was paid | payment core: refunds capped by the remaining refundable amount |
| messages go to an AI provider, or a rule-based method without one | `AI_PROVIDER` is optional; keyword routing is the fallback |
| no advertising cookie, no audience tool | the web app stores only session, language, theme, bag and saved items |
| no self-service deletion | no such endpoint exists (limitation) |
| logs hold no password, token or code | hardening pass 1, security review (log scan) |

## Not done here

- A **cookie banner**: the app sets no advertising or analytics cookie, so none is shown. If an audience tool is added, revisit this.
- A **seller agreement** and a **commission schedule**.
- The **sign-in consent** is written but off until `reviewed` is `true`.
