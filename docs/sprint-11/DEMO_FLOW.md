# Sprint 11 — Demo Flow

## Overview

Complete demo walkthrough showing the Trezutil customer and seller experience, plus WhatsApp equivalent.

---

## Customer Web Demo

### 1. Home

```
Open Trezutil
  → Homepage with search bar
  → "What are you looking for?"
  → Popular categories: Products, Services, Stays, Travel, Restaurants, Vehicles, Events
```

### 2. Search

```
Type "laptop" in search bar
  → Debounce 300ms
  → POST /v1/search { text: "laptop", user_id: "user-123" }
  → See listing cards with:
    - Image
    - Title
    - Price
    - Seller
    - Status badge
  → Use filters: Category, Price, Location, Sort
```

### 3. Listing Detail

```
Click a listing card
  → GET /v1/listings/{id}?user_id=user-123
  → See:
    - Media gallery
    - Title, Price, Currency
    - Seller info
    - Availability status
    - Description
    - Attributes
    - Location
    - Actions: Order, Ask
```

### 4. Ask

```
Click "Ask" button
  → Opens conversation UI
  → Type "Is this still available?"
  → Message sent via WhatsApp channel
  → CommerceAction generated for Ask intent
```

### 5. Order

```
Click "Order" button
  → Confirm quantity and details
  → POST /v1/orders { listing_id, quantity, user_id }
  → See order confirmation with:
    - Order ID
    - Status (draft)
    - Total amount
  → Order appears in Orders page
```

### 6. Orders

```
Navigate to Orders page
  → GET /v1/orders?user_id=user-123
  → See order list with:
    - Reference ID
    - Seller
    - Items
    - Amount
    - Status (Confirmed, Pending, Cancelled, Completed)
    - Created date
  → Click an order to view details
  → Cancel if permitted (status allows)
```

---

## Seller Demo

### 1. Seller Dashboard

```
Open Seller Dashboard
  → See:
    - Today: 12 conversations, 8 orders, 5 pending actions
    - 94 credits remaining
  → Navigation: Listings, Orders, Conversations, Usage
```

### 2. Manage Listings

```
Navigate to Listings
  → See all listings with status
  → Click "Create Listing"
  → Fill form: Title, Price, Category, Media, Availability
  → POST /v1/listings
  → Listing appears in list (status: draft)
  → Click "Publish" → status: published
  → Click "Pause" → status: paused
  → Click "Resume" → status: published
```

### 3. View Orders

```
Navigate to Orders
  → See orders from customers
  → Each shows: Reference, Items, Amount, Status
```

### 4. Usage

```
Navigate to Usage
  → See:
    - Current plan
    - Credits remaining
    - Monthly usage
```

---

## WhatsApp Equivalent

### Search

```
User: "find laptops"
↓
WhatsApp Service.ProcessWebhook
  ↓ Verify HMAC
  ↓ Resolve Tenant (WABA ID → Org)
  ↓ Resolve User (Phone → User)
  ↓ Claim Webhook Event
  ↓ Find/Create Conversation
  ↓ CommerceAction(SEARCH_LISTINGS)
  ↓ SearchHandler → SearchUseCases.ExecuteSearch
  ↓ Response: listing cards with titles and prices
```

### View Listing

```
User: "show me laptop lst-1"
↓
CommerceAction(GET_LISTING)
  ↓ GetListingHandler → ListingUseCases.GetListing
  ↓ Response: listing details
```

### Order

```
User: "buy this laptop"
↓
CommerceAction(CREATE_ORDER, {listing_id: "lst-1"})
  ↓ CreateOrderHandler → OrderUseCases.CreateOrder
  ↓ Response: order_id, status, total
```

### Track Order

```
User: "where is my order ord-1"
↓
CommerceAction(GET_ORDER)
  ↓ GetOrderHandler → OrderUseCases.GetOrder
  ↓ Response: order details, status
```

### Cancel Order

```
User: "cancel order ord-1"
↓
CommerceAction(CANCEL_ORDER)
  ↓ CancelOrderHandler → OrderUseCases.CancelOrder
  ↓ Response: cancelled
```

---

## Key Differences

| Aspect | Web | WhatsApp |
|--------|-----|----------|
| Input | Keyboard, filters, buttons | Text messages |
| Output | Full UI, images, cards | Text, cards, buttons |
| Navigation | Pages, URLs | Conversation turns |
| Search | Faceted filters, sorting | Natural language |
| Actions | Click buttons | Intent-based |
| Core | Same | Same |
