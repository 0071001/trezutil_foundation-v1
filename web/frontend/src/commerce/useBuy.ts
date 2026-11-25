import { useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { ordersApi } from '@/api/commerce'
import { toApiError } from '@/api/client'
import { cart, type CartGroup } from '@/commerce/cart'
import type { Offer } from '@/commerce/offers'
import { t } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { errorText, newId } from '@/ui/kit'

/*
 * Ordering from an offer or from the cart. An order is created only when the person asks (it is "awaiting payment",
 * nothing is charged), then they land on it to pay. A visitor keeps their cart and is asked to sign in first.
 */

function message(code: string | undefined, fallback: string): string {
  switch (code) {
    case 'LISTING_UNAVAILABLE': return t('Cette offre n’est plus disponible.')
    case 'MIXED_STORES': return t('Une commande se fait chez un seul fournisseur et dans une seule devise.')
    case 'FORBIDDEN': return t('Ce compte ne peut pas passer de commande.')
    default: return fallback
  }
}

export function useBuy() {
  const { user } = useSession()
  const navigate = useNavigate()
  const { pathname, search } = useLocation()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const signIn = () => navigate(`/login?returnTo=${encodeURIComponent(pathname + search)}`)

  /** Orders one offer now and opens the order to pay it. */
  const buyNow = async (offer: Offer, quantity: number) => {
    setError(null)
    if (!user) {
      cart.add(offer, quantity)
      signIn()
      return
    }
    setBusy(true)
    try {
      const order = await ordersApi.create(offer.id, quantity, newId())
      navigate(`/purchases/${order.id}`)
    } catch (err) {
      const e = toApiError(err)
      setError(message(e.code, errorText(e)))
    } finally {
      setBusy(false)
    }
  }

  /** Orders one group of the cart (one shop, one currency) and opens the order to pay it. */
  const checkout = async (group: CartGroup) => {
    setError(null)
    if (!user) { signIn(); return }
    setBusy(true)
    try {
      const order = await ordersApi.createCart(group.items.map((i) => ({ listing_id: i.listingId, quantity: i.quantity })), newId())
      cart.removeMany(group.items.map((i) => i.listingId))
      navigate(`/purchases/${order.id}`)
    } catch (err) {
      const e = toApiError(err)
      setError(message(e.code, errorText(e)))
    } finally {
      setBusy(false)
    }
  }

  return { buyNow, checkout, busy, error, signedIn: !!user }
}
