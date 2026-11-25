import { useState } from 'react'
import { adminApi, isAdmin } from '@/api/admin'
import { toApiError } from '@/api/client'
import { adminManageApi, type PlanInput, type PolicyRow } from '@/api/adminManage'
import { adminConsoleApi, type Plan } from '@/api/console'
import { t } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { newId } from '@/ui/kit'
import { CCIcon } from '@/admin/icons'
import { changeError, Empty, Failed, Loading, Modal, money, n, PageHead, Panel, Person, Status, Strip, toneOf, day } from '@/admin/ui'

/*
 * Credits and billing: the plans (create, change, archive or delete an unused one), the subscriptions of accounts to
 * them (subscribe, change plan, cancel; the plan's credits are granted each month into the charged account), the
 * pricing rules of metered actions, and manual credit grants. Administrators change; operators read.
 */

const EMPTY_PLAN: PlanInput = { name: '', slug: '', price_minor: 0, currency: 'USD', billing_cycle: 'monthly', included_credits: 0, credit_pack: 0, status: 'active' }
const slugify = (s: string) => s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')

function PlanDialog({ plan, onClose, onSaved }: { plan: Plan | null; onClose: () => void; onSaved: () => void }) {
  const [v, setV] = useState<PlanInput>(plan ? {
    name: plan.name, slug: plan.slug, price_minor: plan.priceMinor, currency: (plan.currency === 'CDF' ? 'CDF' : 'USD'), billing_cycle: plan.billingCycle === 'yearly' ? 'yearly' : 'monthly',
    included_credits: plan.quota, credit_pack: plan.creditPack, status: (['active', 'inactive', 'archived'].includes(plan.status) ? plan.status : 'active') as PlanInput['status'],
  } : EMPTY_PLAN)
  const [price, setPrice] = useState(plan ? String(plan.priceMinor / 100) : '0')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const save = async () => {
    setBusy(true); setError(null)
    const body = { ...v, price_minor: Math.round(Number(price.replace(',', '.')) * 100) || 0 }
    try { await (plan ? adminManageApi.updatePlan(plan.id, body) : adminManageApi.createPlan(body)); onSaved() } catch (e) { setError(changeError(toApiError(e))) } finally { setBusy(false) }
  }
  return (
    <Modal title={plan ? t('Modifier le plan') : t('Nouveau plan')} onClose={onClose} onSubmit={() => void save()} submitLabel={plan ? t('Enregistrer') : t('Créer le plan')} busy={busy} error={error}>
      <label>{t('Nom')}<input value={v.name} onChange={(e) => setV({ ...v, name: e.target.value, slug: plan ? v.slug : slugify(e.target.value) })} required minLength={2} maxLength={80} /></label>
      <label>{t('Identifiant (slug)')}<input value={v.slug} onChange={(e) => setV({ ...v, slug: e.target.value })} required pattern="[a-z0-9]+(-[a-z0-9]+)*" /></label>
      <div className="cc-two">
        <label>{t('Prix')}<input inputMode="decimal" value={price} onChange={(e) => setPrice(e.target.value)} /></label>
        <label>{t('Devise')}<select value={v.currency} onChange={(e) => setV({ ...v, currency: e.target.value as PlanInput['currency'] })}><option>USD</option><option>CDF</option></select></label>
      </div>
      <div className="cc-two">
        <label>{t('Cycle')}<select value={v.billing_cycle} onChange={(e) => setV({ ...v, billing_cycle: e.target.value as PlanInput['billing_cycle'] })}><option value="monthly">{t('Mensuel')}</option><option value="yearly">{t('Annuel')}</option></select></label>
        <label>{t('Statut')}<select value={v.status} onChange={(e) => setV({ ...v, status: e.target.value as PlanInput['status'] })}><option value="active">{t('Actif')}</option><option value="inactive">{t('Inactif')}</option><option value="archived">{t('Archivé')}</option></select></label>
      </div>
      <div className="cc-two">
        <label>{t('Crédits inclus par mois')}<input type="number" min={0} value={v.included_credits} onChange={(e) => setV({ ...v, included_credits: Number(e.target.value) })} /></label>
        <label>{t('Pack de crédits')} <small>{t('0 : pas un pack')}</small><input type="number" min={0} value={v.credit_pack} onChange={(e) => setV({ ...v, credit_pack: Number(e.target.value) })} /></label>
      </div>
    </Modal>
  )
}

