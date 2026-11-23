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

/*
 * Supplier self-registration. Three steps that are never confused: registered (draft), documents submitted (waiting
 * for Trezutil), verified (decided by Trezutil staff only). Registering does not make a business verified or trusted.
 */

const ORG_TYPES: [string, string][] = [
  ['business', 'Entreprise individuelle / commerce'], ['company', 'Société (SARL, SA…)'], ['cooperative', 'Coopérative'],
  ['association', 'Association'], ['individual', 'Particulier'], ['other', 'Autre'],
]

const list = (s: string) => s.split(',').map((x) => x.trim()).filter(Boolean)

function Steps({ status }: { status: SupplierApplication['status'] }) {
  const reached = status === 'verified' ? 3 : status === 'submitted' ? 2 : 1
  const steps: [string, string][] = [['Inscrit', 'Entreprise enregistrée'], ['Documents soumis', 'En attente de Trezutil'], ['Vérifié', 'Décidé par l’équipe Trezutil']]
  return (
    <ol className="supplier-steps" aria-label="Étapes de l’inscription">
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
      <h3 className="full">L’entreprise</h3>
      <label>Type<select {...field('org_type')}>{ORG_TYPES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</select></label>
      <label>Raison sociale *<input {...field('legal_name')} required maxLength={255} /></label>
      <label>Nom commercial<input {...field('display_name')} maxLength={255} placeholder="Si différent de la raison sociale" /></label>
      <label>Téléphone *<input {...field('phone')} required inputMode="tel" placeholder="+243 81 234 5678" /></label>
      <label>E-mail<input {...field('email')} type="email" /></label>
      <label>Site web<input {...field('website')} placeholder="exemple.cd" /></label>
      <label>Ville *<input {...field('city')} placeholder="Kinshasa" /></label>
      <label>Province<input {...field('province')} /></label>
      <label className="full">Adresse<input {...field('address')} /></label>
      <label className="full">Présentation<textarea {...field('description')} maxLength={2000} /></label>

      <h3 className="full">Informations légales et contact</h3>
      <label>Forme juridique<input {...field('legal_form')} placeholder="SARL, SA, Ets…" /></label>
      <label>RCCM<input {...field('rccm')} /></label>
      <label>Identification nationale<input {...field('id_nat')} /></label>
      <label>NIF<input {...field('nif')} /></label>
      <label>Personne de contact<input {...field('contact_name')} /></label>
      <label>Fonction<input {...field('contact_role')} /></label>

      <h3 className="full">Activité et capacités</h3>
      <label className="full">Secteurs d’activité * <small>(séparés par des virgules)</small><input value={sectors} onChange={(e) => { setSaved(false); setSectors(e.target.value) }} placeholder="Matériaux de construction, Quincaillerie" /></label>
      <label className="full">Capacités *<textarea {...field('capabilities')} maxLength={2000} placeholder="Ce que vous fournissez, volumes, stock, délais habituels…" /></label>
      <label>Commande minimale<input {...field('min_order')} placeholder="10 sacs" /></label>
      <label>Délai habituel (jours)<input type="number" min={0} max={365} value={d.lead_time_days ?? ''} onChange={(e) => { setSaved(false); setD({ ...d, lead_time_days: e.target.value === '' ? null : Number(e.target.value) }) }} /></label>
      <label className="full">Conditions de paiement<input {...field('payment_terms')} placeholder="Mobile Money, 50 % à la commande…" /></label>

      <h3 className="full">Zones desservies</h3>
      <label className="full">Communes, villes ou provinces * <small>(séparées par des virgules)</small><input value={areas} onChange={(e) => { setSaved(false); setAreas(e.target.value) }} placeholder="Gombe, Limete, Ngaliema" /></label>

      {error && <p className="form-error full" role="alert">{errorText(error)}</p>}
      <div className="full form-actions">
        <button type="submit" className="primary-btn" disabled={busy}>{busy ? 'Enregistrement…' : submitLabel}</button>
        {saved && <span className="muted-line" role="status">Enregistré.</span>}
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
      <h3 className="memory-heading">Documents <Badge>{app.documents.length}</Badge></h3>
      <p className="muted-line">PDF, JPEG, PNG ou WebP, 5 Mo au plus. Ils ne sont visibles que par les responsables de l’entreprise et l’équipe Trezutil.</p>
      {app.documents.length > 0 && (
        <div className="memory-list">
          {app.documents.map((doc) => (
            <div className="memory-item" key={doc.id}>
              <span><Icon name="file" /></span>
              <div><b>{DOCUMENT_LABEL[doc.kind] ?? doc.kind}</b><p>{doc.filename} · {Math.ceil(doc.size / 1024)} Ko · {doc.status === 'accepted' ? 'accepté' : doc.status === 'rejected' ? 'refusé' : 'transmis'}</p></div>
              <button type="button" className="icon-btn" aria-label={`Ouvrir ${doc.filename}`} onClick={() => void supplierApi.open(doc).catch((err) => setError(toApiError(err)))}><Icon name="eye" size={16} /></button>
              {app.editable && <button type="button" className="icon-btn" aria-label={`Retirer ${doc.filename}`} onClick={() => void supplierApi.removeDocument(app.organization_id, doc.id).then(onChange, (err) => setError(toApiError(err)))}><Icon name="trash" size={16} /></button>}
            </div>
          ))}
        </div>
      )}
      {app.editable && (
        <div className="supplier-upload">
          <select value={kind} onChange={(e) => setKind(e.target.value as DocumentKind)} aria-label="Type de document">
            {(Object.keys(DOCUMENT_LABEL) as DocumentKind[]).map((k) => <option key={k} value={k}>{DOCUMENT_LABEL[k]}</option>)}
          </select>
          <label className="secondary-btn">
            {busy ? 'Envoi…' : 'Ajouter un document'}
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
  useEffect(() => { document.title = 'Devenir fournisseur — Trezutil' }, [])
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
        <div><p className="eyebrow">Fournisseurs</p><h1>Devenir fournisseur</h1><p>Inscris ton entreprise pour recevoir des demandes d’acheteurs sur Trezutil. L’inscription ne vaut pas vérification : l’équipe Trezutil vérifie chaque dossier. En t’inscrivant, tu acceptes les <Link to="/legal/fournisseurs">conditions fournisseurs</Link>.</p></div>
      </div>
      <div className="page-body">
        {user === undefined || mine.loading ? <Loading /> : !user ? (
          <Empty icon="lock" title="Connecte-toi pour inscrire ton entreprise" action={<Link className="primary-btn" to="/login?returnTo=%2Fsupplier">Se connecter</Link>}>
            Ton compte deviendra le propriétaire de l’entreprise inscrite.
          </Empty>
        ) : mine.error ? <ErrorNote error={mine.error} onRetry={mine.reload} /> : !app ? (
          <DetailsForm initial={{ ...EMPTY_DETAILS, phone: user.phone ?? '' }} submitLabel="Inscrire mon entreprise" onSave={async (d) => { setApp(await supplierApi.start(d)) }} />
        ) : (
          <>
            <Steps status={app.status} />
            <div className={`supplier-status ${app.status}`} role="status">
              <b>{STATUS_LABEL[app.status]}</b>
              {app.status === 'submitted' && <p>Ton dossier est complet et attend l’équipe Trezutil. Il ne peut plus être modifié pendant la vérification.</p>}
              {app.status === 'verified' && <p>Ton entreprise est vérifiée. Elle apparaît dans Paramètres → Profil entreprise.</p>}
              {(app.status === 'changes_requested' || app.status === 'rejected') && app.review_note && <p>Message de Trezutil : {app.review_note}</p>}
            </div>
            {app.editable ? (
              <>
                <DetailsForm key={app.organization_id} initial={app} submitLabel="Enregistrer" onSave={async (d) => { setApp(await supplierApi.update(app.organization_id, d)) }} />
                <Documents app={app} onChange={reload} />
                <div className="supplier-submit">
                  {app.missing.length > 0 ? (
                    <p className="muted-line">Pour soumettre le dossier, il manque : {app.missing.map((m) => MISSING_LABEL[m] ?? m).join(', ')}.</p>
                  ) : <p className="muted-line">Le dossier est complet. Une fois soumis, il ne pourra plus être modifié pendant la vérification.</p>}
                  <button type="button" className="primary-btn" disabled={submitting || app.missing.length > 0} onClick={() => void submit()}>{submitting ? 'Envoi…' : 'Soumettre pour vérification'}</button>
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
