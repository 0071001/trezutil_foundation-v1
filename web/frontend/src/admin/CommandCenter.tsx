import { Link } from 'react-router'
import { adminApi } from '@/api/admin'
import { adminConsoleApi } from '@/api/console'
import { supplierApi } from '@/api/supplier'
import { useExchangeRate } from '@/lib/exchangeRate'
import { t } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { CCIcon } from '@/admin/icons'
import { Empty, Failed, Loading, Metric, n, Notice, PageHead, Panel, Status, toneOf, when, money, shortId, Strip } from '@/admin/ui'

/* The Command Center (the platform at a glance), the alerts (operations stuck or failed) and the services' health. */

export function CommandCenter() {
  const { user } = useSession()
  const overview = useAsync(() => adminApi.overview(), [])
  const health = useAsync(() => adminConsoleApi.health(), [])
  const ops = useAsync(() => adminConsoleApi.operations(7), [])
  const queue = useAsync(() => supplierApi.queue('submitted'), [])
  const channels = useAsync(() => adminConsoleApi.channels(), [])
  const orders = useAsync(() => adminApi.orders({ page: 1, pageSize: 6 }), [])
  const rate = useExchangeRate()

  const o = overview.data
  const exceptions = ops.data ? Object.values(ops.data.counts).reduce((a, b) => a + b, 0) : 0
  const channelFailures = channels.data ? channels.data.items.reduce((a, c) => a + c.failed24h, 0) : 0
  const todo = [
    queue.data?.length ? { icon: 'shield' as const, tone: 'amber', title: t('Dossiers fournisseurs à vérifier'), sub: t('Soumis et en attente d’une décision de l’équipe.'), count: queue.data.length, to: 'verification' } : null,
    exceptions ? { icon: 'alert' as const, tone: 'coral', title: t('Opérations en anomalie'), sub: t('Bloquées ou en échec sur les 7 derniers jours.'), count: exceptions, to: 'alerts' } : null,
    channelFailures ? { icon: 'plug' as const, tone: 'coral', title: t('Échecs sur les canaux'), sub: t('Événements en échec sur les dernières 24 h.'), count: channelFailures, to: 'integrations' } : null,
    !rate ? { icon: 'globe' as const, tone: 'blue', title: t('Aucun taux CDF / USD saisi'), sub: t('Sans taux, aucun équivalent n’est affiché à côté des prix.'), count: '!', to: 'exchange-rate' } : null,
  ].filter((x): x is NonNullable<typeof x> => x !== null)
  const name = (user?.display_name || '').split(/\s+/)[0]

  return (
    <>
      <PageHead eyebrow={t('Vue d’ensemble / Command Center')} title={name ? t('Bonjour, {name}', { name }) : t('Command Center')} description={t('Ce qui demande l’attention de l’équipe sur la plateforme, d’après les données réelles.')} />
      {todo.length > 0 && <Notice title={t('{n} point(s) à traiter', { n: todo.length })}>{todo.map((x) => x.title).join(' · ')}</Notice>}
      {overview.loading ? <Loading /> : overview.error ? <Failed error={overview.error} onRetry={overview.reload} /> : o && (
        <div className="cc-metric-grid">
          <Metric label={t('Comptes')} value={n(o.users.total)} note={t('+{n} en 7 j', { n: n(o.users.newLast7Days) })} icon="users" />
          <Metric label={t('Comptes actifs')} value={n(o.users.active)} note={o.users.suspended ? t('{n} suspendu(s)', { n: n(o.users.suspended) }) : ''} icon="check" tone={o.users.suspended ? 'amber' : undefined} />
          <Metric label={t('Organisations')} value={n(o.organizations)} icon="building" />
          <Metric label={t('Offres publiées')} value={n(o.listingsByStatus.published ?? o.listingsByStatus.PUBLISHED ?? 0)} note={t('{n} au total', { n: n(Object.values(o.listingsByStatus).reduce((a, b) => a + b, 0)) })} icon="layers" />
          <Metric label={t('Commandes')} value={n(o.orders.total)} note={t('+{n} en 7 j', { n: n(o.orders.last7Days) })} icon="cart" />
          <Metric label={t('Conversations (7 j)')} value={n(o.conversationsLast7Days)} note={t('{n} réservation(s)', { n: n(o.reservations) })} icon="message" />
        </div>
      )}
      <div className="cc-dashboard-grid">
        <Panel wide title={t('Commandes récentes')} sub={t('Les dernières commandes de la plateforme')} action={<Link className="cc-text-btn" to="/admin/orders">{t('Tout voir')}</Link>}>
          {orders.loading ? <Loading /> : orders.error ? <Failed error={orders.error} onRetry={orders.reload} /> : !orders.data?.items.length ? <Empty icon="cart" title={t('Aucune commande pour l’instant')} /> : (
            <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Commande')}</th><th>{t('Fournisseur')}</th><th>{t('Client')}</th><th>{t('Montant')}</th><th>{t('Statut')}</th></tr></thead><tbody>
              {orders.data.items.map((r) => <tr key={r.id}><td><strong>{r.title || shortId(r.id)}</strong><small className="cc-sub">{when(r.createdAt)}</small></td><td>{r.organizationName || '—'}</td><td>{r.customerName || '—'}</td><td>{money(r.totalMinor, r.currency)}</td><td><Status tone={toneOf(r.status)}>{r.status}</Status></td></tr>)}
            </tbody></table></div>
          )}
        </Panel>
        <Panel title={t('À traiter')} sub={t('Prioritaire d’abord')}>
          {todo.length === 0 ? <Empty icon="check" title={t('Rien en attente')}>{t('Aucun dossier, incident ou réglage ne demande d’action.')}</Empty> : (
            <div className="cc-attention-list">
              {todo.map((x) => (
                <Link key={x.title} to={`/admin/${x.to}`}>
                  <span className={`cc-attention-icon ${x.tone}`}><CCIcon name={x.icon} /></span>
                  <p><strong>{x.title}</strong><small>{x.sub}</small></p>
                  <span className="cc-count">{x.count}</span>
                </Link>
              ))}
            </div>
          )}
        </Panel>
        <Panel wide title={t('Comptes par rôle')} sub={t('Répartition des comptes de la plateforme')}>
          {o ? (
            <div className="cc-bars">
              {Object.entries(o.users.byRole).sort((a, b) => b[1] - a[1]).map(([role, count]) => (
                <div key={role}><span>{role}</span><strong>{n(count)}</strong><i><b style={{ width: `${o.users.total ? Math.max(2, (count / o.users.total) * 100) : 0}%` }} /></i></div>
              ))}
              {Object.keys(o.users.byRole).length === 0 && <p className="cc-muted">{t('Aucun rôle attribué.')}</p>}
            </div>
          ) : <Loading />}
        </Panel>
        <Panel title={t('Santé de la plateforme')} sub={health.data ? t('Vérifiée le {when}', { when: when(health.data.checkedAt) }) : ''} action={health.data && <Status tone={toneOf(health.data.status)}>{health.data.status}</Status>}>
          {health.loading ? <Loading /> : health.error ? <Failed error={health.error} onRetry={health.reload} /> : (
            <div className="cc-health-list">
              {health.data!.components.slice(0, 7).map((c) => <div key={c.name}><span><i className={toneOf(c.status) === 'good' ? '' : 'warn-dot'} />{c.name}</span><small>{c.latencyMs ? `${n(c.latencyMs)} ms` : ''}</small><Status tone={toneOf(c.status)}>{c.status}</Status></div>)}
              <Link className="cc-text-btn cc-pad" to="/admin/health">{t('Détail des services')} <CCIcon name="arrow" size={14} /></Link>
            </div>
          )}
        </Panel>
      </div>
    </>
  )
}

