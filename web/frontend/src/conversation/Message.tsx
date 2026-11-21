import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { conversationStore, type Turn } from '@/conversation/store'
import { offersOf, type Offer } from '@/commerce/offers'
import { SupplierCard } from '@/commerce/SupplierCard'
import { sizeText } from '@/conversation/Composer'
import { Icon, type IconName } from '@/ui/Icon'
import { clock, Logo } from '@/ui/kit'

/*
 * One message. A user message is a bubble (copy, edit and send again). An answer of Trezutil is the runtime's text,
 * shown progressively when it has just arrived, then what the runtime returned: what it did, the offers as small
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

/** Reveals an answer that has just arrived word by word (the server sends it whole: this is presentation only). */
function useReveal(turn: Turn): { shown: string; done: boolean } {
  const fresh = !!conversationStore.get().fresh[turn.id]
  const reduce = typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  const words = useRef(turn.text.split(/(\s+)/))
  const [count, setCount] = useState(() => (fresh && !reduce ? 0 : words.current.length))
  useEffect(() => {
    if (count >= words.current.length) { conversationStore.seen(turn.id); return }
    const step = Math.max(2, Math.ceil(words.current.length / 40))
    const timer = window.setTimeout(() => setCount((c) => Math.min(words.current.length, c + step)), 24)
    return () => window.clearTimeout(timer)
  }, [count, turn.id])
  const done = count >= words.current.length
  return { shown: done ? turn.text : words.current.slice(0, count).join(''), done }
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
  if (editing) {
    return (
      <div className="user-message editing">
        <label className="sr-only" htmlFor={`edit-${turn.id}`}>Modifier le message</label>
        <textarea id={`edit-${turn.id}`} value={draft} autoFocus rows={3} onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); conversationStore.edit(conversationId, turn.id, draft); setEditing(false) } if (e.key === 'Escape') setEditing(false) }} />
        <p className="edit-note">Les réponses qui suivaient ce message seront remplacées.</p>
        <div className="edit-actions">
          <button type="button" className="secondary-btn small-btn" onClick={() => { setDraft(turn.text); setEditing(false) }}>Annuler</button>
          <button type="button" className="primary-btn small-btn" disabled={!draft.trim() || busy} onClick={() => { conversationStore.edit(conversationId, turn.id, draft); setEditing(false) }}>Envoyer</button>
        </div>
      </div>
    )
  }
  return (
    <div className="user-turn">
      <div className="user-message">
        {turn.attachments?.length ? (
          <div className="turn-files">
            {turn.attachments.map((a) => <span key={`${a.name}-${a.size}`} title="Non transmis : le serveur n’accepte pas encore les fichiers"><Icon name={a.type.startsWith('image/') ? 'image' : 'file'} size={14} />{a.name}<small>{sizeText(a.size)} · non transmis</small></span>)}
          </div>
        ) : null}
        {turn.text}
      </div>
      <div className="message-actions user">
        <span className="turn-time">{clock(turn.at)}{turn.editedFrom ? ' · modifié' : ''}</span>
        <button type="button" onClick={() => copy(turn.text)} aria-label="Copier le message" title="Copier"><Icon name={copied ? 'check' : 'copy'} size={15} /></button>
        {!busy && <button type="button" onClick={() => { setDraft(turn.text); setEditing(true) }} aria-label="Modifier le message" title="Modifier"><Icon name="edit" size={15} /></button>}
      </div>
      {turn.failed && (
        <div className="failed-note" role="alert">
          <span>Non envoyé : {turn.failed.message}</span>
          <button type="button" className="secondary-btn small-btn" disabled={busy} onClick={() => conversationStore.retry(conversationId, turn.id)}>Réessayer</button>
        </div>
      )}
    </div>
  )
}

export function Typing({ label }: { label: string }) {
  return (
    <div className="assistant-message compact-response" role="status" aria-live="polite">
      <div className="assistant-mark"><Logo compact /></div>
      <div className="message-content"><p className="working">{label}</p><div className="typing"><i /><i /><i /></div></div>
    </div>
  )
}

const TRACE: Record<string, [IconName, string]> = {
  SEARCH_LISTINGS: ['search', 'Recherche de fournisseurs terminée'],
  COMPARE_LISTINGS: ['layers', 'Comparaison des offres terminée'],
  CHECK_AVAILABILITY: ['check', 'Disponibilité vérifiée'],
  CHECK_INVENTORY: ['check', 'Stock vérifié'],
  CREATE_ORDER: ['orders', 'Commande préparée'],
  CREATE_RESERVATION: ['calendar', 'Réservation préparée'],
  GET_ORDER: ['orders', 'Commande consultée'],
  GET_RESERVATION: ['calendar', 'Réservation consultée'],
  GET_LISTING: ['file', 'Détails de l’offre'],
  GET_DELIVERY_STATUS: ['truck', 'Livraison suivie'],
  GET_PAYMENT_STATUS: ['wallet', 'Paiement suivi'],
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
}

