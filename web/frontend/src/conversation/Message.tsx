import { asksForCount, asksForDate, CountAnswer, DateAnswer } from '@/conversation/Answers'
import { isReservable } from '@/commerce/offers'
import { NegotiateDialog } from '@/negotiation/NegotiateDialog'
import { useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { conversationStore, type Turn } from '@/conversation/store'
import { offersOf, type Offer } from '@/commerce/offers'
import { SupplierCard } from '@/commerce/SupplierCard'
import { sizeText } from '@/conversation/Composer'
import { Icon, type IconName } from '@/ui/Icon'
import { clock, Logo } from '@/ui/kit'
import { t, tn } from '@/lib/i18n'
import { useMobile } from '@/lib/useMobile'
import { OfferResults, ProgressList } from '@/mobile/Answer'
import { MIcon } from '@/mobile/icons'
import { Trezy } from '@/ui/Trezy'

/*
 * One message. A user message is a bubble (copy, edit and send again). An answer of Trezutil is its text (streamed
 * live while the model writes it, see StreamingAnswer; never replayed word by word afterwards), then what the runtime returned: what it did, the offers as small
 * cards, a question, a confirmation to give, an order or a booking created, quick replies. Nothing is added here.
 */

const technical = /required parameter|is missing|must not be empty|must be positive/i

/** Plain text with paragraphs and simple lists, as the runtime writes it. */
export function RichText({ text }: { text: string }) {
  const blocks: ReactNode[] = []
  let list: string[] = []
  const flush = () => {
    if (list.length) blocks.push(<ul key={`l${blocks.length}`}>{list.map((x, i) => <li key={i}>{x}</li>)}</ul>)
    list = []
  }
  text.split(/\n+/).map((x) => x.trim()).filter(Boolean).forEach((line) => {
    const item = line.match(/^(?:[-•*]|\d+[.)])\s+(.*)$/)
    if (item) { list.push(item[1]!); return }
    flush()
    blocks.push(<p key={`p${blocks.length}`}>{line}</p>)
  })
  flush()
  return <>{blocks}</>
}

function useCopy() {
  const [copied, setCopied] = useState(false)
  const copy = (text: string) => {
    void navigator.clipboard?.writeText(text).then(() => { setCopied(true); window.setTimeout(() => setCopied(false), 1600) }, () => { /* refused */ })
  }
  return { copied, copy }
}

export function UserMessage({ turn, conversationId, busy }: { turn: Turn; conversationId: string; busy: boolean }) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(turn.text)
  const { copied, copy } = useCopy()
  const mobile = useMobile()
  if (editing) {
    return (
      <div className="user-message editing">
        <label className="sr-only" htmlFor={`edit-${turn.id}`}>{t('Modifier le message')}</label>
        <textarea id={`edit-${turn.id}`} value={draft} autoFocus rows={3} onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); conversationStore.edit(conversationId, turn.id, draft); setEditing(false) } if (e.key === 'Escape') setEditing(false) }} />
        <p className="edit-note">{t('Les réponses qui suivaient ce message seront remplacées.')}</p>
        <div className="edit-actions">
          <button type="button" className="secondary-btn small-btn" onClick={() => { setDraft(turn.text); setEditing(false) }}>{t('Annuler')}</button>
          <button type="button" className="primary-btn small-btn" disabled={!draft.trim() || busy} onClick={() => { conversationStore.edit(conversationId, turn.id, draft); setEditing(false) }}>{t('Envoyer')}</button>
        </div>
      </div>
    )
  }
  return (
    <div className="user-turn">
      <div className="user-message" tabIndex={mobile ? 0 : undefined}>
        {turn.attachments?.length ? (
          <div className="turn-files">
            {turn.attachments.map((a) => <span key={`${a.name}-${a.size}`} title={t('Non transmis : le serveur n’accepte pas encore les fichiers')}><Icon name={a.type.startsWith('image/') ? 'image' : 'file'} size={14} />{a.name}<small>{sizeText(a.size)} {t('· non transmis')}</small></span>)}
          </div>
        ) : null}
        {turn.text}
      </div>
      <div className="message-actions user">
        <span className="turn-time">{clock(turn.at)}{turn.editedFrom ? t(' · modifié') : ''}</span>
        <button type="button" onClick={() => copy(turn.text)} aria-label={t('Copier le message')} title={t('Copier')}><Icon name={copied ? 'check' : 'copy'} size={15} /></button>
        {!busy && <button type="button" onClick={() => { setDraft(turn.text); setEditing(true) }} aria-label={t('Modifier le message')} title={t('Modifier')}><Icon name="edit" size={15} /></button>}
      </div>
      {turn.failed && (
        <div className="failed-note" role="alert">
          <span>{t('Non envoyé :')} {turn.failed.message}</span>
          <button type="button" className="secondary-btn small-btn" disabled={busy} onClick={() => conversationStore.retry(conversationId, turn.id)}>{t('Réessayer')}</button>
        </div>
      )}
    </div>
  )
}

