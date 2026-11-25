import { useEffect, useState, type FormEvent } from 'react'
import { adminApi, isAdmin, type AuditFilters } from '@/api/admin'
import { toApiError, type ApiError } from '@/api/client'
import { adminConsoleApi } from '@/api/console'
import { adminManageApi } from '@/api/adminManage'
import { formatMoney } from '@/lib/format'
import { exchangeRateApi, loadExchangeRate, useExchangeRate } from '@/lib/exchangeRate'
import { locale, t } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { errorText } from '@/ui/kit'
import { CCIcon } from '@/admin/icons'
import { changeError, day, Empty, Failed, Loading, money, n, Notice, PageHead, Pager, Panel, Person, SearchBox, shortId, Status, Strip, Tabs, toneOf, when } from '@/admin/ui'

/* Platform: the assistant's activity, credits and billing, payments, channels and providers, the CDF / USD rate,
   the staff and the audit trail. */

function Counts({ data, empty }: { data: Record<string, number>; empty: string }) {
  const entries = Object.entries(data).sort((a, b) => b[1] - a[1])
  const max = Math.max(1, ...entries.map(([, v]) => v))
  if (!entries.length) return <p className="cc-muted cc-pad">{empty}</p>
  return (
    <div className="cc-bars">
      {entries.map(([k, v]) => <div key={k}><span>{k.replace(/_/g, ' ')}</span><strong>{n(v)}</strong><i><b style={{ width: `${Math.max(2, (v / max) * 100)}%` }} /></i></div>)}
    </div>
  )
}

export function AIActivityPage() {
  const [days, setDays] = useState(30)
  const ai = useAsync(() => adminConsoleApi.ai(days), [days])
  const d = ai.data
  const max = d ? Math.max(1, ...d.perDay.map((p) => p.count)) : 1
  return (
    <>
      <PageHead eyebrow={t('IA / Activité')} title={t('Activité de l’assistant')} description={t('Ce que l’assistant a réellement traité : conversations, messages, résultats et confirmations.')}>
        <select className="cc-select" value={days} onChange={(e) => setDays(Number(e.target.value))} aria-label={t('Période')}>{[7, 30, 90].map((v) => <option key={v} value={v}>{t('{n} derniers jours', { n: v })}</option>)}</select>
      </PageHead>
      {ai.loading ? <Loading /> : ai.error ? <Failed error={ai.error} onRetry={ai.reload} /> : d && (
        <>
          <div className="cc-ai-hero">
            <div><span className="cc-live-dot" />{t('Sur les {n} derniers jours', { n: d.days })}</div>
            <strong>{n(d.conversations)}</strong>
            <p>{t('conversations')}</p>
            <div className="cc-ai-rates">
              <span><small>{t('Clients actifs')}</small><b>{n(d.activeCustomers)}</b></span>
              <span><small>{t('Messages reçus')}</small><b>{n(d.messagesIn)}</b></span>
              <span><small>{t('Réponses envoyées')}</small><b>{n(d.messagesOut)}</b></span>
              <span><small>{t('Exécutions IA enregistrées')}</small><b>{n(d.aiExecutionRecords)}</b></span>
            </div>
          </div>
          <div className="cc-dashboard-grid">
            <Panel wide title={t('Conversations par jour')}>
              {d.perDay.length ? (
                <div className="cc-daybars" role="img" aria-label={t('Conversations par jour')}>
                  {d.perDay.map((p) => <span key={p.date} title={`${day(p.date)} : ${p.count}`}><i style={{ height: `${Math.max(3, (p.count / max) * 100)}%` }} /></span>)}
                </div>
              ) : <p className="cc-muted cc-pad">{t('Aucune conversation sur la période.')}</p>}
            </Panel>
            <Panel title={t('Fournisseurs d’IA configurés')} sub={t('Présence des clés sur le serveur')}>
              <div className="cc-health-list">{Object.entries(d.configured).map(([k, v]) => <div key={k}><span><i className={v ? '' : 'warn-dot'} />{k.replace(/_/g, ' ')}</span><small /><Status tone={v ? 'good' : 'neutral'}>{v ? t('Configuré') : t('Absent')}</Status></div>)}</div>
            </Panel>
            <Panel wide title={t('Résultats renvoyés par type d’action')}><Counts data={d.resultsByType} empty={t('Aucun résultat sur la période.')} /></Panel>
            <Panel title={t('Par canal')}><Counts data={d.byChannel} empty={t('Aucune conversation.')} /></Panel>
            <Panel wide title={t('Confirmations demandées')} sub={t('Toute action engageante attend la confirmation explicite du client')}><Counts data={d.confirmations} empty={t('Aucune confirmation sur la période.')} /></Panel>
            <Panel title={t('États des conversations')}><Counts data={d.byState} empty={t('Aucune conversation.')} /></Panel>
          </div>
        </>
      )}
    </>
  )
}

