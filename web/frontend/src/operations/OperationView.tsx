import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import type { MissionStep } from '@/api/missions'
import { formatMinor } from '@/lib/format'
import { conversationStore, useConversations } from '@/conversation/store'
import { conversationNodes, FLOW_KINDS, FLOW_STATUS, layout, NODE_H, NODE_W, planNodes, type FlowNode } from '@/operations/flow'
import { OPERATION_STATUS, OPERATION_TYPES, progressOf, useOperation } from '@/operations/operations'
import { Icon } from '@/ui/Icon'
import { Badge, clock, Empty, ErrorNote, Loading } from '@/ui/kit'
import { locale, t, tn } from '@/lib/i18n'

/*
 * An operation: where it stands, what Trezutil really did (the chronology by default, the Commerce Flow for detail),
 * and the controls the backend offers: prepare the plan, run the steps that are ready, pause, resume, stop, change
 * the instructions. Running steps goes through the Commerce Action Runtime, which checks and charges credits: it is
 * always asked for explicitly, after saying so.
 */

const ROW_STATUS: Record<string, string> = { done: 'done', running: 'active', ready: 'active', waiting: 'future', blocked: 'waiting', failed: 'waiting', skipped: 'future' }

export function OperationView() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const op = useOperation(id)
  const { conversations, pending } = useConversations()
  const conv = conversations.find((c) => c.operationId === id)
  const [tab, setTab] = useState<'timeline' | 'flow'>('timeline')
  const [editing, setEditing] = useState(false)
  const [confirmRun, setConfirmRun] = useState<'run' | 'resume' | null>(null)
  const [menu, setMenu] = useState(false)
  const menuBox = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!menu) return
    const close = (e: MouseEvent) => { if (!menuBox.current?.contains(e.target as Node)) setMenu(false) }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [menu])
  useEffect(() => { if (op.mission) document.title = `${op.mission.title || t('Opération')} — Trezutil` }, [op.mission])

  const nodes = useMemo(() => ({
    conversation: conversationNodes(conv, conv ? pending[conv.id] : undefined),
    plan: planNodes(op.plan?.steps ?? []),
  }), [conv, pending, op.plan])

  const back = () => (conv ? navigate(`/c/${conv.id}`) : navigate('/operations'))

  if (op.loading && !op.mission) return <div className="workspace"><Loading label={t('Chargement de l’opération…')} /></div>
  if (op.error || !op.mission) {
    return (
      <div className="workspace">
        <div className="workspace-head"><button type="button" className="back-btn" onClick={back}><span>‹</span> {t('Retour')}</button></div>
        <div className="workspace-pad">{op.error ? <ErrorNote error={op.error} onRetry={op.reload} /> : <Empty icon="flow" title={t('Opération introuvable')} />}</div>
      </div>
    )
  }

  const m = op.mission
  const status = OPERATION_STATUS[m.status] ?? { label: m.status, tone: 'neutral' as const }
  const progress = progressOf(op.plan)
  const terminal = m.status === 'completed' || m.status === 'cancelled'
  const readySteps = op.plan?.steps.filter((s) => s.canExecute) ?? []
  const hasFailed = op.plan?.steps.some((s) => s.state === 'failed' || s.stale) ?? false
  const created = Date.parse(m.createdAt)
  const timeline = [...nodes.conversation, ...nodes.plan]

  return (
    <div className="workspace operation">
      <div className="workspace-head">
        <button type="button" className="back-btn" onClick={back}><span>‹</span> {conv ? t('Retour à la conversation') : t('Toutes les opérations')}</button>
        <div className="operation-title">
          <div>
            <p className="eyebrow">{t('OPÉRATION ·')} {(OPERATION_TYPES[m.type] ?? m.type ?? '').toUpperCase()}</p>
            <h1>{m.title || m.objective}</h1>
            <p>{Number.isFinite(created) ? t('Créée le {date} à {time}', { date: new Date(created).toLocaleDateString(locale(), { day: 'numeric', month: 'long' }), time: clock(created) }) : t('Opération')}{m.location ? ` · ${m.location}` : ''}</p>
          </div>
          <div className="operation-controls">
            {m.status === 'active' && <button type="button" className="secondary-btn" disabled={!!op.busy} onClick={() => void op.setStatus('paused')}><Icon name="pause" size={15} /> {t('Mettre en pause')}</button>}
            {m.status === 'paused' && <button type="button" className="secondary-btn" disabled={!!op.busy} onClick={() => void op.setStatus('active')}><Icon name="play" size={15} /> {t('Reprendre')}</button>}
            {m.status === 'draft' && <button type="button" className="primary-btn" disabled={!!op.busy} onClick={() => void op.setStatus('active')}><Icon name="play" size={15} /> {t('Démarrer')}</button>}
            <div style={{ position: 'relative' }} ref={menuBox}>
              <button type="button" className="icon-btn" aria-label={t('Plus d’actions')} aria-haspopup="menu" aria-expanded={menu} onClick={() => setMenu((v) => !v)}><Icon name="more" /></button>
              {menu && (
                <div className="menu right" role="menu">
                  {!terminal && <button type="button" role="menuitem" onClick={() => { setMenu(false); setEditing(true) }}><Icon name="edit" size={15} />{t('Modifier les instructions')}</button>}
                  {conv && <button type="button" role="menuitem" onClick={() => navigate(`/c/${conv.id}`)}><Icon name="message" size={15} />{t('Ouvrir la conversation')}</button>}
                  {!terminal && <button type="button" role="menuitem" className="danger" onClick={() => {
                    setMenu(false)
                    if (window.confirm(t('Arrêter cette opération ? Les étapes non exécutées ne le seront pas. Cette action est définitive.'))) void op.setStatus('cancelled')
                  }}><Icon name="stop" size={15} />{t('Arrêter l’opération')}</button>}
                </div>
              )}
            </div>
          </div>
        </div>
        <div className="stats-row">
          <div><small>{t('STATUT')}</small><Badge tone={status.tone}>{status.label}</Badge></div>
          <div><small>{t('BUDGET')}</small><b>{m.budgetMinor > 0 ? formatMinor(m.budgetMinor, m.currency || 'USD') : '—'}</b></div>
          <div><small>{t('ÉTAPES')}</small><b>{progress.total ? `${progress.done} / ${progress.total}` : t('Aucun plan')}</b></div>
          <div><small>{t('CONVERSATION')}</small><b>{conv ? <Link to={`/c/${conv.id}`} className="green-text">{t('Liée')}</Link> : '—'}</b></div>
          <div className="progress-stat"><small>{t('PROGRESSION')}</small><b>{progress.percent} %</b><span><i style={{ width: `${progress.percent}%` }} /></span></div>
        </div>
        {op.actionError && <div className="head-error"><ErrorNote error={op.actionError} /></div>}
      </div>

      <div className="operation-body">
        <div className="tabbar" role="tablist">
          <button type="button" role="tab" aria-selected={tab === 'timeline'} className={tab === 'timeline' ? 'active' : ''} onClick={() => setTab('timeline')}>{t('Chronologie')}</button>
          <button type="button" role="tab" aria-selected={tab === 'flow'} className={tab === 'flow' ? 'active' : ''} onClick={() => setTab('flow')}>{t('Commerce Flow')} <Badge>{t('Avancé')}</Badge></button>
        </div>

        <PlanBar
          hasPlan={!!op.plan && op.plan.planVersion > 0} plannable={op.plan?.plannable ?? true} needsInfo={!!op.plan?.needsInformation}
          terminal={terminal} paused={m.status === 'paused'} busy={op.busy} ready={readySteps.length} failed={hasFailed}
          onPlan={() => void op.preparePlan()} onRun={(mode) => setConfirmRun(mode)}
        />

        {tab === 'timeline'
          ? <Timeline nodes={timeline} busy={op.busy} onRunStep={(s) => void op.runStep(s)} paused={m.status === 'paused' || terminal} />
          : <FlowView lanes={[{ label: t('Conversation'), nodes: nodes.conversation }, { label: t('Plan de l’opération'), nodes: nodes.plan }]} />}
      </div>

      {editing && <EditInstructions mission={{ title: m.title, objective: m.objective, location: m.location }} busy={op.busy === 'edit'} onClose={() => setEditing(false)}
        onSave={async (patch) => { if (await op.edit(patch)) { setEditing(false); if (conv && patch.title) conversationStore.rename(conv.id, patch.title) } }} />}

      {confirmRun && (
        <div className="modal-layer" onMouseDown={(e) => { if (e.target === e.currentTarget) setConfirmRun(null) }}>
          <div className="negotiation-panel" role="dialog" aria-modal="true" aria-labelledby="run-title">
            <div className="modal-head"><div><p className="eyebrow">{t('EXÉCUTION')}</p><h2 id="run-title">{confirmRun === 'run' ? t('Lancer les étapes prêtes') : t('Reprendre les étapes en échec')}</h2></div>
              <button type="button" className="icon-btn" onClick={() => setConfirmRun(null)} aria-label={t('Fermer')}><Icon name="x" /></button></div>
            <p className="panel-copy">{confirmRun === 'run' ? tn(readySteps.length, 'Trezutil va exécuter {n} étape prête par le Commerce Action Runtime : recherches et vérifications de disponibilité. Ces actions peuvent consommer des crédits.', 'Trezutil va exécuter {n} étapes prêtes par le Commerce Action Runtime : recherches et vérifications de disponibilité. Ces actions peuvent consommer des crédits.') : t('Trezutil va exécuter les étapes en échec ou interrompues par le Commerce Action Runtime : recherches et vérifications de disponibilité. Ces actions peuvent consommer des crédits.')}</p>
            <div className="approval-box"><Icon name="shield" /><div><b>{t('Rien n’est commandé ni payé')}</b><p>{t('Toute commande, réservation ou paiement reste soumis à ta confirmation explicite.')}</p></div></div>
            <div className="modal-actions">
              <button type="button" className="secondary-btn" onClick={() => setConfirmRun(null)}>{t('Annuler')}</button>
              <button type="button" className="primary-btn" onClick={() => { const mode = confirmRun; setConfirmRun(null); void op.run(mode) }}>{t('Lancer')}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function PlanBar(p: { hasPlan: boolean; plannable: boolean; needsInfo: boolean; terminal: boolean; paused: boolean; busy: string | null; ready: number; failed: boolean; onPlan: () => void; onRun: (mode: 'run' | 'resume') => void }) {
  if (p.terminal) return null
  return (
    <div className="plan-bar">
      {!p.hasPlan ? (
        <>
          <p><b>{t('Aucun plan pour l’instant.')}</b> {p.plannable ? t('Trezutil peut proposer les étapes de cette opération. Préparer le plan n’exécute rien.') : t('Le planificateur ne sait pas encore découper ce type d’opération : poursuis dans la conversation.')}</p>
          {p.plannable && <button type="button" className="secondary-btn" disabled={!!p.busy} aria-busy={p.busy === 'plan'} onClick={p.onPlan}>{p.busy === 'plan' ? t('Préparation…') : t('Préparer le plan')}</button>}
        </>
      ) : (
        <>
          <p>{p.needsInfo ? <><b>{t('Une information manque.')}</b> {t('Complète-la dans la conversation ou modifie les instructions.')}</> : p.paused ? <><b>{t('Opération en pause.')}</b> {t('Reprends-la pour exécuter des étapes.')}</> : p.ready ? <><b>{tn(p.ready, '{n} étape prête.', '{n} étapes prêtes.')}</b> {t('Rien ne s’exécute sans ta demande.')}</> : t('Aucune étape n’est prête à être exécutée.')}</p>
          <div className="plan-actions">
            {p.failed && <button type="button" className="secondary-btn" disabled={!!p.busy || p.paused} onClick={() => p.onRun('resume')}><Icon name="refresh" size={15} />{t('Reprendre les échecs')}</button>}
            {p.ready > 0 && <button type="button" className="primary-btn" disabled={!!p.busy || p.paused} aria-busy={p.busy === 'run'} onClick={() => p.onRun('run')}><Icon name="play" size={15} />{p.busy === 'run' ? t('Exécution…') : t('Lancer les étapes prêtes')}</button>}
          </div>
        </>
      )}
    </div>
  )
}

function Timeline({ nodes, busy, paused, onRunStep }: { nodes: FlowNode[]; busy: string | null; paused: boolean; onRunStep: (s: MissionStep) => void }) {
  const [all, setAll] = useState(false)
  const [open, setOpen] = useState<string | null>(null)
  const shown = all ? nodes : nodes.slice(-12)
  if (!nodes.length) return <Empty icon="clock" title={t('Rien ne s’est encore passé')}>{t('Les recherches, vérifications et décisions de cette opération apparaîtront ici, au fur et à mesure.')}</Empty>
  return (
    <div className="timeline">
      <div className="section-title">
        <div><h2>{t('Progression de l’opération')}</h2><p>{t('Chaque étape est documentée et reste sous ton contrôle.')}</p></div>
        {nodes.length > 12 && <button type="button" className="text-btn" onClick={() => setAll((v) => !v)}>{all ? t('Réduire') : t('Tout afficher ({length})', { length: nodes.length })}</button>}
      </div>
      {shown.map((n, i) => {
        const row = ROW_STATUS[n.status] ?? 'future'
        const step = n.step
        return (
          <div className={`timeline-row ${row}`} key={n.id}>
            <div className="timeline-rail"><span>{n.status === 'done' ? <Icon name="check" size={13} /> : n.status === 'failed' ? <Icon name="x" size={12} /> : i + 1}</span>{i < shown.length - 1 && <i />}</div>
            <div className="timeline-card-wrap">
              <button type="button" className="timeline-card" aria-expanded={open === n.id} onClick={() => setOpen((o) => (o === n.id ? null : n.id))}>
                <div><b>{n.title}</b><p>{n.summary || FLOW_KINDS[n.kind]}</p></div>
                {n.at ? <small>{clock(n.at)}</small> : null}
                {n.status === 'running' && <Badge tone="blue">{t('En cours')}</Badge>}
                {(n.status === 'waiting' && n.kind === 'decision') && <Badge tone="amber">{t('À décider')}</Badge>}
                {n.status === 'failed' && <Badge tone="red">{t('Échec')}</Badge>}
                {n.source === 'plan' && n.status !== 'done' && n.status !== 'running' && <Badge>{FLOW_STATUS[n.status]}</Badge>}
              </button>
              {open === n.id && (
                <div className="timeline-detail">
                  <dl>{n.facts.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl>
                  {step?.canExecute && !paused && (
                    <button type="button" className="primary-btn small-btn" disabled={!!busy} aria-busy={busy === step.id} onClick={() => onRunStep(step)}>
                      {busy === step.id ? t('Exécution…') : step.state === 'failed' || step.stale ? t('Relancer cette étape') : t('Exécuter cette étape')}
                    </button>
                  )}
                </div>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function FlowView({ lanes }: { lanes: { label: string; nodes: FlowNode[] }[] }) {
  const plan = useMemo(() => layout(lanes), [lanes])
  const [selected, setSelected] = useState<string | null>(null)
  const node = plan.placed.find((p) => p.node.id === selected)?.node
  if (!plan.placed.length) return <Empty icon="flow" title={t('Le flux est vide')}>{t('Il se remplit avec la conversation liée et le plan de l’opération.')}</Empty>
  return (
    <div className="flow-view">
      <div className="section-title">
        <div><h2>{t('Commerce Flow')}</h2><p>{t('Les actions et décisions réellement enregistrées. Sélectionne un nœud pour l’inspecter.')}</p></div>
        <div className="flow-legend"><span><i className="complete" />{t('Terminé')}</span><span><i className="current" />{t('En cours')}</span><span><i className="attention" />{t('À traiter')}</span><span><i />{t('À venir')}</span></div>
      </div>
      <div className="flow-canvas real">
        <div className="flow-scroll">
          <svg width={plan.width} height={plan.height} className="flow-edges" aria-hidden="true">
            {plan.lanes.map((l) => <text key={l.label} x={24} y={l.y + 12} className="flow-lane">{l.label.toUpperCase()}</text>)}
            {plan.edges.map(({ from, to }) => {
              const x1 = from.x + NODE_W, y1 = from.y + NODE_H / 2, x2 = to.x, y2 = to.y + NODE_H / 2, mid = (x1 + x2) / 2
              return <path key={`${from.node.id}-${to.node.id}`} d={`M${x1},${y1} C${mid},${y1} ${mid},${y2} ${x2},${y2}`} className={`flow-edge ${to.node.status}`} />
            })}
          </svg>
          {plan.placed.map(({ node: n, x, y }) => (
            <button type="button" key={n.id} className={`flow-box ${n.status} ${selected === n.id ? 'selected' : ''}`} style={{ left: x, top: y, width: NODE_W, height: NODE_H }}
              onClick={() => setSelected(n.id)} aria-pressed={selected === n.id}>
              <small>{FLOW_KINDS[n.kind]}</small>
              <b>{n.title}</b>
              <span>{FLOW_STATUS[n.status]}</span>
            </button>
          ))}
        </div>
        {node && (
          <div className="node-inspector">
            <div><Badge tone={node.status === 'done' ? 'green' : node.status === 'failed' ? 'red' : node.status === 'waiting' || node.status === 'blocked' ? 'amber' : 'blue'}>{FLOW_KINDS[node.kind].toUpperCase()}</Badge><button type="button" onClick={() => setSelected(null)} aria-label={t('Fermer l’inspecteur')}><Icon name="x" size={15} /></button></div>
            <h3>{node.title}</h3>
            <p>{node.summary}</p>
            <hr />
            <small>{t('ENREGISTRÉ PAR LE BACKEND')}</small>
            {node.facts.length ? <dl>{node.facts.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl> : <p>{t('Aucun détail supplémentaire.')}</p>}
          </div>
        )}
      </div>
    </div>
  )
}

function EditInstructions({ mission, busy, onClose, onSave }: { mission: { title: string; objective: string; location: string }; busy: boolean; onClose: () => void; onSave: (p: { title?: string; objective?: string; location?: string }) => void }) {
  const [title, setTitle] = useState(mission.title)
  const [objective, setObjective] = useState(mission.objective)
  const [location, setLocation] = useState(mission.location)
  const patch = () => {
    const p: { title?: string; objective?: string; location?: string } = {}
    if (title.trim() !== mission.title) p.title = title.trim()
    if (objective.trim() !== mission.objective) p.objective = objective.trim()
    if (location.trim() !== mission.location) p.location = location.trim()
    return p
  }
  return (
    <div className="modal-layer" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <form className="negotiation-panel" role="dialog" aria-modal="true" aria-labelledby="edit-title" onSubmit={(e) => { e.preventDefault(); const p = patch(); if (Object.keys(p).length) onSave(p); else onClose() }}>
        <div className="modal-head"><div><p className="eyebrow">{t('INSTRUCTIONS')}</p><h2 id="edit-title">{t('Modifier l’opération')}</h2></div><button type="button" className="icon-btn" onClick={onClose} aria-label={t('Fermer')}><Icon name="x" /></button></div>
        <div className="form-grid single">
          <label>{t('Titre')}<input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={120} /></label>
          <label>{t('Objectif et consignes')}<textarea value={objective} onChange={(e) => setObjective(e.target.value)} rows={4} maxLength={900} required /></label>
          <label>{t('Lieu')}<input value={location} onChange={(e) => setLocation(e.target.value)} maxLength={120} placeholder={t('Kinshasa, Gombe…')} /></label>
        </div>
        <p className="panel-copy">{t('Un objectif modifié peut changer le plan proposé. Rien n’est exécuté à l’enregistrement.')}</p>
        <div className="modal-actions">
          <button type="button" className="secondary-btn" onClick={onClose}>{t('Annuler')}</button>
          <button type="submit" className="primary-btn" disabled={busy || !objective.trim()} aria-busy={busy}>{busy ? 'Enregistrement…' : t('Enregistrer')}</button>
        </div>
      </form>
    </div>
  )
}
