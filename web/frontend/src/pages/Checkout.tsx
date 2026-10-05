import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { ordersApi, type OrderSummary } from '@/api/commerce'
import { listingsApi } from '@/api/listings'
import type { ListingDetail } from '@/types'
import { ListingImage } from '@/components/cards'
import { PaymentPanel, isAwaitingPayment } from '@/components/PaymentPanel'
import { AppShell } from '@/components/shell'
import { EmptyState, ErrorState, Loading } from '@/components/states'
import { ActionLink, Brand, Button, Heading, Icon } from '@/components/ui'
import { formatMinor, formatMoney } from '@/lib/format'
import { usePrefs } from '@/lib/prefs'
import { useAsync } from '@/lib/useAsync'
import { t, tx, plural, tn } from '@/lib/i18n'

/* Figma CheckoutPage. `?listing_id=` orders one listing; `?bag=1` orders the bag: the
   items of one store and currency become ONE order. The server sets every price
   from the listing when the order is created. */

const steps = [tx('Review'), tx('Payment'), tx('Confirmation')]

function Progress({ step }: { step: number }) {
  return (
    <div role="list" className="checkout-progress" aria-label={t('Checkout progress')}>
      {steps.map((item, i) => (
        <div role="listitem" className={step >= i + 1 ? 'active' : ''} key={item} aria-current={step === i + 1 ? 'step' : undefined}>
          <span>{step > i + 1 ? '✓' : i + 1}</span><strong>{t(item)}</strong>
        </div>
      ))}
    </div>
  )
}

function SingleCheckout() {
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

  useEffect(() => { document.title = t('Secure checkout — Trezutil') }, [])

  if (!listingId) return <AppShell hideDock><EmptyState icon="orders" title={t('Nothing to check out')} copy={t('Choose something to buy first — your bag keeps what you’ve picked.')} action={t('Open your bag')} href="/cart" /></AppShell>
  if (listing.loading) return <AppShell hideDock><Loading label={t('Preparing checkout…')} /></AppShell>
  if (listing.error || !listing.data) return <AppShell hideDock><section className="page-section"><ErrorState error={listing.error ?? t('Listing not found')} onRetry={listing.reload} /></section></AppShell>

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
        <div className="checkout-head"><Brand /><span><Icon name="check" size={15} />{t('Secure checkout')}</span></div>
        <Progress step={step} />
        <div className="checkout-layout">
          <div className="checkout-form">
            {step === 1 && <>
              <Heading level={2}>{t('Review your order')}</Heading>
              <p>{t('Check the quantity. The final price is confirmed by Trezutil when your order is created.')}</p>
              <div className="form-stack">
                <div className="choice-card static"><span><strong>{l.title}</strong><small>{formatMoney(l.price_amount, l.price_currency)} {t('each')}</small></span>
                  <div className="quantity" role="group" aria-label={t('Quantity')}>
                    <Button ariaLabel={t('Decrease quantity')} onClick={() => setQuantity((q) => Math.max(1, q - 1))} disabled={quantity <= 1}>−</Button>
                    <span aria-live="polite">{quantity}</span>
                    <Button ariaLabel={t('Increase quantity')} onClick={() => setQuantity((q) => Math.min(99, q + 1))}>+</Button>
                  </div>
                </div>
                <div className="choice-card static"><span><strong>{t('Pickup or delivery')}</strong><small>{t('Arranged with the seller after your order is confirmed.')}</small></span></div>
              </div>
              {error && <ErrorState error={error} compact />}
              <div className="checkout-buttons">
                <Button className="outline-button" onClick={() => navigate(-1)}>{t('Back')}</Button>
                <Button className="primary-button" onClick={() => void place()} busy={placing} disabled={l.status !== 'published'}>{placing ? t('Placing order…') : t('Place order')}<Icon name="arrow" /></Button>
              </div>
            </>}
            {step === 2 && order && <>
              <Heading level={2}>{t('Payment')}</Heading>
              <p>{t('Your order is saved. Pay now to confirm it with the seller.')}</p>
              <PaymentPanel order={order} onUpdated={onUpdated} />
              <div className="checkout-buttons"><ActionLink className="outline-button" href={`/orders/${encodeURIComponent(order.id)}`}>{t('Pay later')}</ActionLink></div>
            </>}
            {step === 3 && order && <>
              <div className="checkout-success"><span className="empty-mark"><Icon name="check" size={26} /></span>
                <Heading level={2}>{order.status === 'cancelled' ? t('Order cancelled') : t('Thank you — your order is confirmed')}</Heading>
                <p>{order.status === 'cancelled' ? t('Nothing was charged.') : t('The seller has been notified. You can follow every step from your orders.')}</p>
                <div className="checkout-buttons"><ActionLink className="primary-button" href={`/orders/${encodeURIComponent(order.id)}`}>{t('View order')} <Icon name="arrow" /></ActionLink><ActionLink className="outline-button" href="/discover">{t('Keep discovering')}</ActionLink></div>
              </div>
            </>}
          </div>
          <aside className="checkout-summary" aria-label={t('Order summary')}>
            <Heading level={3}>{t('Your order')}</Heading>
            <div className="mini-order"><ListingImage src={l.media[0]?.url} alt="" type={l.listing_type} /><div><strong>{l.title}</strong><span>{t('Qty')} {order?.quantity || quantity}</span></div><b>{formatMoney(l.price_amount, l.price_currency)}</b></div>
            <div className="summary-total"><span>{t('Total')}</span><strong>{order ? formatMinor(order.total_minor, order.currency) : formatMoney(estimate, l.price_currency)}</strong></div>
            {order && <small className="summary-ref">{t('Order #')}{order.id.slice(0, 8)}</small>}
          </aside>
        </div>
      </section>
    </AppShell>
  )
}

