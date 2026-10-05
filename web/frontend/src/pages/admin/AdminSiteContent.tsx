import { useEffect, useMemo, useState, type ChangeEvent } from 'react'
import { toApiError } from '@/api/client'
import {
  HOME_SECTIONS, PARTNER_KINDS, siteApi,
  type HomeSectionKey, type PartnerKind, type PartnerSetting, type PublicPartner, type PublicTrustItem, type SectionSetting, type SiteAdminView, type SiteDocument, type TrustSetting,
} from '@/api/site'
import { ConfirmationDialog } from '@/components/portal/ConfirmationDialog'
import { DetailDrawer } from '@/components/portal/DetailDrawer'
import { PartnersBandView } from '@/components/partners'
import { ErrorState, Loading } from '@/components/states'
import { TrustStripView, metricName } from '@/components/trust'
import { Button, Field, Heading, Icon, Tabs } from '@/components/ui'
import { locale, t, tn, tx } from '@/lib/i18n'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'

/*
 * Site content: what visitors see on the home page, managed by administrators.
 *
 * Three things can be changed: which live figures the trust strip shows (the
 * numbers themselves are counted by the platform and cannot be typed), the
 * partner logos, and which home page sections show and in what order.
 * Changes are a draft until published; publishing goes live at once and names
 * the version that was read, so two editors can never overwrite each other.
 */

const SECTION_LABELS: Record<HomeSectionKey, string> = {
  categories: tx('What can we help you with'), nearby: tx('Discover around you'), how: tx('From need to action'),
  whatsapp: tx('WhatsApp'), trust: tx('Trezutil in numbers'), seller: tx('For businesses'), partners: tx('Partner logos'),
}

const KIND_LABELS: Record<PartnerKind, string> = {
  bank: tx('Bank'), mobile_money: tx('Mobile money'), shop: tx('Shop'), hotel: tx('Hotel'), hub: tx('Innovation hub'), tech: tx('Technology'), other: tx('Other'),
}

type TabId = 'trust' | 'partners' | 'layout'

function move<T>(list: T[], from: number, to: number): T[] {
  if (to < 0 || to >= list.length || from === to) return list
  const next = [...list]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item as T)
  return next
}

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)

function newPartnerId(name: string, taken: Set<string>): string {
  const base = name.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 28) || 'partner'
  let id = base
  for (let i = 2; taken.has(id); i++) id = `${base}-${i}`
  return id
}

const isHttps = (value: string) => { try { return new URL(value).protocol === 'https:' } catch { return false } }

/** Problems with the draft, in the order a person would fix them. The server checks again. */
function problems(doc: SiteDocument, limits: SiteAdminView['limits']): string[] {
  const out: string[] = []
  for (const item of doc.trust) {
    if (item.labelFr.length > limits.maxTrustLabel || item.labelEn.length > limits.maxTrustLabel) out.push(t('“{name}”: the wording is at most {n} characters.', { name: metricName(item.metric), n: limits.maxTrustLabel }))
    if (!Number.isInteger(item.minValue) || item.minValue < 0 || item.minValue > 1_000_000) out.push(t('“{name}”: the minimum must be a whole number from 0 to 1,000,000.', { name: metricName(item.metric) }))
  }
  for (const p of doc.partners) {
    if (!p.name.trim() || p.name.length > limits.maxPartnerName) out.push(t('A partner needs a name of at most {n} characters.', { n: limits.maxPartnerName }))
    if (!p.logoUrl) out.push(t('“{name}” has no logo.', { name: p.name || '—' }))
    if (p.linkUrl && !isHttps(p.linkUrl)) out.push(t('“{name}”: the website must start with https://', { name: p.name || '—' }))
  }
  if (doc.partners.length > limits.maxPartners) out.push(t('At most {n} partners.', { n: limits.maxPartners }))
  return out
}

/**
 * The figures as this draft would publish them. All but one come from the server; the partners
 * figure is counted from the partner list itself, so here it follows the draft, not what is published.
 */
function withDraftPartners(doc: SiteDocument, stats: Record<string, number>): Record<string, number> {
  return { ...stats, partners: doc.partners.filter((p) => p.visible).length }
}