function SubscribeDialog({ plans, onClose, onSaved, presetUser }: { plans: Plan[]; onClose: () => void; onSaved: () => void; presetUser?: { id: string; name: string } }) {
  const [q, setQ] = useState('')
  const users = useAsync(() => (q.trim().length >= 2 ? adminApi.users({ q: q.trim(), pageSize: 8 }) : Promise.resolve(null)), [q])
  const [user, setUser] = useState(presetUser ?? null)
  const active = plans.filter((p) => p.status === 'active')
  const [planId, setPlanId] = useState(active[0]?.id ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const save = async () => {
    if (!user || !planId) { setError(t('Choisis un compte et un plan.')); return }
    setBusy(true); setError(null)
    try { await adminManageApi.subscribe(user.id, planId); onSaved() } catch (e) { setError(changeError(toApiError(e))) } finally { setBusy(false) }
  }
  return (
    <Modal title={t('Abonner un compte')} onClose={onClose} onSubmit={() => void save()} submitLabel={t('Abonner')} busy={busy} error={error}>
      {user ? (
        <div className="cc-chosen"><Person name={user.name} /><button type="button" className="cc-text-btn" onClick={() => setUser(null)}>{t('Changer')}</button></div>
      ) : (
        <label>{t('Compte')}<input value={q} onChange={(e) => setQ(e.target.value)} placeholder={t('Nom, e-mail ou téléphone…')} autoFocus />
          {users.data?.items.length ? (
            <div className="cc-pick">{users.data.items.map((u) => <button type="button" key={u.id} onClick={() => setUser({ id: u.id, name: u.displayName || u.email || u.phone })}><Person name={u.displayName || u.email || u.phone} sub={u.email || u.phone} /></button>)}</div>
          ) : q.trim().length >= 2 && !users.loading ? <small>{t('Aucun compte ne correspond.')}</small> : null}
        </label>
      )}
      <label>{t('Plan')}<select value={planId} onChange={(e) => setPlanId(e.target.value)}>{active.map((p) => <option key={p.id} value={p.id}>{p.name} · {money(p.priceMinor, p.currency)} · {t('{n} crédits/mois', { n: p.quota })}</option>)}</select></label>
      <p className="cc-muted">{t('Le compte reçoit tout de suite les crédits du mois, puis chaque mois tant que l’abonnement est actif. S’il avait déjà un abonnement, il passe sur ce plan.')}</p>
    </Modal>
  )
}

export function GrantCreditsDialog({ user, onClose, onSaved }: { user: { id: string; name: string }; onClose: () => void; onSaved: () => void }) {
  const [amount, setAmount] = useState('')
  const [reason, setReason] = useState('')
  const [key] = useState(() => newId())
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const save = async () => {
    setBusy(true); setError(null)
    try { await adminManageApi.grantCredits(user.id, Number(amount), reason.trim(), key); onSaved() } catch (e) { setError(changeError(toApiError(e))) } finally { setBusy(false) }
  }
  return (
    <Modal title={t('Accorder des crédits à {name}', { name: user.name })} onClose={onClose} onSubmit={() => void save()} submitLabel={t('Accorder')} busy={busy} error={error}>
      <label>{t('Nombre de crédits')}<input type="number" min={1} max={1000000} value={amount} onChange={(e) => setAmount(e.target.value)} required /></label>
      <label>{t('Raison')} <small>{t('inscrite au journal d’audit')}</small><input value={reason} onChange={(e) => setReason(e.target.value)} required minLength={3} maxLength={300} placeholder={t('Ex. : geste commercial, correction')} /></label>
    </Modal>
  )
}

function PolicyDialog({ policy, actions, onClose, onSaved }: { policy: PolicyRow | null; actions: string[]; onClose: () => void; onSaved: () => void }) {
  const [v, setV] = useState<Omit<PolicyRow, 'id'>>(policy ? { action_type: policy.action_type, channel: policy.channel, actor_type: policy.actor_type, cost: policy.cost, active: policy.active } : { action_type: actions[0] ?? '', channel: 'other', actor_type: 'customer', cost: 1, active: true })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const save = async () => {
    setBusy(true); setError(null)
    try { await (policy ? adminManageApi.updatePolicy({ ...v, id: policy.id }) : adminManageApi.createPolicy(v)); onSaved() } catch (e) { setError(changeError(toApiError(e))) } finally { setBusy(false) }
  }
  return (
    <Modal title={policy ? t('Modifier la règle') : t('Nouvelle règle de tarification')} onClose={onClose} onSubmit={() => void save()} submitLabel={policy ? t('Enregistrer') : t('Créer la règle')} busy={busy} error={error}>
      <label>{t('Action')}<select value={v.action_type} onChange={(e) => setV({ ...v, action_type: e.target.value })}>{actions.map((a) => <option key={a} value={a}>{a}</option>)}</select></label>
      <label>{t('Coût en crédits')}<input type="number" min={0} max={100000} value={v.cost} onChange={(e) => setV({ ...v, cost: Number(e.target.value) })} /></label>
      <label className="cc-check"><input type="checkbox" checked={v.active} onChange={(e) => setV({ ...v, active: e.target.checked })} />{t('Active (une seule règle active par action)')}</label>
    </Modal>
  )
}

export function Billing() {
  const { user } = useSession()
  const admin = isAdmin(user)
  const m = useAsync(() => adminConsoleApi.monetization(30), [])
  const subs = useAsync(() => adminManageApi.subscriptions(), [])
  const policies = useAsync(() => adminManageApi.policies(), [])
  const actions = useAsync(() => adminManageApi.billableActions(), [])
  const [dialog, setDialog] = useState<{ kind: 'plan'; plan: Plan | null } | { kind: 'subscribe' } | { kind: 'policy'; policy: PolicyRow | null } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const reload = () => { setDialog(null); m.reload(); subs.reload(); policies.reload() }
  const act = async (fn: () => Promise<unknown>, confirmText?: string) => {
    if (confirmText && !window.confirm(confirmText)) return
    setError(null)
    try { await fn(); reload() } catch (e) { setError(changeError(toApiError(e))) }
  }
  const d = m.data
  const plans = d?.plans ?? []
  return (
    <>
      <PageHead eyebrow={t('Crédits et revenus')} title={t('Crédits et facturation')} description={t('L’économie des actions commerciales : la conversation reste toujours gratuite.')}>
        {admin && <button type="button" className="cc-primary" onClick={() => setDialog({ kind: 'plan', plan: null })}><CCIcon name="plus" />{t('Nouveau plan')}</button>}
      </PageHead>
      <div className="cc-principle"><CCIcon name="coins" /><div><strong>{t('Les crédits financent les actions, jamais la conversation.')}</strong><p>{t('Ils sont réservés avant une action mesurable (recherche, vérification, rédaction par l’IA), consommés seulement si elle aboutit, libérés sinon.')}</p></div></div>
      {error && <p className="cc-error" role="alert">{error}</p>}
      {m.loading ? <Loading /> : m.error ? <Failed error={m.error} onRetry={m.reload} /> : d && (
        <>
          <Strip items={[
            { label: t('Comptes de crédits'), value: n(d.credits.accounts) },
            { label: t('Crédits attribués'), value: n(d.credits.granted) },
            { label: t('Crédits consommés'), value: n(d.credits.consumed) },
            { label: t('Crédits réservés'), value: n(d.credits.reserved) },
          ]} />
          <div className="cc-dashboard-grid">
            <Panel wide title={t('Plans et abonnements')} sub={t('Les plans proposés par Trezutil')} action={admin && <button type="button" className="cc-text-btn" onClick={() => setDialog({ kind: 'plan', plan: null })}><CCIcon name="plus" size={14} />{t('Nouveau plan')}</button>}>
              {!plans.length ? <Empty icon="card" title={t('Aucun plan publié')}>{admin ? t('Crée le premier plan : son prix, ses crédits inclus chaque mois.') : ''}</Empty> : (
                <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Plan')}</th><th>{t('Prix')}</th><th>{t('Cycle')}</th><th>{t('Crédits inclus')}</th><th>{t('Abonnés')}</th><th>{t('Statut')}</th><th /></tr></thead><tbody>
                  {plans.map((p) => (
                    <tr key={p.id}>
                      <td><strong>{p.name}</strong><small className="cc-sub">{p.slug}</small></td><td>{money(p.priceMinor, p.currency)}</td><td>{p.billingCycle === 'yearly' ? t('Annuel') : t('Mensuel')}</td>
                      <td>{n(p.quota)}{p.creditPack > 0 && <small className="cc-sub">{t('Pack : {n}', { n: n(p.creditPack) })}</small>}</td><td>{n(p.subscribers)}</td><td><Status tone={toneOf(p.status)}>{p.status}</Status></td>
                      <td>{admin && <div className="cc-row-actions">
                        <button type="button" className="cc-icon-btn" aria-label={t('Modifier {name}', { name: p.name })} title={t('Modifier')} onClick={() => setDialog({ kind: 'plan', plan: p })}><CCIcon name="settings" size={15} /></button>
                        <button type="button" className="cc-icon-btn danger" aria-label={t('Supprimer {name}', { name: p.name })} title={t('Supprimer')}
                          onClick={() => void act(async () => { const r = await adminManageApi.deletePlan(p.id); if (r.archived) window.alert(t('Ce plan a déjà été utilisé : il est archivé (plus proposé) au lieu d’être supprimé.')) }, t('Supprimer le plan « {name} » ? S’il a déjà été utilisé, il sera archivé.', { name: p.name }))}>
                          <CCIcon name="close" size={15} /></button>
                      </div>}</td>
                    </tr>
                  ))}
                </tbody></table></div>
              )}
            </Panel>
            <Panel title={t('Grand livre par type')} sub={t('Mouvements de crédits sur {n} jours', { n: d.days })}>
              {Object.keys(d.ledgerByType).length ? <div className="cc-bars">{Object.entries(d.ledgerByType).map(([k, v]) => <div key={k}><span>{k}</span><strong>{n(v)}</strong></div>)}</div> : <p className="cc-muted cc-pad">{t('Aucun mouvement.')}</p>}
            </Panel>
          </div>
        </>
      )}
      <h2 className="cc-section-title">{t('Abonnements')}</h2>
      <section className="cc-panel cc-table-panel">
        <div className="cc-table-toolbar"><span className="cc-muted">{t('Un abonnement donne au compte les crédits du plan chaque mois.')}</span>{admin && <button type="button" className="cc-secondary" disabled={!plans.some((p) => p.status === 'active')} onClick={() => setDialog({ kind: 'subscribe' })}><CCIcon name="plus" />{t('Abonner un compte')}</button>}</div>
        {subs.loading ? <Loading /> : subs.error ? <Failed error={subs.error} onRetry={subs.reload} /> : !subs.data?.length ? <Empty icon="users" title={t('Aucun abonnement')} /> : (
          <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Compte')}</th><th>{t('Plan')}</th><th>{t('Depuis')}</th><th>{t('Période')}</th><th>{t('Statut')}</th><th /></tr></thead><tbody>
            {subs.data.map((s) => (
              <tr key={s.id}>
                <td><Person name={s.userName || s.userEmail || s.userPhone} sub={s.userEmail || s.userPhone} /></td><td><strong>{s.planName}</strong></td><td>{day(s.startedAt)}</td><td>{s.periodId}</td><td><Status tone={toneOf(s.status)}>{s.status}</Status></td>
                <td>{admin && s.status === 'active' && <div className="cc-row-actions"><button type="button" className="cc-secondary cc-small" onClick={() => void act(() => adminManageApi.cancelSubscription(s.id), t('Résilier l’abonnement de {name} ? Les crédits déjà accordés restent.', { name: s.userName || s.userEmail }))}>{t('Résilier')}</button></div>}</td>
              </tr>
            ))}
          </tbody></table></div>
        )}
      </section>
      <h2 className="cc-section-title">{t('Tarification des actions')}</h2>
      <section className="cc-panel cc-table-panel">
        <div className="cc-table-toolbar"><span className="cc-muted">{t('Sans règle active, une action est gratuite.')}</span>{admin && <button type="button" className="cc-secondary" disabled={!actions.data?.length} onClick={() => setDialog({ kind: 'policy', policy: null })}><CCIcon name="plus" />{t('Nouvelle règle')}</button>}</div>
        {policies.loading ? <Loading /> : policies.error ? <Failed error={policies.error} onRetry={policies.reload} /> : !policies.data?.length ? <Empty icon="coins" title={t('Aucune règle de tarification')}>{t('Toutes les actions sont gratuites pour l’instant.')}</Empty> : (
          <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Action')}</th><th>{t('Crédits')}</th><th>{t('Statut')}</th><th /></tr></thead><tbody>
            {policies.data.map((p) => (
              <tr key={p.id}>
                <td><strong>{p.action_type}</strong></td><td><strong>{n(p.cost)}</strong></td><td><Status tone={p.active ? 'good' : 'neutral'}>{p.active ? t('Active') : t('Inactive')}</Status></td>
                <td>{admin && <div className="cc-row-actions">
                  <button type="button" className="cc-secondary cc-small" onClick={() => void act(() => adminManageApi.updatePolicy({ ...p, active: !p.active }))}>{p.active ? t('Désactiver') : t('Activer')}</button>
                  <button type="button" className="cc-icon-btn" aria-label={t('Modifier')} title={t('Modifier')} onClick={() => setDialog({ kind: 'policy', policy: p })}><CCIcon name="settings" size={15} /></button>
                  <button type="button" className="cc-icon-btn danger" aria-label={t('Supprimer')} title={t('Supprimer')} onClick={() => void act(() => adminManageApi.deletePolicy(p.id), t('Supprimer cette règle ? L’action {action} deviendra gratuite.', { action: p.action_type }))}><CCIcon name="close" size={15} /></button>
                </div>}</td>
              </tr>
            ))}
          </tbody></table></div>
        )}
      </section>
      {dialog?.kind === 'plan' && <PlanDialog plan={dialog.plan} onClose={() => setDialog(null)} onSaved={reload} />}
      {dialog?.kind === 'subscribe' && <SubscribeDialog plans={plans} onClose={() => setDialog(null)} onSaved={reload} />}
      {dialog?.kind === 'policy' && <PolicyDialog policy={dialog.policy} actions={actions.data ?? []} onClose={() => setDialog(null)} onSaved={reload} />}
    </>
  )
}

export { SubscribeDialog }
