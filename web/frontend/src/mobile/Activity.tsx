import { useEffect } from 'react'
import { Link } from 'react-router'
import { listingsApi } from '@/api/listings'
import { reservationsApi, type OrderSummary, type ReservationSummary } from '@/api/commerce'
import { useConversations, type Conversation } from '@/conversation/store'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { formatDateTime, formatMinor, safeImage, timeAgo } from '@/lib/format'
import { lastRequest } from '@/mobile/MobileHome'
import { Eyebrow, HeadTools } from '@/mobile/parts'
import { useWorkspace } from '@/shell/workspace'
import { Icon } from '@/ui/Icon'
import { t, tn } from '@/lib/i18n'

/*
 * Activity (Design Trezutil Mobile App → Activité): the conversations, the bookings and the orders of the account,
 * from the server. "Up to date" is said only when nothing waits for the person (no order to pay).
 */

type Tone = 'blue' | 'amber' | 'green' | 'red' | 'grey'

const ORDER: Record<string, [string, Tone]> = {
  pending: [t('En attente'), 'amber'], pending_payment: [t('Paiement attendu'), 'amber'], paid: [t('Payée'), 'green'], confirmed: [t('Confirmée'), 'green'], processing: [t('En préparation'), 'blue'],
  shipped: [t('Expédiée'), 'blue'], delivered: [t('Livrée'), 'green'], completed: [t('Terminée'), 'green'], cancelled: [t('Annulée'), 'grey'],
  refunded: [t('Remboursée'), 'grey'], failed: [t('Échouée'), 'red'],
}
const BOOKING: Record<string, [string, Tone]> = {
  pending: [t('En attente'), 'amber'], confirmed: [t('Confirmée'), 'green'], cancelled: [t('Annulée'), 'grey'], completed: [t('Terminée'), 'green'],
  no_show: [t('Absence'), 'grey'], expired: [t('Expirée'), 'grey'],
}

const status = (map: Record<string, [string, Tone]>, s: string): [string, Tone] => map[s] ?? [s.replace(/_/g, ' ') || '—', 'grey']

function ConversationItem({ conv, pending }: { conv: Conversation; pending: boolean }) {
  const last = lastRequest(conv)
  const answer = [...conv.turns].reverse().find((x) => x.role === 'assistant')
  const preview = last.offers > 0 ? tn(last.offers, '{n} résultat trouvé', '{n} résultats trouvés') : (answer?.text ?? last.text)
  const tone: Tone = pending ? 'blue' : conv.operationId ? 'green' : 'grey'
  return (
    <Link className="m-card m-conv" to={`/c/${conv.id}`}>
      <span className="m-card-icon"><Icon name="clock" size={16} /></span>
      <span className="m-card-text">
        <span className="m-card-title"><i className={`m-dot ${tone}`} /><strong>{conv.title}</strong></span>
        <small className="m-clamp">{preview}</small>
        <small className="m-meta">{timeAgo(conv.updatedAt / 1000)} · {tn(conv.turns.length, '{n} message', '{n} messages')}</small>
      </span>
      <Icon name="chevron" size={16} />
    </Link>
  )
}

function BookingItem({ booking }: { booking: ReservationSummary }) {
  const listing = useAsync(() => listingsApi.get(booking.listing_id), [booking.listing_id], !!booking.listing_id)
  const [label, tone] = status(BOOKING, booking.status)
  const picture = safeImage(listing.data?.media?.find((m) => m.primary)?.url ?? listing.data?.media?.[0]?.url)
  return (
    <Link className="m-card" to="/purchases?tab=bookings">
      <span className="m-thumb">{picture ? <img src={picture} alt="" loading="lazy" /> : <Icon name="calendar" size={20} />}</span>
      <span className="m-card-text">
        <strong>{listing.data?.title ?? t('Réservation')}</strong>
        <small>{formatDateTime(booking.start_at)}{booking.party_size > 0 ? ` · ${tn(booking.party_size, '{n} personne', '{n} personnes')}` : ''}</small>
        <em className={`m-status ${tone}`}>{label}</em>
      </span>
    </Link>
  )
}

