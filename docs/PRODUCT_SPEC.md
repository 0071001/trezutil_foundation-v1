# Trezutil — Product Specification

**Status:** Version 1.0 — Product foundation draft to be reviewed before implementation

**Product:** Trezutil

**Meaning:** Trezutil = « très utile »

**Category:** Conversation-first Commerce OS / Conversational Marketplace Infrastructure

**Primary market:** Democratic Republic of the Congo (RDC)

**Expansion:** Central Africa → Africa

**Primary interface:** WhatsApp

**Initial vertical:** Real Estate

---

## 1. Product Vision

Trezutil is a Commerce OS that makes WhatsApp the primary interface for everyday commerce.

A customer should be able to say what they need in natural language, voice or image, and Trezutil should help them discover relevant offers, compare them, assess trust, place an order or reservation, pay when supported, arrange fulfilment, and receive updates — without requiring a dedicated customer app.

A seller should be able to use the same WhatsApp interface to create listings, manage prices and stock, receive and process orders, communicate with customers, launch marketing, and request business reports. A seller may later use optional web tooling for advanced operations, but everyday commerce must remain possible through WhatsApp.

The web is primarily an operational and administration surface. The domain core must remain independent of WhatsApp, n8n, any particular AI provider, and any individual payment provider.

### Product promise

> **You ask. Trezutil finds. You decide.**

Alternative brand expression:

> **Trezutil. Très utile, pour tout.**

---

## 2. Problem Statement

Commerce in the target market is fragmented across:

- WhatsApp conversations;
- Facebook and TikTok posts;
- informal seller networks;
- physical shops and agents;
- phone calls;
- Mobile Money;
- delivery/courier networks;
- spreadsheets and manual records.

Customers often struggle with:

- knowing what is genuinely available;
- comparing prices and sellers;
- trusting unknown sellers;
- finding complete information;
- avoiding unnecessary physical trips;
- coordinating payment and delivery;
- communicating in the language they naturally use.

Sellers often struggle with:

- creating structured catalogues;
- keeping stock and availability current;
- managing WhatsApp conversations at scale;
- publishing to several channels;
- analysing sales;
- following up leads;
- maintaining records;
- operating sophisticated commerce software.

Trezutil addresses this by putting an AI-assisted commerce layer behind a familiar interface: WhatsApp.

---

## 3. Product Principles

### 3.1 WhatsApp-first

WhatsApp is the primary user interface for customers and sellers.

### 3.2 Conversation-first

The user should be able to express an intent naturally rather than learning application menus.

### 3.3 AI interprets, backend executes

AI can understand requests, plan tool calls and generate content. It must not bypass backend authorization, validation, pricing rules, payment controls or audit requirements.

### 3.4 Trust is a product feature

Identity, seller history, listing freshness, transaction history, complaints and verification should contribute to an understandable trust model.

### 3.5 One canonical listing

Trezutil owns the canonical listing record. Social channels are distribution channels, not separate sources of truth.

### 3.6 Local-first commerce

The product must support local currencies, Mobile Money, local languages, local delivery realities, local address patterns, and the practical behaviour of informal and formal commerce.

### 3.7 Progressive complexity

The initial implementation should be a well-structured modular backend. Services should be separated when scale, reliability, ownership or security justify it — not merely for architectural aesthetics.

### 3.8 API and event contracts first

No significant business implementation should begin before the domain and API/event contracts are defined and reviewed.

---

## 4. Target Users

### 4.1 Customer / Buyer

A person looking to find, compare, buy, book, rent or request products and services.

Typical interaction:

> « Je cherche une maison de 3 chambres à Lubumbashi, maximum 500 $. »

or:

> « Trouve-moi un iPhone 15 Pro 256 Go à moins de 700 $. »

or a voice note / photo.

### 4.2 Seller / Merchant

A business or individual offering products, properties, vehicles, food or services.

The seller can use WhatsApp for normal operations and may later use an optional web dashboard for advanced management.

### 4.3 Property Agent / Commissionnaire

A specialised seller/agent managing real-estate listings, visits and prospects.

### 4.4 Service Provider

A provider of an appointment or service rather than a physical product.

### 4.5 Courier / Logistics Partner

An actor involved in fulfilment, delivery, pickup or proof of delivery.

### 4.6 Platform Administrator

Internal Trezutil operator responsible for verification, trust, disputes, moderation, configuration, monitoring and platform operations.