export function Payments() {
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('')
  const [days, setDays] = useState(30)
  const [page, setPage] = useState(1)
  const f = useAsync(() => adminConsoleApi.payments({ q: q.trim() || undefined, status: status || undefined, days, page }), [q, status, days, page])
  const d = f.data
  return (
    <>
      <PageHead eyebrow={t('Crédits et revenus / Paiements')} title={t('Paiements')} description={t('Les paiements des commandes (SasPay, Stripe), tels que les fournisseurs de paiement les ont confirmés.')}>
        <select className="cc-select" value={days} onChange={(e) => { setDays(Number(e.target.value)); setPage(1) }} aria-label={t('Période')}>{[7, 30, 90, 365].map((v) => <option key={v} value={v}>{t('{n} derniers jours', { n: v })}</option>)}</select>
      </PageHead>
      {f.loading && !d ? <Loading /> : f.error ? <Failed error={f.error} onRetry={f.reload} /> : d && (
        <>
          {d.totals.length > 0 ? (
            <Strip items={d.totals.slice(0, 4).map((tot) => ({ label: t('Encaissé ({currency})', { currency: tot.currency }), value: money(tot.netMinor, tot.currency), note: t('{n} paiement(s) · {failed} échec(s)', { n: tot.payments, failed: tot.failed }) }))} />
          ) : <Notice tone="mint" title={t('Aucun paiement sur la période')}>{t('Les montants apparaîtront ici dès le premier paiement confirmé.')}</Notice>}
          <section className="cc-panel cc-table-panel">
            <div className="cc-table-toolbar">
              <Tabs value={status} onChange={(s) => { setStatus(s); setPage(1) }} options={['', 'captured', 'pending', 'failed', 'refunded'].map((s) => ({ id: s, label: s || t('Tous') }))} />
              <SearchBox value={q} onChange={(v) => { setQ(v); setPage(1) }} placeholder={t('Commande, organisation…')} />
            </div>
            {!d.payments.items.length ? <Empty icon="card" title={t('Aucun paiement ne correspond.')} /> : (
              <>
                <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Paiement')}</th><th>{t('Organisation')}</th><th>{t('Moyen')}</th><th>{t('Fournisseur')}</th><th>{t('Montant')}</th><th>{t('Statut')}</th><th>{t('Date')}</th></tr></thead><tbody>
                  {d.payments.items.map((p) => <tr key={p.id}><td><code>{shortId(p.id)}</code><small className="cc-sub">{t('Commande')} {shortId(p.orderId)}</small></td><td>{p.organizationName || '—'}</td><td>{p.method}</td><td>{p.provider || '—'}</td><td><strong>{money(p.amountMinor, p.currency)}</strong>{p.refundedMinor > 0 && <small className="cc-sub">{t('Remboursé')} {money(p.refundedMinor, p.currency)}</small>}</td><td><Status tone={toneOf(p.status)}>{p.status}</Status>{p.failureReason && <small className="cc-sub">{p.failureReason}</small>}</td><td>{when(p.createdAt)}</td></tr>)}
                </tbody></table></div>
                <Pager page={page} pageSize={d.payments.pageSize} total={d.payments.total} onPage={setPage} noun={t('paiements')} />
              </>
            )}
          </section>
        </>
      )}
    </>
  )
}