/** What the public site would show for this draft, with today's real figures. */
function preview(doc: SiteDocument, serverStats: Record<string, number>): { trust: PublicTrustItem[]; partners: PublicPartner[] } {
  const stats = withDraftPartners(doc, serverStats)
  return {
    trust: doc.trust.filter((i) => i.enabled && (stats[i.metric] ?? 0) >= i.minValue).map((i) => ({ metric: i.metric, value: stats[i.metric] ?? 0, labelFr: i.labelFr, labelEn: i.labelEn })),
    partners: doc.partners.filter((p) => p.visible).map((p) => ({ id: p.id, name: p.name, kind: p.kind, logoUrl: p.logoUrl, linkUrl: p.linkUrl })),
  }
}

function Switch({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="tz-switch">
      <input type="checkbox" role="switch" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span>{label}</span>
    </label>
  )
}

function Reorder({ name, index, count, onMove }: { name: string; index: number; count: number; onMove: (to: number) => void }) {
  return (
    <span className="tz-reorder">
      <button type="button" className="tz-small-btn" aria-label={t('Move {name} up', { name })} disabled={index === 0} onClick={() => onMove(index - 1)}><span aria-hidden="true">↑</span></button>
      <button type="button" className="tz-small-btn" aria-label={t('Move {name} down', { name })} disabled={index === count - 1} onClick={() => onMove(index + 1)}><span aria-hidden="true">↓</span></button>
    </span>
  )
}

/* ------------------------------------------------------------ trust strip */

function TrustTab({ draft, stats, limits, onChange }: { draft: SiteDocument; stats: Record<string, number>; limits: SiteAdminView['limits']; onChange: (trust: TrustSetting[]) => void }) {
  const update = (index: number, patch: Partial<TrustSetting>) => onChange(draft.trust.map((item, i) => (i === index ? { ...item, ...patch } : item)))
  const shown = draft.trust.filter((i) => i.enabled).length
  return (
    <>
      <p className="tz-help">{t('Every number here is counted by the platform from its own data. You choose which ones to show, in what order, and from what value: a figure stays hidden until it reaches the minimum, so a young platform never advertises a small number.')}</p>
      <ul className="tz-edit-list" aria-label={t('Live figures')}>
        {draft.trust.map((item, index) => {
          const value = stats[item.metric] ?? 0
          const name = metricName(item.metric)
          const visible = item.enabled && value >= item.minValue
          return (
            <li key={item.metric} className="tz-edit-row" data-visible={visible || undefined}>
              <div className="tz-edit-main">
                <Switch label={name} checked={item.enabled} onChange={(enabled) => update(index, { enabled })} />
                <p className="tz-edit-state">
                  {t('Today: {n}', { n: new Intl.NumberFormat(locale()).format(value) })} · {' '}
                  <strong>{!item.enabled ? t('Off') : visible ? t('Shown on the site') : t('Hidden: below the minimum')}</strong>
                </p>
              </div>
              <Reorder name={name} index={index} count={draft.trust.length} onMove={(to) => onChange(move(draft.trust, index, to))} />
              <div className="tz-edit-fields">
                <Field label={t('Show from')} hint={t('Hidden until the figure reaches this')}>
                  <input type="number" min={0} max={1_000_000} step={1} value={item.minValue} aria-label={t('Show {name} from', { name })} onChange={(e) => update(index, { minValue: e.target.value === '' ? 0 : Number(e.target.value) })} />
                </Field>
                <Field label={t('Wording in French (optional)')}>
                  <input type="text" maxLength={limits.maxTrustLabel} value={item.labelFr} aria-label={t('French wording for {name}', { name })} placeholder={t('Default wording')} onChange={(e) => update(index, { labelFr: e.target.value })} />
                </Field>
                <Field label={t('Wording in English (optional)')}>
                  <input type="text" maxLength={limits.maxTrustLabel} value={item.labelEn} aria-label={t('English wording for {name}', { name })} placeholder={t('Default wording')} onChange={(e) => update(index, { labelEn: e.target.value })} />
                </Field>
              </div>
            </li>
          )
        })}
      </ul>
      <p className="tz-help">{tn(shown, '{n} figure is switched on.', '{n} figures are switched on.')}</p>
    </>
  )
}

/* --------------------------------------------------------------- partners */