### 4.7 Operations / Support Agent

A human who can take over a conversation or operational case when automation is insufficient or risky.

---

## 5. User Interface Model

### Customer interface

**Required:** WhatsApp

Customer actions available through conversation include:

- search;
- compare;
- ask product/listing questions;
- view media;
- shortlist/favourite;
- order;
- reserve/book;
- request delivery;
- initiate payment;
- cancel where policy permits;
- request support;
- review a transaction.

### Seller interface

**Primary:** WhatsApp

**Optional:** Web operations interface for advanced management.

Seller actions through WhatsApp include:

- create/edit/publish/unpublish listing;
- update price;
- update stock;
- confirm availability;
- process orders;
- request reports;
- create marketing campaigns;
- view performance;
- manage profile and verification information;
- respond to customers;
- request assistance.

### Admin interface

**Primary:** Web administration console.

WhatsApp may be used for selected alerts and operational actions, but it is not the authoritative admin interface.

---

## 6. Core Product Concept: Listing

Trezutil uses a generic **Listing** abstraction rather than hard-coding the marketplace around retail products.

A listing may represent:

- physical product;
- real-estate property;
- vehicle;
- food item;
- service;
- accommodation;
- other supported commerce offering.

A listing has common attributes:

- ID;
- seller/owner;
- title;
- description;
- category;
- price and currency;
- availability;
- location;
- media;
- attributes;
- status;
- verification/trust information;
- timestamps;
- distribution state;
- analytics metadata.

Vertical-specific attributes are handled without breaking the common listing model.

---

## 7. Core Product Concept: Intent

Every meaningful WhatsApp request should be translated into a structured intent before the system performs business actions.

Example:

```json
{
  "type": "SEARCH_LISTINGS",
  "category": "REAL_ESTATE",
  "transaction": "RENT",
  "location": "Lubumbashi",
  "filters": {
    "bedrooms": 3,
    "max_price": 500
  },
  "currency": "USD"
}
```

Examples of intents:

- SEARCH_LISTINGS
- GET_LISTING
- COMPARE_LISTINGS
- CREATE_LISTING
- UPDATE_LISTING
- PUBLISH_LISTING
- UNPUBLISH_LISTING
- UPDATE_PRICE
- UPDATE_STOCK
- CONFIRM_AVAILABILITY
- CREATE_ORDER
- ACCEPT_ORDER
- REJECT_ORDER
- CANCEL_ORDER
- REQUEST_PAYMENT
- REQUEST_DELIVERY
- SCHEDULE_VISIT
- CREATE_CAMPAIGN
- GET_REPORT
- GET_SELLER_METRICS
- CONTACT_SELLER
- REPORT_LISTING
- REQUEST_SUPPORT

The intent layer decouples human language from business services.

---

## 8. AI Product Model

Trezutil uses a multi-provider AI gateway rather than hard-coding the product to a single model provider.

### AI responsibilities

- natural-language understanding;
- multilingual interpretation;
- intent extraction;
- conversational context management;
- tool selection and orchestration;
- product/listing description generation;
- image understanding;
- voice transcription integration;
- summarisation;
- seller assistance;
- marketing copy generation;
- business-report explanation;
- trust/fraud signal assistance.

### AI boundaries

AI must not:

- bypass authorisation;
- directly execute arbitrary SQL;
- directly modify financial records without service validation;
- approve high-risk transactions without applicable controls;
- overwrite audit history;
- make unverifiable claims about a listing;
- create a false verification status.

### Tool-calling principle

The AI can call typed backend tools such as:

```text
search_listings()
get_listing()
create_listing()
update_listing()
publish_listing()
check_inventory()
create_order()
get_order_status()
initiate_payment()
schedule_visit()
create_campaign()
generate_report()
```

Business services own the final decision.

---

## 9. Search and Discovery

The search experience is conversational first.

The user should not need to know the exact category or filter vocabulary.

### Search inputs

- text;
- voice;
- image;
- follow-up conversation;
- contextual preferences.

### Search output

Search results should be ranked using a combination of:

- relevance;
- explicit user constraints;
- price fit;
- availability;
- location/distance where applicable;
- trust signals;
- seller quality;
- listing completeness;
- freshness;
- optional popularity/business rules.

### Example

Customer:

> « Je veux une maison 3 chambres, quartier calme, 500$ maximum et avec parking. »

Trezutil should return a small set of qualified results rather than an overwhelming catalogue.

---