function OrderItem({ order }: { order: OrderSummary }) {
  const [label, tone] = status(ORDER, order.status)
  return (
    <Link className="m-card" to={`/purchases/${encodeURIComponent(order.id)}`}>
      <span className="m-thumb"><Icon name="orders" size={20} /></span>
      <span className="m-card-text">
        <strong>{order.title || t('Commande')}</strong>
        <small>{formatMinor(order.total_minor, order.currency)}{order.items.length > 1 ? ` · ${tn(order.items.length, '{n} article', '{n} articles')}` : ''}</small>
        <span className={`m-state ${tone}`}><i />{label}</span>
      </span>
    </Link>
  )
}

export function Activity() {
  const { signedIn, user } = useSession()
  const { conversations, pending } = useConversations()
  const { orders } = useWorkspace()
  const bookings = useAsync(() => reservationsApi.list(), [signedIn], signedIn)
  useEffect(() => { document.title = `${t('Activité')} — Trezutil` }, [])
  const convs = conversations.filter((c) => !c.archived && c.turns.length > 0).sort((a, b) => b.updatedAt - a.updatedAt).slice(0, 6)
  const toPay = (orders.data ?? []).filter((o) => o.status === 'pending_payment').length
  const upcoming = (bookings.data ?? []).filter((b) => b.status === 'pending').length

  return (
    <div className="m-screen m-activity">
      <header className="m-page-head">
        <div className="m-head-row"><Eyebrow>{t('Mon activité')}</Eyebrow><HeadTools /></div>
        <h1 className="m-title">{t('Historique')}</h1>
      </header>
      <div className="m-scroll">
        <section className="m-section">
          <Eyebrow>{t('Conversations')}</Eyebrow>
          {convs.length
            ? <div className="m-list">{convs.map((c) => <ConversationItem key={c.id} conv={c} pending={!!pending[c.id]} />)}</div>
            : <p className="m-note">{t('Aucune conversation pour l’instant. Dis à Trezutil ce que tu cherches depuis l’accueil.')}</p>}
        </section>

        {user === null ? (
          <div className="m-empty">
            <span><Icon name="lock" size={18} /></span>
            <b>{t('Commandes et réservations')}</b>
            <p>{t('Elles sont liées à ton compte.')}</p>
            <Link className="primary-btn" to="/login?returnTo=%2Factivity">{t('Se connecter')}</Link>
          </div>
        ) : signedIn && (
          <>
            <section className="m-section">
              <Eyebrow>{t('Réservations')}</Eyebrow>
              {bookings.loading && !bookings.data ? <p className="m-note">{t('Chargement…')}</p>
                : bookings.error ? <p className="m-note">{t('Les réservations n’ont pas pu être chargées.')} <button type="button" className="m-link" onClick={bookings.reload}>{t('Réessayer')}</button></p>
                  : bookings.data?.length ? <div className="m-list">{bookings.data.slice(0, 5).map((b) => <BookingItem key={b.id} booking={b} />)}</div>
                    : <p className="m-note">{t('Aucune réservation.')}</p>}
            </section>
            <section className="m-section">
              <Eyebrow>{t('Commandes')}</Eyebrow>
              {orders.loading && !orders.data ? <p className="m-note">{t('Chargement…')}</p>
                : orders.error ? <p className="m-note">{t('Les commandes n’ont pas pu être chargées.')} <button type="button" className="m-link" onClick={orders.reload}>{t('Réessayer')}</button></p>
                  : orders.data?.length ? <div className="m-list">{orders.data.slice(0, 5).map((o) => <OrderItem key={o.id} order={o} />)}</div>
                    : <p className="m-note">{t('Aucune commande.')}</p>}
            </section>
            {orders.data && bookings.data && (
              toPay + upcoming === 0 ? (
                <div className="m-empty">
                  <span><Icon name="check" size={18} /></span>
                  <b>{t('Tu es à jour')}</b>
                  <p>{t('Aucune activité en attente de ton attention.')}</p>
                </div>
              ) : (
                <Link className="m-continue" to="/purchases">
                  <b>{t('À traiter')}</b>
                  <span>{[toPay ? tn(toPay, '{n} commande à payer', '{n} commandes à payer') : '', upcoming ? tn(upcoming, '{n} réservation en attente', '{n} réservations en attente') : ''].filter(Boolean).join(' · ')}</span>
                  <small>{t('Voir mes achats et réservations')}</small>
                </Link>
              )
            )}
          </>
        )}
      </div>
    </div>
  )
}