function PartnerEditor({ initial, taken, limits, onSave, onClose }: { initial: PartnerSetting | null; taken: Set<string>; limits: SiteAdminView['limits']; onSave: (partner: PartnerSetting) => void; onClose: () => void }) {
  const [partner, setPartner] = useState<PartnerSetting>(initial ?? { id: '', name: '', kind: 'other', logoUrl: '', linkUrl: '', visible: true })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const upload = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) { setError(t('Use a PNG, JPEG or WebP image.')); return }
    if (file.size > limits.maxAssetBytes) { setError(t('The logo must be {n} KB or smaller.', { n: Math.floor(limits.maxAssetBytes / 1024) })); return }
    setBusy(true)
    setError(null)
    try {
      const { url } = await siteApi.uploadLogo(file)
      setPartner((p) => ({ ...p, logoUrl: url }))
    } catch (err) {
      setError(toApiError(err).message)
    } finally {
      setBusy(false)
    }
  }

  const name = partner.name.trim()
  const linkOk = !partner.linkUrl || isHttps(partner.linkUrl)
  const valid = name.length > 0 && name.length <= limits.maxPartnerName && !!partner.logoUrl && linkOk
  const submit = () => onSave({ ...partner, name, id: partner.id || newPartnerId(name, taken) })

  return (
    <div className="tz-form">
      <Field label={t('Name')} hint={t('At most {n} characters', { n: limits.maxPartnerName })}>
        <input type="text" value={partner.name} maxLength={limits.maxPartnerName} onChange={(e) => setPartner({ ...partner, name: e.target.value })} />
      </Field>
      <Field label={t('Kind')}>
        <select value={partner.kind} onChange={(e) => setPartner({ ...partner, kind: e.target.value as PartnerKind })}>
          {PARTNER_KINDS.map((k) => <option key={k} value={k}>{t(KIND_LABELS[k])}</option>)}
        </select>
      </Field>
      <div className="field">
        <span className="tz-field-label">{t('Logo')}</span>
        <div className="tz-logo-pick">
          <span className="tz-logo-box">{partner.logoUrl ? <img src={partner.logoUrl} alt={partner.name ? t('Logo of {name}', { name: partner.name }) : t('Logo')} /> : <span className="tz-logo-none">{t('No logo yet')}</span>}</span>
          <label className="tz-small-btn tz-file">
            <span>{busy ? t('Uploading…') : partner.logoUrl ? t('Replace the logo') : t('Upload a logo')}</span>
            <input type="file" accept="image/png,image/jpeg,image/webp" disabled={busy} onChange={(e) => void upload(e)} />
          </label>
        </div>
        <small className="field-hint">{t('PNG, JPEG or WebP, up to {n} KB. Use the logo exactly as the partner publishes it.', { n: Math.floor(limits.maxAssetBytes / 1024) })}</small>
      </div>
      <Field label={t('Website (optional)')} error={linkOk ? undefined : t('The website must start with https://')}>
        <input type="url" inputMode="url" value={partner.linkUrl} placeholder="https://" onChange={(e) => setPartner({ ...partner, linkUrl: e.target.value.trim() })} />
      </Field>
      <Switch label={t('Show on the site')} checked={partner.visible} onChange={(visible) => setPartner({ ...partner, visible })} />
      {error && <p className="tz-dialog-error" role="alert">{error}</p>}
      <p className="tz-help">{t('A logo is a trademark. Add only organizations Trezutil really works with, and follow each brand’s rules.')}</p>
      <div className="tz-form-actions">
        <Button className="ghost" onClick={onClose}>{t('Cancel')}</Button>
        <Button className="primary-button" disabled={!valid || busy} onClick={submit}>{initial ? t('Apply') : t('Add partner')}</Button>
      </div>
    </div>
  )
}