export function Alerts() {
  const ops = useAsync(() => adminConsoleApi.operations(7), [])
  const d = ops.data
  return (
    <>
      <PageHead eyebrow={t('Vue d’ensemble / Alertes')} title={t('Alertes et incidents')} description={d ? t('Commandes, paiements et opérations en échec ou bloqués depuis plus de {h} h, sur les {days} derniers jours.', { h: d.stuckAfterHours, days: d.days }) : t('Ce qui est en échec ou bloqué sur la plateforme.')} />
      {ops.loading ? <Loading /> : ops.error ? <Failed error={ops.error} onRetry={ops.reload} /> : d && (
        <>
          <Strip items={Object.entries(d.counts).slice(0, 4).map(([k, v]) => ({ label: k.replace(/_/g, ' '), value: n(v) }))} />
          <Panel title={t('Anomalies récentes')} sub={t('Les plus récentes d’abord')}>
            {!d.recent.length ? <Empty icon="check" title={t('Aucune anomalie')}>{t('Rien n’est bloqué ni en échec sur la période.')}</Empty> : (
              <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Type')}</th><th>{t('Référence')}</th><th>{t('Organisation')}</th><th>{t('Détail')}</th><th>{t('Statut')}</th><th>{t('Date')}</th></tr></thead><tbody>
                {d.recent.map((r) => <tr key={r.kind + r.id}><td><strong>{r.kind}</strong></td><td><code>{r.reference || shortId(r.id)}</code></td><td>{r.business || '—'}</td><td className="cc-wrap">{r.detail || '—'}</td><td><Status tone={toneOf(r.status)}>{r.status}</Status></td><td>{when(r.at)}</td></tr>)}
              </tbody></table></div>
            )}
          </Panel>
        </>
      )}
    </>
  )
}

