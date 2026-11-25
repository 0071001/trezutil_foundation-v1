import { useState, type FormEvent, type ReactNode } from 'react'
import { adminApi, isAdmin } from '@/api/admin'
import { toApiError, type ApiError } from '@/api/client'
import { adminConsoleApi } from '@/api/console'
import { DOCUMENT_LABEL, STATUS_LABEL, supplierApi, type ApplicationStatus } from '@/api/supplier'
import { t } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { errorText } from '@/ui/kit'
import { CCIcon } from '@/admin/icons'
import { adminManageApi } from '@/api/adminManage'
import { GrantCreditsDialog, SubscribeDialog } from '@/admin/BillingPage'
import { changeError, Modal } from '@/admin/ui'
import { day, Empty, Failed, Loading, money, n, PageHead, Pager, Panel, Person, SearchBox, shortId, Status, Tabs, toneOf, when } from '@/admin/ui'

/* Commerce: suppliers (organisations) and their verification, customer accounts, the catalogue, the categories, and
   every order and booking. Read from the administration API; every action is re-checked by the server. */

const PAGE = 20

/** A list page backed by an admin list endpoint: search, status tab, pagination. */
function useList<T>(load: (q: string, status: string, page: number) => Promise<{ items: T[]; total: number; page: number; pageSize: number }>, initialStatus = '') {
  const [q, setQ] = useState('')
  const [status, setStatus] = useState(initialStatus)
  const [page, setPage] = useState(1)
  const list = useAsync(() => load(q.trim(), status, page), [q, status, page])
  return { q, setQ: (v: string) => { setQ(v); setPage(1) }, status, setStatus: (v: string) => { setStatus(v); setPage(1) }, page, setPage, list }
}

function ListPanel<T>({ list, noun, page, setPage, empty, children, toolbar }: {
  list: { loading: boolean; error: ApiError | null; data?: { items: T[]; total: number; pageSize: number }; reload: () => void }
  noun: string; page: number; setPage: (p: number) => void; empty: string; children: (items: T[]) => ReactNode; toolbar: ReactNode
}) {
  return (
    <section className="cc-panel cc-table-panel">
      <div className="cc-table-toolbar">{toolbar}</div>
      {list.loading ? <Loading /> : list.error ? <Failed error={list.error} onRetry={list.reload} /> : !list.data?.items.length ? <Empty title={empty} /> : (
        <>
          <div className="cc-table-wrap">{children(list.data.items)}</div>
          <Pager page={page} pageSize={list.data.pageSize || PAGE} total={list.data.total} onPage={setPage} noun={noun} />
        </>
      )}
    </section>
  )
}

export function Organizations() {
  const l = useList((q, status, page) => adminApi.organizations({ q, status: status || undefined, page, pageSize: PAGE }))
  return (
    <>
      <PageHead eyebrow={t('Commerce / Fournisseurs')} title={t('Fournisseurs')} description={t('Les entreprises inscrites sur Trezutil, leur vérification et leur activité.')} />
      <ListPanel list={l.list} noun={t('organisations')} page={l.page} setPage={l.setPage} empty={t('Aucune organisation inscrite pour l’instant.')}
        toolbar={<><Tabs value={l.status} onChange={l.setStatus} options={[{ id: '', label: t('Toutes') }, { id: 'active', label: t('Actives') }, { id: 'suspended', label: t('Suspendues') }]} /><SearchBox value={l.q} onChange={l.setQ} placeholder={t('Nom, ville…')} /></>}>
        {(items) => (
          <table className="cc-table"><thead><tr><th>{t('Organisation')}</th><th>{t('Type')}</th><th>{t('Ville')}</th><th>{t('Vérification')}</th><th>{t('Membres')}</th><th>{t('Offres publiées')}</th><th>{t('Commandes')}</th><th>{t('Inscrite le')}</th></tr></thead><tbody>
            {items.map((r) => <tr key={r.id}><td><Person name={r.displayName} sub={shortId(r.id)} /></td><td>{r.type || '—'}</td><td>{[r.city, r.country].filter(Boolean).join(', ') || '—'}</td><td><Status tone={toneOf(r.verificationStatus)}>{r.verificationStatus || '—'}</Status></td><td>{n(r.members)}</td><td>{n(r.publishedListings)}</td><td>{n(r.orders)}</td><td>{day(r.createdAt)}</td></tr>)}
          </tbody></table>
        )}
      </ListPanel>
    </>
  )
}

