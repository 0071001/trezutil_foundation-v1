import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import {
  DOCUMENT_LABEL, EMPTY_DETAILS, MISSING_LABEL, STATUS_LABEL, supplierApi,
  type DocumentKind, type SupplierApplication, type SupplierDetails,
} from '@/api/supplier'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { Icon } from '@/ui/Icon'
import { Badge, Empty, ErrorNote, Loading, errorText } from '@/ui/kit'
import { t } from '@/lib/i18n'

/*
 * Supplier self-registration. Three steps that are never confused: registered (draft), documents submitted (waiting
 * for Trezutil), verified (decided by Trezutil staff only). Registering does not make a business verified or trusted.
 */

const ORG_TYPES: [string, string][] = [
  ['business', t('Entreprise individuelle / commerce')], ['company', t('Société (SARL, SA…)')], ['cooperative', t('Coopérative')],
  ['association', t('Association')], ['individual', t('Particulier')], ['other', t('Autre')],
]

const list = (s: string) => s.split(',').map((x) => x.trim()).filter(Boolean)

function Steps({ status }: { status: SupplierApplication['status'] }) {
  const reached = status === 'verified' ? 3 : status === 'submitted' ? 2 : 1
  const steps: [string, string][] = [[t('Inscrit'), t('Entreprise enregistrée')], [t('Documents soumis'), t('En attente de Trezutil')], [t('Vérifié'), t('Décidé par l’équipe Trezutil')]]
  return (
    <ol className="supplier-steps" aria-label={t('Étapes de l’inscription')}>
      {steps.map(([title, hint], i) => (
        <li key={title} className={i < reached ? 'done' : ''} aria-current={i === reached - 1 ? 'step' : undefined}>
          <span>{i < reached ? <Icon name="check" size={13} /> : i + 1}</span><div><b>{title}</b><small>{hint}</small></div>
        </li>
      ))}
    </ol>
  )
}

function DetailsForm({ initial, submitLabel, onSave }: { initial: SupplierDetails; submitLabel: string; onSave: (d: SupplierDetails) => Promise<void> }) {
  const [d, setD] = useState<SupplierDetails>(initial)
  const [sectors, setSectors] = useState(initial.sectors.join(', '))
  const [areas, setAreas] = useState(initial.service_areas.join(', '))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [saved, setSaved] = useState(false)
  const field = (k: keyof SupplierDetails) => ({ value: (d[k] ?? '') as string, onChange: (e: { target: { value: string } }) => { setSaved(false); setD({ ...d, [k]: e.target.value }) } })
  const save = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true); setError(null)
    try { await onSave({ ...d, sectors: list(sectors), service_areas: list(areas) }); setSaved(true) } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }
  return (
    <form className="form-grid supplier-form" onSubmit={(e) => void save(e)}>
      <h3 className="full">{t('L’entreprise')}</h3>
      <label>{t('Type')}<select {...field('org_type')}>{ORG_TYPES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</select></label>
      <label>{t('Raison sociale *')}<input {...field('legal_name')} required maxLength={255} /></label>
      <label>{t('Nom commercial')}<input {...field('display_name')} maxLength={255} placeholder={t('Si différent de la raison sociale')} /></label>
      <label>{t('Téléphone *')}<input {...field('phone')} required inputMode="tel" placeholder="+243 81 234 5678" /></label>
      <label>{t('E-mail')}<input {...field('email')} type="email" /></label>
      <label>{t('Site web')}<input {...field('website')} placeholder={t('exemple.cd')} /></label>
      <label>{t('Ville *')}<input {...field('city')} placeholder="Kinshasa" /></label>
      <label>{t('Province')}<input {...field('province')} /></label>
      <label className="full">{t('Adresse')}<input {...field('address')} /></label>
      <label className="full">{t('Présentation')}<textarea {...field('description')} maxLength={2000} /></label>

      <h3 className="full">{t('Informations légales et contact')}</h3>
      <label>{t('Forme juridique')}<input {...field('legal_form')} placeholder={t('SARL, SA, Ets…')} /></label>
      <label>{t('RCCM')}<input {...field('rccm')} /></label>
      <label>{t('Identification nationale')}<input {...field('id_nat')} /></label>
      <label>{t('NIF')}<input {...field('nif')} /></label>
      <label>{t('Personne de contact')}<input {...field('contact_name')} /></label>
      <label>{t('Fonction')}<input {...field('contact_role')} /></label>

      <h3 className="full">{t('Activité et capacités')}</h3>
      <label className="full">{t('Secteurs d’activité *')} <small>{t('(séparés par des virgules)')}</small><input value={sectors} onChange={(e) => { setSaved(false); setSectors(e.target.value) }} placeholder={t('Matériaux de construction, Quincaillerie')} /></label>
      <label className="full">{t('Capacités *')}<textarea {...field('capabilities')} maxLength={2000} placeholder={t('Ce que vous fournissez, volumes, stock, délais habituels…')} /></label>
      <label>{t('Commande minimale')}<input {...field('min_order')} placeholder={t('10 sacs')} /></label>
      <label>{t('Délai habituel (jours)')}<input type="number" min={0} max={365} value={d.lead_time_days ?? ''} onChange={(e) => { setSaved(false); setD({ ...d, lead_time_days: e.target.value === '' ? null : Number(e.target.value) }) }} /></label>
      <label className="full">{t('Conditions de paiement')}<input {...field('payment_terms')} placeholder={t('Mobile Money, 50 % à la commande…')} /></label>

      <h3 className="full">{t('Zones desservies')}</h3>
      <label className="full">{t('Communes, villes ou provinces *')} <small>{t('(séparées par des virgules)')}</small><input value={areas} onChange={(e) => { setSaved(false); setAreas(e.target.value) }} placeholder={t('Gombe, Limete, Ngaliema')} /></label>

      {error && <p className="form-error full" role="alert">{errorText(error)}</p>}
      <div className="full form-actions">
        <button type="submit" className="primary-btn" disabled={busy}>{busy ? 'Enregistrement…' : submitLabel}</button>
        {saved && <span className="muted-line" role="status">{t('Enregistré.')}</span>}
      </div>
    </form>
  )
}

