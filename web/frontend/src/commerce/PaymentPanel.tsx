import { useEffect, useRef, useState, type FormEvent } from 'react'
import { toApiError } from '@/api/client'
import { ordersApi, paymentsApi, type OrderSummary } from '@/api/commerce'
import { isStripeAction, loadStripe, type ClientAction, type StripeClient, type StripeElements } from '@/api/stripe'
import { Icon } from '@/ui/Icon'
import { Loading } from '@/ui/kit'

/*
 * Paying an order (POST /payments). Card details go into Stripe's Payment Element, served by Stripe; Trezutil learns
 * the result from the signed provider webhook, so after the payer confirms, the order is read again until it leaves
 * "awaiting payment". When the backend has no payment provider configured it answers PAYMENT_PROVIDER_UNAVAILABLE,
 * which is said as it is: nothing is charged and the order stays saved.
 */

const SETTLED = new Set(['paid', 'confirmed', 'processing', 'ready_for_fulfillment', 'fulfilled', 'completed', 'cancelled', 'refunded'])

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
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Le formulaire de paiement n’a pas pu être chargé.'))
    return () => { cancelled = true; element?.destroy() }
  }, [action.client_secret, action.publishable_key])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!stripeRef.current || busy) return
    setBusy(true)
    setError(null)
    const { stripe, elements } = stripeRef.current
    // Methods that need a bank page come back here with ?redirect_status=.
    const result = await stripe.confirmPayment({ elements, confirmParams: { return_url: window.location.href.split('?')[0] }, redirect: 'if_required' })
    setBusy(false)
    if (result.error) { setError(result.error.message ?? 'Le paiement n’a pas abouti.'); return }
    onDone(result.paymentIntent?.status === 'processing' ? 'Paiement en cours de traitement : la commande sera mise à jour sous peu.' : 'Paiement reçu : confirmation de la commande…')
  }

  return (
    <form className="payment-form" onSubmit={(e) => void submit(e)}>
      <div ref={mountRef} className="stripe-element" aria-label="Coordonnées de carte" />
      {!ready && !error && <Loading label="Chargement du formulaire de paiement sécurisé…" />}
      {error && <p className="form-error" role="alert">{error}</p>}
      <button type="submit" className="primary-btn" disabled={!ready || busy} aria-busy={busy}>{busy ? 'Paiement…' : 'Payer maintenant'} <Icon name="arrow" size={15} /></button>
      <p className="muted-line"><Icon name="lock" size={13} /> Les données de carte sont traitées par Stripe et jamais conservées par Trezutil.</p>
    </form>
  )
}

export function PaymentPanel({ order, onUpdated }: { order: OrderSummary; onUpdated: (order: OrderSummary) => void }) {
  const [action, setAction] = useState<ClientAction | null>(null)
  const [notice, setNotice] = useState<{ tone: 'info' | 'error'; text: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [polling, setPolling] = useState(false)

  // Back from a redirect-based payment method.
  useEffect(() => {
    const status = new URLSearchParams(window.location.search).get('redirect_status')
    if (!status) return
    window.history.replaceState(null, '', window.location.pathname)
    if (status === 'succeeded') { setNotice({ tone: 'info', text: 'Paiement reçu : confirmation de la commande…' }); setPolling(true) }
    else setNotice({ tone: 'error', text: 'Le paiement n’a pas abouti. Tu peux réessayer.' })
  }, [])

  useEffect(() => {
    if (!polling) return
    let attempts = 0
    const timer = window.setInterval(() => {
      attempts += 1
      ordersApi.get(order.id).then((o) => {
        const done = SETTLED.has(o.status)
        if (done || attempts >= 10) {
          window.clearInterval(timer)
          setPolling(false)
          onUpdated(o)
          setNotice(done ? null : { tone: 'info', text: 'Le paiement est encore en cours de confirmation. Recharge la page dans un instant.' })
        }
      }).catch(() => undefined)
    }, 2000)
    return () => window.clearInterval(timer)
  }, [polling, order.id, onUpdated])

  const start = async () => {
    if (busy) return
    setBusy(true)
    setNotice(null)
    try {
      const payment = await paymentsApi.payOrder(order)
      if (isStripeAction(payment.client_action)) setAction(payment.client_action)
      else if (payment.status === 'failed') setNotice({ tone: 'error', text: 'Le paiement a été refusé. Essaie une autre carte.' })
      else { setPolling(true); setNotice({ tone: 'info', text: 'Confirmation du paiement…' }) }
    } catch (err) {
      const e = toApiError(err)
      setNotice(e.code === 'PAYMENT_PROVIDER_UNAVAILABLE'
        ? { tone: 'info', text: 'Le paiement en ligne n’est pas encore activé pour ce vendeur. Ta commande est enregistrée : rien n’a été débité.' }
        : { tone: 'error', text: e.message || 'Le paiement n’a pas pu démarrer.' })
    } finally {
      setBusy(false)
    }
  }

  if (!isAwaitingPayment(order)) return null
  return (
    <div className="payment-panel">
      <h3 className="memory-heading">Paiement</h3>
      {notice && <div className={notice.tone === 'error' ? 'state-error' : 'pending-note'} role={notice.tone === 'error' ? 'alert' : 'status'}><Icon name="bell" size={16} /><span>{notice.text}</span></div>}
      {polling && <Loading label="Confirmation du paiement…" />}
      {!polling && action && <StripeForm action={action} onDone={(text) => { setAction(null); setNotice({ tone: 'info', text }); setPolling(true) }} />}
      {!polling && !action && <button type="button" className="primary-btn" onClick={() => void start()} disabled={busy} aria-busy={busy}>{busy ? 'Démarrage du paiement sécurisé…' : 'Payer en toute sécurité'} <Icon name="arrow" size={15} /></button>}
    </div>
  )
}
