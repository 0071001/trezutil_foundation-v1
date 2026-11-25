import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { toApiError } from '@/api/client'
import { ordersApi, paymentsApi, type OrderSummary, type Payer, type PayNetwork } from '@/api/commerce'
import { isStripeAction, loadStripe, type ClientAction, type StripeClient, type StripeElements } from '@/api/stripe'
import { formatMinor, formatMoney } from '@/lib/format'
import { useSession } from '@/lib/session'
import { Icon } from '@/ui/Icon'
import { Loading, newId } from '@/ui/kit'
import { t } from '@/lib/i18n'

/*
 * Paying an order (POST /payments). Trezutil's SasPay account takes mobile money in DR Congo (M-Pesa, Airtel Money,
 * Orange Money: the request is pushed to the payer's phone, who approves it with their PIN) and cards (USD, on
 * SasPay's page). A seller with its own Stripe account gets Stripe's card form instead. The amount is always the
 * order's, in the order's currency: Trezutil never converts what is paid.
 *
 * The outcome is read back from the server (which asks the provider), then the order, until it leaves "awaiting
 * payment". Nothing is said to be paid before the provider says so.
 */

const SETTLED = new Set(['paid', 'confirmed', 'processing', 'ready_for_fulfillment', 'fulfilled', 'completed', 'cancelled', 'refunded'])
const PENDING_KEY = (orderId: string) => `trezutil_payment_${orderId}`

const NETWORKS: { id: PayNetwork; label: string; hint: string }[] = [
  { id: 'vodacom_cd', label: 'M-Pesa', hint: 'Vodacom' },
  { id: 'airtel_cd', label: 'Airtel Money', hint: 'Airtel' },
  { id: 'orange_cd', label: 'Orange Money', hint: 'Orange' },
  { id: 'card', label: t('Carte'), hint: 'Visa, Mastercard' },
]

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
      .catch((err: unknown) => setError(err instanceof Error ? err.message : t('Le formulaire de paiement n’a pas pu être chargé.')))
    return () => { cancelled = true; element?.destroy() }
  }, [action.client_secret, action.publishable_key])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!stripeRef.current || busy) return
    setBusy(true)
    setError(null)
    const { stripe, elements } = stripeRef.current
    const result = await stripe.confirmPayment({ elements, confirmParams: { return_url: window.location.href.split('?')[0] }, redirect: 'if_required' })
    setBusy(false)
    if (result.error) { setError(result.error.message ?? t('Le paiement n’a pas abouti.')); return }
    onDone(result.paymentIntent?.status === 'processing' ? t('Paiement en cours de traitement : la commande sera mise à jour sous peu.') : t('Paiement reçu : confirmation de la commande…'))
  }

  return (
    <form className="payment-form" onSubmit={(e) => void submit(e)}>
      <div ref={mountRef} className="stripe-element" aria-label={t('Coordonnées de carte')} />
      {!ready && !error && <Loading label={t('Chargement du formulaire de paiement sécurisé…')} />}
      {error && <p className="form-error" role="alert">{error}</p>}
      <button type="submit" className="primary-btn" disabled={!ready || busy} aria-busy={busy}>{busy ? t('Paiement…') : t('Payer maintenant')} <Icon name="arrow" size={15} /></button>
      <p className="muted-line"><Icon name="lock" size={13} /> {t('Les données de carte sont traitées par Stripe et jamais conservées par Trezutil.')}</p>
    </form>
  )
}

function splitName(name: string): [string, string] {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length < 2) return [parts[0] ?? '', '']
  return [parts[0]!, parts.slice(1).join(' ')]
}

