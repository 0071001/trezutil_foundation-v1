import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { isOpen, negotiationsApi, openOffer, type Negotiation, type NegotiationParticipant } from '@/api/negotiations'
import { formatDateTime, formatMinor } from '@/lib/format'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { deadlineText, eventText, NegotiationError, PARTICIPANT, STATUS } from '@/negotiation/labels'
import { Icon } from '@/ui/Icon'
import { Badge, Empty, ErrorNote, Loading } from '@/ui/kit'
import { t, tn } from '@/lib/i18n'

/*
 * The buyer's negotiations: the list, then one negotiation with each supplier's offer side by side, what Trezutil did
 * on the buyer's behalf, and the buyer's decisions (accept, counter, turn down, withdraw). Accepting creates the order
 * at the agreed price; nothing else does.
 */

function Shell({ title, lede, children, back }: { title: string; lede?: string; children: React.ReactNode; back?: boolean }) {
  useEffect(() => { document.title = `${title} — Trezutil` }, [title])
  return (
    <div className="settings-page nego-page">
      <div className="settings-header page-head">
        <div>
          {back ? <Link className="eyebrow nego-back" to="/negotiations"><Icon name="back" size={13} />{t('NÉGOCIATIONS')}</Link> : <p className="eyebrow">{t('ESPACE COMMERCIAL')}</p>}
          <h1>{title}</h1>{lede && <p>{lede}</p>}
        </div>
      </div>
      <div className="page-body">{children}</div>
    </div>
  )
}

function SignIn() {
  return (
    <Empty icon="lock" title={t('Connecte-toi pour continuer')} action={<Link className="primary-btn" to={`/login?returnTo=${encodeURIComponent(window.location.pathname)}`}>{t('Se connecter')}</Link>}>
      {t('Tes négociations sont liées à ton compte.')}
    </Empty>
  )
}

/** Why a WhatsApp message to a supplier did not leave, in words. */
function waReason(error: string): string {
  if (/template/i.test(error)) return t('Les messages WhatsApp ne sont pas partis : le modèle de message n’est pas encore approuvé par Meta.')
  if (/phone/i.test(error)) return t('Les messages WhatsApp ne sont pas partis : le fournisseur n’a pas de numéro.')
  return t('Les messages WhatsApp ne sont pas partis.')
}

export function NegotiationsList() {
  const { signedIn, user } = useSession()
  const list = useAsync(() => negotiationsApi.list(), [signedIn], signedIn)
  return (
    <Shell title={t('Négociations')} lede={t('Les demandes de prix envoyées aux fournisseurs pour toi, dans les limites que tu as fixées.')}>
      {user === null ? <SignIn />
        : list.loading && !list.data ? <Loading />
          : list.error ? <ErrorNote error={list.error} onRetry={list.reload} />
            : !list.data?.length ? (
              <Empty trezy="accueil" icon="handshake" title={t('Aucune négociation')}>
                {t('Ouvre une offre dans une conversation et choisis « Négocier » : Trezutil demande le meilleur prix au fournisseur, sans révéler ton plafond.')}
              </Empty>
            ) : (
              <div className="list">
                {list.data.map((n) => {
                  const [label, tone] = STATUS[n.status] ?? [n.status, 'neutral']
                  const offers = n.participants.map((p) => openOffer(n, p.id, 'supplier')).filter(Boolean)
                  const best = offers.length ? Math.min(...offers.map((o) => o!.unit_minor)) : 0
                  return (
                    <Link key={n.id} className="list-row" to={`/negotiations/${n.id}`}>
                      <span className="list-icon"><Icon name="handshake" size={17} /></span>
                      <span className="list-copy">
                        <b>{n.title}</b>
                        <small>{tn(n.participants.length, '{n} fournisseur', '{n} fournisseurs')} · {n.quantity} × · {best ? t('meilleure offre {price}', { price: formatMinor(best, n.currency) }) : t('pas encore d’offre')}{isOpen(n.status) && n.status !== 'awaiting_decision' ? ` · ${deadlineText(n.deadline)}` : ''}</small>
                      </span>
                      <Badge tone={tone}>{label}</Badge>
                    </Link>
                  )
                })}
              </div>
            )}
    </Shell>
  )
}

