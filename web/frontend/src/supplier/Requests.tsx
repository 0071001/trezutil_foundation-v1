import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Link, useParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { isOpen, openOffer, supplierRequestsApi, type Negotiation } from '@/api/negotiations'
import { formatDateTime, formatMinor } from '@/lib/format'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { deadlineText, eventText, NegotiationError, PARTICIPANT } from '@/negotiation/labels'
import { Icon } from '@/ui/Icon'
import { Badge, Empty, ErrorNote, Loading } from '@/ui/kit'
import { t } from '@/lib/i18n'

/*
 * The supplier's portal for price requests: what buyers ask (never who they are, never their budget), the answer form,
 * the buyer's counter-offers, and the outcome. An offer sent here is binding until its validity date.
 */

function Shell({ title, back, children }: { title: string; back?: boolean; children: ReactNode }) {
  useEffect(() => { document.title = `${title} — Trezutil` }, [title])
  return (
    <div className="settings-page nego-page">
      <div className="settings-header page-head">
        <div>
          {back ? <Link className="eyebrow nego-back" to="/supplier/requests"><Icon name="back" size={13} />{t('DEMANDES DE PRIX')}</Link> : <p className="eyebrow">{t('ESPACE FOURNISSEUR')}</p>}
          <h1>{title}</h1>
          {!back && <p>{t('Les acheteurs Trezutil te demandent ton meilleur prix. Réponds avant l’échéance : l’acheteur choisit.')}</p>}
        </div>
        {!back && <Link className="secondary-btn small-btn" to="/supplier/offers">{t('Mes offres')}</Link>}
      </div>
      <div className="page-body">{children}</div>
    </div>
  )
}

export function SupplierRequests() {
  const { signedIn, user } = useSession()
  const list = useAsync(() => supplierRequestsApi.list(), [signedIn], signedIn)
  return (
    <Shell title={t('Demandes de prix')}>
      {user === null ? (
        <Empty icon="lock" title={t('Connecte-toi pour continuer')} action={<Link className="primary-btn" to="/login?returnTo=%2Fsupplier%2Frequests">{t('Se connecter')}</Link>}>{t('Les demandes de prix sont adressées à ton entreprise.')}</Empty>
      ) : list.loading && !list.data ? <Loading />
        : list.error ? <ErrorNote error={list.error} onRetry={list.reload} />
          : !list.data?.length ? (
            <Empty trezy="accueil" icon="handshake" title={t('Aucune demande de prix')} action={<Link className="primary-btn" to="/supplier/offers">{t('Publier une offre')}</Link>}>
              {t('Quand un acheteur demande ton meilleur prix sur une de tes offres, la demande arrive ici. Publie tes offres pour être trouvé.')}
            </Empty>
          ) : (
            <div className="list">
              {list.data.map((n) => {
                const p = n.participants[0]
                if (!p) return null
                const [label, tone] = PARTICIPANT[p.status] ?? [p.status, 'neutral']
                const counter = openOffer(n, p.id, 'buyer')
                const waiting = isOpen(n.status) && (p.status === 'invited' || p.status === 'countered')
                return (
                  <Link key={p.id} className="list-row" to={`/supplier/requests/${p.id}`}>
                    <span className="list-icon"><Icon name="handshake" size={17} /></span>
                    <span className="list-copy">
                      <b>{n.quantity} × {p.listing_title}</b>
                      <small>{counter ? t('Contre-proposition : {price} l’unité', { price: formatMinor(counter.unit_minor, n.currency) }) : t('Prix affiché : {price}', { price: formatMinor(p.list_unit_minor, n.currency) })}{waiting ? ` · ${deadlineText(n.deadline)}` : ''}</small>
                    </span>
                    <Badge tone={waiting ? 'amber' : tone}>{waiting ? t('À répondre') : label}</Badge>
                  </Link>
                )
              })}
            </div>
          )}
    </Shell>
  )
}