function Documents({ app, onChange }: { app: SupplierApplication; onChange: () => void }) {
  const [kind, setKind] = useState<DocumentKind>('rccm')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const upload = async (file: File | undefined) => {
    if (!file) return
    setBusy(true); setError(null)
    try { await supplierApi.upload(app.organization_id, kind, file); onChange() } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }
  return (
    <>
      <h3 className="memory-heading">{t('Documents')} <Badge>{app.documents.length}</Badge></h3>
      <p className="muted-line">{t('PDF, JPEG, PNG ou WebP, 5 Mo au plus. Ils ne sont visibles que par les responsables de l’entreprise et l’équipe Trezutil.')}</p>
      {app.documents.length > 0 && (
        <div className="memory-list">
          {app.documents.map((doc) => (
            <div className="memory-item" key={doc.id}>
              <span><Icon name="file" /></span>
              <div><b>{DOCUMENT_LABEL[doc.kind] ?? doc.kind}</b><p>{doc.filename} · {Math.ceil(doc.size / 1024)} {t('Ko ·')} {doc.status === 'accepted' ? t('accepté') : doc.status === 'rejected' ? t('refusé') : 'transmis'}</p></div>
              <button type="button" className="icon-btn" aria-label={t('Ouvrir {filename}', { filename: doc.filename })} onClick={() => void supplierApi.open(doc).catch((err) => setError(toApiError(err)))}><Icon name="eye" size={16} /></button>
              {app.editable && <button type="button" className="icon-btn" aria-label={t('Retirer {name}', { name: doc.filename })} onClick={() => void supplierApi.removeDocument(app.organization_id, doc.id).then(onChange, (err) => setError(toApiError(err)))}><Icon name="trash" size={16} /></button>}
            </div>
          ))}
        </div>
      )}
      {app.editable && (
        <div className="supplier-upload">
          <select value={kind} onChange={(e) => setKind(e.target.value as DocumentKind)} aria-label={t('Type de document')}>
            {(Object.keys(DOCUMENT_LABEL) as DocumentKind[]).map((k) => <option key={k} value={k}>{DOCUMENT_LABEL[k]}</option>)}
          </select>
          <label className="secondary-btn">
            {busy ? 'Envoi…' : t('Ajouter un document')}
            <input type="file" accept="application/pdf,image/jpeg,image/png,image/webp" hidden disabled={busy} onChange={(e) => { void upload(e.target.files?.[0]); e.target.value = '' }} />
          </label>
        </div>
      )}
      {error && <p className="form-error" role="alert">{errorText(error)}</p>}
    </>
  )
}