const CHANNEL_ICON: Record<string, 'send' | 'message' | 'globe'> = { whatsapp: 'send', web: 'globe' }

export function Integrations() {
  const { user } = useSession()
  const admin = isAdmin(user)
  const channels = useAsync(() => adminConsoleApi.channels(), [])
  const providers = useAsync(() => adminConsoleApi.providers(7), [])
  const [error, setError] = useState<ApiError | null>(null)
  const toggle = async (id: string, enable: boolean) => {
    const reason = enable ? '' : window.prompt(t('Raison de la désactivation (inscrite au journal d’audit)')) ?? ''
    if (!enable && !reason.trim()) return
    setError(null)
    try { await (enable ? adminConsoleApi.enableProvider(id) : adminConsoleApi.disableProvider(id, reason.trim())); providers.reload() } catch (err) { setError(toApiError(err)) }
  }
  const failing = (channels.data?.items ?? []).filter((c) => c.failed24h > 0)
  return (
    <>
      <PageHead eyebrow={t('Intégrations')} title={t('Canaux et fournisseurs')} description={t('Les canaux de conversation (web, WhatsApp) et les fournisseurs branchés (paiement, disponibilité…), avec leur santé réelle.')} />
      {channels.data && (failing.length
        ? <Notice tone="coral" title={t('{n} canal(aux) en échec sur 24 h', { n: failing.length })}>{failing.map((c) => `${c.channel} : ${c.lastError || t('erreur')}`).join(' · ')}</Notice>
        : <Notice tone="mint" title={t('Aucun échec sur les canaux depuis 24 h')} />)}
      {error && <Failed error={error} />}
      <h2 className="cc-section-title">{t('Canaux de conversation')}</h2>
      {channels.loading ? <Loading /> : channels.error ? <Failed error={channels.error} onRetry={channels.reload} /> : (
        <div className="cc-integration-grid">
          {channels.data!.items.map((c) => (
            <div className="cc-integration-card" key={c.channel}>
              <div className="cc-integration-top"><span className="cc-integration-logo"><CCIcon name={CHANNEL_ICON[c.channel.toLowerCase()] ?? 'message'} /></span><Status tone={channels.data!.configured[c.channel.toLowerCase()] === false ? 'neutral' : c.failed24h ? 'bad' : 'good'}>{channels.data!.configured[c.channel.toLowerCase()] === false ? t('Non configuré') : c.failed24h ? t('Échecs') : t('Opérationnel')}</Status></div>
              <h3>{c.channel}</h3>
              <p>{t('{n} conversation(s) en 7 j · {a} compte(s) relié(s)', { n: n(c.conversations7d), a: n(c.accounts) })}</p>
              <div className="cc-integration-meta"><span><small>{t('Événements 24 h')}</small><b>{n(c.events24h)}</b></span><span><small>{t('Échecs 24 h')}</small><b className={c.failed24h ? 'bad-text' : ''}>{n(c.failed24h)}</b></span></div>
              <footer><span>{t('Dernier événement')}</span><strong>{when(c.lastEventAt)}</strong></footer>
              {c.lastError && <p className="cc-error">{c.lastError}</p>}
            </div>
          ))}
          {Object.entries(channels.data!.configured).filter(([k]) => !channels.data!.items.some((c) => c.channel.toLowerCase() === k)).map(([k, v]) => (
            <div className="cc-integration-card" key={k}>
              <div className="cc-integration-top"><span className="cc-integration-logo"><CCIcon name="key" /></span><Status tone={v ? 'good' : 'neutral'}>{v ? t('Configuré') : t('Absent')}</Status></div>
              <h3>{k.replace(/_/g, ' ')}</h3><p>{t('Configuration détectée sur le serveur.')}</p>
            </div>
          ))}
        </div>
      )}
      <h2 className="cc-section-title">{t('Fournisseurs branchés')}</h2>
      <section className="cc-panel cc-table-panel">
        {providers.loading ? <Loading /> : providers.error ? <Failed error={providers.error} onRetry={providers.reload} /> : !providers.data?.length ? <Empty icon="plug" title={t('Aucun fournisseur branché')} /> : (
          <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Fournisseur')}</th><th>{t('Organisation')}</th><th>{t('Capacités')}</th><th>{t('Appels 7 j')}</th><th>{t('Échecs')}</th><th>{t('Latence p95')}</th><th>{t('Statut')}</th><th /></tr></thead><tbody>
            {providers.data.map((p) => (
              <tr key={p.id}>
                <td><strong>{p.name}</strong><small className="cc-sub">{p.code}</small></td><td>{p.organizationName || '—'}</td><td>{p.capabilities.join(', ')}</td><td>{n(p.attempts)}</td>
                <td>{n(p.failures)}{p.lastError && <small className="cc-sub" title={p.lastError}>{p.lastError.slice(0, 60)}</small>}</td><td>{p.p95Ms ? `${n(p.p95Ms)} ms` : '—'}</td>
                <td><Status tone={toneOf(p.status)}>{p.status}</Status> <Status tone={toneOf(p.healthStatus)}>{p.healthStatus}</Status></td>
                <td>{admin && (p.status === 'ACTIVE'
                  ? <button type="button" className="cc-secondary cc-small" onClick={() => void toggle(p.id, false)}>{t('Désactiver')}</button>
                  : <button type="button" className="cc-secondary cc-small" onClick={() => void toggle(p.id, true)}>{t('Activer')}</button>)}</td>
              </tr>
            ))}
          </tbody></table></div>
        )}
      </section>
    </>
  )
}

