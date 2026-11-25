import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { searchApi } from '@/api/search'
import { MANAGER_ROLES, sellerApi, type SellerOrganization } from '@/api/seller'
import { formatMoney } from '@/lib/format'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { Icon } from '@/ui/Icon'
import { Badge, Empty, ErrorNote, Loading, newId } from '@/ui/kit'
import { t, tx } from '@/lib/i18n'

/*
 * "My offers": a supplier publishes what it sells, so buyers find it and can ask for its best price. The business
 * opens its shop once; each offer is created then published. Being registered is not being verified: the offer shows
 * the supplier's real trust level.
 */

const KINDS: [string, string, string][] = [
  ['physical_product', 'RETAIL', tx('Produit')],
  ['service', 'SERVICES', tx('Service')],
  ['food', 'RESTAURANT', tx('Restauration')],
  ['real_estate', 'REAL_ESTATE', tx('Immobilier')],
]

const STATUS: Record<string, [string, 'green' | 'amber' | 'neutral']> = {
  published: [t('Publiée'), 'green'], draft: [t('Brouillon'), 'amber'], paused: [t('En pause'), 'neutral'],
  pending_review: [t('En revue'), 'amber'], rejected: [t('Refusée'), 'neutral'], archived: [t('Archivée'), 'neutral'],
}