export function AssistantMessage(p: AssistantProps) {
  const { turn, latest } = p
  const reply = turn.reply
  const { shown, done } = useReveal(turn)
  const { copied, copy } = useCopy()
  const offers = reply ? offersOf(reply.result_items) : []
  const trace = reply ? TRACE[reply.action_type] : undefined
  const questions = reply?.missing_params.filter((q) => q.name !== 'listing_id') ?? []
  const confirmation = reply?.is_confirmation ? reply.confirmation : undefined
  const canRegenerate = latest && !p.busy && conversationStore.canRegenerate(p.conversationId, turn.id)

  return (
    <div className="assistant-message">
      <div className="assistant-mark"><Logo compact /></div>
      <div className="message-content">
        <RichText text={shown} />
        {done && reply && (
          <>
            {trace && (
              <div className="activity">
                <span className="activity-check"><Icon name="check" size={13} /></span>
                <div><b>{trace[1]}</b><small>{offers.length ? `${offers.length} offre${offers.length > 1 ? 's' : ''} trouvée${offers.length > 1 ? 's' : ''}` : reply.action_type === 'SEARCH_LISTINGS' ? 'Aucune offre correspondante pour l’instant' : 'Exécuté par le Commerce Action Runtime'} · {clock(turn.at)}</small></div>
                {p.operationId
                  ? <Link className="activity-link" to={`/operations/${encodeURIComponent(p.operationId)}`}>Voir l’opération <Icon name="arrow" size={13} /></Link>
                  : offers.length > 0 && latest && (p.signedIn
                    ? <button type="button" onClick={p.onOpenOperation} disabled={p.openingOperation} aria-busy={p.openingOperation}>{p.openingOperation ? 'Ouverture…' : 'Suivre en opération'} <Icon name="arrow" size={13} /></button>
                    : <Link className="activity-link" to={p.signInHref}>Se connecter pour suivre <Icon name="arrow" size={13} /></Link>)}
              </div>
            )}

            {offers.length > 0 && (
              <>
                <div className="supplier-stack">
                  {offers.slice(0, 3).map((o) => <SupplierCard key={o.id} offer={o} onOpen={() => p.onOpenOffer(turn.id, o, latest)} onAsk={latest ? () => p.onSend(`Parle-moi de l’offre ${o.n} : ${o.title}`) : undefined} />)}
                </div>
                {offers.length > 1 && (
                  <button type="button" className="compare-link" onClick={() => p.onCompare(turn.id)}>
                    <Icon name="layers" size={15} />Comparer {offers.length > 3 ? `les ${offers.length} offres` : 'ces offres'}
                  </button>
                )}
              </>
            )}

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

            {reply.is_clarification && questions.length > 0 && (
              <div className="insight question">
                <span><Icon name="message" size={17} /></span>
                <div>{questions.map((q) => <p key={q.name}>{q.prompt && !technical.test(q.prompt) ? q.prompt : `Précise : ${q.name.replace(/_/g, ' ')}.`}</p>)}</div>
              </div>
            )}

            {confirmation && (
              <div className={`approval-box ${turn.resolution ? 'resolved' : ''}`} role="group" aria-label="Confirmation requise">
                <Icon name="shield" />
                <div>
                  <b>{turn.resolution ? 'Décision enregistrée' : 'Ton approbation est requise'}</b>
                  <p>{confirmation.summary || 'Confirme pour continuer. Rien ne se passe tant que tu ne l’as pas fait.'}</p>
                  {turn.resolution ? (
                    <p className="resolution"><Icon name={turn.resolution === 'confirmed' ? 'check' : 'x'} size={13} />
                      {turn.resolution === 'confirmed' ? 'Tu as approuvé.' : turn.resolution === 'rejected' ? 'Tu as refusé. Rien n’a été modifié.' : 'Cette confirmation a expiré.'}
                    </p>
                  ) : latest ? (
                    p.signedIn ? (
                      <div className="approval-actions">
                        <button type="button" className="secondary-btn" disabled={p.confirming} onClick={() => void conversationStore.answer(p.conversationId, turn.id, false)}>Refuser</button>
                        <button type="button" className="primary-btn" disabled={p.confirming} aria-busy={p.confirming} onClick={() => void conversationStore.answer(p.conversationId, turn.id, true)}>{p.confirming ? 'Envoi…' : 'Approuver'}</button>
                      </div>
                    ) : (
                      <div className="approval-actions"><Link className="primary-btn" to={p.signInHref}>Se connecter pour approuver</Link></div>
                    )
                  ) : null}
                </div>
              </div>
            )}

            {reply.refs.order_id && (
              <div className="activity success">
                <span className="activity-check"><Icon name="orders" size={13} /></span>
                <div><b>Commande créée</b><small>Elle est enregistrée dans tes achats.</small></div>
                <Link className="activity-link" to={`/purchases/${encodeURIComponent(reply.refs.order_id)}`}>Voir la commande <Icon name="arrow" size={13} /></Link>
              </div>
            )}
            {reply.refs.reservation_id && reply.action_type === 'CREATE_RESERVATION' && (
              <div className="activity success">
                <span className="activity-check"><Icon name="calendar" size={13} /></span>
                <div><b>Réservation enregistrée</b><small>Retrouve-la dans tes achats et réservations.</small></div>
                <Link className="activity-link" to="/purchases?tab=bookings">Voir <Icon name="arrow" size={13} /></Link>
              </div>
            )}

            {latest && !confirmation && reply.suggestions.length > 0 && (
              <div className="quick-replies" role="group" aria-label="Réponses suggérées">
                {reply.suggestions.map((s) => <button type="button" key={s} onClick={() => p.onSend(s)} disabled={p.busy}>{s}</button>)}
              </div>
            )}
          </>
        )}
        {done && (
          <div className="message-actions">
            <button type="button" onClick={() => copy(turn.text)} aria-label="Copier la réponse" title="Copier"><Icon name={copied ? 'check' : 'copy'} size={15} /></button>
            {canRegenerate && <button type="button" onClick={() => conversationStore.regenerate(p.conversationId, turn.id)} aria-label="Régénérer la réponse" title="Redemander (la recherche est relancée)"><Icon name="refresh" size={15} /></button>}
          </div>
        )}
      </div>
    </div>
  )
}
