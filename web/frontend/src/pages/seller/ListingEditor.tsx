import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { searchApi } from '@/api/search'
import { sellerApi } from '@/api/seller'
import { ListingImage } from '@/components/cards'
import { EmptyState, ErrorState, Loading } from '@/components/states'
import { ActionLink, Button, Field, Heading, Icon, type IconName } from '@/components/ui'
import { formatMoney, safeImage, sentence } from '@/lib/format'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { useSeller } from '@/pages/seller/SellerShell'
import type { ListingDetail, ListingType } from '@/types'

const listingTypes: { value: ListingType; label: string; icon: IconName }[] = [
  { value: 'physical_product', label: 'Product', icon: 'shop' },
  { value: 'service', label: 'Service', icon: 'service' },
  { value: 'food', label: 'Food', icon: 'food' },
  { value: 'real_estate', label: 'Real estate & stays', icon: 'home' },
  { value: 'vehicle', label: 'Vehicle', icon: 'car' },
  { value: 'other', label: 'Other', icon: 'spark' },
]

const steps = ['Basic information', 'Category', 'Price', 'Images', 'Availability', 'Publish'] as const

interface Draft {
  title: string
  description: string
  listing_type: ListingType
  category_id: string
  price: string
  currency: string
  pricing_model: string
  images: string
  availability: 'available' | 'on_request'
  quantity: string
  store_id: string
}

