import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { listingsApi } from '@/api/listings'
import { searchApi } from '@/api/search'
import { DiscountBadge, ListingImage, PriceTag, ResultCard } from '@/components/cards'
import { AppShell } from '@/components/shell'
import { ErrorState, Loading } from '@/components/states'
import { ActionLink, Button, Heading, Icon, Tabs, type IconName } from '@/components/ui'
import { formatDateTime, sentence } from '@/lib/format'
import { listingSale } from '@/lib/promo'
import { usePrefs } from '@/lib/prefs'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { flowsFor, useVerticals } from '@/lib/verticals'
import { HotelDetail } from '@/pages/HotelDetail'
import type { ListingDetail } from '@/types'
import { t, tx } from '@/lib/i18n'

/*
 * Figma DetailPage — one layout. Whether the listing is booked or bought comes
 * from its vertical's customer flows (GET /verticals); the tables below only
 * hold presentation copy.
 */

const verticalCopy: Record<string, { icon: IconName; book?: string; by?: string }> = {
  HOTEL: { icon: 'building', book: tx('Choose a room'), by: tx('Hosted by') },
  RESTAURANT: { icon: 'food', book: tx('Reserve a table'), by: tx('Served by') },
  SERVICES: { icon: 'service', book: tx('Book an appointment'), by: tx('Provided by') },
  REAL_ESTATE: { icon: 'home', book: tx('Reserve dates'), by: tx('Listed by') },
  VEHICLE_RENTAL: { icon: 'car', book: tx('Reserve dates'), by: tx('Rented by') },
  TRANSPORT: { icon: 'car', book: tx('Book a seat'), by: tx('Operated by') },
  EVENTS: { icon: 'calendar', by: tx('Organised by') },
  TRAVEL: { icon: 'calendar', book: tx('Reserve dates'), by: tx('Organised by') },
  RETAIL: { icon: 'shop', by: tx('Sold by') },
}

const typeCopy: Record<string, { crumb: string; crumbHref: string; icon: IconName; primary: string; by: string }> = {
  physical_product: { crumb: tx('Products'), crumbHref: '/search?vertical=RETAIL', icon: 'shop', primary: tx('Add to bag'), by: tx('Sold by') },
  vehicle: { crumb: tx('Transport'), crumbHref: '/search?vertical=TRANSPORT', icon: 'car', primary: tx('Add to bag'), by: tx('Sold by') },
  other: { crumb: tx('Listings'), crumbHref: '/search', icon: 'spark', primary: tx('Add to bag'), by: tx('Listed by') },
  service: { crumb: tx('Services'), crumbHref: '/search?vertical=SERVICES', icon: 'service', primary: tx('Book this service'), by: tx('Provided by') },
  real_estate: { crumb: tx('Real Estate'), crumbHref: '/search?vertical=REAL_ESTATE', icon: 'home', primary: tx('Reserve dates'), by: tx('Listed by') },
  food: { crumb: tx('Food'), crumbHref: '/search?vertical=RESTAURANT', icon: 'food', primary: tx('Reserve a table'), by: tx('Served by') },
}

function Facts({ listing, icon }: { listing: ListingDetail; icon: IconName }) {
  const entries = Object.entries(listing.metadata)
    .filter(([, v]) => ['string', 'number', 'boolean'].includes(typeof v))
    .slice(0, 8)
  if (!entries.length) return null
  return (
    <div className="fact-grid">
      {entries.map(([key, value]) => (
        <div key={key}><span><Icon name={icon} size={18} /></span><strong>{typeof value === 'boolean' ? (value ? sentence(key) : t('No {replace}', { replace: key.replace(/_/g, ' ') })) : `${value} ${key.replace(/_/g, ' ')}`}</strong><small>{t('Provided by the seller')}</small></div>
      ))}
    </div>
  )
}