export function HealthPage() {
  const health = useAsync(() => adminConsoleApi.health(), [])
  const h = health.data
  return (
    <>
      <PageHead eyebrow={t('Système / Santé')} title={t('Santé des services')} description={t('L’état réel de chaque service et dépendance, mesuré à l’ouverture de cette page.')}>
        <button type="button" className="cc-secondary" onClick={health.reload}><CCIcon name="pulse" />{t('Revérifier')}</button>
      </PageHead>
      {health.loading ? <Loading /> : health.error ? <Failed error={health.error} onRetry={health.reload} /> : h && (
        <>
          <Notice tone={toneOf(h.status) === 'good' ? 'mint' : 'amber'} title={toneOf(h.status) === 'good' ? t('Tous les services répondent') : t('Au moins un service ne répond pas normalement')}>{t('Vérifié le {when}', { when: when(h.checkedAt) })}</Notice>
          <div className="cc-dashboard-grid">
            <Panel wide title={t('Services et dépendances')}>
              <div className="cc-table-wrap"><table className="cc-table"><thead><tr><th>{t('Composant')}</th><th>{t('Type')}</th><th>{t('Latence')}</th><th>{t('Détail')}</th><th>{t('État')}</th></tr></thead><tbody>
                {h.components.map((c) => <tr key={c.name}><td><strong>{c.name}</strong></td><td>{c.kind}</td><td>{c.latencyMs ? `${n(c.latencyMs)} ms` : '—'}</td><td className="cc-wrap">{c.detail || '—'}</td><td><Status tone={toneOf(c.status)}>{c.status}</Status></td></tr>)}
              </tbody></table></div>
            </Panel>
            <Panel title={t('Configuration détectée')} sub={t('Clés et services présents sur le serveur (jamais leur valeur)')}>
              <div className="cc-health-list">
                {Object.entries(h.configured).map(([k, v]) => <div key={k}><span><i className={v ? '' : 'warn-dot'} />{k.replace(/_/g, ' ')}</span><small /><Status tone={v ? 'good' : 'neutral'}>{v ? t('Configuré') : t('Absent')}</Status></div>)}
              </div>
            </Panel>
          </div>
        </>
      )}
    </>
  )
}