function validate(step: number, d: Draft): string | null {
  if (step === 1 && (!d.title.trim() || d.title.length > 255)) return 'Give your listing a name (up to 255 characters).'
  if (step === 2 && !d.category_id) return 'Choose a category.'
  if (step === 3 && !(Number(d.price) > 0)) return 'Enter a price greater than zero.'
  if (step === 4) {
    const urls = d.images.split(/\s+/).filter(Boolean)
    if (urls.some((u) => !/^https:\/\//i.test(u) || !safeImage(u))) return 'Image links must be public https:// addresses.'
    if (urls.length > 8) return 'Use up to 8 images.'
  }
  if (step === 5 && d.availability === 'available' && d.quantity && !(Number.isInteger(Number(d.quantity)) && Number(d.quantity) > 0)) return 'Stock must be a whole number.'
  return null
}

/** Figma CreateListingPage: a six-step flow ending in a real draft (and optional publish). */
export function CreateListingPage() {
  const { org, canManage } = useSeller()
  const notify = useToast()
  const categories = useAsync(() => searchApi.categories(), [])
  const mine = org.seller_profiles.find((p) => p.mine) ?? org.seller_profiles[0]
  const [step, setStep] = useState(1)
  const [draft, setDraft] = useState<Draft>({ title: '', description: '', listing_type: 'physical_product', category_id: '', price: '', currency: 'USD', pricing_model: 'fixed', images: '', availability: 'available', quantity: '', store_id: org.stores[0]?.id ?? '' })
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState<'draft' | 'publish' | null>(null)
  const [created, setCreated] = useState<ListingDetail | null>(null)
  const idempotencyKey = useMemo(() => crypto.randomUUID(), [])
  useEffect(() => { document.title = 'Create a listing — Trezutil' }, [])

  if (!canManage) return <EmptyState icon="user" title="View only" copy="Only the business owner, admins and managers can create listings." action="Back to listings" href="/seller/listings" />
  if (!mine || org.stores.length === 0) return <EmptyState icon="building" title="Setup not finished" copy="A seller profile and a store are needed before creating listings. The Trezutil team completes this during onboarding." action="See business details" href="/seller/business" />

  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => { setDraft((d) => ({ ...d, [key]: value })); setError(null) }
  const next = () => {
    const problem = validate(step, draft)
    if (problem) { setError(problem); return }
    setStep((s) => Math.min(steps.length, s + 1))
  }

  const save = async (publish: boolean) => {
    for (let s = 1; s <= 5; s++) { const problem = validate(s, draft); if (problem) { setStep(s); setError(problem); return } }
    setSaving(publish ? 'publish' : 'draft')
    setError(null)
    try {
      const listing = await sellerApi.createListing({
        store_id: draft.store_id, seller_id: mine.id, category_id: draft.category_id,
        title: draft.title.trim(), description: draft.description.trim(), listing_type: draft.listing_type,
        pricing_model: draft.pricing_model, price_amount: Number(draft.price), price_currency: draft.currency,
        image_urls: draft.images.split(/\s+/).filter(Boolean), availability: draft.availability,
        quantity: draft.quantity ? Number(draft.quantity) : undefined,
      }, idempotencyKey)
      if (publish) await sellerApi.setListingState(listing.id, 'publish')
      setCreated({ ...listing, status: publish ? 'published' : listing.status })
      notify(publish ? 'Your listing is live.' : 'Draft saved.', 'success')
    } catch (err) {
      setError(toApiError(err).message)
    } finally {
      setSaving(null)
    }
  }

  if (created) {
    return (
      <div className="create-success">
        <EmptyState icon="check" title={created.status === 'published' ? 'Your listing is live' : 'Draft saved'} copy={created.status === 'published' ? `${created.title} can now be discovered in search, on the web and in conversations.` : `${created.title} is saved as a draft. Publish it when you’re ready.`} action={created.status === 'published' ? 'View listing' : 'Open listing'} href={created.status === 'published' ? `/products/${encodeURIComponent(created.id)}` : `/seller/listings/${encodeURIComponent(created.id)}`} />
        <ActionLink href="/seller/listings">Back to listings</ActionLink>
      </div>
    )
  }

  const images = draft.images.split(/\s+/).filter(Boolean)
  const categoryName = categories.data?.find((c) => c.id === draft.category_id)?.name

  return (
    <>
      <div className="create-head"><ActionLink href="/seller/listings">← Back to listings</ActionLink><Heading level={1}>Create a listing</Heading><p>Clear details help Trezutil match your listing with the right people.</p></div>
      <div className="create-progress" role="list" aria-label="Steps">
        {steps.map((item, i) => <div role="listitem" className={step >= i + 1 ? 'active' : ''} key={item} aria-current={step === i + 1 ? 'step' : undefined}><span>{i + 1}</span><small>{item}</small></div>)}
      </div>
      <div className="create-layout">
        <form className="create-form" noValidate onSubmit={(e: FormEvent) => { e.preventDefault(); if (step < steps.length) next() }}>
          <Heading level={2}>{['Basic information', 'Choose a category', 'Set your price', 'Add clear images', 'Set availability', 'Ready to publish'][step - 1]}</Heading>
          {step === 1 && <div className="form-stack">
            <Field label="Listing name"><input value={draft.title} onChange={(e) => set('title', e.target.value)} maxLength={255} placeholder="Samsung Galaxy A55" autoFocus /></Field>
            <Field label="Description" hint="What makes it useful, its condition and what’s included."><textarea value={draft.description} onChange={(e) => set('description', e.target.value)} maxLength={4000} /></Field>
            {org.stores.length > 1 && <Field label="Store"><select value={draft.store_id} onChange={(e) => set('store_id', e.target.value)}>{org.stores.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select></Field>}
          </div>}
          {step === 2 && <>
            <fieldset className="category-choice" aria-label="Listing type">
              {listingTypes.map((t) => <label key={t.value}><input type="radio" name="type" checked={draft.listing_type === t.value} onChange={() => set('listing_type', t.value)} /><span><Icon name={t.icon} /><strong>{t.label}</strong></span></label>)}
            </fieldset>
            <Field label="Category">
              {categories.loading ? <select disabled><option>Loading…</option></select> : (
                <select value={draft.category_id} onChange={(e) => set('category_id', e.target.value)}>
                  <option value="">Choose a category</option>
                  {(categories.data ?? []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
                </select>
              )}
            </Field>
            {categories.data?.length === 0 && <p className="field-hint">No categories are configured yet — ask the Trezutil team to add one.</p>}
          </>}
          {step === 3 && <div className="form-grid">
            <Field label="Currency"><select value={draft.currency} onChange={(e) => set('currency', e.target.value)}><option value="USD">USD ($)</option><option value="CDF">CDF (FC)</option></select></Field>
            <Field label="Price"><input type="number" inputMode="decimal" min="0.01" step="0.01" value={draft.price} onChange={(e) => set('price', e.target.value)} placeholder="349" /></Field>
            <Field label="Price type"><select value={draft.pricing_model} onChange={(e) => set('pricing_model', e.target.value)}><option value="fixed">Fixed price</option><option value="negotiable">Negotiable</option></select></Field>
          </div>}
          {step === 4 && <div className="form-stack">
            <div className="image-upload"><Icon name="shop" size={30} /><strong>Add image links</strong><span>Paste public https:// image addresses, one per line. The first is the cover. Up to 8.</span></div>
            <Field label="Image links" hint="Direct photo upload isn’t available yet — use links to images you host."><textarea value={draft.images} onChange={(e) => set('images', e.target.value)} placeholder="https://…/photo.jpg" /></Field>
            {images.length > 0 && <div className="image-previews">{images.slice(0, 8).map((u) => <div key={u}><ListingImage src={u} alt="Preview" type={draft.listing_type} /></div>)}</div>}
          </div>}
          {step === 5 && <div className="form-stack">
            <label className="choice-card"><input type="radio" name="availability" checked={draft.availability === 'available'} onChange={() => set('availability', 'available')} /><span><strong>Available now</strong><small>Customers can order or book immediately</small></span></label>
            <label className="choice-card"><input type="radio" name="availability" checked={draft.availability === 'on_request'} onChange={() => set('availability', 'on_request')} /><span><strong>Available on request</strong><small>Confirm availability before the customer acts</small></span></label>
            {draft.availability === 'available' && <Field label="Stock quantity" hint="Leave empty if you don’t track stock."><input type="number" min="1" step="1" value={draft.quantity} onChange={(e) => set('quantity', e.target.value)} placeholder="12" /></Field>}
          </div>}
          {step === 6 && <div className="publish-review">
            <span className="empty-mark"><Icon name="spark" /></span>
            <strong>Everything looks ready.</strong>
            <p>Publish now to be visible in search, discovery and relevant conversations — or save a draft and publish later.</p>
            <div className="info-rows"><div><span>Name</span><strong>{draft.title}</strong></div><div><span>Type</span><strong>{sentence(draft.listing_type)}{categoryName ? ` · ${categoryName}` : ''}</strong></div><div><span>Price</span><strong>{formatMoney(Number(draft.price), draft.currency)}{draft.pricing_model === 'negotiable' ? ' (negotiable)' : ''}</strong></div><div><span>Images</span><strong>{images.length}</strong></div></div>
          </div>}
          {error && <p className="auth-error" role="alert">{error}</p>}
          <div className="checkout-buttons">
            {step > 1 && <Button className="outline-button" onClick={() => { setError(null); setStep(step - 1) }}>Back</Button>}
            {step < steps.length && <Button className="primary-button" type="submit">Save and continue<Icon name="arrow" /></Button>}
            {step === steps.length && <>
              <Button className="outline-button" busy={saving === 'draft'} disabled={!!saving} onClick={() => void save(false)}>Save as draft</Button>
              <Button className="primary-button" busy={saving === 'publish'} disabled={!!saving} onClick={() => void save(true)}>Publish listing<Icon name="arrow" /></Button>
            </>}
          </div>
        </form>
        <aside className="listing-preview" aria-label="Live preview">
          <span>Live preview</span>
          <article className="result-card result-grid">
            <div className="result-image"><ListingImage src={images[0]} alt="" type={draft.listing_type} /><span className="availability"><span />{draft.availability === 'available' ? 'Available now' : 'On request'}</span></div>
            <div className="result-body"><div className="product-meta">{org.display_name}</div><Heading level={3}>{draft.title || 'Your listing name'}</Heading><strong className="price">{Number(draft.price) > 0 ? formatMoney(Number(draft.price), draft.currency) : '—'}</strong></div>
          </article>
        </aside>
      </div>
    </>
  )
}

/** Edits a draft or paused listing (live listings must be paused first). */
export function EditListingPage() {
  const { id = '' } = useParams()
  const { org, canManage } = useSeller()
  const navigate = useNavigate()
  const notify = useToast()
  const listing = useAsync(async () => (await sellerApi.listings(org.organization_id)).find((l) => l.id === id) ?? null, [id, org.organization_id])
  const [form, setForm] = useState<{ title: string; description: string; price: string } | null>(null)
  const [error, setError] = useState<ApiError | string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const l = listing.data
    if (l) setForm({ title: l.title, description: l.description ?? '', price: String(l.price_amount) })
  }, [listing.data])

  if (!canManage) return <EmptyState icon="user" title="View only" copy="Only the business owner, admins and managers can edit listings." action="Back to listings" href="/seller/listings" />
  if (listing.loading || (listing.data && !form)) return <Loading label="Loading listing…" />
  if (listing.error) return <ErrorState error={listing.error} onRetry={listing.reload} />
  const l = listing.data
  if (!l || !form) return <EmptyState icon="search" title="Listing not found" copy="It doesn’t belong to this business or was removed." action="Back to listings" href="/seller/listings" />
  const editable = ['draft', 'pending_review', 'paused'].includes(l.status)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const price = Number(form.price)
    if (!form.title.trim()) { setError('A title is required.'); return }
    if (!(price > 0)) { setError('Enter a price greater than zero.'); return }
    setSaving(true); setError(null)
    try {
      await sellerApi.updateListing(l.id, { title: form.title.trim(), description: form.description, price_amount: price })
      notify('Changes saved.', 'success')
      navigate(`/seller/listings/${encodeURIComponent(l.id)}`)
    } catch (err) {
      const apiErr = toApiError(err)
      setError(apiErr.code === 'LISTING_NOT_EDITABLE' ? 'Pause this listing before editing it.' : apiErr)
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <div className="create-head"><ActionLink href={`/seller/listings/${encodeURIComponent(l.id)}`}>← Back to listing</ActionLink><Heading level={1}>Edit listing</Heading><p>Existing orders keep the price the customer agreed to.</p></div>
      {!editable && <p className="inline-notice info" role="note"><Icon name="bell" size={16} /><span>This listing is live. Pause it from the listings page before editing.</span></p>}
      <form className="create-form edit-form" noValidate onSubmit={submit}>
        <Field label="Listing name"><input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} maxLength={255} disabled={!editable} /></Field>
        <Field label="Description"><textarea value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} maxLength={4000} disabled={!editable} /></Field>
        <Field label={`Price (${l.price_currency})`}><input type="number" min="0.01" step="0.01" value={form.price} onChange={(e) => setForm({ ...form, price: e.target.value })} disabled={!editable} /></Field>
        {error && (typeof error === 'string' ? <p className="auth-error" role="alert">{error}</p> : <ErrorState error={error} compact />)}
        <div className="checkout-buttons"><ActionLink className="outline-button" href={`/seller/listings/${encodeURIComponent(l.id)}`}>Cancel</ActionLink><Button className="primary-button" type="submit" busy={saving} disabled={!editable}>Save changes<Icon name="arrow" /></Button></div>
      </form>
    </>
  )
}
