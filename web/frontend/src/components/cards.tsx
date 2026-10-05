import { useState } from 'react'
import { ActionLink, Button, Heading, Icon, type IconName } from '@/components/ui'
import { usePrefs, type SavedListing } from '@/lib/prefs'
import { detailPath, formatDistance, formatMoney, place, safeImage } from '@/lib/format'
import { saleOf, type Sale } from '@/lib/promo'
import type { SearchResult } from '@/types'
import { t } from '@/lib/i18n'

const typeIcon: Record<string, IconName> = {
  physical_product: 'shop', real_estate: 'home', vehicle: 'car', food: 'food', service: 'service', other: 'spark',
}

/** Listing image with a lazy load and a branded fallback when there is no (valid) media. */
export function ListingImage({ src, alt, type, eager = false }: { src: string | null | undefined; alt: string; type?: string; eager?: boolean }) {
  const [failed, setFailed] = useState(false)
  const url = safeImage(src)
  if (!url || failed) {
    return <div className="listing-fallback" {...(alt ? { role: 'img', 'aria-label': alt } : { 'aria-hidden': true })}><Icon name={typeIcon[type ?? ''] ?? 'shop'} size={34} /></div>
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
    <Button className={`save-button ${light ? 'light' : ''} ${saved ? 'saved' : ''}`} ariaLabel={saved ? t('Remove {title} from saved', { title: item.title }) : t('Save {title}', { title: item.title })} pressed={saved} onClick={() => toggleSaved(item)}>
      <Icon name="heart" size={18} />
    </Button>
  )
}

/** "−20%" on the image of a listing that is on sale. */
export function DiscountBadge({ percent }: { percent: number }) {
  return <span className="discount-badge" role="img" aria-label={t('{percent}% off', { percent })}><span aria-hidden="true">−{percent}%</span></span>
}

/** The selling price, with the regular price struck through when the listing is on sale. */
export function PriceTag({ amount, currency, sale, className = '' }: { amount: number; currency: string; sale?: Sale | null; className?: string }) {
  if (!sale) return <strong className={`price ${className}`}>{formatMoney(amount, currency)}</strong>
  return (
    <span className={`price-tag on-sale ${className}`}>
      <strong className="price"><span className="sr-only">{t('Now')} </span>{formatMoney(amount, currency)}</strong>
      <s className="was-price"><span className="sr-only">{t('Regular price')} </span>{formatMoney(sale.compareAt, currency)}</s>
    </span>
  )
}

function availabilityText(status: string): string {
  if (!status || status === 'in_stock' || status === 'available') return t('Available now')
  if (status === 'pre_order') return t('On request')
  return status.replace(/_/g, ' ')
}

/** Figma ResultCard bound to a search result. */
export function ResultCard({ item, layout = 'grid', askHref }: { item: SearchResult; layout?: 'grid' | 'list'; askHref?: string }) {
  const href = detailPath(item.listing_id, undefined, item.vertical)
  const seller = item.store?.name || item.seller?.display_name || t('Local seller')
  const where = [formatDistance(item.distance_meters) && t('{distance} away', { distance: formatDistance(item.distance_meters) }), place(item)].filter(Boolean).join(' · ')
  const unavailable = item.availability_status === 'out_of_stock' || item.availability_status === 'unavailable'
  const sale = saleOf(item)
  return (
    <article className={`result-card result-${layout}`}>
      <ActionLink className="result-image" href={href} ariaLabel={item.title}>
        <ListingImage src={item.primary_media_url} alt={item.title} />
        {sale && <DiscountBadge percent={sale.percent} />}
        <span className={`availability ${unavailable ? 'is-unavailable' : ''}`}><span />{availabilityText(item.availability_status)}</span>
      </ActionLink>
      <div className="result-body">
        <div className="product-meta">{seller}{(item.seller?.trust_level ?? 0) > 0 && <span>{t('Verified')}</span>}</div>
        <ActionLink href={href}><Heading level={3}>{item.title}</Heading></ActionLink>
        <PriceTag amount={item.price_amount} currency={item.price_currency} sale={sale} />
        {where && <div className="distance"><Icon name="location" size={15} />{where}</div>}
        <div className="result-actions">
          <ActionLink className="small-primary" href={href}>{t('View details')}</ActionLink>
          <ActionLink className="ask-mini" href={askHref ?? `/conversation?q=${encodeURIComponent(t('Tell me more about {title}', { title: item.title }))}`}><Icon name="spark" size={15} />{t('Ask')}</ActionLink>
        </div>
      </div>
      <SaveButton item={toSaved(item)} />
    </article>
  )
}