function AnswerForm({ n, onDone }: { n: Negotiation; onDone: (next: Negotiation) => void }) {
  const p = n.participants[0]!
  const last = openOffer(n, p.id, 'supplier')
  const counter = openOffer(n, p.id, 'buyer')
  const [price, setPrice] = useState('')
  const [days, setDays] = useState(last?.delivery_days != null ? String(last.delivery_days) : '')
  const [valid, setValid] = useState(7)
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const value = Number(price.replace(',', '.'))
  const run = async (fn: () => Promise<Negotiation>) => {
    setBusy(true)
    setError(null)
    try { onDone(await fn()) } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }
  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!(value > 0)) return
    void run(() => supplierRequestsApi.offer(p.id, { unit_price: value, delivery_days: days.trim() === '' ? null : Math.max(0, Math.floor(Number(days))), valid_days: valid, note }))
  }
  return (
    <section className="nego-answer">
      {counter && (
        <div className="nego-counter">
          <p><b>{t('L’acheteur propose {price} l’unité', { price: formatMinor(counter.unit_minor, n.currency) })}</b>{last ? ` · ${t('ton offre : {price}', { price: formatMinor(last.unit_minor, n.currency) })}` : ''}</p>
          <p className="nego-help">{t('Accepter envoie une offre à ce prix. L’acheteur confirme ensuite ; rien n’est commandé sans lui.')}</p>
          <button type="button" className="primary-btn small-btn" disabled={busy} onClick={() => void run(() => supplierRequestsApi.acceptCounter(p.id))}>{t('Accepter {price} l’unité', { price: formatMinor(counter.unit_minor, n.currency) })}</button>
        </div>
      )}
      <form className="nego-form-inline" onSubmit={submit}>
        <h2 className="nego-h2">{last ? t('Faire une nouvelle offre') : t('Ton offre')}</h2>
        <div className="nego-grid">
          <label>{t('Prix par unité ({currency})', { currency: n.currency })}<input inputMode="decimal" value={price} onChange={(e) => setPrice(e.target.value)} required /></label>
          <label>{t('Livraison (jours)')}<input type="number" min={0} max={365} value={days} onChange={(e) => setDays(e.target.value)} placeholder={t('facultatif')} /></label>
          <label>{t('Valable')}
            <select value={valid} onChange={(e) => setValid(Number(e.target.value))}>{[3, 7, 14, 30].map((d) => <option key={d} value={d}>{t('{n} jours', { n: d })}</option>)}</select>
          </label>
        </div>
        <label className="nego-note-field">{t('Message (conditions, garantie…)')}<textarea value={note} maxLength={1000} rows={3} onChange={(e) => setNote(e.target.value)} /></label>
        {value > 0 && <p className="nego-help">{t('Total pour {qty} : {total}', { qty: n.quantity, total: formatMinor(Math.round(value * 100) * n.quantity, n.currency) })}</p>}
        <div className="modal-actions">
          <button type="button" className="secondary-btn" disabled={busy} onClick={() => { const reason = window.prompt(t('Décliner cette demande ? Tu peux indiquer pourquoi (facultatif).')); if (reason !== null) void run(() => supplierRequestsApi.decline(p.id, reason)) }}>{t('Décliner')}</button>
          <button type="submit" className="primary-btn" disabled={busy || !(value > 0)} aria-busy={busy}>{busy ? t('Envoi…') : t('Envoyer mon offre')}</button>
        </div>
      </form>
      {error && <NegotiationError error={error} />}
    </section>
  )
}

export function SupplierRequest() {
  const { pid = '' } = useParams()
  const { signedIn } = useSession()
  const loaded = useAsync(() => supplierRequestsApi.get(pid), [pid, signedIn], signedIn)
  const [n, setN] = useState<Negotiation | null>(null)
  useEffect(() => { if (loaded.data) setN(loaded.data) }, [loaded.data])
  if (!n) return <Shell title={t('Demande de prix')} back>{loaded.error ? <ErrorNote error={loaded.error} onRetry={loaded.reload} /> : <Loading />}</Shell>
  const p = n.participants[0]
  if (!p) return <Shell title={t('Demande de prix')} back><Empty icon="handshake" title={t('Demande introuvable')} /></Shell>
  const [label, tone] = PARTICIPANT[p.status] ?? [p.status, 'neutral']
  const canAnswer = isOpen(n.status) && (p.status === 'invited' || p.status === 'countered' || p.status === 'responded') && n.deadline * 1000 > Date.now()
  return (
    <Shell title={`${n.quantity} × ${p.listing_title}`} back>
      <section className="nego-head">
        <div className="nego-status"><Badge tone={tone}>{label}</Badge>{canAnswer && <span>{deadlineText(n.deadline)}</span>}</div>
        <p className="nego-request">{t('Un acheteur Trezutil te demande ton meilleur prix pour {qty} × « {title} ». Ton prix affiché : {price} l’unité.', { qty: n.quantity, title: p.listing_title, price: formatMinor(p.list_unit_minor, n.currency) })}</p>
        <dl className="nego-mandate">
          <div><dt>{t('Répondre avant le')}</dt><dd>{formatDateTime(n.deadline)}</dd></div>
          <div><dt>{t('Tours de négociation')}</dt><dd>{t('{n} au plus', { n: n.mandate.max_rounds + 1 })}</dd></div>
        </dl>
        {p.status === 'accepted' && <p className="nego-callout good"><Icon name="orders" size={15} />{t('L’acheteur a retenu ton offre : la commande est créée, en attente de son paiement.')}</p>}
        {p.status === 'rejected' && <p className="nego-callout">{t('L’acheteur n’a pas retenu ton offre, ou a retiré sa demande.')}</p>}
        {p.status === 'responded' && isOpen(n.status) && <p className="nego-callout">{t('Ton offre est envoyée. L’acheteur décide ; tu peux encore l’améliorer avant l’échéance.')}</p>}
      </section>
      {canAnswer && <AnswerForm n={n} onDone={setN} />}
      <section>
        <h2 className="nego-h2">{t('Historique')}</h2>
        <ol className="nego-journal">
          {[...n.events].reverse().map((e, i) => <li key={i}><span className={`nego-dot ${e.actor}`} /><div><p>{eventText(n, e, true)}</p><small>{formatDateTime(e.created_at)}</small></div></li>)}
        </ol>
      </section>
    </Shell>
  )
}