## 10. Trust and Verification

Trust is a first-class system, not a decorative rating.

### Seller trust signals

- phone verified;
- identity verified;
- business verified where applicable;
- address/location verified where applicable;
- account age;
- transaction history;
- completion rate;
- complaints;
- dispute outcomes;
- review history;
- fraud/risk signals.

### Listing trust signals

- seller verified;
- listing freshness;
- availability freshness;
- media completeness;
- identity/ownership evidence where applicable;
- price confirmation;
- location confirmation;
- prior reports.

### Trust presentation

The system should present understandable badges and explanations, not merely an opaque score.

Example:

> 🟢 Seller verified
> 
> Availability confirmed today
> 
> 214 completed orders

An aggregate Trust Score may exist internally and as a user-facing signal where appropriate, but it must be explainable and auditable.

---

## 11. Social Commerce / Distribution

Trezutil maintains one canonical listing and can distribute or prepare content for external channels.

Target channels include:

- WhatsApp;
- Facebook;
- Instagram;
- TikTok;
- public shareable links.

Capabilities:

- generate public listing link;
- generate channel-specific copy;
- resize/prepare media;
- create campaign metadata;
- support direct publishing where the relevant platform API and permissions allow it;
- provide assisted/manual publishing when direct API publication is unavailable;
- attribute traffic and conversions to a channel when technically possible.

Important rule:

> External social networks are distribution channels. Trezutil remains the canonical source of truth for the listing.

---

## 12. Orders and Reservations

Trezutil must support both immediate commerce and reservation-based commerce.

### Order types

- purchase;
- booking;
- property viewing reservation;
- service appointment;
- future fulfilment order.

### Order lifecycle example

```text
DRAFT
  ↓
PENDING_CONFIRMATION
  ↓
CONFIRMED
  ↓
PAYMENT_PENDING / PAID
  ↓
FULFILLING
  ↓
READY / OUT_FOR_DELIVERY
  ↓
COMPLETED
```

Cancellation, refund and dispute states are handled separately according to policy.

---

## 13. Payments

Payments are abstracted behind Trezutil's payment layer.

Potential methods:

- Mobile Money;
- card;
- bank transfer where supported;
- cash on delivery for eligible flows;
- escrow-like flows for eligible high-trust transactions.

The payment domain must support:

- payment intent;
- authorization/confirmation;
- provider references;
- reconciliation;
- refund;
- failure;
- idempotency;
- audit trail.

The system must not assume one provider or one currency.

---

## 14. Logistics

Trezutil must be able to coordinate fulfilment where relevant.

Capabilities:

- delivery request;
- delivery pricing;
- assignment;
- pickup;
- tracking;
- proof of delivery;
- completion;
- delivery failure;
- return where supported.

For non-delivery transactions such as digital services, logistics is omitted.

---

## 15. Seller Commerce Assistant

The seller can operate the business conversationally.

Examples:

> « Ajoute ce produit à mon catalogue. »

> « Mets le prix à 75 dollars. »

> « J'ai reçu 20 nouvelles unités. »

> « Combien ai-je vendu cette semaine ? »

> « Quels sont mes produits les plus vendus ? »

> « Crée une promotion de 10 % pour les produits qui se vendent mal. »

> « Prépare cette maison pour Facebook et TikTok. »

> « Montre-moi les commandes qui ne sont pas encore livrées. »

Responses should be concise by default and provide action buttons when WhatsApp supports the relevant interaction pattern.

---

## 16. Seller Reporting

Seller reports must be available conversationally.

Example metrics:

- gross sales;
- net sales where applicable;
- number of orders;
- completed orders;
- average order value;
- top listings;
- low-stock listings;
- customer acquisition channel;
- campaign performance;
- refunds;
- delivery performance.

Trezutil may optionally generate a detailed document/report for download or sharing.

---

## 17. Initial Real-Estate Vertical

Real estate is the first vertical because it demonstrates a clear pain point: users can waste money and time visiting unsuitable properties.

### Property listing requirements

A listing should support:

- property type;
- rent/sale mode;
- price;
- currency;
- bedroom count;
- bathroom count;
- surface area where known;
- parking;
- water/electricity information;
- generator/solar where applicable;
- furnishing;
- security features;
- neighbourhood/area;
- approximate location;
- photos;
- video;
- virtual tour where available;
- availability status;
- agent/owner identity;
- visit fee where applicable;
- visit scheduling.

