import { useState, type FormEvent } from 'react'
import { useParams } from 'react-router'
import { toApiError } from '@/api/client'
import { sellerApi } from '@/api/seller'
import { ListingImage, PriceTag } from '@/components/cards'
import { EmptyState, ErrorState, Loading, SkeletonGrid } from '@/components/states'
import { ActionLink, Button, Field, Heading, Icon, StatusBadge, Tabs } from '@/components/ui'
import { formatDateTime, formatMoney, sentence, timeAgo } from '@/lib/format'
import { discountPercent, listingSale } from '@/lib/promo'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { SellerHead, ViewOnly, useSeller } from '@/pages/seller/SellerShell'
import type { ListingDetail } from '@/types'
import { t } from '@/lib/i18n'

type ListingTab = 'Live' | 'Paused' | 'Drafts' | 'Unavailable'
const tabOf = (l: ListingDetail): ListingTab =>
  l.status === 'published' ? 'Live' : l.status === 'paused' ? 'Paused' : l.status === 'draft' || l.status === 'pending_review' ? 'Drafts' : 'Unavailable'

const editable = (l: ListingDetail) => ['draft', 'pending_review', 'paused'].includes(l.status)

function useListingActions(reload: () => void) {
  const notify = useToast()
  const [busy, setBusy] = useState<string | null>(null)
  const change = async (l: ListingDetail, action: 'publish' | 'pause' | 'resume') => {
    setBusy(l.id)
    try {
      await sellerApi.setListingState(l.id, action)
      notify(action === 'pause' ? t('“{title}” is paused and hidden from search.', { title: l.title }) : t('“{title}” is live and searchable.', { title: l.title }), 'success')
      reload()
    } catch (err) {
      notify(toApiError(err).message, 'error')
    } finally {
      setBusy(null)
    }
  }
  return { busy, change }
}

function ListingActions({ l, canManage, busy, change }: { l: ListingDetail; canManage: boolean; busy: boolean; change: (l: ListingDetail, a: 'publish' | 'pause' | 'resume') => void }) {
  if (!canManage) return null
  return (
    <div className="row-actions">
      {(l.status === 'draft' || l.status === 'pending_review') && <Button className="small-primary" busy={busy} onClick={() => change(l, 'publish')}>{t('Publish')}</Button>}
      {l.status === 'published' && <Button className="outline-button" busy={busy} onClick={() => change(l, 'pause')}>{t('Pause')}</Button>}
      {l.status === 'paused' && <Button className="small-primary" busy={busy} onClick={() => change(l, 'resume')}>{t('Resume')}</Button>}
      {editable(l) && <ActionLink className="outline-button" href={`/seller/listings/${encodeURIComponent(l.id)}/edit`}>{t('Edit')}</ActionLink>}
    </div>
  )
}

/** Figma SellerListingsPage (listing-table) on the organization's real listings. */
export function SellerListingsPage() {
  const { org, canManage } = useSeller()
  const listings = useAsync(() => sellerApi.listings(org.organization_id), [org.organization_id])
  const [tab, setTab] = useState<ListingTab>('Live')
  const { busy, change } = useListingActions(listings.reload)
  const all = listings.data ?? []
  const rows = all.filter((l) => tabOf(l) === tab)
  return (
    <>
      <SellerHead eyebrow={t('Your catalogue')} title={t('Listings')} copy={t('Keep what customers can discover accurate and available.')} action={canManage ? <ActionLink className="primary-button" href="/seller/listings/new">{t('Create listing')} <Icon name="arrow" /></ActionLink> : undefined} />
      {!canManage && <ViewOnly />}
      <Tabs label={t('Listing status')} items={(['Live', 'Paused', 'Drafts', 'Unavailable'] as ListingTab[]).map((t) => ({ value: t, label: t, count: listings.data ? all.filter((l) => tabOf(l) === t).length : undefined }))} value={tab} onChange={setTab} />
      {listings.loading && <SkeletonGrid count={3} layout="list" />}
      {listings.error && <ErrorState error={listings.error} onRetry={listings.reload} />}
      {listings.data && rows.length === 0 && <EmptyState icon="shop" title={tab === 'Live' ? t('Nothing live yet') : ({ Paused: t('No paused listings'), Drafts: t('No draft listings'), Unavailable: t('No unavailable listings'), Live: t('Nothing live yet') } as Record<string, string>)[tab]} copy={tab === 'Live' ? t('Create a listing and publish it to appear in search, discovery and conversations.') : t('Listings move here as their status changes.')} action={canManage && tab === 'Live' ? t('Create listing') : undefined} href="/seller/listings/new" />}
      {rows.length > 0 && (
        <div className="listing-table" role="table" aria-label={t('Listings')}>
          <div className="table-head" role="row"><span role="columnheader">{t('Listing')}</span><span role="columnheader">{t('Price')}</span><span role="columnheader">{t('Status')}</span><span role="columnheader">{t('Updated')}</span><span role="columnheader"><span className="sr-only">{t('Actions')}</span></span></div>
          {rows.map((l) => (
            <div className="listing-row" role="row" key={l.id}>
              <div role="cell"><ListingImage src={l.media[0]?.url} alt="" type={l.listing_type} /><span><ActionLink href={`/seller/listings/${encodeURIComponent(l.id)}`}><strong>{l.title}</strong></ActionLink><small>{sentence(l.listing_type)}</small></span></div>
              <b role="cell"><PriceTag amount={l.price_amount} currency={l.price_currency} sale={listingSale(l)} /></b>
              <span role="cell"><StatusBadge status={l.status} /></span>
              <span role="cell">{timeAgo(l.updated_at)}</span>
              <span role="cell"><ListingActions l={l} canManage={canManage} busy={busy === l.id} change={change} /></span>
            </div>
          ))}
        </div>
      )}
    </>
  )
}

