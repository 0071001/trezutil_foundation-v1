import { API_BASE, ApiError, http, tokens } from '@/api/client'
import { arr, num, obj, str } from '@/api/json'
import { t } from '@/lib/i18n'

/*
 * Supplier self-registration (/supplier/applications) and its review by Trezutil staff (/admin/supplier-applications).
 * Registering a business never makes it verified: "submitted" means complete and waiting; only staff verify.
 */

export type ApplicationStatus = 'draft' | 'submitted' | 'changes_requested' | 'verified' | 'rejected'
export type DocumentKind = 'rccm' | 'id_nat' | 'nif' | 'identity' | 'other'

export interface SupplierDetails {
  org_type: string
  legal_name: string
  display_name: string
  description: string
  phone: string
  email: string
  country: string
  province: string
  city: string
  address: string
  legal_form: string
  rccm: string
  id_nat: string
  nif: string
  website: string
  contact_name: string
  contact_role: string
  sectors: string[]
  capabilities: string
  min_order: string
  lead_time_days: number | null
  payment_terms: string
  service_areas: string[]
}

export interface SupplierDocument { id: string; kind: DocumentKind; filename: string; contentType: string; size: number; status: string; createdAt: string; url: string }

export interface SupplierApplication extends SupplierDetails {
  organization_id: string
  status: ApplicationStatus
  editable: boolean
  missing: string[]
  review_note: string
  documents: SupplierDocument[]
  submitted_at: string
  reviewed_at: string
}

export const EMPTY_DETAILS: SupplierDetails = {
  org_type: 'business', legal_name: '', display_name: '', description: '', phone: '', email: '', country: 'CD', province: '', city: '',
  address: '', legal_form: '', rccm: '', id_nat: '', nif: '', website: '', contact_name: '', contact_role: '', sectors: [],
  capabilities: '', min_order: '', lead_time_days: null, payment_terms: '', service_areas: [],
}

function mapApplication(value: unknown): SupplierApplication {
  const a = obj(value)
  const list = (v: unknown) => arr(v).map(String)
  return {
    organization_id: str(a.organization_id), status: (str(a.status) || 'draft') as ApplicationStatus, editable: a.editable === true,
    missing: list(a.missing), review_note: str(a.review_note), submitted_at: str(a.submitted_at), reviewed_at: str(a.reviewed_at),
    org_type: str(a.org_type), legal_name: str(a.legal_name), display_name: str(a.display_name), description: str(a.description),
    phone: str(a.phone), email: str(a.email), country: str(a.country), province: str(a.province), city: str(a.city), address: str(a.address),
    legal_form: str(a.legal_form), rccm: str(a.rccm), id_nat: str(a.id_nat), nif: str(a.nif), website: str(a.website),
    contact_name: str(a.contact_name), contact_role: str(a.contact_role), sectors: list(a.sectors), capabilities: str(a.capabilities),
    min_order: str(a.min_order), lead_time_days: a.lead_time_days == null ? null : num(a.lead_time_days), payment_terms: str(a.payment_terms),
    service_areas: list(a.service_areas),
    documents: arr(a.documents).map((v) => {
      const d = obj(v)
      return { id: str(d.id), kind: str(d.kind) as DocumentKind, filename: str(d.filename), contentType: str(d.content_type), size: num(d.size_bytes), status: str(d.status), createdAt: str(d.created_at), url: str(d.url) }
    }),
  }
}

const base = (org: string) => `/supplier/applications/${encodeURIComponent(org)}`

export const supplierApi = {
  async mine(): Promise<SupplierApplication[]> {
    return arr(obj(await http.get<unknown>('/supplier/applications')).items).map(mapApplication)
  },
  async start(details: Partial<SupplierDetails>): Promise<SupplierApplication> {
    return mapApplication(await http.post<unknown>('/supplier/applications', details))
  },
  async update(org: string, details: SupplierDetails): Promise<SupplierApplication> {
    return mapApplication(await http.patch<unknown>(base(org), details))
  },
  async upload(org: string, kind: DocumentKind, file: File): Promise<void> {
    const form = new FormData()
    form.append('kind', kind)
    form.append('file', file)
    await http.post<unknown>(`${base(org)}/documents`, form)
  },
  async removeDocument(org: string, id: string): Promise<void> {
    await http.del<unknown>(`${base(org)}/documents/${encodeURIComponent(id)}`)
  },
  async submit(org: string): Promise<SupplierApplication> {
    return mapApplication(await http.post<unknown>(`${base(org)}/submit`, {}))
  },
  /** Opens a document in a new tab (it needs the session, so it is fetched, never linked). */
  async open(doc: SupplierDocument): Promise<void> {
    const res = await fetch(`${API_BASE}${doc.url.replace(/^\/api\/v1/, '')}`, { headers: { Authorization: `Bearer ${tokens.access() ?? ''}` }, credentials: 'same-origin' })
    if (!res.ok) throw new ApiError(t('Le document n’a pas pu être ouvert.'), res.status, 'DOCUMENT_FAILED')
    const url = URL.createObjectURL(await res.blob())
    window.open(url, '_blank', 'noopener')
    setTimeout(() => URL.revokeObjectURL(url), 60_000)
  },
  /** Staff: the applications waiting for a decision (or another status). */
  async queue(status: ApplicationStatus = 'submitted'): Promise<SupplierApplication[]> {
    return arr(obj(await http.get<unknown>(`/admin/supplier-applications?status=${status}`)).items).map(mapApplication)
  },
  async review(org: string, decision: 'verify' | 'reject' | 'request_changes', note: string): Promise<SupplierApplication> {
    return mapApplication(await http.post<unknown>(`/admin/supplier-applications/${encodeURIComponent(org)}/review`, { decision, note }))
  },
}

export const STATUS_LABEL: Record<ApplicationStatus, string> = {
  draft: t('Inscrit · dossier à compléter'),
  submitted: t('Documents soumis · vérification en attente'),
  changes_requested: t('Modifications demandées'),
  verified: t('Vérifié par Trezutil'),
  rejected: t('Refusé'),
}

export const MISSING_LABEL: Record<string, string> = {
  legal_name: t('la raison sociale'), display_name: t('le nom commercial'), phone: t('le téléphone'), city: t('la ville'),
  sectors: t('au moins un secteur d’activité'), capabilities: t('vos capacités'), service_areas: t('au moins une zone desservie'),
  legal_document: t('un document légal (RCCM, identification nationale ou pièce d’identité)'),
}

export const DOCUMENT_LABEL: Record<DocumentKind, string> = {
  rccm: 'RCCM', id_nat: t('Identification nationale'), nif: t('NIF (impôts)'), identity: t('Pièce d’identité du responsable'), other: t('Autre document'),
}