### Property availability

The platform must distinguish:

- AVAILABLE — recently confirmed;
- PENDING_CONFIRMATION;
- RESERVED;
- RENTED;
- SOLD;
- UNAVAILABLE;
- EXPIRED.

Listings should not remain indefinitely represented as available without a freshness/confirmation mechanism.

### Virtual visit

The product should progressively support:

1. photo-rich listing;
2. guided video tour;
3. interactive 360°/virtual tour;
4. floor plan and richer media for premium listings.

### Physical visit

The goal is:

> **Visit physically only after remote qualification.**

The platform should help the customer avoid unnecessary visits and provide enough information to decide whether a physical visit is worth the cost.

---

## 18. Customer Conversation Examples

### Search

Customer:
> Je cherche un iPhone 15 Pro 256 Go à Kinshasa, moins de 700$.

Trezutil:
> J'ai trouvé 9 offres. Voici les 3 qui correspondent le mieux à ton budget, à la disponibilité et au niveau de confiance du vendeur.

### Compare

Customer:
> Compare le 1 et le 3.

Trezutil returns a structured comparison.

### Order

Customer:
> Je prends le 1.

Trezutil verifies price/availability and creates the order flow.

### Real estate

Customer:
> Cherche-moi une maison 3 chambres à Lubumbashi, moins de 500$, avec parking.

Trezutil returns relevant verified/fresh listings.

### Voice

Customer sends a voice note in French, Lingala, Swahili, English or a mixed form.

Trezutil transcribes/understands the request and continues the same conversation context.

---

## 19. Conversation State

The platform must preserve enough context to support multi-turn commerce.

Example:

> Client: Je cherche une maison 3 chambres.
>
> Trezutil: Dans quelle ville ?
>
> Client: Lubumbashi.
>
> Trezutil: Quel budget ?
>
> Client: 500 dollars maximum.
>
> Trezutil: Avec parking ?
>
> Client: Oui.

The resulting structured intent is the combination of the turns, not only the final message.

The system should distinguish:

- ephemeral conversation context;
- durable customer preferences;
- transaction state;
- compliance/audit records.

---

## 20. Human Handoff

AI must be able to escalate to a human when:

- the user explicitly asks for a human;
- the transaction is high-risk;
- the bot cannot confidently resolve the intent;
- a dispute requires human intervention;
- seller verification requires manual review;
- policy requires human approval.

The human operator must receive the relevant conversation context without needing to ask the customer to repeat everything.

---

## 21. Notifications

WhatsApp is the primary notification channel for customers and sellers.

Important events include:

- order created;
- order accepted;
- payment requested;
- payment received;
- delivery status changes;
- booking confirmation;
- visit reminder;
- availability re-confirmation;
- security alerts;
- campaign summary;
- seller reports.

Fallback channels may include SMS/email where configured and appropriate.

---

## 22. Admin and Operations

The admin console must provide at least:

- global overview;
- seller management;
- verification queue;
- listings moderation;
- orders;
- payments;
- disputes;
- fraud/risk cases;
- trust investigations;
- conversation/handoff monitoring;
- category management;
- configuration;
- integration status;
- system health;
- audit logs;
- analytics.

Admin actions must be permissioned and audited.

---

## 23. Analytics and Data

Trezutil must collect product analytics around:

- search requests;
- successful matches;
- conversion to order;
- payment success;
- delivery success;
- seller activation;
- listing quality;
- listing freshness;
- customer retention;
- channel attribution;
- AI tool usage;
- AI fallback/handoff rate.

Business analytics should be separated from raw operational transactions where appropriate.

---

## 24. MVP Scope

The first production-oriented MVP should focus on a narrow but complete loop.

### Customer

- WhatsApp onboarding;
- text/voice search;
- listing results;
- listing details;
- seller trust indicators;
- shortlist;
- basic reservation/order;
- notifications.

### Seller

- WhatsApp onboarding;
- seller profile;
- create listing by text/photo/voice;
- edit/publish/unpublish listing;
- basic inventory/availability;
- receive and process orders/reservations;
- basic sales report.

### Platform

- seller verification;
- listing moderation;
- conversation monitoring;
- basic trust signals;
- audit logging;
- basic analytics.

### Real estate MVP

- property listing;
- photos;
- video;
- availability confirmation;
- property search;
- virtual tour link/media where supported;
- visit scheduling;
- agent profile and verification.

### Explicitly deferred from first MVP