export function ExchangeRatePage() {
  const { user } = useSession()
  const admin = isAdmin(user)
  const current = useExchangeRate()
  const history = useAsync(() => exchangeRateApi.history(), [])
  const [rate, setRate] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [saved, setSaved] = useState(false)
  useEffect(() => { void loadExchangeRate(true) }, [])
  const fmt = (v: number) => v.toLocaleString(locale(), { maximumFractionDigits: 2 })
  const typed = Number(rate.replace(/\s/g, '').replace(',', '.'))
  const save = async (e: FormEvent) => {
    e.preventDefault()
    // A large jump is more often a typo than the market: ask before every price shows it.
    if (current && typed > 0 && Math.abs(typed - current.rate) / current.rate > 0.2
      && !window.confirm(t('Ce taux s’écarte de plus de 20 % du taux actuel ({old}). Enregistrer {new} CDF pour 1 USD ?', { old: fmt(current.rate), new: fmt(typed) }))) return
    setBusy(true); setError(null); setSaved(false)
    try { await exchangeRateApi.set(typed, note); setRate(''); setNote(''); setSaved(true); history.reload() } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }
  return (
    <>
      <PageHead eyebrow={t('Plateforme / Taux de change')} title={t('Taux de change')} description={t('Le taux CDF / USD saisi par un administrateur. Il sert uniquement à afficher un équivalent indicatif « ≈ » à côté des prix et à lire un budget donné en francs : les prix, commandes et paiements restent dans la devise du fournisseur.')} />
      <div className="cc-dashboard-grid">
        <Panel wide title={t('Taux en vigueur')} sub={current ? t('Saisi le {date}', { date: when(current.setAt) }) : t('Sans taux, aucune conversion n’est affichée.')}>
          <div className="cc-rate"><strong>{current ? t('1 USD = {rate} CDF', { rate: fmt(current.rate) }) : t('Aucun taux saisi')}</strong></div>
          <h3 className="cc-subtitle">{t('Historique')}</h3>
          {history.loading ? <Loading /> : history.error ? <Failed error={history.error} onRetry={history.reload} /> : !history.data?.length ? <p className="cc-muted cc-pad">{t('Aucun taux saisi')}</p> : (
            <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Taux')}</th><th>{t('Source ou remarque')}</th><th>{t('Date')}</th></tr></thead><tbody>
              {history.data.map((h) => <tr key={h.setAt + h.rate}><td><strong>{t('1 USD = {rate} CDF', { rate: fmt(h.rate) })}</strong></td><td>{h.note || '—'}</td><td>{when(h.setAt)}</td></tr>)}
            </tbody></table></div>
          )}
        </Panel>
        <Panel title={t('Nouveau taux')}>
          {admin ? (
            <form className="cc-form" onSubmit={(e) => void save(e)}>
              <label>{t('Nouveau taux (francs congolais pour 1 USD)')}<input inputMode="decimal" value={rate} onChange={(e) => setRate(e.target.value)} placeholder="2850" required /></label>
              {typed > 0 && <p className="cc-muted" role="status">{t('Aperçu : 100 USD ≈ {cdf}', { cdf: formatMoney(Math.round(typed * 100), 'CDF') })}</p>}
              <label>{t('Source ou remarque')} <small>{t('facultatif')}</small><input value={note} onChange={(e) => setNote(e.target.value)} maxLength={200} placeholder={t('Ex. : taux BCC du jour')} /></label>
              {error && <p className="cc-error" role="alert">{error.code === 'INVALID_RATE' ? t('Le taux doit être compris entre 1 000 et 10 000 francs pour 1 USD.') : errorText(error)}</p>}
              {saved && <p className="cc-ok" role="status">{t('Taux enregistré.')}</p>}
              <button type="submit" className="cc-primary" disabled={busy || !rate.trim()}>{busy ? t('Enregistrement…') : t('Enregistrer le taux')}</button>
            </form>
          ) : <p className="cc-muted cc-pad">{t('Seul un administrateur peut modifier le taux.')}</p>}
        </Panel>
      </div>
    </>
  )
}

