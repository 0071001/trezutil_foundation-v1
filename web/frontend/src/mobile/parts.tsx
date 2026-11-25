import { createContext, useContext, type ReactNode } from 'react'
import type { Offer } from '@/commerce/offers'
import { kindLabel } from '@/mobile/catalog'
import { Icon, type IconName } from '@/ui/Icon'
import { t } from '@/lib/i18n'
import { useCart } from '@/commerce/cart'

/** The unread notifications count, provided by the frame. */
export const UnreadContext = createContext<{ unread: number }>({ unread: 0 })
const useUnread = () => useContext(UnreadContext)

/* Pieces shared by the mobile screens (Design Trezutil Mobile App): section titles and offer tiles. */

export function Eyebrow({ icon, children }: { icon?: IconName; children: ReactNode }) {
  return <p className="m-eyebrow">{icon && <Icon name={icon} size={13} />}{children}</p>
}

export function SectionHead({ title, action }: { title: string; action?: ReactNode }) {
  return <div className="m-section-head"><h2>{title}</h2>{action}</div>
}

/** A published offer as a tile: picture, its availability, title, kind and place, price. No rating is invented. */
export function OfferTile({ offer, onOpen, tag = 'availability' }: { offer: Offer; onOpen: () => void; tag?: 'availability' | 'kind' }) {
  const kind = kindLabel(offer.vertical)
  const label = tag === 'kind' ? kind : offer.availability
  const sub = [tag === 'kind' ? '' : kind, offer.place.split(',')[0]].filter(Boolean).join(' · ')
  return (
    <button type="button" className="m-tile" onClick={onOpen} aria-label={t('Voir l’offre {title}', { title: offer.title })}>
      <span className="m-tile-pic" style={offer.image ? undefined : { background: offer.color }}>
        {offer.image ? <img src={offer.image} alt="" loading="lazy" /> : <b>{offer.initials}</b>}
        {label && <em className={tag === 'kind' ? 'm-tag light' : 'm-tag'}>{label}</em>}
      </span>
      <span className="m-tile-body">
        <strong>{offer.title}</strong>
        {sub && <small>{sub}</small>}
        <span className="m-tile-price">{offer.price}</span>
      </span>
    </button>
  )
}

/** A short line for an offer in a list ("Pour toi"). */
export function OfferRow({ offer, context, onOpen }: { offer: Offer; context: string; onOpen: () => void }) {
  return (
    <button type="button" className="m-row" onClick={onOpen}>
      <span className="m-row-pic" style={offer.image ? undefined : { background: offer.color }}>{offer.image ? <img src={offer.image} alt="" loading="lazy" /> : <b>{offer.initials}</b>}</span>
      <span className="m-row-text"><strong>{offer.title} · {offer.price}</strong><small>{context}</small></span>
      <Icon name="chevron" size={16} />
    </button>
  )
}

export function TileSkeleton({ count = 3 }: { count?: number }) {
  return <>{Array.from({ length: count }, (_, i) => <span key={i} className="m-tile skeleton" aria-hidden="true"><span className="m-tile-pic" /><span className="m-tile-body"><i /><i /></span></span>)}</>
}

/** Asks the frame to open the cart or the notifications (they live in the shell). */
export const openPanel = (which: 'cart' | 'notifications') => window.dispatchEvent(new CustomEvent('trezutil:open', { detail: which }))

/** The cart (when it holds something) and the notifications, at the top of a mobile screen. */
export function HeadTools() {
  const items = useCart()
  const count = items.reduce((n, i) => n + i.quantity, 0)
  const { unread } = useUnread()
  return (
    <span className="m-tools">
      {count > 0 && <button type="button" className="m-icon-btn m-badge" onClick={() => openPanel('cart')} aria-label={t('Panier, {n} article(s)', { n: count })}><Icon name="cart" size={17} /><i>{count > 9 ? '9+' : count}</i></button>}
      <button type="button" className="m-icon-btn m-badge" onClick={() => openPanel('notifications')} aria-label={unread ? t('Notifications, {unread} non lues', { unread }) : t('Notifications')}>
        <Icon name="bell" size={17} />{unread > 0 && <i>{unread > 9 ? '9+' : unread}</i>}
      </button>
    </span>
  )
}
