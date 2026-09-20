# Trezutil — Product Specification

**Status:** Version 1.1 — Baseline for architecture and implementation

**Meaning:** Trezutil = « très utile »

**Category:** Conversation-first Commerce OS

**Primary market:** Democratic Republic of the Congo

**Expansion path:** Central Africa → Africa

**Primary interface:** WhatsApp

**Initial vertical:** Real Estate, with a generic commerce core

## 1. Vision

Trezutil turns WhatsApp into a practical operating interface for commerce. A customer can search, compare, ask questions, buy, book, pay and track. A seller can publish, manage offers, process orders, market products and request business reports. The platform's core remains independent from WhatsApp so other channels can be added later.

## 2. Product promise

> **Tu demandes. Tu trouves. Tu vends. Tu achètes. Tu gères.**

Brand expression:

> **Trezutil. Très utile, pour tout.**

## 3. Problem

Target-market commerce is fragmented across WhatsApp, Facebook, TikTok, physical shops, agents, phone calls, spreadsheets, Mobile Money and informal delivery networks. Availability is often unclear, trust signals are weak, listings can become stale, and sellers lack accessible operational tools.

## 4. Product principles

1. WhatsApp-first, not WhatsApp-dependent.
2. Conversation-first instead of menu-first.
3. AI interprets; deterministic backend logic executes.
4. Trust and verification are native features.
5. One canonical listing feeds all distribution channels.
6. Local payments, languages, currencies and fulfilment are first-class concerns.
7. API and event contracts precede business code.
8. Complexity is introduced progressively.

## 5. Actors

- customer/buyer
- individual seller
- organization/merchant
- real-estate agent
- service provider
- logistics partner
- support/operations agent
- administrator

## 6. Core experiences

### 6.1 Buyer discovery
User sends a natural-language request, voice note, photo or shared listing link. Trezutil extracts intent, searches the marketplace, applies availability/trust constraints and returns concise results.

### 6.2 Seller onboarding
Seller starts through WhatsApp. Trezutil captures basic identity/business information, requests verification material when needed, creates a seller profile and explains available functions.

### 6.3 Listing creation
Seller can send text, photos or voice. AI structures the content into a draft, identifies missing fields, asks targeted questions, validates the draft, and requests confirmation before publishing.

### 6.4 Publication/distribution
Canonical listing is published to Trezutil first. Eligible external destinations can be added through Social Commerce connectors. External publishing depends on the platform's available APIs and permissions. Where direct API publication is unavailable, Trezutil prepares optimized content and a handoff flow.

### 6.5 Order / booking
Buyer confirms the selected listing. Trezutil creates an order or reservation according to listing type, captures a price snapshot, and moves to payment/fulfilment when applicable.

### 6.6 Payment
Payment intent is created via the payment layer. External provider confirmation changes payment state. Conversational statements alone cannot mark an order as paid.

### 6.7 Fulfilment
For physical goods, Trezutil may create a delivery request, assign a delivery partner, track progress and record proof of delivery. For property, fulfilment becomes a visit/booking workflow rather than shipping.

### 6.8 Seller reporting
Seller asks questions in WhatsApp such as:

> « Combien ai-je vendu cette semaine ? »

The reporting engine returns computed metrics from trusted data. AI explains results but must not invent numbers.

## 7. Listing model

The listing abstraction supports:

- product
- property
- vehicle
- food
- service
- accommodation
- other supported verticals

Each listing has lifecycle states, pricing, availability, media, seller ownership, location and type-specific attributes.

## 8. Real-estate MVP

The initial vertical addresses the local pain of stale property listings, insufficient photos, wasted physical visits and weak seller trust.

MVP features:

- structured property listings;
- photo/video galleries;
- listing freshness/availability timestamp;
- seller/agent verification;
- natural-language search;
- visit scheduling;
- WhatsApp conversation;
- location/area filtering;
- price comparison;
- optional virtual-tour media;
- visit reminders;
- lead tracking.

## 9. AI capabilities

### Conversation AI
Intent detection, contextual replies, tool selection, multilingual interaction.

### Vision
Extract listing information from photos and documents, classify media, detect duplicates/quality issues.

### Speech
Transcribe voice notes for supported languages and feed structured content into the intent pipeline.