export function PaymentPanel({ order, onUpdated }: { order: OrderSummary; onUpdated: (order: OrderSummary) => void }) {
  const { user } = useSession()
  const [first, last] = splitName(user?.display_name ?? '')
  const [network, setNetwork] = useState<PayNetwork>(order.currency === 'USD' ? 'card' : 'vodacom_cd')
  const [payer, setPayer] = useState<Payer>({ phone: user?.phone ?? '', email: user?.email ?? '', first_name: first, last_name: last })
  const [action, setAction] = useState<ClientAction | null>(null)
  const [notice, setNotice] = useState<{ tone: 'info' | 'error'; text: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [waiting, setWaiting] = useState<string | null>(null) // payment id being confirmed
  const mobile = network !== 'card'

  const settle = useCallback((o: OrderSummary) => { try { sessionStorage.removeItem(PENDING_KEY(order.id)) } catch { /* ignore */ } onUpdated(o) }, [onUpdated, order.id])

  // Back from SasPay's or the bank's page: read the payment started before leaving.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const stripeStatus = params.get('redirect_status')
    if (!params.has('payment') && !stripeStatus) return
    window.history.replaceState(null, '', window.location.pathname)
    if (stripeStatus && stripeStatus !== 'succeeded') { setNotice({ tone: 'error', text: t('Le paiement n’a pas abouti. Tu peux réessayer.') }); return }
    let id: string | null = null
    try { id = sessionStorage.getItem(PENDING_KEY(order.id)) } catch { /* ignore */ }
    setNotice({ tone: 'info', text: t('Confirmation du paiement…') })
    setWaiting(id ?? 'order')
  }, [order.id])

  // Read the payment back (the server asks SasPay), then the order, for up to about four minutes.
  useEffect(() => {
    if (!waiting) return
    let attempts = 0
    let stopped = false
    const tick = async () => {
      attempts += 1
      try {
        if (waiting !== 'order') {
          const p = await paymentsApi.status(waiting, order.organization_id)
          const s = p.status.toUpperCase()
          if (s === 'FAILED' || s === 'CANCELLED' || s === 'EXPIRED') {
            stopped = true
            setWaiting(null)
            setNotice({ tone: 'error', text: t('Le paiement n’a pas abouti : il a été refusé, annulé ou n’a pas été validé à temps. Rien n’a été débité ; tu peux réessayer.') })
            return
          }
        }
        const o = await ordersApi.get(order.id)
        if (SETTLED.has(o.status)) { stopped = true; setWaiting(null); setNotice(null); settle(o); return }
      } catch { /* try again */ }
      if (attempts >= 60) {
        stopped = true
        setWaiting(null)
        setNotice({ tone: 'info', text: t('Le paiement est encore en cours de confirmation. Recharge la page dans un instant.') })
      }
    }
    const timer = window.setInterval(() => { if (!stopped) void tick() }, 4000)
    return () => window.clearInterval(timer)
  }, [waiting, order.id, order.organization_id, settle])

  const start = async (event: FormEvent) => {
    event.preventDefault()
    if (busy) return
    setBusy(true)
    setNotice(null)
    try {
      const payment = await paymentsApi.payOrder(order, newId(), network, payer)
      const act = payment.client_action
      if (isStripeAction(act)) { setAction(act); return }
      if (payment.status?.toUpperCase() === 'FAILED') {
        setNotice({ tone: 'error', text: mobile && order.currency === 'USD'
          ? t('Ce paiement mobile money en USD n’a pas été accepté. Tu peux payer par carte.')
          : t('Le paiement a été refusé. Vérifie le numéro et le réseau, puis réessaie.') })
        return
      }
      try { sessionStorage.setItem(PENDING_KEY(order.id), payment.payment_id) } catch { /* ignore */ }
      if (act?.checkout_url) { window.location.assign(act.checkout_url); return }
      const charged = act?.charged_amount && Number(act.fee_amount) > 0
        ? ' ' + t('Montant débité : {charged} (dont {fee} de frais SasPay).', { charged: formatMoney(Number(act.charged_amount), order.currency), fee: formatMoney(Number(act.fee_amount), order.currency) })
        : ''
      setNotice({ tone: 'info', text: (act?.instructions || t('Une demande de paiement a été envoyée sur ton téléphone. Valide-la avec ton code PIN.')) + charged })
      setWaiting(payment.payment_id)
    } catch (err) {
      const e = toApiError(err)
      setNotice(e.code === 'PAYMENT_PROVIDER_UNAVAILABLE'
        ? { tone: 'info', text: t('Le paiement en ligne n’est pas encore activé pour ce vendeur. Ta commande est enregistrée : rien n’a été débité.') }
        : { tone: 'error', text: mobile && order.currency === 'USD'
          ? t('Ce paiement mobile money en USD n’a pas pu démarrer. Tu peux payer par carte.')
          : e.message || t('Le paiement n’a pas pu démarrer.') })
    } finally {
      setBusy(false)
    }
  }

  if (!isAwaitingPayment(order)) return null
  const field = (k: keyof Payer) => ({ value: payer[k], onChange: (e: { target: { value: string } }) => setPayer((p) => ({ ...p, [k]: e.target.value })) })
  const cardOnlyUSD = network === 'card' && order.currency !== 'USD'
  // SasPay's minimum for a mobile money payment in Congolese francs (seen in production, 2026-11-25).
  const belowMinimum = mobile && order.currency === 'CDF' && order.total_minor < 79900
  return (
    <div className="payment-panel">
      <h3 className="memory-heading">{t('Paiement')}</h3>
      {notice && <div className={notice.tone === 'error' ? 'state-error' : 'pending-note'} role={notice.tone === 'error' ? 'alert' : 'status'}><Icon name="bell" size={16} /><span>{notice.text}</span></div>}
      {waiting && <Loading label={t('En attente de la confirmation du paiement…')} />}
      {!waiting && action && <StripeForm action={action} onDone={(text) => { setAction(null); setNotice({ tone: 'info', text }); setWaiting('order') }} />}
      {!waiting && !action && (
        <form className="payment-form" onSubmit={(e) => void start(e)}>
          <div className="pay-networks" role="radiogroup" aria-label={t('Moyen de paiement')}>
            {NETWORKS.map((n) => (
              <button type="button" key={n.id} role="radio" aria-checked={network === n.id} className={network === n.id ? 'on' : ''} onClick={() => setNetwork(n.id)}>
                <b>{n.label}</b><small>{n.hint}</small>
              </button>
            ))}
          </div>
          {mobile && <label>{t('Numéro mobile money')}<input type="tel" inputMode="tel" autoComplete="tel" placeholder="+243 81 234 5678" required {...field('phone')} /></label>}
          <div className="pay-names">
            <label>{t('Prénom')}<input autoComplete="given-name" required {...field('first_name')} /></label>
            <label>{t('Nom')}<input autoComplete="family-name" required {...field('last_name')} /></label>
          </div>
          <label>{t('E-mail')} <small>{t('pour le reçu de paiement')}</small><input type="email" autoComplete="email" required {...field('email')} /></label>
          {cardOnlyUSD && <p className="form-error" role="alert">{t('Le paiement par carte se fait en USD uniquement ; cette commande est en {currency}. Choisis le mobile money.', { currency: order.currency })}</p>}
          {belowMinimum && <p className="form-error" role="alert">{t('Le mobile money demande au moins 799 FC par paiement.')}</p>}
          <button type="submit" className="primary-btn" disabled={busy || cardOnlyUSD || belowMinimum} aria-busy={busy}>
            {busy ? t('Démarrage du paiement sécurisé…') : t('Payer {amount}', { amount: formatMinor(order.total_minor, order.currency) })} <Icon name="arrow" size={15} />
          </button>
          <p className="muted-line"><Icon name="lock" size={13} /> {mobile
            ? t('Tu recevras une demande de paiement sur ton téléphone, à valider avec ton code PIN. Trezutil ne connaît jamais ton code. Des frais SasPay peuvent s’ajouter au montant débité : ils sont affichés sur ton téléphone avant que tu valides.')
            : t('La carte est saisie sur la page sécurisée de SasPay ; Trezutil ne voit jamais ses données.')}</p>
        </form>
      )}
    </div>
  )
}
