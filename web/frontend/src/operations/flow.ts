import type { MissionStep } from '@/api/missions'
import type { Conversation } from '@/conversation/store'

/*
 * The Commerce Flow of an operation: what happened and what waits, as events. It is built only from what the backend
 * really did or planned:
 *   - the linked conversation: each request and each answer of the Commerce Action Runtime (search, comparison,
 *     availability check, confirmation asked, order created, failure…), in order;
 *   - the plan of the mission: its steps, their dependencies and their state.
 * Kinds the backend cannot produce yet (solicitation, proposals received, negotiation, counter-offer) belong to the
 * vocabulary so the views are ready for them, but no node of those kinds is ever made up here.
 */

export type FlowKind =
  | 'request' | 'search' | 'solicit' | 'proposals' | 'compare' | 'negotiate' | 'counter' | 'verify'
  | 'decision' | 'approval' | 'human' | 'commit' | 'answer' | 'failure'

export type FlowStatus = 'done' | 'running' | 'ready' | 'waiting' | 'blocked' | 'failed' | 'skipped'

export interface FlowNode {
  id: string
  kind: FlowKind
  title: string
  summary: string
  status: FlowStatus
  at?: number
  source: 'conversation' | 'plan'
  dependsOn: string[]
  /** What can be inspected: label → value, as recorded by the backend. */
  facts: [string, string][]
  step?: MissionStep
  turnId?: string
}

export const FLOW_KINDS: Record<FlowKind, string> = {
  request: 'Demande', search: 'Recherche de fournisseurs', solicit: 'Sollicitation', proposals: 'Propositions reçues',
  compare: 'Comparaison', negotiate: 'Négociation', counter: 'Contre-proposition', verify: 'Vérification',
  decision: 'Décision', approval: 'Approbation', human: 'Ton intervention', commit: 'Engagement', answer: 'Information', failure: 'Échec',
}

export const FLOW_STATUS: Record<FlowStatus, string> = {
  done: 'Terminé', running: 'En cours', ready: 'Prêt', waiting: 'En attente', blocked: 'Bloqué', failed: 'Échec', skipped: 'Ignoré',
}

export const STEP_TITLES: Record<string, string> = {
  FIND_VENUE: 'Trouver un lieu', FIND_CATERING: 'Trouver un traiteur', FIND_DECORATION: 'Trouver la décoration',
  FIND_ENTERTAINMENT: 'Trouver une animation', FIND_PHOTOGRAPHY: 'Trouver un photographe', FIND_TRANSPORT: 'Trouver un transport',
  FIND_ACCOMMODATION: 'Trouver un hébergement', FIND_LOCAL_TRANSPORT: 'Trouver un transport sur place', FIND_ACTIVITIES: 'Trouver des activités',
  FIND_MOVING_TRANSPORT: 'Trouver un transport de déménagement', FIND_MOVERS: 'Trouver des déménageurs', FIND_STORAGE: 'Trouver un stockage',
  FIND_TRANSPORT_OPTION: 'Trouver une option de transport', CHECK_AVAILABILITY: 'Vérifier la disponibilité',
}

const REASONS: Record<string, string> = {
  dependency_failed: 'Une étape dont elle dépend a échoué.',
  dependency_blocked: 'Une étape dont elle dépend est bloquée.',
  dependency_skipped: 'Une étape dont elle dépend a été ignorée.',
  needs_input: 'Une information de ta part est nécessaire.',
  needs_confirmation: 'Ta confirmation est nécessaire.',
  waiting: 'Attend les étapes précédentes.',
  not_executable: 'Aucune action du runtime ne sait encore faire cette étape.',
  abandoned: 'La tentative ne répond plus ; elle peut être relancée.',
}

export const reasonText = (reason: string) => REASONS[reason] ?? reason

const ACTION_KIND: Record<string, FlowKind> = {
  SEARCH_LISTINGS: 'search', COMPARE_LISTINGS: 'compare', CHECK_AVAILABILITY: 'verify', CHECK_INVENTORY: 'verify',
  CREATE_ORDER: 'commit', CREATE_RESERVATION: 'commit', CANCEL_ORDER: 'commit', CANCEL_RESERVATION: 'commit',
}

function clip(text: string, n = 140): string {
  const one = text.replace(/\s+/g, ' ').trim()
  return one.length > n ? `${one.slice(0, n - 1).trimEnd()}…` : one
}

const countOf = (n: number, one: string, many: string) => `${n} ${n > 1 ? many : one}`