/**
 * The answer while it is being written: the text received so far, as it arrives from the server, and the real step
 * the server reports. Nothing here is simulated: an empty text shows the step only.
 */
export function StreamingAnswer({ text, label }: { text: string; label: string }) {
  if (!text) return <Typing label={label} />
  return (
    <div className="assistant-message streaming" aria-busy="true">
      <div className="assistant-mark"><Logo compact /></div>
      <div className="message-content">
        <RichText text={text} />
        <p className="working small" role="status" aria-live="polite"><span className="stream-caret" aria-hidden="true" />{label}</p>
      </div>
    </div>
  )
}

export function Typing({ label }: { label: string }) {
  const mobile = useMobile()
  if (mobile) {
    return (
      <div className="mx-thinking" role="status" aria-live="polite"><Trezy pose="je-m-en-occupe" size={64} /><span /><span /><span /><em>{label}</em></div>
    )
  }
  return (
    <div className="assistant-message compact-response working-trezy" role="status" aria-live="polite">
      <Trezy pose="je-m-en-occupe" size={72} />
      <div className="message-content"><p className="working">{label}</p><div className="typing"><i /><i /><i /></div></div>
    </div>
  )
}

/** The picture of a suggested reply, from what it asks. */
function suggestionIcon(text: string): IconName {
  const s = text.toLowerCase()
  if (/r[ée]serv|book/.test(s)) return 'calendar'
  if (/ach[èe]t|buy|panier|cart/.test(s)) return 'cart'
  if (/command|order/.test(s)) return 'orders'
  if (/cher|cheap|prix|price/.test(s)) return 'wallet'
  if (/similaire|similar|r[ée]sultats|results/.test(s)) return 'layers'
  if (/date|aujourd|today|week/.test(s)) return 'clock'
  return 'message'
}

const TRACE: Record<string, [IconName, string]> = {
  SEARCH_LISTINGS: ['search', t('Recherche de fournisseurs terminée')],
  COMPARE_LISTINGS: ['layers', t('Comparaison des offres terminée')],
  CHECK_AVAILABILITY: ['check', t('Disponibilité vérifiée')],
  CHECK_INVENTORY: ['check', t('Stock vérifié')],
  CREATE_ORDER: ['orders', t('Commande préparée')],
  CREATE_RESERVATION: ['calendar', t('Réservation préparée')],
  GET_ORDER: ['orders', t('Commande consultée')],
  GET_RESERVATION: ['calendar', t('Réservation consultée')],
  GET_LISTING: ['file', t('Détails de l’offre')],
  GET_DELIVERY_STATUS: ['truck', t('Livraison suivie')],
  GET_PAYMENT_STATUS: ['wallet', t('Paiement suivi')],
}

interface AssistantProps {
  turn: Turn
  conversationId: string
  latest: boolean
  busy: boolean
  confirming: boolean
  signedIn: boolean
  signInHref: string
  operationId?: string
  openingOperation: boolean
  onSend: (text: string) => void
  onOpenOperation: () => void
  onCompare: (turnId: string, offer?: Offer) => void
  /** Opens the detail of one offer beside the conversation. */
  onOpenOffer: (turnId: string, offer: Offer, latest: boolean) => void
  /** The date the person last typed or picked: an end date comes after it. */
  lastDate?: string
}