export function Verification({ onChange }: { onChange: () => void }) {
  const [status, setStatus] = useState<ApplicationStatus>('submitted')
  const queue = useAsync(() => supplierApi.queue(status), [status])
  const [open, setOpen] = useState<string | null>(null)
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const decide = async (org: string, decision: 'verify' | 'reject' | 'request_changes') => {
    setBusy(true); setError(null)
    try { await supplierApi.review(org, decision, note); setNote(''); setOpen(null); queue.reload(); onChange() } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }
  const tabs = (Object.keys(STATUS_LABEL) as ApplicationStatus[]).map((k) => ({ id: k, label: STATUS_LABEL[k] }))
  return (
    <>
      <PageHead eyebrow={t('Commerce / Vérification')} title={t('Vérification fournisseurs')} description={t('S’inscrire ne vaut jamais vérification : seule une décision de l’équipe vérifie une entreprise. Tu ne peux pas vérifier une entreprise dont tu es membre.')} />
      <section className="cc-panel cc-table-panel">
        <div className="cc-table-toolbar"><Tabs value={status} onChange={(s) => { setStatus(s); setOpen(null) }} options={tabs} /></div>
        {queue.loading ? <Loading /> : queue.error ? <Failed error={queue.error} onRetry={queue.reload} /> : !queue.data?.length ? <Empty icon="shield" title={t('Aucun dossier dans cet état.')} /> : (
          <div className="cc-review-list">
            {queue.data.map((a) => (
              <div key={a.organization_id} className={`cc-review ${open === a.organization_id ? 'open' : ''}`}>
                <button type="button" className="cc-review-head" aria-expanded={open === a.organization_id} onClick={() => { setOpen(open === a.organization_id ? null : a.organization_id); setNote(''); setError(null) }}>
                  <Person name={a.display_name} sub={a.legal_name} />
                  <span>{[a.city, a.province].filter(Boolean).join(', ') || '—'}</span>
                  <span>{a.phone}</span>
                  <span>{a.submitted_at ? when(a.submitted_at) : '—'}</span>
                  <Status tone={toneOf(a.status)}>{STATUS_LABEL[a.status]}</Status>
                  <CCIcon name="chevron" />
                </button>
                {open === a.organization_id && (
                  <div className="cc-review-body">
                    <dl>
                      <div><dt>{t('Forme juridique')}</dt><dd>{a.legal_form || '—'}</dd></div>
                      <div><dt>RCCM</dt><dd>{a.rccm || '—'}</dd></div>
                      <div><dt>{t('Identification nationale')}</dt><dd>{a.id_nat || '—'}</dd></div>
                      <div><dt>NIF</dt><dd>{a.nif || '—'}</dd></div>
                      <div><dt>{t('Contact')}</dt><dd>{[a.contact_name, a.contact_role, a.email].filter(Boolean).join(' · ') || '—'}</dd></div>
                      <div><dt>{t('Secteurs')}</dt><dd>{a.sectors.join(', ') || '—'}</dd></div>
                      <div><dt>{t('Zones desservies')}</dt><dd>{a.service_areas.join(', ') || '—'}</dd></div>
                      <div className="full"><dt>{t('Capacités')}</dt><dd>{a.capabilities || '—'}</dd></div>
                    </dl>
                    <div className="cc-docs">
                      {a.documents.length === 0 ? <p className="cc-muted">{t('Aucun document envoyé.')}</p> : a.documents.map((d) => (
                        <button type="button" key={d.id} className="cc-secondary" onClick={() => void supplierApi.open(d).catch((err) => setError(toApiError(err)))}>
                          <CCIcon name="receipt" />{DOCUMENT_LABEL[d.kind] ?? d.kind} · {d.filename}
                        </button>
                      ))}
                    </div>
                    {a.review_note && <p className="cc-muted">{t('Dernier message de l’équipe :')} {a.review_note}</p>}
                    {a.status === 'submitted' && (
                      <div className="cc-decision">
                        <textarea value={note} onChange={(e) => setNote(e.target.value)} placeholder={t('Message au fournisseur (obligatoire pour refuser ou demander des modifications)')} aria-label={t('Message au fournisseur')} />
                        <div>
                          <button type="button" className="cc-secondary" disabled={busy || !note.trim()} onClick={() => void decide(a.organization_id, 'reject')}>{t('Refuser')}</button>
                          <button type="button" className="cc-secondary" disabled={busy || !note.trim()} onClick={() => void decide(a.organization_id, 'request_changes')}>{t('Demander des modifications')}</button>
                          <button type="button" className="cc-primary" disabled={busy} onClick={() => void decide(a.organization_id, 'verify')}><CCIcon name="check" />{t('Vérifier l’entreprise')}</button>
                        </div>
                      </div>
                    )}
                    {error && <p className="cc-error" role="alert">{errorText(error)}</p>}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </section>
    </>
  )
}

export function Accounts() {
  const { user } = useSession()
  const admin = isAdmin(user)
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const list = useAsync(() => adminApi.users({ q: q.trim() || undefined, page, pageSize: PAGE }), [q, page])
  const [acting, setActing] = useState<{ id: string; name: string; suspend: boolean } | null>(null)
  const [extra, setExtra] = useState<{ kind: 'credits' | 'plan' | 'role'; id: string; name: string } | null>(null)
  const plans = useAsync(() => (extra?.kind === 'plan' ? adminConsoleApi.monetization(30).then((m) => m.plans) : Promise.resolve([])), [extra?.kind])
  const [role, setRole] = useState('operator')
  const [roleError, setRoleError] = useState<string | null>(null)
  const grantRole = async () => {
    if (!extra) return
    setRoleError(null)
    try { await adminManageApi.grantRole(extra.id, role); setExtra(null); list.reload() } catch (err) { setRoleError(changeError(toApiError(err))) }
  }
  const [reason, setReason] = useState('')
  const [error, setError] = useState<ApiError | null>(null)
  const act = async (e: FormEvent) => {
    e.preventDefault()
    if (!acting) return
    setError(null)
    try {
      if (acting.suspend) await adminApi.suspendUser(acting.id, reason.trim())
      else await adminApi.reactivateUser(acting.id, reason.trim())
      setActing(null); setReason(''); list.reload()
    } catch (err) { setError(toApiError(err)) }
  }
  return (
    <>
      <PageHead eyebrow={t('Commerce / Comptes')} title={t('Comptes clients')} description={admin ? t('Tous les comptes de la plateforme. Suspendre un compte coupe ses sessions ; la raison est inscrite au journal d’audit.') : t('La liste des comptes (données personnelles) est réservée aux administrateurs.')} />
      <ListPanel list={list} noun={t('comptes')} page={page} setPage={setPage} empty={t('Aucun compte ne correspond.')}
        toolbar={<SearchBox value={q} onChange={(v) => { setQ(v); setPage(1) }} placeholder={t('Nom, e-mail, téléphone…')} />}>
        {(items) => (
          <table className="cc-table"><thead><tr><th>{t('Compte')}</th><th>{t('Téléphone')}</th><th>{t('Rôles')}</th><th>{t('Statut')}</th><th>{t('Créé le')}</th><th>{t('Dernière connexion')}</th><th /></tr></thead><tbody>
            {items.map((r) => (
              <tr key={r.id}>
                <td><Person name={r.displayName || r.email || r.phone} sub={r.email || shortId(r.id)} /></td><td>{r.phone || '—'}</td><td>{r.roles.join(', ') || '—'}</td>
                <td><Status tone={toneOf(r.status)}>{r.status}</Status></td><td>{day(r.createdAt)}</td><td>{when(r.lastLoginAt)}</td>
                <td>{admin && (
                  <div className="cc-row-actions">
                    <button type="button" className="cc-secondary cc-small" onClick={() => setExtra({ kind: 'credits', id: r.id, name: r.displayName || r.email || r.phone })}>{t('Crédits')}</button>
                    <button type="button" className="cc-secondary cc-small" onClick={() => setExtra({ kind: 'plan', id: r.id, name: r.displayName || r.email || r.phone })}>{t('Plan')}</button>
                    <button type="button" className="cc-secondary cc-small" onClick={() => { setExtra({ kind: 'role', id: r.id, name: r.displayName || r.email || r.phone }); setRoleError(null) }}>{t('Rôle')}</button>
                    {r.id !== user?.id && (
                      <button type="button" className="cc-secondary cc-small" onClick={() => { setActing({ id: r.id, name: r.displayName || r.email || r.phone, suspend: r.status !== 'suspended' }); setReason(''); setError(null) }}>
                        {r.status === 'suspended' ? t('Réactiver') : t('Suspendre')}
                      </button>
                    )}
                  </div>
                )}</td>
              </tr>
            ))}
          </tbody></table>
        )}
      </ListPanel>
      {extra?.kind === 'credits' && <GrantCreditsDialog user={extra} onClose={() => setExtra(null)} onSaved={() => setExtra(null)} />}
      {extra?.kind === 'plan' && (plans.loading ? null : <SubscribeDialog plans={plans.data ?? []} presetUser={extra} onClose={() => setExtra(null)} onSaved={() => setExtra(null)} />)}
      {extra?.kind === 'role' && (
        <Modal title={t('Donner un rôle d’équipe à {name}', { name: extra.name })} onClose={() => setExtra(null)} onSubmit={() => void grantRole()} submitLabel={t('Donner le rôle')} error={roleError}>
          <label>{t('Rôle')}<select value={role} onChange={(e) => setRole(e.target.value)}><option value="operator">{t('Opérateur')}</option><option value="moderator">{t('Modérateur')}</option><option value="admin">{t('Administrateur')}</option></select></label>
          <p className="cc-muted">{t('Le compte ouvre alors le Control Center avec les droits de ce rôle. Le changement est inscrit au journal d’audit.')}</p>
        </Modal>
      )}
      {acting && (
        <div className="cc-modal-backdrop" onMouseDown={(e) => e.target === e.currentTarget && setActing(null)}>
          <form className="cc-modal" onSubmit={(e) => void act(e)} aria-label={acting.suspend ? t('Suspendre le compte') : t('Réactiver le compte')}>
            <h2>{acting.suspend ? t('Suspendre {name} ?', { name: acting.name }) : t('Réactiver {name} ?', { name: acting.name })}</h2>
            <p>{acting.suspend ? t('Le compte ne pourra plus se connecter et ses sessions sont fermées. La raison est inscrite au journal d’audit.') : t('Le compte pourra de nouveau se connecter.')}</p>
            <label>{t('Raison')}<textarea value={reason} onChange={(e) => setReason(e.target.value)} minLength={acting.suspend ? 3 : 0} maxLength={300} required={acting.suspend} /></label>
            {error && <p className="cc-error" role="alert">{errorText(error)}</p>}
            <div className="cc-modal-actions"><button type="button" className="cc-secondary" onClick={() => setActing(null)}>{t('Annuler')}</button><button type="submit" className="cc-primary">{acting.suspend ? t('Suspendre') : t('Réactiver')}</button></div>
          </form>
        </div>
      )}
    </>
  )
}

export function Catalogue() {
  const l = useList((q, status, page) => adminConsoleApi.listings({ q: q || undefined, status: status || undefined, page }))
  const [error, setError] = useState<ApiError | null>(null)
  const toggle = async (id: string, suspend: boolean) => {
    const reason = suspend ? window.prompt(t('Raison de la suspension (inscrite au journal d’audit)')) : ''
    if (suspend && !reason?.trim()) return
    setError(null)
    try { await (suspend ? adminConsoleApi.suspendListing(id, reason!.trim()) : adminConsoleApi.reinstateListing(id)); l.list.reload() } catch (err) { setError(toApiError(err)) }
  }
  return (
    <>
      <PageHead eyebrow={t('Commerce / Catalogue')} title={t('Catalogue')} description={t('Toutes les offres publiées par les fournisseurs. Une offre suspendue disparaît des recherches.')} />
      {error && <Failed error={error} />}
      <ListPanel list={l.list} noun={t('offres')} page={l.page} setPage={l.setPage} empty={t('Aucune offre ne correspond.')}
        toolbar={<><Tabs value={l.status} onChange={l.setStatus} options={[{ id: '', label: t('Toutes') }, { id: 'published', label: t('Publiées') }, { id: 'draft', label: t('Brouillons') }, { id: 'suspended', label: t('Suspendues') }]} /><SearchBox value={l.q} onChange={l.setQ} placeholder={t('Titre, fournisseur…')} /></>}>
        {(items) => (
          <table className="cc-table"><thead><tr><th>{t('Offre')}</th><th>{t('Fournisseur')}</th><th>{t('Secteur')}</th><th>{t('Prix')}</th><th>{t('Photos')}</th><th>{t('Statut')}</th><th>{t('Mise à jour')}</th><th /></tr></thead><tbody>
            {items.map((r) => (
              <tr key={r.id}>
                <td><strong>{r.title}</strong><small className="cc-sub">{r.category || r.listingType}{r.flags.length ? ` · ${r.flags.join(', ')}` : ''}</small></td><td>{r.organizationName || '—'}</td><td>{r.vertical || '—'}</td>
                <td>{r.priceAmount > 0 ? money(Math.round(r.priceAmount * 100), r.currency) : '—'}</td><td>{n(r.images)}</td><td><Status tone={toneOf(r.status)}>{r.status}</Status></td><td>{day(r.updatedAt)}</td>
                <td>{r.status.toLowerCase() === 'suspended'
                  ? <button type="button" className="cc-secondary cc-small" onClick={() => void toggle(r.id, false)}>{t('Rétablir')}</button>
                  : <button type="button" className="cc-secondary cc-small" onClick={() => void toggle(r.id, true)}>{t('Suspendre')}</button>}</td>
              </tr>
            ))}
          </tbody></table>
        )}
      </ListPanel>
    </>
  )
}

export function Categories() {
  const cats = useAsync(() => adminConsoleApi.categories(), [])
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [parent, setParent] = useState('')
  const [error, setError] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState(false)
  const create = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true); setError(null)
    try { await adminConsoleApi.createCategory(name.trim(), slug.trim(), parent || undefined); setName(''); setSlug(''); setParent(''); cats.reload() } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }
  const names = new Map((cats.data ?? []).map((c) => [c.id, c.name]))
  const [editing, setEditing] = useState<{ id: string; name: string; slug: string; parent: string } | null>(null)
  const [editError, setEditError] = useState<string | null>(null)
  const saveEdit = async () => {
    if (!editing) return
    setEditError(null)
    try { await adminManageApi.updateCategory(editing.id, { name: editing.name.trim(), slug: editing.slug.trim(), parent_id: editing.parent || null }); setEditing(null); cats.reload() } catch (err) { setEditError(changeError(toApiError(err))) }
  }
  const remove = async (id: string, label: string) => {
    if (!window.confirm(t('Supprimer la catégorie « {name} » ? Seule une catégorie sans offre ni sous-catégorie peut l’être.', { name: label }))) return
    setError(null)
    try { await adminManageApi.deleteCategory(id); cats.reload() } catch (err) { setError(toApiError(err)) }
  }
  return (
    <>
      <PageHead eyebrow={t('Commerce / Catégories')} title={t('Catégories')} description={t('Les catégories que l’assistant et la recherche proposent. L’assistant en reçoit la liste à jour en quelques minutes.')} />
      <div className="cc-dashboard-grid">
        <Panel wide title={t('Catégories existantes')} sub={cats.data ? t('{n} catégorie(s)', { n: cats.data.length }) : ''}>
          {cats.loading ? <Loading /> : cats.error ? <Failed error={cats.error} onRetry={cats.reload} /> : !cats.data?.length ? <Empty icon="tag" title={t('Aucune catégorie')} /> : (
            <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Nom')}</th><th>{t('Identifiant')}</th><th>{t('Catégorie parente')}</th><th /></tr></thead><tbody>
              {[...cats.data].sort((a, b) => a.name.localeCompare(b.name)).map((c) => (
                <tr key={c.id}><td><strong>{c.name}</strong></td><td><code>{c.slug}</code></td><td>{c.parentId ? names.get(c.parentId) ?? '—' : '—'}</td>
                  <td><div className="cc-row-actions">
                    <button type="button" className="cc-icon-btn" title={t('Modifier')} aria-label={t('Modifier {name}', { name: c.name })} onClick={() => { setEditing({ id: c.id, name: c.name, slug: c.slug, parent: c.parentId }); setEditError(null) }}><CCIcon name="settings" size={15} /></button>
                    <button type="button" className="cc-icon-btn danger" title={t('Supprimer')} aria-label={t('Supprimer {name}', { name: c.name })} onClick={() => void remove(c.id, c.name)}><CCIcon name="close" size={15} /></button>
                  </div></td>
                </tr>
              ))}
            </tbody></table></div>
          )}
        </Panel>
        <Panel title={t('Nouvelle catégorie')}>
          <form className="cc-form" onSubmit={(e) => void create(e)}>
            <label>{t('Nom')}<input value={name} onChange={(e) => { setName(e.target.value); setSlug(e.target.value.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')) }} required /></label>
            <label>{t('Identifiant (slug)')}<input value={slug} onChange={(e) => setSlug(e.target.value)} pattern="[a-z0-9-]+" required /></label>
            <label>{t('Catégorie parente')}<select value={parent} onChange={(e) => setParent(e.target.value)}><option value="">{t('Aucune')}</option>{(cats.data ?? []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select></label>
            {error && <p className="cc-error" role="alert">{errorText(error)}</p>}
            <button type="submit" className="cc-primary" disabled={busy}><CCIcon name="plus" />{t('Créer la catégorie')}</button>
          </form>
        </Panel>
      </div>
      {error && !busy && <p className="cc-error" role="alert">{error.code === 'IN_USE' ? t('Cette catégorie contient des offres ou des sous-catégories : elle ne peut pas être supprimée.') : changeError(error)}</p>}
      {editing && (
        <Modal title={t('Modifier la catégorie')} onClose={() => setEditing(null)} onSubmit={() => void saveEdit()} submitLabel={t('Enregistrer')} error={editError}>
          <label>{t('Nom')}<input value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} required /></label>
          <label>{t('Identifiant (slug)')}<input value={editing.slug} onChange={(e) => setEditing({ ...editing, slug: e.target.value })} required pattern="[a-z0-9]+(-[a-z0-9]+)*" /></label>
          <label>{t('Catégorie parente')}<select value={editing.parent} onChange={(e) => setEditing({ ...editing, parent: e.target.value })}><option value="">{t('Aucune')}</option>{(cats.data ?? []).filter((c) => c.id !== editing.id).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select></label>
        </Modal>
      )}
    </>
  )
}

const ORDER_TABS = ['', 'pending_payment', 'paid', 'confirmed', 'fulfilled', 'cancelled']

export function Orders() {
  const l = useList((q, status, page) => adminApi.orders({ q: q || undefined, status: status || undefined, page, pageSize: PAGE }))
  return (
    <>
      <PageHead eyebrow={t('Opérations / Commandes')} title={t('Commandes')} description={t('Toutes les commandes passées sur Trezutil, les plus récentes d’abord.')} />
      <ListPanel list={l.list} noun={t('commandes')} page={l.page} setPage={l.setPage} empty={t('Aucune commande ne correspond.')}
        toolbar={<><Tabs value={l.status} onChange={l.setStatus} options={ORDER_TABS.map((s) => ({ id: s, label: s ? s.replace(/_/g, ' ') : t('Toutes') }))} /><SearchBox value={l.q} onChange={l.setQ} placeholder={t('Titre, client, fournisseur…')} /></>}>
        {(items) => (
          <table className="cc-table"><thead><tr><th>{t('Commande')}</th><th>{t('Fournisseur')}</th><th>{t('Client')}</th><th>{t('Articles')}</th><th>{t('Montant')}</th><th>{t('Statut')}</th><th>{t('Date')}</th></tr></thead><tbody>
            {items.map((r) => <tr key={r.id}><td><strong>{r.title || t('Commande')}</strong><small className="cc-sub"><code>{shortId(r.id)}</code></small></td><td>{r.organizationName || '—'}</td><td>{r.customerName || '—'}</td><td>{n(r.itemCount)}</td><td><strong>{money(r.totalMinor, r.currency)}</strong></td><td><Status tone={toneOf(r.status)}>{r.status}</Status></td><td>{when(r.createdAt)}</td></tr>)}
          </tbody></table>
        )}
      </ListPanel>
    </>
  )
}

export function Reservations() {
  const l = useList((q, status, page) => adminApi.reservations({ q: q || undefined, status: status || undefined, page, pageSize: PAGE }))
  return (
    <>
      <PageHead eyebrow={t('Opérations / Réservations')} title={t('Réservations')} description={t('Toutes les réservations (hébergement, restaurants, services), les plus récentes d’abord.')} />
      <ListPanel list={l.list} noun={t('réservations')} page={l.page} setPage={l.setPage} empty={t('Aucune réservation ne correspond.')}
        toolbar={<><Tabs value={l.status} onChange={l.setStatus} options={[{ id: '', label: t('Toutes') }, { id: 'confirmed', label: 'confirmed' }, { id: 'pending', label: 'pending' }, { id: 'cancelled', label: 'cancelled' }]} /><SearchBox value={l.q} onChange={l.setQ} placeholder={t('Titre, client, fournisseur…')} /></>}>
        {(items) => (
          <table className="cc-table"><thead><tr><th>{t('Réservation')}</th><th>{t('Fournisseur')}</th><th>{t('Client')}</th><th>{t('Période')}</th><th>{t('Personnes')}</th><th>{t('Statut')}</th></tr></thead><tbody>
            {items.map((r) => <tr key={r.id}><td><strong>{r.title || t('Réservation')}</strong><small className="cc-sub"><code>{shortId(r.id)}</code></small></td><td>{r.organizationName || '—'}</td><td>{r.customerName || '—'}</td><td>{day(r.startAt)}{r.endAt && r.endAt !== r.startAt ? ` → ${day(r.endAt)}` : ''}</td><td>{n(r.partySize || r.quantity)}</td><td><Status tone={toneOf(r.status)}>{r.status}</Status></td></tr>)}
          </tbody></table>
        )}
      </ListPanel>
    </>
  )
}