/** The events of a conversation, as a chain. */
export function conversationNodes(conv: Conversation | undefined, pendingLabel?: string): FlowNode[] {
  if (!conv) return []
  const nodes: FlowNode[] = []
  let prev = ''
  const push = (n: Omit<FlowNode, 'dependsOn' | 'source'>) => {
    nodes.push({ ...n, source: 'conversation', dependsOn: prev ? [prev] : [] })
    prev = n.id
  }
  const turns = conv.turns
  turns.forEach((turn, index) => {
    const later = turns.slice(index + 1)
    if (turn.role === 'user') {
      push({ id: `c-${turn.id}`, kind: 'request', title: index === 0 ? 'Besoin exprimé' : 'Précision', summary: clip(turn.text), status: 'done', at: turn.at, turnId: turn.id, facts: [['Envoyé', new Date(turn.at).toLocaleString('fr-FR')]] })
      if (turn.failed) {
        push({ id: `f-${turn.id}`, kind: 'failure', title: 'Message non remis', summary: turn.failed.message, status: 'failed', at: turn.at, turnId: turn.id, facts: [['Cause', turn.failed.network ? 'Réseau' : 'Serveur'], ['Nouvel essai', 'Même clé d’idempotence : exécuté au plus une fois']] })
      }
      return
    }
    const r = turn.reply
    if (!r) {
      push({ id: `c-${turn.id}`, kind: 'answer', title: 'Message', summary: clip(turn.text), status: 'done', at: turn.at, turnId: turn.id, facts: [] })
      return
    }
    const facts: [string, string][] = []
    if (r.action_type) facts.push(['Action du runtime', r.action_type])
    if (r.source) facts.push(['Interprété par', r.source])
    if (r.result_items.length) facts.push(['Offres renvoyées', String(r.result_items.length)])
    if (r.refs.order_id) facts.push(['Commande', r.refs.order_id])
    if (r.refs.reservation_id) facts.push(['Réservation', r.refs.reservation_id])

    if (r.is_confirmation && r.confirmation) {
      const answered = turn.resolution
      push({
        id: `c-${turn.id}`, kind: 'decision', title: 'Confirmation demandée', summary: clip(r.confirmation.summary || r.text), at: turn.at, turnId: turn.id,
        status: answered === 'expired' ? 'failed' : answered ? 'done' : 'waiting', facts: [...facts, ['Action à confirmer', r.confirmation.action_type || r.action_type || '—']],
      })
      if (answered === 'confirmed' || answered === 'rejected') {
        push({ id: `a-${turn.id}`, kind: 'approval', title: answered === 'confirmed' ? 'Approuvé par toi' : 'Refusé par toi', summary: answered === 'confirmed' ? 'Le runtime peut exécuter l’action.' : 'Rien n’a été modifié.', status: 'done', at: later[0]?.at, turnId: turn.id, facts: [] })
      }
      return
    }
    if (r.is_clarification) {
      const answered = later.some((x) => x.role === 'user')
      push({ id: `c-${turn.id}`, kind: 'human', title: 'Précision demandée', summary: clip(r.missing_params.map((p) => p.prompt || p.name).join(' · ') || r.text), status: answered ? 'done' : 'waiting', at: turn.at, turnId: turn.id, facts })
      return
    }
    const kind = ACTION_KIND[r.action_type] ?? 'answer'
    const found = r.result_items.length
    const title = kind === 'search' ? 'Recherche de fournisseurs' : kind === 'compare' ? 'Comparaison des offres' : kind === 'verify' ? 'Vérification'
      : kind === 'commit' ? (r.refs.order_id ? 'Commande créée' : r.refs.reservation_id ? 'Réservation créée' : 'Action exécutée') : 'Information'
    push({ id: `c-${turn.id}`, kind, title, at: turn.at, turnId: turn.id, facts, status: 'done', summary: kind === 'search' ? (found ? countOf(found, 'offre trouvée', 'offres trouvées') : clip(r.text)) : clip(r.text) })
  })
  if (pendingLabel) push({ id: 'c-pending', kind: 'answer', title: 'En cours', summary: pendingLabel, status: 'running', facts: [] })
  return nodes
}

const STATE_STATUS: Record<string, FlowStatus> = {
  completed: 'done', running: 'running', ready: 'ready', pending: 'waiting', blocked: 'blocked', failed: 'failed', skipped: 'skipped',
}

