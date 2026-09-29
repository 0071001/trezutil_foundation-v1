import { useState } from 'react'
import { useParams } from 'react-router'
import { toApiError } from '@/api/client'
import { sellerApi } from '@/api/seller'
import { ListingImage } from '@/components/cards'
import { EmptyState, ErrorState, Loading, SkeletonGrid } from '@/components/states'
import { ActionLink, Button, Heading, Icon, StatusBadge, Tabs } from '@/components/ui'
import { formatDateTime, formatMoney, sentence, timeAgo } from '@/lib/format'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { SellerHead, ViewOnly, useSeller } from '@/pages/seller/SellerShell'
import type { ListingDetail } from '@/types'

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
      notify(action === 'pause' ? `“${l.title}” is paused and hidden from search.` : `“${l.title}” is live and searchable.`, 'success')
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
      {(l.status === 'draft' || l.status === 'pending_review') && <Button className="small-primary" busy={busy} onClick={() => change(l, 'publish')}>Publish</Button>}
      {l.status === 'published' && <Button className="outline-button" busy={busy} onClick={() => change(l, 'pause')}>Pause</Button>}
      {l.status === 'paused' && <Button className="small-primary" busy={busy} onClick={() => change(l, 'resume')}>Resume</Button>}
      {editable(l) && <ActionLink className="outline-button" href={`/seller/listings/${encodeURIComponent(l.id)}/edit`}>Edit</ActionLink>}
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
      <SellerHead eyebrow="Your catalogue" title="Listings" copy="Keep what customers can discover accurate and available." action={canManage ? <ActionLink className="primary-button" href="/seller/listings/new">Create listing <Icon name="arrow" /></ActionLink> : undefined} />
      {!canManage && <ViewOnly />}
      <Tabs label="Listing status" items={(['Live', 'Paused', 'Drafts', 'Unavailable'] as ListingTab[]).map((t) => ({ value: t, label: t, count: listings.data ? all.filter((l) => tabOf(l) === t).length : undefined }))} value={tab} onChange={setTab} />
      {listings.loading && <SkeletonGrid count={3} layout="list" />}
      {listings.error && <ErrorState error={listings.error} onRetry={listings.reload} />}
      {listings.data && rows.length === 0 && <EmptyState icon="shop" title={tab === 'Live' ? 'Nothing live yet' : `No ${tab.toLowerCase()} listings`} copy={tab === 'Live' ? 'Create a listing and publish it to appear in search, discovery and conversations.' : 'Listings move here as their status changes.'} action={canManage && tab === 'Live' ? 'Create listing' : undefined} href="/seller/listings/new" />}
      {rows.length > 0 && (
        <div className="listing-table" role="table" aria-label={`${tab} listings`}>
          <div className="table-head" role="row"><span role="columnheader">Listing</span><span role="columnheader">Price</span><span role="columnheader">Status</span><span role="columnheader">Updated</span><span role="columnheader"><span className="sr-only">Actions</span></span></div>
          {rows.map((l) => (
            <div className="listing-row" role="row" key={l.id}>
              <div role="cell"><ListingImage src={l.media[0]?.url} alt="" type={l.listing_type} /><span><ActionLink href={`/seller/listings/${encodeURIComponent(l.id)}`}><strong>{l.title}</strong></ActionLink><small>{sentence(l.listing_type)}</small></span></div>
              <b role="cell">{formatMoney(l.price_amount, l.price_currency)}</b>
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
  if (data.loading) return <Loading label="Loading listing…" />
  if (data.error) return <ErrorState error={data.error} onRetry={data.reload} />
  const l = data.data?.listing
  if (!l) return <EmptyState icon="search" title="Listing not found" copy="It doesn’t belong to this business or was removed." action="Back to listings" href="/seller/listings" />
  return (
    <>
      <div className="create-head"><ActionLink href="/seller/listings">← Back to listings</ActionLink></div>
      <div className="seller-listing-detail">
        <div className="gallery-main"><ListingImage src={l.media[0]?.url} alt={l.title} type={l.listing_type} eager /></div>
        <div>
          <StatusBadge status={l.status} />
          <Heading level={1}>{l.title}</Heading>
          <strong className="detail-price">{formatMoney(l.price_amount, l.price_currency)}</strong>
          <p>{l.description || 'No description yet.'}</p>
          <ListingActions l={l} canManage={canManage} busy={busy === l.id} change={change} />
          {l.status === 'published' && <ActionLink className="text-action" href={`/products/${encodeURIComponent(l.id)}`}>View as a customer <Icon name="arrow" size={16} /></ActionLink>}
          <div className="info-rows">
            <div><span>Type</span><strong>{sentence(l.listing_type)}</strong></div>
            <div><span>Pricing</span><strong>{sentence(l.pricing_model || 'fixed')}</strong></div>
            <div><span>Created</span><strong>{formatDateTime(l.created_at)}</strong></div>
            {l.published_at && <div><span>First published</span><strong>{formatDateTime(l.published_at)}</strong></div>}
            {data.data!.stock.map((s) => <div key={s.id}><span>Stock</span><strong>{s.available} available · {s.reserved} reserved · {s.consumed} sold</strong></div>)}
          </div>
          {editable(l) ? null : canManage && l.status === 'published' && <p className="field-hint">Pause the listing to edit its details. Existing orders keep the price the customer agreed to.</p>}
        </div>
      </div>
    </>
  )
}