/** Bag items grouped by what one order can hold: the same store and currency. */
interface BagGroup { key: string; seller: string; currency: string; lines: { listing: ListingDetail; quantity: number }[] }

function BagCheckout() {
  const navigate = useNavigate()
  const { bag, removeFromBag } = usePrefs()
  // The bag holds snapshots; the order is built from the live listings.
  const bagKey = bag.map((b) => `${b.listing_id}:${b.quantity}`).join(',')
  const listings = useAsync(async () => {
    const settled = await Promise.allSettled(bag.map((b) => listingsApi.get(b.listing_id)))
    return settled.map((r, i) => (r.status === 'fulfilled' ? { listing: r.value, quantity: bag[i].quantity } : null))
  }, [bagKey], bag.length > 0)
  const [chosen, setChosen] = useState<string | null>(null)
  const [quantities, setQuantities] = useState<Record<string, number>>({})
  const [order, setOrder] = useState<OrderSummary | null>(null)
  const [placing, setPlacing] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  // One key per set of lines: a double click or retry returns the same order, while
  // changed quantities get a fresh key (the same key with other items would conflict).
  const keyRef = useRef<{ sig: string; key: string }>({ sig: '', key: '' })
  const onUpdated = useCallback((o: OrderSummary) => setOrder(o), [])

  useEffect(() => { document.title = t('Secure checkout — Trezutil') }, [])

  const found = (listings.data ?? []).filter((x): x is { listing: ListingDetail; quantity: number } => x !== null)
  const missing = (listings.data ?? []).length - found.length
  const groups = useMemo(() => {
    const map = new Map<string, BagGroup>()
    for (const x of found) {
      const key = `${x.listing.organization_id}|${x.listing.store_id}|${x.listing.price_currency}`
      const g = map.get(key) ?? { key, seller: bag.find((b) => b.listing_id === x.listing.id)?.seller ?? '', currency: x.listing.price_currency, lines: [] }
      g.lines.push(x)
      map.set(key, g)
    }
    return [...map.values()]
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [listings.data])

  if (bag.length === 0 && !order) return <AppShell hideDock><EmptyState icon="orders" title={t('Your bag is empty')} copy={t('Add products while you explore — they stay here until you’re ready to order.')} action={t('Start discovering')} href="/discover" /></AppShell>
  if (listings.loading) return <AppShell hideDock><Loading label={t('Preparing checkout…')} /></AppShell>
  if (listings.error) return <AppShell hideDock><section className="page-section"><ErrorState error={listings.error} onRetry={listings.reload} /></section></AppShell>

  const group = groups.length === 1 ? groups[0] : groups.find((g) => g.key === chosen)
  const qty = (l: ListingDetail, fallback: number) => Math.max(1, Math.min(99, quantities[l.id] ?? fallback))
  const lines = group?.lines.map((x) => ({ listing_id: x.listing.id, quantity: qty(x.listing, x.quantity) })) ?? []
  const estimate = group?.lines.reduce((n, x) => n + x.listing.price_amount * qty(x.listing, x.quantity), 0) ?? 0
  const step = !order ? 1 : isAwaitingPayment(order) ? 2 : 3
  const unavailable = group?.lines.some((x) => x.listing.status !== 'published') ?? false

  const place = async () => {
    if (!group) return
    const sig = JSON.stringify(lines)
    if (keyRef.current.sig !== sig) keyRef.current = { sig, key: crypto.randomUUID() }
    setPlacing(true)
    setError(null)
    try {
      const created = await ordersApi.createCart(lines, keyRef.current.key)
      setOrder(created)
      lines.forEach((l) => removeFromBag(l.listing_id))
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setPlacing(false)
    }
  }

  return (
    <AppShell hideDock>
      <section className="page-section checkout-page">
        <div className="checkout-head"><Brand /><span><Icon name="check" size={15} />{t('Secure checkout')}</span></div>
        <Progress step={step} />
        <div className="checkout-layout">
          <div className="checkout-form">
            {step === 1 && !group && <>
              <Heading level={2}>{t('Choose a seller')}</Heading>
              <p>{t('An order holds the items of one seller and one currency. Order one now; the rest stay in your bag.')}</p>
              <div className="form-stack">
                {groups.map((g) => (
                  <div className="choice-card static" key={g.key}>
                    <span><strong>{g.seller || t('Trezutil seller')}</strong><small>{plural(g.lines.length, 'item')} · {g.currency}</small></span>
                    <Button className="small-primary" onClick={() => setChosen(g.key)}>{t('Order these')}</Button>
                  </div>
                ))}
              </div>
              <div className="checkout-buttons"><Button className="outline-button" onClick={() => navigate(-1)}>{t('Back')}</Button></div>
            </>}
            {step === 1 && group && <>
              <Heading level={2}>{t('Review your order')}</Heading>
              <p>{t('Check the quantities. The final prices are confirmed by Trezutil when your order is created.')}</p>
              {missing > 0 && <p role="status">{tn(missing, '{n} item is no longer available and was left out.', '{n} items are no longer available and were left out.')}</p>}
              <div className="form-stack">
                {group.lines.map((x) => (
                  <div className="choice-card static" key={x.listing.id}><span><strong>{x.listing.title}</strong><small>{formatMoney(x.listing.price_amount, x.listing.price_currency)} {t('each')}</small></span>
                    <div className="quantity" role="group" aria-label={t('Quantity of {title}', { title: x.listing.title })}>
                      <Button ariaLabel={t('Decrease quantity of {title}', { title: x.listing.title })} onClick={() => setQuantities((q) => ({ ...q, [x.listing.id]: Math.max(1, qty(x.listing, x.quantity) - 1) }))} disabled={qty(x.listing, x.quantity) <= 1}>−</Button>
                      <span aria-live="polite">{qty(x.listing, x.quantity)}</span>
                      <Button ariaLabel={t('Increase quantity of {title}', { title: x.listing.title })} onClick={() => setQuantities((q) => ({ ...q, [x.listing.id]: Math.min(99, qty(x.listing, x.quantity) + 1) }))}>+</Button>
                    </div>
                  </div>
                ))}
              </div>
              {unavailable && <p role="alert">{t('An item in this order is no longer published. Remove it from your bag to continue.')}</p>}
              {error && <ErrorState error={error} compact />}
              <div className="checkout-buttons">
                <Button className="outline-button" onClick={() => (groups.length > 1 ? setChosen(null) : navigate(-1))}>{t('Back')}</Button>
                <Button className="primary-button" onClick={() => void place()} busy={placing} disabled={unavailable || lines.length === 0}>{placing ? t('Placing order…') : t('Place order')}<Icon name="arrow" /></Button>
              </div>
            </>}
            {step === 2 && order && <>
              <Heading level={2}>{t('Payment')}</Heading>
              <p>{t('Your order is saved. Pay now to confirm it with the seller.')}</p>
              <PaymentPanel order={order} onUpdated={onUpdated} />
              <div className="checkout-buttons"><ActionLink className="outline-button" href={`/orders/${encodeURIComponent(order.id)}`}>{t('Pay later')}</ActionLink></div>
            </>}
            {step === 3 && order && <>
              <div className="checkout-success"><span className="empty-mark"><Icon name="check" size={26} /></span>
                <Heading level={2}>{order.status === 'cancelled' ? t('Order cancelled') : t('Thank you — your order is confirmed')}</Heading>
                <p>{order.status === 'cancelled' ? t('Nothing was charged.') : t('The seller has been notified. You can follow every step from your orders.')}</p>
                <div className="checkout-buttons"><ActionLink className="primary-button" href={`/orders/${encodeURIComponent(order.id)}`}>{t('View order')} <Icon name="arrow" /></ActionLink><ActionLink className="outline-button" href="/discover">{t('Keep discovering')}</ActionLink></div>
              </div>
            </>}
          </div>
          <aside className="checkout-summary" aria-label={t('Order summary')}>
            <Heading level={3}>{t('Your order')}</Heading>
            {order && order.items.map((it) => (
              <div className="mini-order" key={it.listing_id}><div><strong>{it.title}</strong><span>{t('Qty')} {it.quantity}</span></div><b>{formatMinor(it.unit_minor * it.quantity, order.currency)}</b></div>
            ))}
            {!order && group?.lines.map((x) => (
              <div className="mini-order" key={x.listing.id}><ListingImage src={x.listing.media[0]?.url} alt="" type={x.listing.listing_type} /><div><strong>{x.listing.title}</strong><span>{t('Qty')} {qty(x.listing, x.quantity)}</span></div><b>{formatMoney(x.listing.price_amount * qty(x.listing, x.quantity), x.listing.price_currency)}</b></div>
            ))}
            <div className="summary-total"><span>{t('Total')}</span><strong>{order ? formatMinor(order.total_minor, order.currency) : group ? formatMoney(estimate, group.currency) : '—'}</strong></div>
            {order && <small className="summary-ref">{t('Order #')}{order.id.slice(0, 8)}</small>}
          </aside>
        </div>
      </section>
    </AppShell>
  )
}

export default function CheckoutPage() {
  const [params] = useSearchParams()
  return params.get('bag') ? <BagCheckout /> : <SingleCheckout />
}
