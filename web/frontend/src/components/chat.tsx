import { mapSearchResult } from '@/api/search'
import { ResultCard } from '@/components/cards'
import { ActionLink, Button, Icon } from '@/components/ui'
import { detailPath } from '@/lib/format'
import { locale, t } from '@/lib/i18n'
import type { Turn } from '@/lib/chat'

/* Message rendering shared by the full conversation page and the floating chat. */

const technicalWording = /required parameter|is missing|must not be empty|must be positive/i

/** The question for a missing detail, in the person's language (never the server's technical wording). */
function missingQuestion(p: { name: string; prompt: string }): string {
  if (p.prompt && !technicalWording.test(p.prompt)) return p.prompt
  return t('Please tell me the {replace}.', { replace: p.name.replace(/_/g, ' ') })
}

export function clock(ts: number) {
  return new Date(ts).toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' })
}

/** Plain-text message body; newlines kept, nothing rendered as HTML. */
export function MessageText({ text }: { text: string }) {
  return <>{text.split('\n').map((line, i) => <p key={i}>{line}</p>)}</>
}

export function UserTurn({ turn, onRetry }: { turn: Turn; onRetry: (text: string, key: string, id: string) => void }) {
  return (
    <div className="user-turn">
      <div className="user-bubble">{turn.text}<span>{clock(turn.at)}</span></div>
      {turn.failed && (
        <div className="message-notice error" role="alert"><span><Icon name="bell" /></span><div><strong>{turn.failed.error.isNetwork ? t('I couldn’t reach Trezutil.') : t('I couldn’t process that.')}</strong><p>{turn.failed.error.message}</p><Button onClick={() => onRetry(turn.text, turn.failed!.key, turn.id)}>{t('Try again')} <Icon name="arrow" size={15} /></Button></div></div>
      )}
    </div>
  )
}

export function Typing({ label }: { label: string }) {
  return <div className="trez-response"><span className="chat-logo" aria-hidden="true"><Icon name="spark" size={18} /></span><div className="message-content"><div className="thinking" role="status"><i /><i /><i /><span>{label}</span></div></div></div>
}

interface AssistantTurnProps {
  turn: Turn
  onSend: (text: string) => void
  onConfirm: (token: string, accept: boolean) => void
  confirming: boolean
  latest: boolean
  /** The floating chat is narrow: fewer results, with a way to see the rest on the full page. */
  compact?: boolean
}

export function AssistantTurn({ turn, onSend, onConfirm, confirming, latest, compact = false }: AssistantTurnProps) {
  const reply = turn.reply!
  const results = reply.result_items.map(mapSearchResult).filter((r) => r.listing_id)
  const shown = results.slice(0, compact ? 3 : 6)
  const listingId = reply.refs.listing_id
  // Which listing is meant is already asked in the reply text ("reply with its number").
  const asked = reply.missing_params.filter((p) => p.name !== 'listing_id')
  return (
    <div className="trez-response">
      <span className="chat-logo" aria-hidden="true"><Icon name="spark" size={18} /></span>
      <div className="message-content">
        <MessageText text={reply.text} />
        {shown.length > 0 && (
          <div className="chat-results">
            {shown.map((item, i) => (
              <div className="chat-result" key={item.listing_id}>
                <span className="chat-result-index" aria-hidden="true">{i + 1}</span>
                <ResultCard item={item} layout="list" askHref={`/conversation?q=${encodeURIComponent(t('Tell me about number {n}', { n: i + 1 }))}`} />
              </div>
            ))}
          </div>
        )}
        {compact && results.length > shown.length && <ActionLink className="chat-more" href="/conversation">{t('See all in the full conversation')} <Icon name="arrow" size={14} /></ActionLink>}
        {results.length === 0 && reply.cards.length > 0 && (
          <div className="chat-card-list">
            {reply.cards.map((card) => (
              <Button key={card.id} className="chat-card" onClick={() => onSend(String(card.position || card.title))}>
                <strong>{card.position ? `${card.position}. ` : ''}{card.title}</strong>{card.description && <small>{card.description}</small>}
              </Button>
            ))}
          </div>
        )}
        {latest && !reply.is_confirmation && reply.suggestions.length > 0 && (
          <div className="chat-suggestions" role="group" aria-label={t('Suggested replies')}>
            {reply.suggestions.map((s) => <Button key={s} className="suggestion-pill" onClick={() => onSend(s)}>{s}</Button>)}
            {listingId && <ActionLink className="suggestion-pill" href={detailPath(listingId)}>{t('View listing')}</ActionLink>}
          </div>
        )}
        {results.length > 1 && latest && reply.suggestions.length === 0 && (
          <div className="chat-followup"><strong>{t('Want me to narrow these down?')}</strong><div>
            <Button className="filter-pill" onClick={() => onSend(t('Show me the second one'))}>{t('Open the second one')}</Button>
            <Button className="filter-pill" onClick={() => onSend(t('Only the cheapest ones'))}>{t('Lowest price')}</Button>
            <Button className="filter-pill" onClick={() => onSend(t('Which are available today?'))}>{t('Available today')}</Button>
          </div></div>
        )}
        {listingId && latest && reply.suggestions.length === 0 && (
          <div className="chat-followup"><strong>{t('What next?')}</strong><div>
            <Button className="filter-pill" onClick={() => onSend(t('Book it'))}>{t('Book it')}</Button>
            <Button className="filter-pill" onClick={() => onSend(t('Buy it'))}>{t('Buy it')}</Button>
            <ActionLink className="filter-pill" href={detailPath(listingId)}>{t('View listing')}</ActionLink>
          </div></div>
        )}
        {reply.is_clarification && asked.length > 0 && (
          <div className="message-notice question"><span><Icon name="spark" /></span><div><strong>{t('One quick question')}</strong>
            {asked.map((p) => <p key={p.name}>{missingQuestion(p)}</p>)}
          </div></div>
        )}
        {reply.is_confirmation && reply.confirmation && latest && (
          <div className="message-notice confirm" role="group" aria-label={t('Confirmation required')}>
            <span><Icon name="orders" /></span>
            <div>
              <strong>{t('Please confirm')}</strong>
              <p>{reply.confirmation.summary || t('Confirm to continue. Nothing happens until you do.')}</p>
              <div className="notice-actions">
                <Button className="primary-button" busy={confirming} onClick={() => onConfirm(reply.confirmation!.token, true)}>{t('Confirm')} <Icon name="check" size={15} /></Button>
                <Button className="outline-button" disabled={confirming} onClick={() => onConfirm(reply.confirmation!.token, false)}>{t('Cancel')}</Button>
              </div>
            </div>
          </div>
        )}
        {reply.refs.order_id && <div className="message-notice success"><span><Icon name="orders" /></span><div><strong>{t('Order created')}</strong><p>{t('Review and pay for it whenever you’re ready.')}</p><ActionLink className="button" href={`/orders/${encodeURIComponent(reply.refs.order_id)}`}>{t('View order')} <Icon name="arrow" size={15} /></ActionLink></div></div>}
        {reply.refs.reservation_id && reply.action_type === 'CREATE_RESERVATION' && <div className="message-notice success"><span><Icon name="calendar" /></span><div><strong>{t('Reservation confirmed')}</strong><p>{t('Your booking is saved to your reservations.')}</p><ActionLink className="button" href={`/reservations/${encodeURIComponent(reply.refs.reservation_id)}`}>{t('View booking')} <Icon name="arrow" size={15} /></ActionLink></div></div>}
      </div>
    </div>
  )
}