function PartnersTab({ draft, limits, onChange }: { draft: SiteDocument; limits: SiteAdminView['limits']; onChange: (partners: PartnerSetting[]) => void }) {
  const [editing, setEditing] = useState<{ index: number | null; partner: PartnerSetting | null } | null>(null)
  const [removing, setRemoving] = useState<number | null>(null)
  const taken = useMemo(() => new Set(draft.partners.map((p) => p.id)), [draft.partners])
  const full = draft.partners.length >= limits.maxPartners
  return (
    <>
      <div className="tz-edit-bar">
        <p className="tz-help">{t('The logos in the scrolling band at the foot of the home page. Only partners switched on are shown.')}</p>
        <Button className="primary-button" disabled={full} onClick={() => setEditing({ index: null, partner: null })}><Icon name="check" size={16} /> {t('Add a partner')}</Button>
      </div>
      {draft.partners.length === 0 ? <p className="tzc-empty">{t('No partners. The partner band is not shown.')}</p> : (
        <ul className="tz-edit-list" aria-label={t('Partners')}>
          {draft.partners.map((p, index) => (
            <li key={p.id} className="tz-edit-row tz-partner-row" data-visible={p.visible || undefined}>
              <span className="tz-logo-box small"><img src={p.logoUrl} alt="" /></span>
              <div className="tz-edit-main">
                <strong>{p.name}</strong>
                <p className="tz-edit-state">{t(KIND_LABELS[p.kind])}{p.linkUrl ? ` · ${new URL(p.linkUrl).hostname}` : ''} · {p.visible ? t('Shown on the site') : t('Hidden')}</p>
              </div>
              <Switch label={t('Show {name}', { name: p.name })} checked={p.visible} onChange={(visible) => onChange(draft.partners.map((x, i) => (i === index ? { ...x, visible } : x)))} />
              <Reorder name={p.name} index={index} count={draft.partners.length} onMove={(to) => onChange(move(draft.partners, index, to))} />
              <span className="tz-row-actions">
                <button type="button" className="tz-small-btn" onClick={() => setEditing({ index, partner: p })}>{t('Edit')}<span className="sr-only"> {p.name}</span></button>
                <button type="button" className="tz-small-btn danger" onClick={() => setRemoving(index)}>{t('Remove')}<span className="sr-only"> {p.name}</span></button>
              </span>
            </li>
          ))}
        </ul>
      )}
      <DetailDrawer open={editing !== null} onClose={() => setEditing(null)} title={editing?.partner ? t('Edit partner') : t('Add a partner')}>
        {editing && (
          <PartnerEditor
            key={editing.partner?.id ?? 'new'}
            initial={editing.partner}
            taken={taken}
            limits={limits}
            onClose={() => setEditing(null)}
            onSave={(partner) => {
              onChange(editing.index === null ? [...draft.partners, partner] : draft.partners.map((x, i) => (i === editing.index ? partner : x)))
              setEditing(null)
            }}
          />
        )}
      </DetailDrawer>
      <ConfirmationDialog
        open={removing !== null}
        tone="danger"
        title={t('Remove this partner?')}
        confirmLabel={t('Remove')}
        onClose={() => setRemoving(null)}
        onConfirm={() => { if (removing !== null) onChange(draft.partners.filter((_, i) => i !== removing)) }}
      >
        {removing !== null && draft.partners[removing] ? t('“{name}” leaves the draft. It disappears from the site only when you publish.', { name: draft.partners[removing]!.name }) : ''}
      </ConfirmationDialog>
    </>
  )
}

/* ----------------------------------------------------------------- layout */

function LayoutTab({ draft, onChange }: { draft: SiteDocument; onChange: (sections: SectionSetting[]) => void }) {
  // The order is the product narrative and lives in the code; staff choose what shows.
  const settings = HOME_SECTIONS.map((key) => draft.sections.find((s) => s.key === key) ?? { key, visible: true })
  const set = (key: HomeSectionKey, visible: boolean) => onChange(draft.sections.some((s) => s.key === key) ? draft.sections.map((s) => (s.key === key ? { ...s, visible } : s)) : [...draft.sections, { key, visible }])
  return (
    <>
      <p className="tz-help">{t('The home page always starts with its welcome banner. Below it, the order follows the story of the product: you say what you need, we help, you see what exists, you act, then the businesses. Choose which blocks show.')}</p>
      <ul className="tz-edit-list" aria-label={t('Home page sections')}>
        {settings.map((s, index) => (
          <li key={s.key} className="tz-edit-row tz-section-row" data-visible={s.visible || undefined}>
            <span className="tz-pos" aria-hidden="true">{index + 1}</span>
            <div className="tz-edit-main"><Switch label={t(SECTION_LABELS[s.key])} checked={s.visible} onChange={(visible) => set(s.key, visible)} /></div>
          </li>
        ))}
      </ul>
    </>
  )
}

/* ------------------------------------------------------------------- page */