function SupplierCard({ n, p, onChange }: { n: Negotiation; p: NegotiationParticipant; onChange: (next: Negotiation) => void }) {
  // The supplier's open offer, or the one the buyer accepted.
  const offer = openOffer(n, p.id, 'supplier') ?? n.offers.find((o) => o.id === n.accepted_offer_id && o.participant_id === p.id)
  const counter = openOffer(n, p.id, 'buyer')
  const accepted = n.accepted_offer_id && n.offers.find((o) => o.id === n.accepted_offer_id)?.participant_id === p.id
  const [label, tone] = PARTICIPANT[p.status] ?? [p.status, 'neutral']
  const [mode, setMode] = useState<'' | 'counter' | 'accept'>('')
  const [price, setPrice] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const open = isOpen(n.status) && (p.status === 'invited' || p.status === 'responded' || p.status === 'countered')
  const meetsTarget = offer && offer.unit_minor <= n.mandate.target_unit_minor
  const aboveCeiling = offer && offer.unit_minor > n.mandate.max_unit_minor
  const roundsLeft = p.round <= n.mandate.max_rounds
  const expired = offer?.valid_until ? offer.valid_until * 1000 < Date.now() : false
  const run = async (fn: () => Promise<Negotiation>) => {
    setBusy(true)
    setError(null)
    try { onChange(await fn()); setMode('') } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }
  return (
    <article className={`nego-supplier${accepted ? ' chosen' : ''}`} aria-label={t('Offre de {supplier}', { supplier: p.supplier })}>
      <header>
        <div><b>{p.supplier}</b><small>{p.listing_title}</small></div>
        <Badge tone={tone}>{label}</Badge>
      </header>
      <dl className="nego-figures">
        <div><dt>{t('Prix affiché')}</dt><dd>{formatMinor(p.list_unit_minor, n.currency)}</dd></div>
        <div><dt>{t('Son offre')}</dt><dd className={meetsTarget ? 'good' : ''}>{offer ? formatMinor(offer.unit_minor, n.currency) : '—'}</dd></div>
        <div><dt>{t('Contre-proposition')}</dt><dd>{counter ? formatMinor(counter.unit_minor, n.currency) : '—'}</dd></div>
      </dl>
      {offer && (
        <p className="nego-terms">
          {t('Total : {total}', { total: formatMinor(offer.unit_minor * offer.quantity, n.currency) })}
          {offer.delivery_days !== null && ` · ${tn(offer.delivery_days, 'livraison en {n} jour', 'livraison en {n} jours')}`}
          {offer.valid_until && ` · ${expired ? t('offre expirée') : t('valable jusqu’au {date}', { date: formatDateTime(offer.valid_until) })}`}
          {offer.note && <><br /><q>{offer.note}</q></>}
        </p>
      )}
      {offer && aboveCeiling && open && <p className="nego-warn"><Icon name="shield" size={13} />{t('Au-dessus de ton plafond.')}</p>}
      {open && (
        <div className="nego-actions">
          {offer && !expired && <button type="button" className="primary-btn small-btn" onClick={() => setMode('accept')} disabled={busy}>{t('Accepter')}</button>}
          {offer && roundsLeft && <button type="button" className="secondary-btn small-btn" onClick={() => { setMode('counter'); setPrice('') }} disabled={busy}>{t('Contre-proposer')}</button>}
          <button type="button" className="secondary-btn small-btn" onClick={() => { if (window.confirm(t('Refuser l’offre de {supplier} ? Il en sera informé.', { supplier: p.supplier }))) void run(() => negotiationsApi.reject(n.id, p.id)) }} disabled={busy}>{t('Refuser')}</button>
        </div>
      )}
      {mode === 'counter' && offer && (
        <form className="nego-inline" onSubmit={(e) => { e.preventDefault(); void run(() => negotiationsApi.counter(n.id, p.id, Number(price.replace(',', '.')))) }}>
          <label>{t('Ton prix par unité ({currency})', { currency: n.currency })}<input inputMode="decimal" value={price} onChange={(e) => setPrice(e.target.value)} autoFocus required /></label>
          <button type="submit" className="primary-btn small-btn" disabled={busy || !(Number(price.replace(',', '.')) > 0)}>{t('Envoyer')}</button>
          <button type="button" className="secondary-btn small-btn" onClick={() => setMode('')}>{t('Annuler')}</button>
        </form>
      )}
      {mode === 'accept' && offer && (
        <div className="approval-box" role="group" aria-label={t('Confirmer l’offre')}>
          <Icon name="shield" />
          <div>
            <b>{t('Accepter {qty} × à {price} ?', { qty: offer.quantity, price: formatMinor(offer.unit_minor, n.currency) })}</b>
            <p>{t('Une commande de {total} est créée chez {supplier}, en attente de paiement. Les autres fournisseurs sont informés que leur offre n’est pas retenue.', { total: formatMinor(offer.unit_minor * offer.quantity, n.currency), supplier: p.supplier })}</p>
            <div className="approval-actions">
              <button type="button" className="secondary-btn" onClick={() => setMode('')} disabled={busy}>{t('Annuler')}</button>
              <button type="button" className="primary-btn" onClick={() => void run(() => negotiationsApi.accept(n.id, offer.id))} disabled={busy} aria-busy={busy}>{busy ? t('Envoi…') : t('Accepter et commander')}</button>
            </div>
          </div>
        </div>
      )}
      {error && <NegotiationError error={error} />}
    </article>
  )
}