- full multi-country operations;
- advanced escrow;
- sophisticated dynamic pricing;
- full social auto-publishing across every network;
- highly autonomous fraud decisions;
- complex seller ERP;
- full consumer mobile app;
- advanced AI business forecasting.

---

## 25. Non-Functional Requirements

### Availability

The platform should be designed for high availability, with graceful degradation of optional AI/integration components.

### Idempotency

Payment, order and webhook processing must be idempotent.

### Security

Authentication, authorization, secrets management, audit and least privilege are mandatory.

### Observability

Logs, metrics, traces and correlation IDs must be available for critical flows.

### Performance

Common conversational operations should return quickly enough to preserve a natural chat experience. Long-running tasks should be asynchronous with status updates.

### Localization

The data model must support multilingual content and currencies without redesign.

### Privacy

Customer and seller data must be handled according to applicable legal and contractual requirements.

---

## 26. Business Model — Initial Direction

The business model should remain flexible during validation.

Potential revenue streams:

- seller subscriptions for advanced capabilities;
- transaction fees;
- listing boosts/promotions;
- premium verification;
- qualified lead / booking fees where commercially appropriate;
- logistics margin/fees;
- business analytics and automation packages;
- enterprise integrations/API access.

Real-estate visits should not be monetised in a way that encourages unnecessary visits. The product should create incentives around qualified demand and successful transactions rather than simply increasing physical visits.

---

## 27. Success Metrics

### Customer

- successful search rate;
- search-to-contact conversion;
- search-to-order conversion;
- repeat usage;
- customer support escalation rate;
- unnecessary-visit reduction in real estate.

### Seller

- seller activation rate;
- listing creation completion;
- listing freshness;
- order acceptance rate;
- sales growth;
- report usage;
- seller retention.

### Trust

- verification completion;
- dispute rate;
- fraud-loss rate;
- successful resolution rate;
- user trust signal engagement.

### Platform

- WhatsApp delivery success;
- median conversational response time;
- AI tool success rate;
- payment success;
- delivery completion;
- platform uptime.

---

## 28. Product Roadmap

### Phase 0 — Foundation

- product specification;
- architecture;
- domain model;
- API contracts;
- event contracts;
- security model.

### Phase 1 — Commerce Core

- identity;
- sellers;
- listings;
- catalog;
- inventory;
- search.

### Phase 2 — Trust

- verification;
- reviews;
- complaints;
- trust signals;
- moderation.

### Phase 3 — WhatsApp + AI

- WhatsApp webhooks;
- conversation engine;
- intent model;
- AI tool calling;
- seller copilot;
- human handoff.

### Phase 4 — Transactions

- orders;
- reservations;
- payments;
- refunds;
- logistics.

### Phase 5 — Social Commerce

- public listing URLs;
- channel distribution;
- campaign creation;
- attribution;
- social integrations.

### Phase 6 — Real Estate Intelligence

- virtual tours;
- property matching;
- visit optimisation;
- freshness automation;
- richer trust verification.

### Phase 7 — General Commerce Expansion

- vehicles;
- electronics;
- food;
- construction;
- services;
- other categories.

---

## 29. Architectural Guardrails Derived from the Product

1. WhatsApp is a channel, never the domain core.
2. n8n is an integration/automation layer, never the authoritative transaction engine.
3. AI is never the system of record and never bypasses backend rules.
4. Payments are provider-agnostic behind a payment abstraction.
5. Listings remain canonical in Trezutil even when shared externally.
6. Trust signals must be traceable to evidence/events.
7. Long-running actions are asynchronous and observable.
8. Every important financial/order mutation is auditable.
9. API/event contracts precede significant implementation.
10. The initial backend may be modular rather than prematurely decomposed into many deployable services.

---

## 30. Open Decisions to Resolve Before Domain Implementation

The following decisions must be explicitly recorded before implementing the corresponding domain:

- exact WhatsApp Business onboarding model and account ownership model;
- first AI providers and benchmark criteria;
- supported Mobile Money providers for the initial market;
- first logistics partners / delivery model;
- exact seller verification levels;
- exact Trust Score methodology and explainability model;
- initial currencies and exchange-rate policy;
- real-estate visit fee handling;
- moderation policy;
- terms, refunds and dispute policies;
- data retention and privacy policies;
- direct vs assisted publication capabilities for each social platform.

These decisions should become ADRs or explicit product policies before the affected domain is considered production-ready.