export function Staff() {
  const { user } = useSession()
  const admin = isAdmin(user)
  const s = useAsync(() => adminConsoleApi.security(), [])
  const [error, setError] = useState<string | null>(null)
  const revoke = async (userId: string, name: string, role: string) => {
    if (!window.confirm(t('Retirer le rôle « {role} » à {name} ?', { role, name }))) return
    setError(null)
    try { await adminManageApi.revokeRole(userId, role); s.reload() } catch (err) { setError(changeError(toApiError(err))) }
  }
  const d = s.data
  return (
    <>
      <PageHead eyebrow={t('Sécurité / Équipe')} title={t('Administrateurs et rôles')} description={t('Les comptes de l’équipe Trezutil, leurs rôles, leurs connexions et les derniers événements de sécurité. Pour donner un rôle, ouvre le compte dans Comptes clients.')} />
      {error && <p className="cc-error" role="alert">{error}</p>}
      {s.loading ? <Loading /> : s.error ? <Failed error={s.error} onRetry={s.reload} /> : d && (
        <div className="cc-dashboard-grid">
          <Panel wide title={t('Équipe')} sub={t('{n} compte(s)', { n: d.staff.length })}>
            {!d.staff.length ? <Empty icon="lock" title={t('Aucun compte d’équipe')} /> : (
              <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Membre')}</th><th>{t('Rôles')}</th><th>{t('Statut')}</th><th>{t('Dernière connexion')}</th><th>{t('Échecs de connexion')}</th></tr></thead><tbody>
                {d.staff.map((m) => <tr key={m.userId}><td><Person name={m.displayName || m.email} sub={m.email} /></td><td>{m.roles.map((r) => <span className="cc-chip" key={r}>{r}{admin && ['admin', 'operator', 'moderator'].includes(r) && <button type="button" aria-label={t('Retirer le rôle {role}', { role: r })} title={t('Retirer')} onClick={() => void revoke(m.userId, m.displayName || m.email, r)}>×</button>}</span>)}</td><td><Status tone={m.locked ? 'bad' : toneOf(m.status)}>{m.locked ? t('Verrouillé') : m.status}</Status></td><td>{when(m.lastLoginAt)}</td><td>{n(m.failedAttempts)}</td></tr>)}
              </tbody></table></div>
            )}
          </Panel>
          <Panel title={t('Événements de sécurité')} sub={t('Les plus récents')}>
            {!d.events.length ? <p className="cc-muted cc-pad">{t('Aucun événement.')}</p> : (
              <div className="cc-audit-mini">{d.events.slice(0, 12).map((e) => <div key={e.id}><span><CCIcon name="lock" size={14} /></span><p><strong>{e.action}</strong><small>{e.actorName || t('Système')} · {when(e.at)}</small></p></div>)}</div>
            )}
          </Panel>
        </div>
      )}
    </>
  )
}

