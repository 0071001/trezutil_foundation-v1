import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { listingsApi } from '@/api/listings'
import { searchApi } from '@/api/search'
import { ListingImage, ResultCard } from '@/components/cards'
import { AppShell } from '@/components/shell'
import { ErrorState, Loading } from '@/components/states'
import { ActionLink, Button, Heading, Icon, Tabs, type IconName } from '@/components/ui'
import { formatDateTime, formatMoney, isBookable, sentence } from '@/lib/format'
import { usePrefs } from '@/lib/prefs'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import type { ListingDetail } from '@/types'

/* Figma DetailPage — one layout, adapted to the listing type returned by the API. */

const typeCopy: Record<string, { crumb: string; crumbHref: string; icon: IconName; primary: string; by: string }> = {
  physical_product: { crumb: 'Products', crumbHref: '/search?type=products', icon: 'shop', primary: 'Add to bag', by: 'Sold by' },
  vehicle: { crumb: 'Transport', crumbHref: '/search?type=vehicles', icon: 'car', primary: 'Add to bag', by: 'Sold by' },
  other: { crumb: 'Listings', crumbHref: '/search', icon: 'spark', primary: 'Add to bag', by: 'Listed by' },
  service: { crumb: 'Services', crumbHref: '/search?type=services', icon: 'service', primary: 'Book this service', by: 'Provided by' },
  real_estate: { crumb: 'Real Estate', crumbHref: '/search?type=property', icon: 'home', primary: 'Reserve dates', by: 'Listed by' },
  food: { crumb: 'Food', crumbHref: '/search?type=food', icon: 'food', primary: 'Reserve a table', by: 'Served by' },
}