export function AssistantMessage(p: AssistantProps) {
  const { turn, latest } = p
  const mobile = useMobile()
  const reply = turn.reply
  const [reading, setReading] = useState(false)
  const { copied, copy } = useCopy()
  const offers = reply ? offersOf(reply.result_items) : []
  const trace = reply ? TRACE[reply.action_type] : undefined
  const questions = reply?.missing_params.filter((q) => q.name !== 'listing_id') ?? []
  const confirmation = reply?.is_confirmation ? reply.confirmation : undefined
  const canRegenerate = latest && !p.busy && conversationStore.canRegenerate(p.conversationId, turn.id)
  const [negotiating, setNegotiating] = useState(false)
  const asked = reply?.is_clarification ? questions[0]?.name ?? '' : ''
  // A date or a number has its own picker below the question: nothing more to spell out.
  const worded = questions.filter((q) => !asksForDate(q.name) && !asksForCount(q.name))
  // The latest answer's offers get action buttons; earlier answers keep their comparison link.
  const actionable = offers.length > 0 && latest && !confirmation
  // Offers a price can be asked for: one currency, five at most.
  const currency = offers.find((o) => o.priceValue > 0)?.currency
  const negotiable = offers.filter((o) => o.priceValue > 0 && o.currency === currency).slice(0, 5)
  const reservable = offers.find(isReservable)

  const interrupted = turn.interrupted ? (
    <div className="interrupted-note" role="status">
      <span>{t('Réponse interrompue.')}</span>
      <button type="button" className="secondary-btn small-btn" disabled={reading || p.busy}
        onClick={() => { setReading(true); void conversationStore.completeInterrupted(p.conversationId, turn.id).finally(() => setReading(false)) }}>
        {reading ? 'Lecture…' : t('Afficher la réponse complète')}
      </button>
    </div>
  ) : null

  const extras = reply ? (
    <>
      {offers.length === 0 && reply.cards.length > 0 && (
        <div className="choice-list">
          {reply.cards.map((card) => (
            <button type="button" key={card.id} onClick={() => p.onSend(String(card.position || card.title))} disabled={!latest || p.busy}>
              {card.position > 0 && <b>{card.position}</b>}
              <span><strong>{card.title}</strong>{card.description && <small>{card.description}</small>}</span>
              <Icon name="arrow" size={14} />
            </button>
          ))}
        </div>
      )}

      {reply.is_clarification && worded.length > 0 && (
        <div className="insight question">
          <span><Icon name="message" size={17} /></span>
          <div>{worded.map((q) => <p key={q.name}>{q.prompt && !technical.test(q.prompt) ? q.prompt : t('Précise : {name}.', { name: q.name.replace(/_/g, ' ') })}</p>)}</div>
        </div>
      )}

      {latest && asked && asksForDate(asked) && <DateAnswer key={asked} param={asked} after={p.lastDate} disabled={p.busy} onSend={p.onSend} />}
      {latest && asked && asksForCount(asked) && <CountAnswer key={asked} param={asked} disabled={p.busy} onSend={p.onSend} />}

      {confirmation && (
        <div className={`approval-box ${turn.resolution ? 'resolved' : ''}`} role="group" aria-label={t('Confirmation requise')}>
          <Icon name="shield" />
          <div>
            <b>{turn.resolution ? t('Décision enregistrée') : t('Ton approbation est requise')}</b>
            {/* A summary written as a code (ACTION_REQUIRES_CONFIRMATION) is not shown to the person. */}
            <p>{confirmation.summary && !/^[A-Z0-9_]+$/.test(confirmation.summary) ? confirmation.summary : t('Confirme pour continuer. Rien ne se passe tant que tu ne l’as pas fait.')}</p>
            {turn.resolution ? (
              <p className="resolution"><Icon name={turn.resolution === 'confirmed' ? 'check' : 'x'} size={13} />
                {turn.resolution === 'confirmed' ? t('Tu as approuvé.') : turn.resolution === 'rejected' ? t('Tu as refusé. Rien n’a été modifié.') : t('Cette confirmation a expiré.')}
              </p>
            ) : latest ? (
              p.signedIn ? (
                <div className="approval-actions">
                  <button type="button" className="secondary-btn" disabled={p.confirming} onClick={() => void conversationStore.answer(p.conversationId, turn.id, false)}>{t('Refuser')}</button>
                  <button type="button" className="primary-btn" disabled={p.confirming} aria-busy={p.confirming} onClick={() => void conversationStore.answer(p.conversationId, turn.id, true)}>{p.confirming ? 'Envoi…' : t('Approuver')}</button>
                </div>
              ) : (
                <div className="approval-actions"><Link className="primary-btn" to={p.signInHref}>{t('Se connecter pour approuver')}</Link></div>
              )
            ) : null}
          </div>
        </div>
      )}

      {latest && (reply.refs.order_id || (reply.refs.reservation_id && reply.action_type === 'CREATE_RESERVATION')) && <Trezy pose="commande-reussie" size={112} className="trezy-success" />}

      {reply.refs.order_id && (
        <div className="activity success">
          <span className="activity-check"><Icon name="orders" size={13} /></span>
          <div><b>{t('Commande créée')}</b><small>{t('Elle est enregistrée dans tes achats.')}</small></div>
          <Link className="activity-link" to={`/purchases/${encodeURIComponent(reply.refs.order_id)}`}>{t('Voir la commande')} <Icon name="arrow" size={13} /></Link>
        </div>
      )}
      {reply.refs.reservation_id && reply.action_type === 'CREATE_RESERVATION' && (
        <div className="activity success">
          <span className="activity-check"><Icon name="calendar" size={13} /></span>
          <div><b>{t('Réservation enregistrée')}</b><small>{t('Retrouve-la dans tes achats et réservations.')}</small></div>
          <Link className="activity-link" to="/purchases?tab=bookings">{t('Voir')} <Icon name="arrow" size={13} /></Link>
        </div>
      )}

      {actionable && (
        <div className="action-bar" role="group" aria-label={t('Actions sur ces offres')}>
          {!mobile && offers.length > 1 && <button type="button" className="cta primary" onClick={() => p.onCompare(turn.id)}><Icon name="layers" size={15} />{t('Comparer')} {offers.length > 3 ? tn(offers.length, 'l’offre', 'les {n} offres') : t('ces offres')}</button>}
          {negotiable.length > 0 && <button type="button" className="cta" onClick={() => setNegotiating(true)}><Icon name="handshake" size={15} />{tn(negotiable.length, 'Négocier cette offre', 'Négocier ces {n} offres')}</button>}
          {reservable && <button type="button" className="cta" disabled={p.busy} onClick={() => p.onSend(t('Réserve le numéro {n}', { n: reservable.n }))}><Icon name="calendar" size={15} />{t('Réserver le n° {n} avec des dates', { n: reservable.n })}</button>}
        </div>
      )}
      {negotiating && <NegotiateDialog offers={negotiable} conversationId={p.conversationId} onClose={() => setNegotiating(false)} />}

      {latest && !confirmation && reply.suggestions.length > 0 && (
        <div className="quick-replies" role="group" aria-label={t('Réponses suggérées')}>
          {reply.suggestions.map((s) => <button type="button" key={s} onClick={() => p.onSend(s)} disabled={p.busy}><Icon name={suggestionIcon(s)} size={14} />{s}</button>)}
        </div>
      )}
    </>
  ) : null

  if (mobile) {
    const steps: { label: string; state: 'done' }[] = reply && trace ? [
      { label: t('Besoin analysé'), state: 'done' },
      { label: trace[1], state: 'done' },
      ...(reply.action_type === 'SEARCH_LISTINGS' ? [{ label: offers.length ? tn(offers.length, '{n} offre pertinente identifiée', '{n} offres pertinentes identifiées') : t('Aucune offre correspondante pour l’instant'), state: 'done' as const }] : []),
    ] : []
    return (
      <div className="assistant-message mx-answer">
        <div className="assistant-mark"><Logo compact /></div>
        <div className="message-content">
          <RichText text={turn.text} />
          {interrupted}
          {steps.length > 0 && <ProgressList steps={steps} />}
          {p.operationId && <Link className="mx-text-action" to={`/operations/${encodeURIComponent(p.operationId)}`}>{t('Voir la mission')} <MIcon name="chevron" size={15} /></Link>}
          {offers.length > 0 && <OfferResults offers={offers} onOpen={(o) => p.onOpenOffer(turn.id, o, latest)} onCompare={() => p.onCompare(turn.id)} />}
          {!p.operationId && offers.length > 0 && latest && (p.signedIn
            ? <button type="button" className="mx-text-action" onClick={p.onOpenOperation} disabled={p.openingOperation}>{p.openingOperation ? t('Ouverture…') : t('Suivre en mission')} <MIcon name="chevron" size={15} /></button>
            : <Link className="mx-text-action" to={p.signInHref}>{t('Se connecter pour suivre')} <MIcon name="chevron" size={15} /></Link>)}
          {extras}
          <div className="message-actions">
            <button type="button" onClick={() => copy(turn.text)} aria-label={t('Copier la réponse')}>{copied ? <MIcon name="check" size={16} /> : <Icon name="copy" size={15} />}</button>
            {canRegenerate && <button type="button" onClick={() => conversationStore.regenerate(p.conversationId, turn.id)} aria-label={t('Régénérer la réponse')}><Icon name="refresh" size={15} /></button>}
            <span className="turn-time">{clock(turn.at)}</span>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="assistant-message">
      <div className="assistant-mark"><Logo compact /></div>
      <div className="message-content">
        <RichText text={turn.text} />
        {interrupted}
        {reply && (
          <>
            {trace && (
              <div className="activity">
                <span className="activity-check"><Icon name="check" size={13} /></span>
                <div><b>{trace[1]}</b><small>{offers.length ? tn(offers.length, '{n} offre trouvée', '{n} offres trouvées') : reply.action_type === 'SEARCH_LISTINGS' ? t('Aucune offre correspondante pour l’instant') : t('Exécuté par le Commerce Action Runtime')} · {clock(turn.at)}</small></div>
                {p.operationId
                  ? <Link className="activity-link" to={`/operations/${encodeURIComponent(p.operationId)}`}>{t('Voir l’opération')} <Icon name="arrow" size={13} /></Link>
                  : offers.length > 0 && latest && (p.signedIn
                    ? <button type="button" onClick={p.onOpenOperation} disabled={p.openingOperation} aria-busy={p.openingOperation}>{p.openingOperation ? 'Ouverture…' : t('Suivre en opération')} <Icon name="arrow" size={13} /></button>
                    : <Link className="activity-link" to={p.signInHref}>{t('Se connecter pour suivre')} <Icon name="arrow" size={13} /></Link>)}
              </div>
            )}

            {offers.length > 0 && (
              <>
                <div className="supplier-stack">
                  {offers.slice(0, 3).map((o) => <SupplierCard key={o.id} offer={o} onReserve={latest && !p.busy && isReservable(o) ? () => p.onSend(t('Réserve le numéro {n}', { n: o.n })) : undefined} onOpen={() => p.onOpenOffer(turn.id, o, latest)} onAsk={latest ? () => p.onSend(t('Parle-moi de l’offre {n} : {title}', { n: o.n, title: o.title })) : undefined} />)}
                </div>
                {offers.length > 1 && !actionable && (
                  <button type="button" className="compare-link" onClick={() => p.onCompare(turn.id)}>
                    <Icon name="layers" size={15} />{t('Comparer')} {offers.length > 3 ? tn(offers.length, 'l’offre', 'les {n} offres') : t('ces offres')}
                  </button>
                )}
              </>
            )}

            {extras}
          </>
        )}
        {(
          <div className="message-actions">
            <button type="button" onClick={() => copy(turn.text)} aria-label={t('Copier la réponse')} title={t('Copier')}><Icon name={copied ? 'check' : 'copy'} size={15} /></button>
            {canRegenerate && <button type="button" onClick={() => conversationStore.regenerate(p.conversationId, turn.id)} aria-label={t('Régénérer la réponse')} title={t('Redemander (la recherche est relancée)')}><Icon name="refresh" size={15} /></button>}
          </div>
        )}
      </div>
    </div>
  )
}