export function Audit() {
  const [filters, setFilters] = useState<AuditFilters>({})
  const [draft, setDraft] = useState<AuditFilters>({})
  const [page, setPage] = useState(1)
  const audit = useAsync(() => adminApi.audit(filters, page, 30), [filters, page])
  const apply = (e: FormEvent) => { e.preventDefault(); setFilters({ ...draft }); setPage(1) }
  return (
    <>
      <PageHead eyebrow={t('Sécurité / Audit')} title={t('Journal d’audit')} description={t('Chaque action sensible, qui l’a faite et quand. Rien ici ne peut modifier une entrée.')} />
      <form className="cc-filter-bar" onSubmit={apply}>
        <input value={draft.action ?? ''} onChange={(e) => setDraft({ ...draft, action: e.target.value })} placeholder={t('Action (ex. order.)')} aria-label={t('Action')} />
        <input value={draft.targetType ?? ''} onChange={(e) => setDraft({ ...draft, targetType: e.target.value })} placeholder={t('Type de cible')} aria-label={t('Type de cible')} />
        <input type="date" value={draft.from ?? ''} onChange={(e) => setDraft({ ...draft, from: e.target.value })} aria-label={t('Du')} />
        <input type="date" value={draft.to ?? ''} onChange={(e) => setDraft({ ...draft, to: e.target.value })} aria-label={t('Au')} />
        <button type="submit" className="cc-secondary"><CCIcon name="filter" />{t('Filtrer')}</button>
        <button type="button" className="cc-text-btn" onClick={() => { setDraft({}); setFilters({}); setPage(1) }}>{t('Réinitialiser')}</button>
      </form>
      <section className="cc-panel cc-table-panel">
        {audit.loading ? <Loading /> : audit.error ? <Failed error={audit.error} onRetry={audit.reload} /> : !audit.data?.items.length ? <Empty icon="book" title={t('Aucune entrée ne correspond.')} /> : (
          <>
            <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Date')}</th><th>{t('Action')}</th><th>{t('Par')}</th><th>{t('Cible')}</th><th>{t('Détails')}</th></tr></thead><tbody>
              {audit.data.items.map((e) => <tr key={e.id}><td>{when(e.at)}</td><td><code>{e.action}</code></td><td>{e.actorName || (e.actorId ? shortId(e.actorId) : t('Système'))}</td><td>{e.targetType}{e.targetId && <small className="cc-sub"><code>{shortId(e.targetId)}</code></small>}</td><td className="cc-wrap"><code className="cc-meta">{Object.keys(e.metadata).length ? JSON.stringify(e.metadata).slice(0, 160) : '—'}</code></td></tr>)}
            </tbody></table></div>
            <Pager page={page} pageSize={audit.data.pageSize} total={audit.data.total} onPage={setPage} noun={t('entrées')} />
          </>
        )}
      </section>
    </>
  )
}