export function SupplierOnboarding() {
  const { user } = useSession()
  const mine = useAsync(() => supplierApi.mine(), [user?.id], !!user)
  const [app, setApp] = useState<SupplierApplication | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<ApiError | null>(null)
  useEffect(() => { document.title = t('Devenir fournisseur — Trezutil') }, [])
  useEffect(() => { if (mine.data) setApp(mine.data[0] ?? null) }, [mine.data])

  const reload = () => { if (app) void supplierApi.mine().then((l) => setApp(l.find((x) => x.organization_id === app.organization_id) ?? l[0] ?? null)) }
  const submit = async () => {
    if (!app) return
    setSubmitting(true); setSubmitError(null)
    try { setApp(await supplierApi.submit(app.organization_id)) } catch (err) { setSubmitError(toApiError(err)) } finally { setSubmitting(false) }
  }

  return (
    <div className="settings-page">
      <div className="settings-header page-head">
        <div><p className="eyebrow">{t('Fournisseurs')}</p><h1>{t('Devenir fournisseur')}</h1><p>{t('Inscris ton entreprise pour recevoir des demandes d’acheteurs sur Trezutil. L’inscription ne vaut pas vérification : l’équipe Trezutil vérifie chaque dossier. En t’inscrivant, tu acceptes les')} <Link to="/legal/fournisseurs">{t('conditions fournisseurs')}</Link>.</p></div>
      </div>
      <div className="page-body">
        {user === undefined || mine.loading ? <Loading /> : !user ? (
          <Empty icon="lock" title={t('Connecte-toi pour inscrire ton entreprise')} action={<Link className="primary-btn" to="/login?returnTo=%2Fsupplier">{t('Se connecter')}</Link>}>
            {t('Ton compte deviendra le propriétaire de l’entreprise inscrite.')}
          </Empty>
        ) : mine.error ? <ErrorNote error={mine.error} onRetry={mine.reload} /> : !app ? (
          <DetailsForm initial={{ ...EMPTY_DETAILS, phone: user.phone ?? '' }} submitLabel={t('Inscrire mon entreprise')} onSave={async (d) => { setApp(await supplierApi.start(d)) }} />
        ) : (
          <>
            <Steps status={app.status} />
            <div className={`supplier-status ${app.status}`} role="status">
              <b>{STATUS_LABEL[app.status]}</b>
              {app.status === 'submitted' && <p>{t('Ton dossier est complet et attend l’équipe Trezutil. Il ne peut plus être modifié pendant la vérification.')}</p>}
              {app.status === 'verified' && <p>{t('Ton entreprise est vérifiée. Elle apparaît dans Paramètres → Profil entreprise.')}</p>}
              {(app.status === 'changes_requested' || app.status === 'rejected') && app.review_note && <p>{t('Message de Trezutil :')} {app.review_note}</p>}
            </div>
            {app.editable ? (
              <>
                <DetailsForm key={app.organization_id} initial={app} submitLabel={t('Enregistrer')} onSave={async (d) => { setApp(await supplierApi.update(app.organization_id, d)) }} />
                <Documents app={app} onChange={reload} />
                <div className="supplier-submit">
                  {app.missing.length > 0 ? (
                    <p className="muted-line">{t('Pour soumettre le dossier, il manque :')} {app.missing.map((m) => MISSING_LABEL[m] ?? m).join(', ')}.</p>
                  ) : <p className="muted-line">{t('Le dossier est complet. Une fois soumis, il ne pourra plus être modifié pendant la vérification.')}</p>}
                  <button type="button" className="primary-btn" disabled={submitting || app.missing.length > 0} onClick={() => void submit()}>{submitting ? 'Envoi…' : t('Soumettre pour vérification')}</button>
                  {submitError && <p className="form-error" role="alert">{errorText(submitError)}</p>}
                </div>
              </>
            ) : <Documents app={app} onChange={reload} />}
          </>
        )}
      </div>
    </div>
  )
}