export default function DetailPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const notify = useToast()
  const { addToBag, isSaved, toggleSaved } = usePrefs()
  const listing = useAsync(() => listingsApi.get(id), [id])
  const { byCode } = useVerticals()
  const l = listing.data
  const store = useAsync(() => listingsApi.store(l!.store_id), [l?.store_id], !!l?.store_id)
  const related = useAsync(() => searchApi.search({ text: l!.title.split(/\s+/).slice(0, 2).join(' '), page_size: 5 }), [l?.id], !!l)
  const [selected, setSelected] = useState(0)
  const [tab, setTab] = useState<'Overview' | 'Details'>('Overview')
  const [quantity, setQuantity] = useState(1)

  useEffect(() => { if (l) document.title = t('{title} — Trezutil', { title: l.title }) }, [l])
  useEffect(() => { setSelected(0); setQuantity(1) }, [id])

  if (listing.loading) return <AppShell><Loading label={t('Loading listing…')} /></AppShell>
  if (listing.error || !l) return <AppShell><section className="page-section"><ErrorState error={listing.error ?? t('Listing not found')} onRetry={listing.reload} /></section></AppShell>
  // Hotel room types open the hotel page (property, rooms, dates, availability).
  if (l.vertical === 'HOTEL') return <HotelDetail listing={l} />

  const vertical = byCode[l.vertical]
  const flows = flowsFor(vertical, l.listing_type)
  const bookable = flows[0] === 'BOOK'
  const canBuy = flows.includes('BUY')
  const canBook = flows.includes('BOOK')
  const vc = verticalCopy[l.vertical]
  const legacy = typeCopy[l.listing_type] ?? typeCopy.other
  const copy = {
    crumb: vertical?.name ?? legacy.crumb,
    crumbHref: vertical ? `/search?vertical=${encodeURIComponent(vertical.code)}` : legacy.crumbHref,
    icon: vc?.icon ?? legacy.icon,
    primary: bookable ? (vc?.book ?? t('Reserve dates')) : t('Add to bag'),
    by: vc?.by ?? legacy.by,
  }
  const live = l.status === 'published'
  const images = l.media.length ? l.media.map((m) => m.url) : [null]
  const saved = isSaved(l.id)
  const sale = listingSale(l)
  const snapshot = { listing_id: l.id, title: l.title, price_amount: l.price_amount, price_currency: l.price_currency, image: l.media[0]?.url ?? null, seller: store.data?.name ?? '', listing_type: l.listing_type, vertical: l.vertical }
  const sellerName = store.data?.name || t('Trezutil seller')
  const ask = t('Show me {title}', { title: l.title })

  const primary = () => {
    if (bookable) { navigate(`/book/${encodeURIComponent(l.id)}`); return }
    addToBag(snapshot, quantity)
    notify(t('{title} added to your bag.', { title: l.title }), 'success')
  }

  return (
    <AppShell>
      <section className="page-section detail-page">
        <nav className="breadcrumbs" aria-label={t('Breadcrumb')}><ActionLink href="/">{t('Home')}</ActionLink><Icon name="chevron" size={13} /><ActionLink href={copy.crumbHref}>{t(copy.crumb)}</ActionLink><Icon name="chevron" size={13} /><span aria-current="page">{l.title}</span></nav>
        <div className="detail-grid">
          <div className="gallery">
            <div className="gallery-main"><ListingImage src={images[selected]} alt={l.title} type={l.listing_type} eager />{images.length > 1 && <span className="gallery-count">{selected + 1} / {images.length}</span>}</div>
            {images.length > 1 && (
              <div className="gallery-thumbs">{images.map((image, index) => <Button className={selected === index ? 'active' : ''} ariaLabel={t('Show image {v}', { v: index + 1 })} pressed={selected === index} onClick={() => setSelected(index)} key={`${image}-${index}`}><ListingImage src={image} alt="" type={l.listing_type} /></Button>)}</div>
            )}
          </div>
          <div className="detail-summary">
            <div className="detail-eyebrow">{vertical?.name ?? sentence(l.listing_type)}{l.pricing_model === 'negotiable' ? t(' · Price negotiable') : ''}</div>
            <Heading level={1}>{l.title}</Heading>
            {store.data?.city && <div className="detail-location"><Icon name="location" size={17} />{store.data.city}</div>}
            <div className="detail-price-row"><PriceTag className="detail-price" amount={l.price_amount} currency={l.price_currency} sale={sale} />{sale && <DiscountBadge percent={sale.percent} />}</div>
            <div className="availability-line"><span className="status-dot" />{live ? (bookable ? t('Check dates to see availability') : t('Available to order')) : t('Not available — {sentence}', { sentence: sentence(l.status) })}{l.updated_at ? <small> {t('· Updated')} {formatDateTime(l.updated_at)}</small> : null}</div>
            {!bookable && live && (
              <div className="quantity detail-quantity" role="group" aria-label={t('Quantity')}>
                <Button ariaLabel={t('Decrease quantity')} onClick={() => setQuantity((q) => Math.max(1, q - 1))} disabled={quantity <= 1}>−</Button>
                <span aria-live="polite">{quantity}</span>
                <Button ariaLabel={t('Increase quantity')} onClick={() => setQuantity((q) => Math.min(99, q + 1))}>+</Button>
              </div>
            )}
            <div className="detail-actions">
              <Button className="primary-button" onClick={primary} disabled={!live}>{t(copy.primary)}<Icon name={bookable ? 'calendar' : 'orders'} size={18} /></Button>
              <Button className={saved ? 'outline-button saved' : 'outline-button'} pressed={saved} onClick={() => toggleSaved(snapshot)}><Icon name="heart" size={18} />{saved ? t('Saved') : t('Save')}</Button>
            </div>
            {live && (bookable ? canBuy : true) && (
              <div className="detail-secondary-actions">
                <ActionLink className="outline-button" href={`/checkout?listing_id=${encodeURIComponent(l.id)}&quantity=${quantity}`}>{bookable ? t('Order instead') : t('Buy now')}</ActionLink>
                {!bookable && canBook && <ActionLink className="outline-button" href={`/book/${encodeURIComponent(l.id)}`}>{t('Reserve dates')}</ActionLink>}
                {!bookable && <ActionLink className="text-action" href="/cart">{t('View bag')}</ActionLink>}
              </div>
            )}
            <ActionLink className="detail-ask" href={`/conversation?q=${encodeURIComponent(ask)}`}><span className="dock-logo"><Icon name="spark" /></span><span><strong>{t('Ask Trezutil about this listing')}</strong><small>{t('Compare, check details or ask what matters')}</small></span><Icon name="arrow" /></ActionLink>
            <div className="seller-card"><span className="seller-avatar" aria-hidden="true">{sellerName.slice(0, 1)}</span><div><small>{t(copy.by)}</small><strong>{sellerName}</strong><span>{store.data?.status === 'active' ? t('Active store on Trezutil') : t('Seller on Trezutil')}</span></div><ActionLink className="outline-button" href={`/conversation?q=${encodeURIComponent(`I have a question about ${l.title}`)}`}>{t('Ask')}</ActionLink></div>
          </div>
        </div>
        <Tabs label={t('Listing information')} items={['Overview', 'Details'] as const} value={tab} onChange={setTab} />
        <div className="detail-content">
          <div role="tabpanel">
            {tab === 'Overview' ? <>
              <Heading level={2}>{t('About this listing')}</Heading>
              <p>{l.description || t('The seller hasn’t added a description yet. Ask Trezutil and we’ll help you find out more.')}</p>
              <Facts listing={l} icon={copy.icon} />
            </> : <>
              <Heading level={2}>{t('Details')}</Heading>
              <div className="info-rows">
                {vertical && <div><span>{t('Category')}</span><strong>{vertical.name}</strong></div>}
                <div><span>{t('Type')}</span><strong>{sentence(l.listing_type)}</strong></div>
                <div><span>{t('Pricing')}</span><strong>{sentence(l.pricing_model || 'fixed')}</strong></div>
                <div><span>{t('Currency')}</span><strong>{l.price_currency}</strong></div>
                {l.published_at && <div><span>{t('Listed')}</span><strong>{formatDateTime(l.published_at)}</strong></div>}
                <div><span>{t('Reference')}</span><strong>{l.id.slice(0, 8)}</strong></div>
              </div>
            </>}
          </div>
          <aside className="location-card"><div className="map-pattern"><span><Icon name={bookable ? 'calendar' : 'location'} /></span></div><Heading level={3}>{bookable ? t('Plan your booking') : t('Pickup & delivery')}</Heading><p>{bookable ? t('Pick your dates to check live availability before you reserve.') : t('Arrange pickup or delivery with the seller after ordering.')}</p>{bookable ? <ActionLink className="outline-button" href={`/book/${encodeURIComponent(l.id)}`}>{t('Check availability')}</ActionLink> : <ActionLink className="outline-button" href={`/conversation?q=${encodeURIComponent(`How do I get ${l.title}?`)}`}>{t('Ask about delivery')}</ActionLink>}</aside>
        </div>
        {(related.data?.items.filter((r) => r.listing_id !== l.id).length ?? 0) > 0 && (
          <section className="app-rail" aria-label={t('More options')}>
            <div className="rail-head"><div><Heading>{bookable ? t('More options nearby') : t('You may also like')}</Heading><p>{t('Similar listings on Trezutil')}</p></div><ActionLink href={`/search?q=${encodeURIComponent(l.title)}`}>{t('See all')} <Icon name="arrow" size={17} /></ActionLink></div>
            <div className="results-grid">{related.data!.items.filter((r) => r.listing_id !== l.id).slice(0, 4).map((item) => <ResultCard item={item} key={item.listing_id} />)}</div>
          </section>
        )}
      </section>
    </AppShell>
  )
}