function Editor({ view, onSaved }: { view: SiteAdminView; onSaved: (view: SiteAdminView) => void }) {
  const notify = useToast()
  const [draft, setDraft] = useState<SiteDocument>(view.content)
  const [tab, setTab] = useState<TabId>('trust')
  const [confirming, setConfirming] = useState(false)
  const [conflict, setConflict] = useState(false)
  const dirty = !same(draft, view.content)
  const issues = useMemo(() => problems(draft, view.limits), [draft, view.limits])
  const shown = useMemo(() => preview(draft, view.stats), [draft, view.stats])

  // Leaving with unpublished changes asks first.
  useEffect(() => {
    if (!dirty) return
    const warn = (e: BeforeUnloadEvent) => { e.preventDefault() }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])

  const publish = async () => {
    try {
      const saved = await siteApi.save(draft, view.version)
      setConflict(false)
      onSaved(saved)
      setDraft(saved.content)
      notify(t('Site content published. Visitors see it now.'), 'success')
    } catch (err) {
      const apiError = toApiError(err)
      if (apiError.status === 409) setConflict(true)
      throw apiError
    }
  }

  const saved = view.stored && view.updatedAt
    ? t('Last published {time}{by}', { time: new Date(view.updatedAt).toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' }), by: view.updatedByName ? t(' by {name}', { name: view.updatedByName }) : '' })
    : t('Nothing has been published yet: the site shows the built-in defaults.')

  return (
    <>
      <div className="tz-dash-head">
        <div>
          <Heading level={1}>{t('Site content')}</Heading>
          <p className="tz-asof">{saved}</p>
        </div>
        <div className="tz-dash-controls">
          {dirty && <Button className="ghost" onClick={() => setDraft(view.content)}>{t('Discard changes')}</Button>}
          <Button className="primary-button" disabled={!dirty || issues.length > 0} onClick={() => setConfirming(true)}>{t('Publish changes')}</Button>
        </div>
      </div>

      {conflict && (
        <p className="inline-notice info" role="alert">
          <Icon name="bell" size={16} />
          <span>{t('Someone else published changes while you were editing. Reload to see them; your draft will be lost.')} <button type="button" className="tz-link-btn" onClick={() => window.location.reload()}>{t('Reload')}</button></span>
        </p>
      )}
      {dirty && issues.length > 0 && (
        <div className="inline-notice info" role="alert">
          <Icon name="bell" size={16} />
          <ul>{issues.map((issue) => <li key={issue}>{issue}</li>)}</ul>
        </div>
      )}

      <Tabs<TabId> label={t('Site content sections')} value={tab} onChange={setTab} items={[{ value: 'trust', label: tx('Live figures') }, { value: 'partners', label: tx('Partners'), count: draft.partners.length }, { value: 'layout', label: tx('Home layout') }]} />
      <div className="tz-edit-panel" role="tabpanel">
        {tab === 'trust' && <TrustTab draft={draft} stats={withDraftPartners(draft, view.stats)} limits={view.limits} onChange={(trust) => setDraft({ ...draft, trust })} />}
        {tab === 'partners' && <PartnersTab draft={draft} limits={view.limits} onChange={(partners) => setDraft({ ...draft, partners })} />}
        {tab === 'layout' && <LayoutTab draft={draft} onChange={(sections) => setDraft({ ...draft, sections })} />}
      </div>

      <section className="tz-dash-section" aria-label={t('Preview')}>
        <h2>{t('Preview')}</h2>
        <p className="tz-help">{t('This is what the home page would show, with today’s real figures.')}</p>
        <div className="tz-preview">
          {tab === 'partners' ? <PartnersBandView partners={shown.partners} /> : <TrustStripView items={shown.trust} />}
          {(tab === 'partners' ? shown.partners.length : shown.trust.length) === 0 && <p className="tzc-empty">{tab === 'partners' ? t('No partners to show: the band is hidden.') : t('No figure to show yet: the strip is hidden.')}</p>}
        </div>
      </section>

      <ConfirmationDialog
        open={confirming}
        title={t('Publish these changes?')}
        confirmLabel={t('Publish')}
        onClose={() => setConfirming(false)}
        onConfirm={publish}
      >
        {t('They go live on the public home page immediately.')}
      </ConfirmationDialog>
    </>
  )
}

export default function AdminSiteContentPage() {
  const loaded = useAsync(() => siteApi.adminContent(), [])
  const [view, setView] = useState<SiteAdminView | null>(null)
  useEffect(() => { if (loaded.data) setView(loaded.data) }, [loaded.data])
  useEffect(() => { document.title = `${t('Site content')} — Trezutil` }, [])

  return (
    <section className="admin-page">
      {loaded.loading && !view && <Loading label={t('Loading the site content…')} />}
      {loaded.error && !view && <ErrorState error={loaded.error} onRetry={loaded.reload} />}
      {view && <Editor key={view.version} view={view} onSaved={setView} />}
    </section>
  )
}