/** The steps of a plan, with their dependencies. */
export function planNodes(steps: MissionStep[]): FlowNode[] {
  const ids = new Set(steps.map((s) => s.id))
  return [...steps].sort((a, b) => a.position - b.position).map((s) => {
    const x = s.execution
    const waitsForYou = s.state === 'blocked' && (s.reason === 'needs_input' || s.reason === 'needs_confirmation')
    const kind: FlowKind = s.state === 'failed' ? 'failure' : waitsForYou ? 'human' : s.actionType === 'CHECK_AVAILABILITY' ? 'verify' : s.actionType === 'SEARCH_LISTINGS' ? 'search' : 'human'
    const facts: [string, string][] = [['Étape', s.type]]
    if (s.actionType) facts.push(['Action du runtime', s.actionType])
    if (s.actionSupport) facts.push(['Prise en charge', s.actionSupport === 'supported' ? 'Oui' : 'Pas encore'])
    if (s.state) facts.push(['État', s.state])
    if (s.reason) facts.push(['Raison', reasonText(s.reason)])
    if (x) {
      facts.push(['Tentative', String(x.attempt)])
      if (x.status) facts.push(['Dernière tentative', x.status])
      if (x.total) facts.push(['Résultats', String(x.total)])
      if (x.available !== null) facts.push(['Disponible', x.available ? 'Oui' : 'Non'])
      if (x.errorCode) facts.push(['Code d’erreur', x.errorCode])
    }
    const summary = s.state === 'completed' && x
      ? (x.available === true ? 'Disponible' : x.available === false ? 'Indisponible' : countOf(x.total, 'résultat trouvé', 'résultats trouvés'))
      : s.reason ? reasonText(s.reason) : s.description || ''
    return {
      id: s.id, kind, source: 'plan' as const, step: s, title: STEP_TITLES[s.type] ?? s.title,
      summary, status: STATE_STATUS[s.state] ?? 'waiting', dependsOn: s.dependsOn.filter((d) => ids.has(d)), facts,
    }
  })
}

export interface Placed { node: FlowNode; x: number; y: number }
export interface Layout { placed: Placed[]; edges: { from: Placed; to: Placed }[]; width: number; height: number; lanes: { label: string; y: number; height: number }[] }

export const NODE_W = 200
export const NODE_H = 76
const GAP_X = 48
const GAP_Y = 16
const LANE_GAP = 44
const PAD = 24

/** Places each lane's nodes by depth (longest chain of dependencies); lanes one under the other. */
export function layout(lanes: { label: string; nodes: FlowNode[] }[]): Layout {
  const placed: Placed[] = []
  const lanesOut: Layout['lanes'] = []
  let top = PAD
  let width = 0
  for (const lane of lanes) {
    if (!lane.nodes.length) continue
    const byId = new Map(lane.nodes.map((n) => [n.id, n]))
    const depth = new Map<string, number>()
    const depthOf = (n: FlowNode, seen: Set<string>): number => {
      if (depth.has(n.id)) return depth.get(n.id)!
      if (seen.has(n.id)) return 0
      seen.add(n.id)
      const d = n.dependsOn.reduce((m, id) => { const dep = byId.get(id); return dep ? Math.max(m, depthOf(dep, seen) + 1) : m }, 0)
      depth.set(n.id, d)
      return d
    }
    lane.nodes.forEach((n) => depthOf(n, new Set()))
    const rows = new Map<number, number>()
    let laneRows = 1
    for (const n of lane.nodes) {
      const d = depth.get(n.id) ?? 0
      const row = rows.get(d) ?? 0
      rows.set(d, row + 1)
      laneRows = Math.max(laneRows, row + 1)
      const x = PAD + d * (NODE_W + GAP_X)
      placed.push({ node: n, x, y: top + 24 + row * (NODE_H + GAP_Y) })
      width = Math.max(width, x + NODE_W + PAD)
    }
    const height = 24 + laneRows * (NODE_H + GAP_Y) - GAP_Y
    lanesOut.push({ label: lane.label, y: top, height })
    top += height + LANE_GAP
  }
  const index = new Map(placed.map((p) => [p.node.id, p]))
  const edges = placed.flatMap((to) => to.node.dependsOn.map((id) => index.get(id)).filter((f): f is Placed => !!f).map((from) => ({ from, to })))
  return { placed, edges, width: Math.max(width, 320), height: Math.max(top - LANE_GAP + PAD, 160), lanes: lanesOut }
}
