import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { ordersApi, type OrderSummary } from '@/api/commerce'
import { listingsApi } from '@/api/listings'
import { ListingImage } from '@/components/cards'
import { PaymentPanel, isAwaitingPayment } from '@/components/PaymentPanel'
import { AppShell } from '@/components/shell'
import { EmptyState, ErrorState, Loading } from '@/components/states'
import { ActionLink, Brand, Button, Heading, Icon } from '@/components/ui'
import { formatMinor, formatMoney } from '@/lib/format'
import { usePrefs } from '@/lib/prefs'
import { useAsync } from '@/lib/useAsync'

/* Figma CheckoutPage. One order per listing (the order API is single-listing);
   the price is set by the server from the listing when the order is created. */

const steps = ['Review', 'Payment', 'Confirmation']

function Progress({ step }: { step: number }) {
  return (
    <div role="list" className="checkout-progress" aria-label="Checkout progress">
      {steps.map((item, i) => (
        <div role="listitem" className={step >= i + 1 ? 'active' : ''} key={item} aria-current={step === i + 1 ? 'step' : undefined}>
          <span>{step > i + 1 ? '✓' : i + 1}</span><strong>{item}</strong>
        </div>
      ))}
    </div>
  )
}

export default function CheckoutPage() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { removeFromBag } = usePrefs()
  const listingId = params.get('listing_id') ?? ''
  const [quantity, setQuantity] = useState(() => Math.max(1, Math.min(99, Number(params.get('quantity')) || 1)))
  const listing = useAsync(() => listingsApi.get(listingId), [listingId], !!listingId)
  const [order, setOrder] = useState<OrderSummary | null>(null)
  const [placing, setPlacing] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  // One key per checkout visit: a double click or retry never creates two orders.
  const idempotencyKey = useMemo(() => crypto.randomUUID(), [])
  const onUpdated = useCallback((o: OrderSummary) => setOrder(o), [])

  useEffect(() => { document.title = 'Secure checkout — Trezutil' }, [])

  if (!listingId) return <AppShell hideDock><EmptyState icon="orders" title="Nothing to check out" copy="Choose something to buy first — your bag keeps what you’ve picked." action="Open your bag" href="/cart" /></AppShell>
  if (listing.loading) return <AppShell hideDock><Loading label="Preparing checkout…" /></AppShell>
  if (listing.error || !listing.data) return <AppShell hideDock><section className="page-section"><ErrorState error={listing.error ?? 'Listing not found'} onRetry={listing.reload} /></section></AppShell>

  const l = listing.data
  const step = !order ? 1 : isAwaitingPayment(order) ? 2 : 3
  const estimate = l.price_amount * quantity

  const place = async () => {
    setPlacing(true)
    setError(null)
    try {
      const created = await ordersApi.create(l.id, quantity, idempotencyKey)
      setOrder(created)
      removeFromBag(l.id)
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setPlacing(false)
    }
  }

  return (
    <AppShell hideDock>
      <section className="page-section checkout-page">
        <div className="checkout-head"><Brand /><span><Icon name="check" size={15} />Secure checkout</span></div>
        <Progress step={step} />
        <div className="checkout-layout">
          <div className="checkout-form">
            {step === 1 && <>
              <Heading level={2}>Review your order</Heading>
              <p>Check the quantity. The final price is confirmed by Trezutil when your order is created.</p>
              <div className="form-stack">
                <div className="choice-card static"><span><strong>{l.title}</strong><small>{formatMoney(l.price_amount, l.price_currency)} each</small></span>
                  <div className="quantity" role="group" aria-label="Quantity">
                    <Button ariaLabel="Decrease quantity" onClick={() => setQuantity((q) => Math.max(1, q - 1))} disabled={quantity <= 1}>−</Button>
                    <span aria-live="polite">{quantity}</span>
                    <Button ariaLabel="Increase quantity" onClick={() => setQuantity((q) => Math.min(99, q + 1))}>+</Button>
                  </div>
                </div>
                <div className="choice-card static"><span><strong>Pickup or delivery</strong><small>Arranged with the seller after your order is confirmed.</small></span></div>
              </div>
              {error && <ErrorState error={error} compact />}
              <div className="checkout-buttons">
                <Button className="outline-button" onClick={() => navigate(-1)}>Back</Button>
                <Button className="primary-button" onClick={() => void place()} busy={placing} disabled={l.status !== 'published'}>{placing ? 'Placing order…' : 'Place order'}<Icon name="arrow" /></Button>
              </div>
            </>}
            {step === 2 && order && <>
              <Heading level={2}>Payment</Heading>
              <p>Your order is saved. Pay now to confirm it with the seller.</p>
              <PaymentPanel order={order} onUpdated={onUpdated} />
              <div className="checkout-buttons"><ActionLink className="outline-button" href={`/orders/${encodeURIComponent(order.id)}`}>Pay later</ActionLink></div>
            </>}
            {step === 3 && order && <>
              <div className="checkout-success"><span className="empty-mark"><Icon name="check" size={26} /></span>
                <Heading level={2}>{order.status === 'cancelled' ? 'Order cancelled' : 'Thank you — your order is confirmed'}</Heading>
                <p>{order.status === 'cancelled' ? 'Nothing was charged.' : 'The seller has been notified. You can follow every step from your orders.'}</p>
                <div className="checkout-buttons"><ActionLink className="primary-button" href={`/orders/${encodeURIComponent(order.id)}`}>View order <Icon name="arrow" /></ActionLink><ActionLink className="outline-button" href="/discover">Keep discovering</ActionLink></div>
              </div>
            </>}
          </div>
          <aside className="checkout-summary" aria-label="Order summary">
            <Heading level={3}>Your order</Heading>
            <div className="mini-order"><ListingImage src={l.media[0]?.url} alt="" type={l.listing_type} /><div><strong>{l.title}</strong><span>Qty {order?.quantity || quantity}</span></div><b>{formatMoney(l.price_amount, l.price_currency)}</b></div>
            <div className="summary-total"><span>Total</span><strong>{order ? formatMinor(order.total_minor, order.currency) : formatMoney(estimate, l.price_currency)}</strong></div>
            {order && <small className="summary-ref">Order #{order.id.slice(0, 8)}</small>}
          </aside>
        </div>
      </section>
    </AppShell>
  )
}