### Search intelligence
Convert natural-language requests to validated filters and semantic retrieval queries.

### Seller Copilot
Publishing assistance, availability updates, order handling, business summaries and campaign assistance.

### Trust/Risk assistance
Detect suspicious signals and route high-risk cases to deterministic policies/human review.

### Marketing generation
Generate channel-specific copy and content suggestions from the canonical listing.

## 10. Trust model

Trust is multi-signal. The system should consider:

- identity verification;
- business verification;
- listing freshness;
- transaction history;
- review history;
- complaints/disputes;
- suspicious behaviour indicators;
- platform moderation outcomes.

Internal scoring must retain explainable signals. A score alone must not be treated as proof of legitimacy.

## 11. Social Commerce

Seller can request distribution of a canonical listing across supported channels.

Capabilities:

- Facebook/Instagram connectors where supported;
- TikTok connector where supported;
- public share links;
- campaign tracking;
- source/attribution parameters;
- view/click/conversation/order analytics.

Example tracking:
`https://trezutil.example/l/123?src=tiktok&campaign=spring`

The platform should distinguish between API-based publication and assisted manual publication.

## 12. Commerce lifecycle

```text
Discover
  ↓
Compare
  ↓
Select
  ↓
Order / Reserve
  ↓
Pay (when applicable)
  ↓
Fulfil / Visit
  ↓
Complete
  ↓
Review / Support
```

## 13. Seller lifecycle

```text
Onboard
  ↓
Verify
  ↓
Create Draft
  ↓
Publish
  ↓
Receive Leads/Orders
  ↓
Fulfil
  ↓
Analyse
  ↓
Improve / Market
```

## 14. Human handoff

Trezutil MUST allow a conversation to be transferred to a human operator when:

- the user explicitly requests a person;
- the request is outside model/tool capability;
- fraud/risk is elevated;
- a dispute is opened;
- a sensitive payment issue occurs;
- policy requires review.

The handoff state and conversation history remain available to the operator according to authorization rules.

## 15. Notification model

Notifications may be delivered through WhatsApp, SMS, email or future push channels. Business events determine when notifications are required.

## 16. Admin / moderation

Admin capabilities:

- seller verification;
- listing moderation;
- complaint/dispute handling;
- fraud/risk review;
- content policy enforcement;
- channel integration management;
- configuration;
- analytics;
- audit search.

## 17. MVP boundary

The first production MVP should focus on:

1. WhatsApp conversation foundation;
2. identity/seller onboarding;
3. generic listings with real-estate extension;
4. natural-language search;
5. listing publication;
6. trust signals;
7. visit/request workflow;
8. basic orders/reservations;
9. payment abstraction;
10. admin moderation;
11. eventing and observability.

Out of initial MVP unless directly required:

- full social auto-publishing across every platform;
- complex loyalty systems;
- advanced predictive ML;
- large-scale logistics marketplace;
- native customer mobile app.

## 18. Non-functional requirements

Initial targets should be validated during load testing, but the architecture should aim for:

- API p95 under 500ms for simple non-AI reads under normal load;
- webhook acknowledgement independent of long AI processing;
- no synchronous dependency on an unavailable third-party provider where avoidable;
- full auditability of privileged actions;
- repeatable local environment through Docker Compose.

## 19. Business model directions

Potential monetisation:

- seller subscriptions;
- transaction fees where appropriate;
- premium listing/distribution;
- business analytics;
- verification services;
- enterprise/API integrations;
- logistics/lead services.

Pricing is not a technical contract and will be validated independently.

## 20. Success metrics

Product KPIs:

- active sellers;
- active buyers;
- listings published;
- listing freshness rate;
- search-to-contact conversion;
- contact-to-order conversion;
- completed transactions;
- gross merchandise value where relevant;
- seller retention;
- repeat buyer rate;
- trust dispute rate;
- median time from seller message to published listing;
- AI resolution rate without human handoff.

## 21. Product guardrails

Trezutil MUST NOT:

- invent product availability;
- fabricate prices or transaction statuses;
- mark payments successful based only on user text;
- expose private seller/customer data without authorization;
- let AI bypass permission checks;
- treat a trust score as guaranteed truth;
- make a social channel the canonical data source.