function Facts({ listing, icon }: { listing: ListingDetail; icon: IconName }) {
  const entries = Object.entries(listing.metadata)
    .filter(([, v]) => ['string', 'number', 'boolean'].includes(typeof v))
    .slice(0, 8)
  if (!entries.length) return null
  return (
    <div className="fact-grid">
      {entries.map(([key, value]) => (
        <div key={key}><span><Icon name={icon} size={18} /></span><strong>{typeof value === 'boolean' ? (value ? sentence(key) : `No ${key.replace(/_/g, ' ')}`) : `${value} ${key.replace(/_/g, ' ')}`}</strong><small>Provided by the seller</small></div>
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
  const l = listing.data
  const store = useAsync(() => listingsApi.store(l!.store_id), [l?.store_id], !!l?.store_id)
  const related = useAsync(() => searchApi.search({ text: l!.title.split(/\s+/).slice(0, 2).join(' '), page_size: 5 }), [l?.id], !!l)
  const [selected, setSelected] = useState(0)
  const [tab, setTab] = useState<'Overview' | 'Details'>('Overview')
  const [quantity, setQuantity] = useState(1)

  useEffect(() => { if (l) document.title = `${l.title} — Trezutil` }, [l])
  useEffect(() => { setSelected(0); setQuantity(1) }, [id])

  if (listing.loading) return <AppShell><Loading label="Loading listing…" /></AppShell>
  if (listing.error || !l) return <AppShell><section className="page-section"><ErrorState error={listing.error ?? 'Listing not found'} onRetry={listing.reload} /></section></AppShell>

  const copy = typeCopy[l.listing_type] ?? typeCopy.other
  const bookable = isBookable(l.listing_type)
  const live = l.status === 'published'
  const images = l.media.length ? l.media.map((m) => m.url) : [null]
  const saved = isSaved(l.id)
  const snapshot = { listing_id: l.id, title: l.title, price_amount: l.price_amount, price_currency: l.price_currency, image: l.media[0]?.url ?? null, seller: store.data?.name ?? '', listing_type: l.listing_type }
  const sellerName = store.data?.name || 'Trezutil seller'
  const ask = `Tell me more about ${l.title}`

  const primary = () => {
    if (bookable) { navigate(`/book/${encodeURIComponent(l.id)}`); return }
    addToBag(snapshot, quantity)
    notify(`${l.title} added to your bag.`, 'success')
  }

  return (
    <AppShell dockPrompt={ask}>
      <section className="page-section detail-page">
        <nav className="breadcrumbs" aria-label="Breadcrumb"><ActionLink href="/">Home</ActionLink><Icon name="chevron" size={13} /><ActionLink href={copy.crumbHref}>{copy.crumb}</ActionLink><Icon name="chevron" size={13} /><span aria-current="page">{l.title}</span></nav>
        <div className="detail-grid">
          <div className="gallery">
            <div className="gallery-main"><ListingImage src={images[selected]} alt={l.title} type={l.listing_type} eager />{images.length > 1 && <span className="gallery-count">{selected + 1} / {images.length}</span>}</div>
            {images.length > 1 && (
              <div className="gallery-thumbs">{images.map((image, index) => <Button className={selected === index ? 'active' : ''} ariaLabel={`Show image ${index + 1}`} pressed={selected === index} onClick={() => setSelected(index)} key={`${image}-${index}`}><ListingImage src={image} alt="" type={l.listing_type} /></Button>)}</div>
            )}
          </div>
          <div className="detail-summary">
            <div className="detail-eyebrow">{sentence(l.listing_type)}{l.pricing_model === 'negotiable' ? ' · Price negotiable' : ''}</div>
            <Heading level={1}>{l.title}</Heading>
            {store.data?.city && <div className="detail-location"><Icon name="location" size={17} />{store.data.city}</div>}
            <strong className="detail-price">{formatMoney(l.price_amount, l.price_currency)}</strong>
            <div className="availability-line"><span className="status-dot" />{live ? (bookable ? 'Check dates to see availability' : 'Available to order') : `Not available — ${sentence(l.status)}`}{l.updated_at ? <small> · Updated {formatDateTime(l.updated_at)}</small> : null}</div>
            {!bookable && live && (
              <div className="quantity detail-quantity" role="group" aria-label="Quantity">
                <Button ariaLabel="Decrease quantity" onClick={() => setQuantity((q) => Math.max(1, q - 1))} disabled={quantity <= 1}>−</Button>
                <span aria-live="polite">{quantity}</span>
                <Button ariaLabel="Increase quantity" onClick={() => setQuantity((q) => Math.min(99, q + 1))}>+</Button>
              </div>
            )}
            <div className="detail-actions">
              <Button className="primary-button" onClick={primary} disabled={!live}>{copy.primary}<Icon name={bookable ? 'calendar' : 'orders'} size={18} /></Button>
              <Button className={saved ? 'outline-button saved' : 'outline-button'} pressed={saved} onClick={() => toggleSaved(snapshot)}><Icon name="heart" size={18} />{saved ? 'Saved' : 'Save'}</Button>
            </div>
            {live && (
              <div className="detail-secondary-actions">
                <ActionLink className="outline-button" href={`/checkout?listing_id=${encodeURIComponent(l.id)}&quantity=${quantity}`}>{bookable ? 'Order instead' : 'Buy now'}</ActionLink>
                {!bookable && <ActionLink className="text-action" href="/cart">View bag</ActionLink>}
              </div>
            )}
            <ActionLink className="detail-ask" href={`/conversation?q=${encodeURIComponent(ask)}`}><span className="dock-logo"><Icon name="spark" /></span><span><strong>Ask Trezutil about this listing</strong><small>Compare, check details or ask what matters</small></span><Icon name="arrow" /></ActionLink>
            <div className="seller-card"><span className="seller-avatar" aria-hidden="true">{sellerName.slice(0, 1)}</span><div><small>{copy.by}</small><strong>{sellerName}</strong><span>{store.data?.status === 'active' ? 'Active store on Trezutil' : 'Seller on Trezutil'}</span></div><ActionLink className="outline-button" href={`/conversation?q=${encodeURIComponent(`I have a question about ${l.title}`)}`}>Ask</ActionLink></div>
          </div>
        </div>
        <Tabs label="Listing information" items={['Overview', 'Details'] as const} value={tab} onChange={setTab} />
        <div className="detail-content">
          <div role="tabpanel">
            {tab === 'Overview' ? <>
              <Heading level={2}>About this listing</Heading>
              <p>{l.description || 'The seller hasn’t added a description yet. Ask Trezutil and we’ll help you find out more.'}</p>
              <Facts listing={l} icon={copy.icon} />
            </> : <>
              <Heading level={2}>Details</Heading>
              <div className="info-rows">
                <div><span>Type</span><strong>{sentence(l.listing_type)}</strong></div>
                <div><span>Pricing</span><strong>{sentence(l.pricing_model || 'fixed')}</strong></div>
                <div><span>Currency</span><strong>{l.price_currency}</strong></div>
                {l.published_at && <div><span>Listed</span><strong>{formatDateTime(l.published_at)}</strong></div>}
                <div><span>Reference</span><strong>{l.id.slice(0, 8)}</strong></div>
              </div>
            </>}
          </div>
          <aside className="location-card"><div className="map-pattern"><span><Icon name={bookable ? 'calendar' : 'location'} /></span></div><Heading level={3}>{bookable ? 'Plan your booking' : 'Pickup & delivery'}</Heading><p>{bookable ? 'Pick your dates to check live availability before you reserve.' : 'Arrange pickup or delivery with the seller after ordering.'}</p>{bookable ? <ActionLink className="outline-button" href={`/book/${encodeURIComponent(l.id)}`}>Check availability</ActionLink> : <ActionLink className="outline-button" href={`/conversation?q=${encodeURIComponent(`How do I get ${l.title}?`)}`}>Ask about delivery</ActionLink>}</aside>
        </div>
        {(related.data?.items.filter((r) => r.listing_id !== l.id).length ?? 0) > 0 && (
          <section className="app-rail" aria-label="More options">
            <div className="rail-head"><div><Heading>{bookable ? 'More options nearby' : 'You may also like'}</Heading><p>Similar listings on Trezutil</p></div><ActionLink href={`/search?q=${encodeURIComponent(l.title)}`}>See all <Icon name="arrow" size={17} /></ActionLink></div>
            <div className="results-grid">{related.data!.items.filter((r) => r.listing_id !== l.id).slice(0, 4).map((item) => <ResultCard item={item} key={item.listing_id} />)}</div>
          </section>
        )}
      </section>
    </AppShell>
  )
}
