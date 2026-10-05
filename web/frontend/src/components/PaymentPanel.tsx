import { useEffect, useRef, useState, type FormEvent } from 'react'
import { toApiError } from '@/api/client'
import { ordersApi, paymentsApi, type OrderSummary } from '@/api/commerce'
import { isStripeAction, loadStripe, type ClientAction, type StripeClient, type StripeElements } from '@/api/stripe'
import { Button, Icon } from '@/components/ui'
import { Loading } from '@/components/states'
import { t } from '@/lib/i18n'

/**
 * Pays an order. Card details are entered in Stripe's Payment Element (served
 * by Stripe); Trezutil learns the result from the signed provider webhook, so
 * after the payer confirms we poll the order until it leaves "awaiting payment".
 */

const settled = new Set(['paid', 'confirmed', 'processing', 'ready_for_fulfillment', 'fulfilled', 'completed', 'cancelled', 'refunded'])

export function isAwaitingPayment(order: OrderSummary) {
  return order.status === 'draft' || order.status === 'pending_payment'
}

function StripeForm({ action, onDone }: { action: ClientAction; onDone: (message: string) => void }) {
  const mountRef = useRef<HTMLDivElement>(null)
  const stripeRef = useRef<{ stripe: StripeClient; elements: StripeElements } | null>(null)
  const [ready, setReady] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    let element: { destroy(): void } | null = null
    loadStripe(action.publishable_key ?? '')
      .then((stripe) => {
        if (cancelled || !mountRef.current) return
        const elements = stripe.elements({ clientSecret: action.client_secret ?? '' })
        const payment = elements.create('payment')
        payment.mount(mountRef.current)
        element = payment
        stripeRef.current = { stripe, elements }
        setReady(true)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : t('The payment form could not be loaded.')))
    return () => { cancelled = true; element?.destroy() }
  }, [action.client_secret, action.publishable_key])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!stripeRef.current) return
    setBusy(true)
    setError(null)
    const { stripe, elements } = stripeRef.current
    // Methods that need a bank page redirect back to this page with ?redirect_status=.
    const result = await stripe.confirmPayment({ elements, confirmParams: { return_url: window.location.href.split('?')[0] }, redirect: 'if_required' })
    setBusy(false)
    if (result.error) { setError(result.error.message ?? t('The payment was not completed.')); return }
    onDone(result.paymentIntent?.status === 'processing' ? t('Payment processing — we’ll update your order shortly.') : t('Payment received — confirming your order…'))
  }

  return (
    <form className="form-stack payment-form" onSubmit={submit}>
      <div ref={mountRef} className="stripe-element" aria-label={t('Card details')} />
      {!ready && !error && <Loading label={t('Loading secure payment form…')} />}
      {error && <p className="auth-error" role="alert">{error}</p>}
      <Button className="primary-button full-button" type="submit" disabled={!ready} busy={busy}>{busy ? t('Paying…') : t('Pay now')} <Icon name="arrow" /></Button>
      <p className="payment-note"><Icon name="check" size={14} />{t('Card details are handled by Stripe and never stored by Trezutil.')}</p>
    </form>
  )
}

export function PaymentPanel({ order, onUpdated }: { order: OrderSummary; onUpdated: (order: OrderSummary) => void }) {
  const [action, setAction] = useState<ClientAction | null>(null)
  const [notice, setNotice] = useState<{ tone: 'info' | 'error'; text: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [polling, setPolling] = useState(false)

  // Returning from a redirect-based payment method.
  useEffect(() => {
    const status = new URLSearchParams(window.location.search).get('redirect_status')
    if (!status) return
    window.history.replaceState(null, '', window.location.pathname)
    if (status === 'succeeded') { setNotice({ tone: 'info', text: t('Payment received — confirming your order…') }); setPolling(true) }
    else setNotice({ tone: 'error', text: t('The payment was not completed. You can try again.') })
  }, [])

  useEffect(() => {
    if (!polling) return
    let attempts = 0
    const timer = window.setInterval(() => {
      attempts += 1
      ordersApi.get(order.id).then((o) => {
        const done = settled.has(o.status)
        if (done || attempts >= 10) {
          window.clearInterval(timer)
          setPolling(false)
          onUpdated(o)
          setNotice(done ? null : { tone: 'info', text: t('Your payment is still being confirmed. Refresh this page in a moment.') })
        }
      }).catch(() => undefined)
    }, 2000)
    return () => window.clearInterval(timer)
  }, [polling, order.id, onUpdated])

  const start = async () => {
    setBusy(true)
    setNotice(null)
    try {
      const payment = await paymentsApi.payOrder(order)
      if (isStripeAction(payment.client_action)) setAction(payment.client_action)
      else if (payment.status === 'failed') setNotice({ tone: 'error', text: t('The payment was declined. Please try another card.') })
      else { setPolling(true); setNotice({ tone: 'info', text: t('Confirming your payment…') }) }
    } catch (err) {
      const e = toApiError(err)
      setNotice({
        tone: e.code === 'PAYMENT_PROVIDER_UNAVAILABLE' ? 'info' : 'error',
        text: e.code === 'PAYMENT_PROVIDER_UNAVAILABLE'
          ? t('Online payment isn’t enabled for this seller yet. Your order is saved — the seller will contact you to arrange payment.')
          : e.message,
      })
    } finally {
      setBusy(false)
    }
  }

  if (!isAwaitingPayment(order)) return null
  return (
    <div className="payment-panel">
      {notice && <div className={`inline-notice ${notice.tone}`} role={notice.tone === 'error' ? 'alert' : 'status'}><Icon name="bell" size={16} /><span>{notice.text}</span></div>}
      {polling && <Loading label={t('Confirming payment…')} />}
      {!polling && action && <StripeForm action={action} onDone={(text) => { setAction(null); setNotice({ tone: 'info', text }); setPolling(true) }} />}
      {!polling && !action && <Button className="primary-button full-button" onClick={() => void start()} busy={busy}>{busy ? t('Starting secure payment…') : t('Pay securely')} <Icon name="arrow" /></Button>}
    </div>
  )
}
