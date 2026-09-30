import { useState } from 'react'
import { ActionLink, Button, Heading, Icon, type IconName } from '@/components/ui'
import { usePrefs, type SavedListing } from '@/lib/prefs'
import { detailPath, formatDistance, formatMoney, place, safeImage } from '@/lib/format'
import type { SearchResult } from '@/types'

const typeIcon: Record<string, IconName> = {
  physical_product: 'shop', real_estate: 'home', vehicle: 'car', food: 'food', service: 'service', other: 'spark',
}

/** Listing image with a lazy load and a branded fallback when there is no (valid) media. */
export function ListingImage({ src, alt, type, eager = false }: { src: string | null | undefined; alt: string; type?: string; eager?: boolean }) {
  const [failed, setFailed] = useState(false)
  const url = safeImage(src)
  if (!url || failed) {
    return <div className="listing-fallback" role="img" aria-label={alt}><Icon name={typeIcon[type ?? ''] ?? 'spark'} size={34} /></div>
  }
  return <img src={url} alt={alt} loading={eager ? 'eager' : 'lazy'} decoding="async" onError={() => setFailed(true)} />
}

export function toSaved(r: SearchResult, listingType?: string): SavedListing {
  return {
    listing_id: r.listing_id, title: r.title, price_amount: r.price_amount, price_currency: r.price_currency,
    image: r.primary_media_url, seller: r.store?.name || r.seller?.display_name || '', listing_type: listingType,
    vertical: r.vertical || undefined,
  }
}

export function SaveButton({ item, light = false }: { item: SavedListing; light?: boolean }) {
  const { isSaved, toggleSaved } = usePrefs()
  const saved = isSaved(item.listing_id)
  return (
    <Button className={`save-button ${light ? 'light' : ''} ${saved ? 'saved' : ''}`} ariaLabel={saved ? `Remove ${item.title} from saved` : `Save ${item.title}`} pressed={saved} onClick={() => toggleSaved(item)}>
      <Icon name="heart" size={18} />
    </Button>
  )
}

function availabilityText(status: string): string {
  if (!status || status === 'in_stock' || status === 'available') return 'Available now'
  if (status === 'pre_order') return 'On request'
  return status.replace(/_/g, ' ')
}

/** Figma ResultCard bound to a search result. */
export function ResultCard({ item, layout = 'grid', askHref }: { item: SearchResult; layout?: 'grid' | 'list'; askHref?: string }) {
  const href = detailPath(item.listing_id, undefined, item.vertical)
  const seller = item.store?.name || item.seller?.display_name || 'Local seller'
  const where = [formatDistance(item.distance_meters) && `${formatDistance(item.distance_meters)} away`, place(item)].filter(Boolean).join(' · ')
  const unavailable = item.availability_status === 'out_of_stock' || item.availability_status === 'unavailable'
  return (
    <article className={`result-card result-${layout}`}>
      <ActionLink className="result-image" href={href} ariaLabel={item.title}>
        <ListingImage src={item.primary_media_url} alt={item.title} />
        <span className={`availability ${unavailable ? 'is-unavailable' : ''}`}><span />{availabilityText(item.availability_status)}</span>
      </ActionLink>
      <div className="result-body">
        <div className="product-meta">{seller}{(item.seller?.trust_level ?? 0) > 0 && <span>Verified</span>}</div>
        <ActionLink href={href}><Heading level={3}>{item.title}</Heading></ActionLink>
        <strong className="price">{formatMoney(item.price_amount, item.price_currency)}</strong>
        {where && <div className="distance"><Icon name="location" size={15} />{where}</div>}
        <div className="result-actions">
          <ActionLink className="small-primary" href={href}>View details</ActionLink>
          <ActionLink className="ask-mini" href={askHref ?? `/conversation?q=${encodeURIComponent(`Tell me more about ${item.title}`)}`}><Icon name="spark" size={15} />Ask</ActionLink>
        </div>
      </div>
      <SaveButton item={toSaved(item)} />
    </article>
  )
}
