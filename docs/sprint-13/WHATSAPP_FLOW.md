# Sprint 13 — WhatsApp Flow

## Overview

WhatsApp integration for Inventory, Capacity, and Fulfillment follows the established pattern: **Conversation → CommerceActionRuntime → Response**.

No inventory logic in WhatsApp layer.

---

## Supported Intents

### Check Inventory / Availability

**User says:**
- "Est-ce que cet article est encore disponible ?"
- "Avez-vous une voiture disponible demain ?"
- "Combien de places restantes pour le restaurant ce soir ?"
- "Is this product in stock?"

**Intent Mapping:**
| User Phrase | Intent | Action Type |
|-------------|--------|-------------|
| "disponible", "available", "in stock" | `check_availability` | `CHECK_AVAILABILITY` |
| "combien", "how many", "stock" | `check_inventory` | `CHECK_INVENTORY` |
| "places", "slots", "capacity" | `check_capacity` | `CHECK_AVAILABILITY` |

**Entities Extracted:**
- `listing_id` / `product_name` (fuzzy match)
- `date` / `time_range`
- `quantity` / `party_size`

**Flow:**
```
WhatsApp Message
    ↓
Conversation Service
    ↓
Intent Classifier → CHECK_AVAILABILITY / CHECK_INVENTORY
    ↓
CommerceActionRuntime
    ↓
AvailabilityUseCases.CheckAvailability()
    ↓
Response Formatter
    ↓
WhatsApp Reply
```

**Response Templates:**

*Available:*
> ✅ Oui, cet article est disponible ! Il reste **7 unités** en stock.
> Voulez-vous passer commande ?

*Limited:*
> ⚠️ Stock limité : **2 unités** restantes.
> Commandez vite !

*Out of Stock:*
> ❌ Cet article n'est plus disponible pour le moment.
> Voulez-vous être notifié quand il revient ?

*Time-based (capacity):*
> ✅ Il reste **3 places** pour demain 19h-20h.
> Voulez-vous réserver ?

---

### Get Fulfillment Status

**User says:**
- "Où est ma commande ?"
- "Quand sera livré mon colis ?"
- "Statut de ma réservation"
- "Where is my order?"

**Intent Mapping:**
| User Phrase | Intent | Action Type |
|-------------|--------|-------------|
| "où est", "where is", "statut", "status" | `get_fulfillment_status` | `GET_FULFILLMENT_STATUS` |
| "livré", "delivered", "arrivé" | `get_fulfillment_status` | `GET_FULFILLMENT_STATUS` |

**Entities:**
- `order_id` / `order_reference`
- `reservation_id` / `booking_reference`

**Flow:**
```
WhatsApp Message
    ↓
Conversation Service
    ↓
Intent Classifier → GET_FULFILLMENT_STATUS
    ↓
CommerceActionRuntime
    ↓
FulfillmentUseCases.GetFulfillmentByOrder() / GetFulfillmentByReservation()
    ↓
Response Formatter
    ↓
WhatsApp Reply
```

**Response Templates:**

*In Progress:*
> 📦 Votre commande **#CMD-2024-001** est **en préparation**.
> Livraison estimée : **demain avant 18h**.
> Suivi : `TRK-12345` → [Lien de suivi]

*Ready for Pickup:*
> ✅ Votre commande **#CMD-2024-001** est **prête à retirer**.
> 📍 Magasin : 123 Rue Principale
> 🕐 Horaires : 9h-19h

*Delivered/Completed:*
> ✅ Votre commande **#CMD-2024-001** a été **livrée** le 15 janvier à 14h30.
> Merci pour votre commande !

*Appointment Confirmed:*
> 📅 Votre rendez-vous est **confirmé** pour le **15 janvier à 10h00**.
> 📍 123 Rue Principale, Salle 2
> 👤 Praticien : Dr. Martin

---

### Notifications (Outbound)

**Triggers:**
- Fulfillment status change → Notify customer
- Low inventory alert → Notify seller
- Capacity fully booked → Notify seller

**Templates:**

*Fulfillment Update:*
> 📦 Mise à jour commande **#CMD-2024-001** :
> Statut : **Expédié** → **En livraison**
> Suivi : `TRK-12345` → [Lien]

*Low Stock Alert (Seller):*
> ⚠️ Alerte stock : **"Produit X"** (SKU: ABC-123)
> Stock restant : **3 unités**
> [Réapprovisionner]

---

## Implementation

### Intent Registration

```go
// In WhatsApp intent registry
registry.RegisterIntent("check_availability", domain.ActionCheckAvailability)
registry.RegisterIntent("check_inventory", domain.ActionCheckInventory)
registry.RegisterIntent("get_fulfillment_status", domain.ActionGetFulfillmentStatus)
```

### Handler Mapping

```go
// In CommerceActionRuntime setup
runtime.RegisterHandler(NewCheckAvailabilityHandler(availabilityUseCases))
runtime.RegisterHandler(NewCheckInventoryHandler(inventoryUseCases))
runtime.RegisterHandler(NewGetFulfillmentStatusHandler(fulfillmentUseCases))
```

### Parameter Extraction

```go
func extractAvailabilityParams(entities []ConversationEntity) map[string]interface{} {
    params := make(map[string]interface{})
    for _, e := range entities {
        switch e.Name {
        case "listing_id":
            params["listing_id"] = e.NormalizedValue
        case "date":
            params["start_at"] = parseDate(e.NormalizedValue).Unix()
        case "time_range":
            params["start_at"] = parseTimeRangeStart(e.NormalizedValue)
            params["end_at"] = parseTimeRangeEnd(e.NormalizedValue)
        case "quantity":
            params["quantity"] = parseInt(e.NormalizedValue)
        case "party_size":
            params["party_size"] = parseInt(e.NormalizedValue)
        }
    }
    return params
}
```

---

## Security

- Actor identity from WhatsApp session (verified phone)
- Tenant resolved from actor's organization
- No inventory data in WhatsApp payloads
- All mutations via CommerceActionRuntime (idempotency, usage, audit)

---

## Unsupported in Sprint 13

- Inventory adjustments via WhatsApp
- Fulfillment transitions via WhatsApp
- Seller operations via WhatsApp
- Complex capacity scheduling via WhatsApp