export function NegotiationView() {
  const { id = '' } = useParams()
  const { signedIn, user } = useSession()
  const loaded = useAsync(() => negotiationsApi.get(id), [id, signedIn], signedIn)
  const [n, setN] = useState<Negotiation | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  useEffect(() => { if (loaded.data) setN(loaded.data) }, [loaded.data])
  // An open negotiation follows the suppliers' answers.
  useEffect(() => {
    if (!n || !isOpen(n.status)) return
    const timer = window.setInterval(() => { void negotiationsApi.get(id).then(setN).catch(() => {}) }, 20000)
    return () => window.clearInterval(timer)
  }, [id, n?.status]) // eslint-disable-line react-hooks/exhaustive-deps

  if (user === null) return <Shell title={t('Négociation')} back><SignIn /></Shell>
  if (!n) return <Shell title={t('Négociation')} back>{loaded.error ? <ErrorNote error={loaded.error} onRetry={loaded.reload} /> : <Loading />}</Shell>
  const [label, tone] = STATUS[n.status] ?? [n.status, 'neutral']
  const whatsapp = n.messages.filter((m) => m.channel === 'whatsapp')
  const waSent = whatsapp.filter((m) => m.state === 'sent').length
  const waSkipped = whatsapp.filter((m) => m.state === 'skipped' || m.state === 'failed').length
  const cancel = async () => {
    if (!window.confirm(t('Retirer cette demande de prix ? Les fournisseurs seront informés.'))) return
    setError(null)
    try { setN(await negotiationsApi.cancel(n.id)) } catch (err) { setError(toApiError(err)) }
  }
  // The choice was recorded but its order was not written (the server stopped in between): the buyer finishes it.
  const finish = async () => {
    setError(null)
    try { setN(await negotiationsApi.accept(n.id, n.accepted_offer_id)) } catch (err) { setError(toApiError(err)) }
  }
  return (
    <Shell title={n.title} back>
      <section className="nego-head">
        <div className="nego-status"><Badge tone={tone}>{label}</Badge>{isOpen(n.status) && n.status !== 'awaiting_decision' && <span>{deadlineText(n.deadline)}</span>}</div>
        <dl className="nego-mandate">
          <div><dt>{t('Quantité')}</dt><dd>{n.quantity}</dd></div>
          <div><dt>{t('Prix cible')}</dt><dd>{formatMinor(n.mandate.target_unit_minor, n.currency)}</dd></div>
          <div><dt>{t('Plafond (privé)')}</dt><dd>{formatMinor(n.mandate.max_unit_minor, n.currency)}</dd></div>
          <div><dt>{t('Contre-propositions')}</dt><dd>{n.mandate.auto_counter ? t('par Trezutil, {n} au plus', { n: n.mandate.max_rounds }) : t('par toi seulement')}</dd></div>
          <div><dt>{t('Échéance')}</dt><dd>{formatDateTime(n.deadline)}</dd></div>
        </dl>
        {n.status === 'awaiting_decision' && <p className="nego-callout"><Icon name="check" size={15} />{t('Au moins une offre attend ta décision. Rien n’est commandé tant que tu n’as pas accepté.')}</p>}
        {n.order_id && <p className="nego-callout good"><Icon name="orders" size={15} />{t('Commande créée au prix négocié.')} <Link to={`/purchases/${encodeURIComponent(n.order_id)}`}>{t('Voir et payer la commande')}</Link></p>}
        {n.status === 'accepted' && !n.order_id && (
          <p className="nego-callout">{t('Ton choix est enregistré, mais la commande n’est pas encore créée.')} <button type="button" className="primary-btn small-btn" onClick={() => void finish()}>{t('Finaliser la commande')}</button></p>
        )}
        {isOpen(n.status) && <button type="button" className="secondary-btn small-btn" onClick={() => void cancel()}>{t('Retirer la demande')}</button>}
        {error && <NegotiationError error={error} />}
      </section>

      <section>
        <h2 className="nego-h2">{t('Les offres')}</h2>
        <div className="nego-suppliers">{n.participants.map((p) => <SupplierCard key={p.id} n={n} p={p} onChange={setN} />)}</div>
      </section>

      <section>
        <h2 className="nego-h2">{t('Ce qui s’est passé')}</h2>
        <ol className="nego-journal">
          {[...n.events].reverse().map((e, i) => (
            <li key={i}><span className={`nego-dot ${e.actor}`} /><div><p>{eventText(n, e)}</p><small>{formatDateTime(e.created_at)}</small></div></li>
          ))}
        </ol>
        <p className="nego-channels">
          {t('Les fournisseurs voient tes demandes dans leur espace Trezutil.')}{' '}
          {whatsapp.length === 0 ? t('WhatsApp n’est pas activé pour les fournisseurs.')
            : waSent > 0 ? tn(waSent, '{n} message envoyé aussi sur WhatsApp.', '{n} messages envoyés aussi sur WhatsApp.')
              : waSkipped > 0 ? waReason(whatsapp.find((m) => m.last_error)?.last_error ?? '') : t('Messages WhatsApp en attente d’envoi.')}
        </p>
      </section>
    </Shell>
  )
}