function NewOffer({ org, storeId, sellerId, onDone }: { org: SellerOrganization; storeId: string; sellerId: string; onDone: () => void }) {
  const categories = useAsync(() => searchApi.categories(), [])
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [price, setPrice] = useState('')
  const [currency, setCurrency] = useState('USD')
  const [kind, setKind] = useState(KINDS[0]![0])
  const [category, setCategory] = useState('')
  const [image, setImage] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  useEffect(() => { if (!category && categories.data?.length) setCategory(categories.data[0]!.id) }, [categories.data]) // eslint-disable-line react-hooks/exhaustive-deps
  const value = Number(price.replace(',', '.'))
  const imageOk = !image || /^https:\/\/\S+$/.test(image.trim())
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!title.trim() || !(value > 0) || !category || !imageOk || busy) return
    setBusy(true)
    setError(null)
    try {
      const k = KINDS.find(([type]) => type === kind)!
      const listing = await sellerApi.createListing({
        store_id: storeId, seller_id: sellerId, category_id: category, title: title.trim(), description: description.trim(),
        listing_type: k[0], vertical: k[1], pricing_model: 'fixed', price_amount: value, price_currency: currency,
        image_urls: image.trim() ? [image.trim()] : [], availability: 'available',
      }, newId())
      await sellerApi.setListingState(listing.id, 'publish')
      onDone()
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <form className="nego-form-inline offer-new" onSubmit={(e) => void submit(e)} aria-label={t('Nouvelle offre de {org}', { org: org.display_name })}>
      <h2 className="nego-h2">{t('Publier une offre')}</h2>
      <div className="nego-grid">
        <label className="wide">{t('Titre')}<input value={title} maxLength={200} onChange={(e) => setTitle(e.target.value)} required placeholder={t('Ex. Ciment CPJ 42.5 – sac de 50 kg')} /></label>
        <label>{t('Type')}<select value={kind} onChange={(e) => setKind(e.target.value)}>{KINDS.map(([type, , label]) => <option key={type} value={type}>{t(label)}</option>)}</select></label>
        <label>{t('Catégorie')}
          <select value={category} onChange={(e) => setCategory(e.target.value)} required>{(categories.data ?? []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select>
        </label>
        <label>{t('Prix par unité')}<input inputMode="decimal" value={price} onChange={(e) => setPrice(e.target.value)} required /></label>
        <label>{t('Devise')}<select value={currency} onChange={(e) => setCurrency(e.target.value)}><option value="USD">USD</option><option value="CDF">CDF</option></select></label>
        <label className="wide">{t('Description')}<textarea value={description} rows={3} maxLength={4000} onChange={(e) => setDescription(e.target.value)} /></label>
        <label className="wide">{t('Lien d’une photo (https://…, facultatif)')}<input value={image} onChange={(e) => setImage(e.target.value)} inputMode="url" /></label>
      </div>
      {!imageOk && <p className="form-error" role="alert">{t('Le lien de la photo doit commencer par https://')}</p>}
      {error && <ErrorNote error={error} />}
      <div className="modal-actions">
        <button type="submit" className="primary-btn" disabled={busy || !title.trim() || !(value > 0) || !category || !imageOk} aria-busy={busy}>{busy ? t('Publication…') : t('Publier l’offre')}</button>
      </div>
    </form>
  )
}

function OrgOffers({ org, onChange }: { org: SellerOrganization; onChange: () => void }) {
  const listings = useAsync(() => sellerApi.listings(org.organization_id), [org.organization_id])
  const [busy, setBusy] = useState('')
  const [error, setError] = useState<ApiError | null>(null)
  const seller = org.seller_profiles.find((s) => s.mine) ?? org.seller_profiles[0]
  const store = org.stores[0]
  const manager = MANAGER_ROLES.has(org.role)
  const act = async (id: string, fn: () => Promise<unknown>) => {
    setBusy(id)
    setError(null)
    try { await fn(); listings.reload(); onChange() } catch (err) { setError(toApiError(err)) } finally { setBusy('') }
  }
  return (
    <section className="nego-head">
      <div className="nego-status"><b>{org.display_name}</b><Badge tone={org.verification_status === 'verified' ? 'green' : 'neutral'}>{org.verification_status === 'verified' ? t('Vérifié') : t('Inscrit')}</Badge></div>
      {!manager ? <p className="nego-help">{t('Seuls le propriétaire, un administrateur ou un responsable de l’entreprise publient des offres.')}</p>
        : !seller ? <p className="nego-help">{t('Ton entreprise n’a pas encore de profil vendeur. Termine ton inscription fournisseur.')} <Link to="/supplier">{t('Devenir fournisseur')}</Link></p>
          : !store ? (
            <div className="nego-callout">
              <span>{t('Ouvre la boutique de ton entreprise pour y publier tes offres.')}</span>
              <button type="button" className="primary-btn small-btn" disabled={busy === 'store'} onClick={() => void act('store', () => sellerApi.createStore(org.organization_id, org.display_name, ''))}>{t('Ouvrir ma boutique')}</button>
            </div>
          ) : (
            <>
              {listings.loading && !listings.data ? <Loading /> : listings.error ? <ErrorNote error={listings.error} onRetry={listings.reload} />
                : listings.data?.length ? (
                  <div className="list">
                    {listings.data.map((l) => {
                      const [label, tone] = STATUS[l.status] ?? [l.status, 'neutral']
                      return (
                        <div key={l.id} className="list-row">
                          <span className="list-icon"><Icon name="orders" size={17} /></span>
                          <span className="list-copy"><b>{l.title}</b><small>{formatMoney(l.price_amount, l.price_currency)}</small></span>
                          <Badge tone={tone}>{label}</Badge>
                          {l.status === 'draft' && <button type="button" className="secondary-btn small-btn" disabled={busy === l.id} onClick={() => void act(l.id, () => sellerApi.setListingState(l.id, 'publish'))}>{t('Publier')}</button>}
                          {l.status === 'published' && <button type="button" className="secondary-btn small-btn" disabled={busy === l.id} onClick={() => void act(l.id, () => sellerApi.setListingState(l.id, 'pause'))}>{t('Mettre en pause')}</button>}
                          {l.status === 'paused' && <button type="button" className="secondary-btn small-btn" disabled={busy === l.id} onClick={() => void act(l.id, () => sellerApi.setListingState(l.id, 'resume'))}>{t('Remettre en ligne')}</button>}
                        </div>
                      )
                    })}
                  </div>
                ) : <p className="nego-help">{t('Aucune offre pour l’instant.')}</p>}
              <NewOffer org={org} storeId={store.id} sellerId={seller.id} onDone={() => listings.reload()} />
            </>
          )}
      {error && <ErrorNote error={error} />}
    </section>
  )
}

export function MyOffers() {
  const { signedIn, user } = useSession()
  const context = useAsync(() => sellerApi.context(), [signedIn], signedIn)
  useEffect(() => { document.title = `${t('Mes offres')} — Trezutil` }, [])
  return (
    <div className="settings-page nego-page">
      <div className="settings-header page-head">
        <div><p className="eyebrow">{t('ESPACE FOURNISSEUR')}</p><h1>{t('Mes offres')}</h1><p>{t('Ce que tu vends sur Trezutil. Les acheteurs te trouvent dans leurs recherches et peuvent te demander ton meilleur prix.')}</p></div>
        <Link className="secondary-btn small-btn" to="/supplier/requests">{t('Demandes de prix')}</Link>
      </div>
      <div className="page-body">
        {user === null ? <Empty icon="lock" title={t('Connecte-toi pour continuer')} action={<Link className="primary-btn" to="/login?returnTo=%2Fsupplier%2Foffers">{t('Se connecter')}</Link>} />
          : context.loading && !context.data ? <Loading />
            : context.error ? <ErrorNote error={context.error} onRetry={context.reload} />
              : !context.data?.length ? (
                <Empty icon="building" title={t('Tu n’as pas encore d’entreprise fournisseur')} action={<Link className="primary-btn" to="/supplier">{t('Devenir fournisseur')}</Link>}>
                  {t('Inscris ton entreprise : tu pourras ensuite publier tes offres et répondre aux demandes de prix.')}
                </Empty>
              ) : context.data.map((org) => <OrgOffers key={org.organization_id} org={org} onChange={context.reload} />)}
      </div>
    </div>
  )
}