/** A sale can be started or ended while a listing is being prepared, live or paused. */
const saleAllowed = (l: ListingDetail) => ['draft', 'pending_review', 'published', 'paused'].includes(l.status)

/**
 * Puts a listing on sale (or changes / ends the sale). The listing sells at the
 * sale price, so orders charge exactly what customers see; ending the sale
 * restores the regular price. It works on a live listing, no pause needed.
 */
function PromotionPanel({ l, canManage, onChanged }: { l: ListingDetail; canManage: boolean; onChanged: () => void }) {
  const notify = useToast()
  const sale = listingSale(l)
  const [price, setPrice] = useState('')
  const [busy, setBusy] = useState(false)
  if (!saleAllowed(l)) return null
  const regular = sale?.compareAt ?? l.price_amount
  const amount = Math.round(Number(price.replace(',', '.')) * 100) / 100
  const valid = Number.isFinite(amount) && amount > 0 && discountPercent(regular, amount) >= 1
  const hint = price !== '' && !valid ? t('Enter a price below {regular} (at least 1% less).', { regular: formatMoney(regular, l.price_currency) }) : undefined

  const run = async (action: () => Promise<unknown>, done: string) => {
    setBusy(true)
    try {
      await action()
      setPrice('')
      notify(done, 'success')
      onChanged()
    } catch (err) {
      notify(toApiError(err).message, 'error')
    } finally {
      setBusy(false)
    }
  }
  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (valid) void run(() => sellerApi.setPromotion(l.id, amount), t('The promotion is live.'))
  }

  return (
    <section className="promotion-panel" aria-labelledby="promotion-title">
      <Heading level={3} id="promotion-title">{t('Promotion')}</Heading>
      {sale
        ? <p>{t('On sale: {percent}% off the regular price of {regular}.', { percent: sale.percent, regular: formatMoney(sale.compareAt, l.price_currency) })}</p>
        : <p>{t('Not on promotion. Put it on sale to show a discount badge in search and on the homepage.')}</p>}
      {canManage && (
        <form className="promotion-form" noValidate onSubmit={submit}>
          <Field label={t('Sale price ({price_currency})', { price_currency: l.price_currency })} error={hint}>
            <input type="number" inputMode="decimal" min="0.01" step="0.01" value={price} onChange={(e) => setPrice(e.target.value)} placeholder={String(Math.round(regular * 0.8 * 100) / 100)} />
          </Field>
          <div className="row-actions">
            <Button className="small-primary" type="submit" busy={busy} disabled={!valid}>{sale ? t('Change sale price') : t('Start promotion')}</Button>
            {sale && <Button className="outline-button" busy={busy} onClick={() => void run(() => sellerApi.endPromotion(l.id), t('The promotion has ended; the regular price is back.'))}>{t('End promotion')}</Button>}
          </div>
        </form>
      )}
    </section>
  )
}

/** Seller view of one listing: status, actions and stock. */
export function SellerListingDetailPage() {
  const { id = '' } = useParams()
  const { org, canManage } = useSeller()
  const data = useAsync(async () => {
    const [listings, inventory] = await Promise.all([
      sellerApi.listings(org.organization_id),
      canManage ? sellerApi.inventory(org.organization_id).catch(() => []) : Promise.resolve([]),
    ])
    return { listing: listings.find((l) => l.id === id), stock: inventory.filter((i) => i.listing_id === id) }
  }, [id, org.organization_id])
  const { busy, change } = useListingActions(data.reload)
  if (data.loading) return <Loading label={t('Loading listing…')} />
  if (data.error) return <ErrorState error={data.error} onRetry={data.reload} />
  const l = data.data?.listing
  if (!l) return <EmptyState icon="search" title={t('Listing not found')} copy={t('It doesn’t belong to this business or was removed.')} action={t('Back to listings')} href="/seller/listings" />
  return (
    <>
      <div className="create-head"><ActionLink href="/seller/listings">{t('← Back to listings')}</ActionLink></div>
      <div className="seller-listing-detail">
        <div className="gallery-main"><ListingImage src={l.media[0]?.url} alt={l.title} type={l.listing_type} eager /></div>
        <div>
          <StatusBadge status={l.status} />
          <Heading level={1}>{l.title}</Heading>
          <PriceTag className="detail-price" amount={l.price_amount} currency={l.price_currency} sale={listingSale(l)} />
          <p>{l.description || t('No description yet.')}</p>
          <ListingActions l={l} canManage={canManage} busy={busy === l.id} change={change} />
          <PromotionPanel l={l} canManage={canManage} onChanged={data.reload} />
          {l.status === 'published' && <ActionLink className="text-action" href={`/products/${encodeURIComponent(l.id)}`}>{t('View as a customer')} <Icon name="arrow" size={16} /></ActionLink>}
          <div className="info-rows">
            <div><span>{t('Type')}</span><strong>{sentence(l.listing_type)}</strong></div>
            <div><span>{t('Pricing')}</span><strong>{sentence(l.pricing_model || 'fixed')}</strong></div>
            <div><span>{t('Created')}</span><strong>{formatDateTime(l.created_at)}</strong></div>
            {l.published_at && <div><span>{t('First published')}</span><strong>{formatDateTime(l.published_at)}</strong></div>}
            {data.data!.stock.map((s) => <div key={s.id}><span>{t('Stock')}</span><strong>{s.available} {t('available ·')} {s.reserved} {t('reserved ·')} {s.consumed} {t('sold')}</strong></div>)}
          </div>
          {editable(l) ? null : canManage && l.status === 'published' && <p className="field-hint">{t('Pause the listing to edit its details. Existing orders keep the price the customer agreed to.')}</p>}
        </div>
      </div>
    </>
  )
}
