import { http, query } from '@/api/client'
import { arr, num, obj, str } from '@/api/json'

/*
 * Organiser's missions (/api/v1/missions): a goal a person delegates to Trezutil. Only the person's own missions
 * are ever returned. Recording a mission starts nothing: no search, no booking, no order, no credit.
 */

export type MissionStatus = 'draft' | 'active' | 'paused' | 'completed' | 'cancelled'
export type MissionType = 'EVENT' | 'TRAVEL' | 'STAY' | 'MOVE' | 'TRANSPORT' | 'BUSINESS_TRIP' | 'PROJECT' | 'OTHER'

export interface Mission {
  id: string
  type: string
  title: string
  objective: string
  status: string
  location: string
  /** YYYY-MM-DD, or '' when not set. */
  startDate: string
  endDate: string
  participants: number
  budgetMinor: number
  currency: string
  preferences: Record<string, string>
  createdAt: string
  updatedAt: string
}

export interface MissionStep {
  id: string
  missionId: string
  type: string
  title: string
  description: string
  status: string
  position: number
  actionType: string
  /** 'supported' when the Commerce OS can run the step's action today, 'not_yet_supported' when it cannot, '' for a step added by hand. */
  actionSupport: string
  /** The plan the step belongs to; 0 for a step added by hand. */
  planVersion: number
  /** Whether the step has an executable action behind it that the Commerce OS supports. */
  executable: boolean
  /** Whether it can be executed now: executable, the mission allows it, and the step is pending, failed or stuck. */
  canExecute: boolean
  /** In progress, but the attempt has gone silent: it can be executed again. */
  stale: boolean
  /** The step's latest attempt to run, null when it has none. */
  execution: MissionExecution | null
  /**
   * Where the step stands in the plan's graph: 'pending' (waiting), 'ready', 'running', 'completed', 'failed',
   * 'blocked' or 'skipped'. Empty for a step that is not part of the current plan (added by hand).
   */
  state: string
  /** The steps this one waits for (their ids). */
  dependsOn: string[]
  /** The dependencies that keep a blocked step blocked. */
  blockedBy: string[]
  /** Why a step is blocked or waits: 'dependency_failed', 'dependency_blocked', 'dependency_skipped', 'needs_input', 'needs_confirmation', 'waiting', 'not_executable', 'abandoned'. */
  reason: string
  /** What is missing when the reason is 'needs_input': 'dates' or 'listing'. */
  detail: string
}

/**
 * One attempt to run a step through the Commerce Action Runtime: a stable status and, when it worked, a small summary
 * with references (never a copy of commerce data), or a stable error code when it did not.
 */
export interface MissionExecution {
  id: string
  attempt: number
  status: string
  actionType: string
  /** What the attempt produced; for a search, this execution. */
  reference: string
  /** How many listings a search found; 0 when there is no result. */
  total: number
  /** The first listings found (references to existing listings). */
  listingIds: string[]
  errorCode: string
  /** For an availability check: whether the listing is free then; null for any other attempt. */
  available: boolean | null
}

/** The answer to running a step. */
export interface StepRun {
  execution: MissionExecution
  /** The step as it now is, with its state. */
  step: MissionStep
  /** True when the request repeated an idempotency key and nothing was run. */
  replayed: boolean
}

/**
 * A mission's plan: the steps that will need doing, as intentions. They are pending and carry no result: nothing is
 * searched, reserved or ordered by planning.
 */
export interface MissionPlan {
  missionId: string
  /** The mission's kind now. */
  missionType: string
  /** 0 when there is no plan. */
  planVersion: number
  /** 'current', 'superseded', 'none', or 'needs_information' when the mission cannot be planned as it is. */
  status: string
  /** Whether a plan exists for the mission's kind. */
  plannable: boolean
  needsInformation: boolean
  /** The kind the plan was prepared for; differs from missionType when the mission was changed since. */
  planMissionType: string
  steps: MissionStep[]
  versions: number[]
}

/** How far a mission is, derived from its steps (the mission's own status is not changed by it). */
export type MissionOutcome = 'empty' | 'completed' | 'in_progress' | 'needs_attention' | 'ready' | 'waiting'

/** The execution state of a mission: what running it (or reading it) answers. */
export interface MissionRunState {
  missionId: string
  planVersion: number
  /** The mission's own status after the request. */
  missionStatus: string
  outcome: string
  /** True when another run holds the mission: this request ran nothing. */
  busy: boolean
  progress: { completed: number; skipped: number; total: number }
  /** The steps this request ran, in order. */
  ran: { stepId: string; stepType: string; status: string; errorCode: string }[]
  /** Every step of the current plan with its state. */
  steps: MissionStep[]
}

export interface MissionUpdate {
  type?: string
  /** YYYY-MM-DD: what a check of availability needs. */
  start_date?: string
  end_date?: string
}

export interface MissionCreate {
  objective: string
  type?: string
  title?: string
  location?: string
  participants?: number
}

export function mapMission(value: unknown): Mission {
  const raw = obj(value)
  const prefs: Record<string, string> = {}
  for (const [k, v] of Object.entries(obj(raw.preferences))) if (typeof v === 'string') prefs[k] = v
  return {
    id: str(raw.id), type: str(raw.type), title: str(raw.title), objective: str(raw.objective), status: str(raw.status),
    location: str(raw.location), startDate: str(raw.start_date), endDate: str(raw.end_date), participants: num(raw.participants),
    budgetMinor: num(raw.budget_minor), currency: str(raw.currency), preferences: prefs, createdAt: str(raw.created_at), updatedAt: str(raw.updated_at),
  }
}

export function mapExecution(value: unknown): MissionExecution | null {
  const raw = obj(value)
  if (!str(raw.id)) return null
  const result = obj(raw.result)
  return {
    id: str(raw.id), attempt: num(raw.attempt), status: str(raw.status), actionType: str(raw.action_type), reference: str(result.reference),
    total: num(result.total), listingIds: arr(result.listing_ids).filter((v): v is string => typeof v === 'string'), errorCode: str(obj(raw.error).code),
    available: typeof result.available === 'boolean' ? result.available : null,
  }
}

export function mapStep(value: unknown): MissionStep {
  const raw = obj(value)
  return {
    id: str(raw.id), missionId: str(raw.mission_id), type: str(raw.type), title: str(raw.title), description: str(raw.description),
    status: str(raw.status), position: num(raw.position), actionType: str(raw.action_type),
    actionSupport: str(raw.action_support), planVersion: num(raw.plan_version),
    executable: raw.executable === true, canExecute: raw.can_execute === true, stale: raw.stale === true, execution: mapExecution(raw.execution),
    state: str(raw.state), dependsOn: strings(raw.depends_on), blockedBy: strings(raw.blocked_by), reason: str(raw.reason), detail: str(raw.detail),
  }
}

const strings = (v: unknown): string[] => arr(v).filter((x): x is string => typeof x === 'string')

export function mapRunState(value: unknown): MissionRunState {
  const raw = obj(value)
  const progress = obj(raw.progress)
  return {
    missionId: str(raw.mission_id), planVersion: num(raw.plan_version), missionStatus: str(raw.mission_status), outcome: str(raw.outcome),
    busy: raw.busy === true, progress: { completed: num(progress.completed), skipped: num(progress.skipped), total: num(progress.total) },
    ran: arr(raw.ran).map((r) => {
      const x = obj(r)
      return { stepId: str(x.step_id), stepType: str(x.step_type), status: str(x.status), errorCode: str(x.error_code) }
    }),
    steps: arr(raw.steps).map(mapStep),
  }
}

export function mapPlan(value: unknown): MissionPlan {
  const raw = obj(value)
  return {
    missionId: str(raw.mission_id), missionType: str(raw.mission_type), planVersion: num(raw.plan_version), status: str(raw.status) || 'none',
    plannable: raw.plannable === true, needsInformation: raw.needs_information === true, planMissionType: str(raw.plan_mission_type),
    steps: arr(raw.steps).map(mapStep), versions: arr(raw.versions).map(num),
  }
}

function emptyExecution(): MissionExecution {
  return { id: '', attempt: 0, status: '', actionType: '', reference: '', total: 0, listingIds: [], errorCode: '', available: null }
}

export const missionsApi = {
  /** Records a draft mission. The idempotency key makes a retried send return the first mission, not a second one. */
  async create(input: MissionCreate, idempotencyKey: string): Promise<Mission> {
    return mapMission(await http.post<unknown>('/missions', input, { idempotencyKey }))
  },
  async list(status?: MissionStatus): Promise<{ items: Mission[]; total: number }> {
    const raw = obj(await http.get<unknown>(`/missions${query({ status, page_size: 20 })}`))
    return { items: arr(raw.items).map(mapMission), total: num(raw.total) }
  },
  async get(id: string): Promise<Mission> {
    return mapMission(await http.get<unknown>(`/missions/${encodeURIComponent(id)}`))
  },
  /** Changes the kind of a mission (the only change the page makes). */
  async update(id: string, patch: MissionUpdate): Promise<Mission> {
    return mapMission(await http.patch<unknown>(`/missions/${encodeURIComponent(id)}`, patch))
  },
  /** The current plan; a mission without one answers status 'none'. */
  async plan(id: string): Promise<MissionPlan> {
    return mapPlan(await http.get<unknown>(`/missions/${encodeURIComponent(id)}/plan`))
  },
  /**
   * Prepares the plan. The same request twice gives the same plan; replacing a different existing plan has to be
   * asked for with `replace`. The key makes a retry return what the first request stored.
   */
  async preparePlan(id: string, idempotencyKey: string, replace = false): Promise<MissionPlan> {
    return mapPlan(await http.post<unknown>(`/missions/${encodeURIComponent(id)}/plan`, replace ? { replace: true } : {}, { idempotencyKey }))
  },
  /**
   * Runs ONE step through the Commerce Action Runtime. The request says nothing about what to run: the server decides
   * that from the mission and the step. The same step with the same key is the same execution (nothing runs twice),
   * so a retry after a network failure reuses its key; a new attempt after a failed run uses a new one.
   */
  async executeStep(id: string, stepId: string, idempotencyKey: string): Promise<StepRun> {
    const raw = obj(await http.post<unknown>(`/missions/${encodeURIComponent(id)}/steps/${encodeURIComponent(stepId)}/execute`, {}, { idempotencyKey }))
    return { execution: mapExecution(raw) ?? emptyExecution(), step: mapStep(raw.step), replayed: raw.replayed === true }
  },
  /**
   * Runs the whole mission, on request: the steps that are ready, in dependency order, each through the Commerce Action
   * Runtime (which alone checks and charges credits). Nothing is booked or ordered. Repeating it is safe: a completed
   * step is never run again. When another run holds the mission the answer says `busy` and nothing was run.
   */
  async run(id: string, onFailure: 'stop' | 'continue' = 'stop'): Promise<MissionRunState> {
    return mapRunState(await http.post<unknown>(`/missions/${encodeURIComponent(id)}/run`, onFailure === 'stop' ? {} : { on_failure: onFailure }))
  },
  /** Like run, and also runs once the steps that failed or went silent. A step that needs the person's confirmation is never retried. */
  async resume(id: string): Promise<MissionRunState> {
    return mapRunState(await http.post<unknown>(`/missions/${encodeURIComponent(id)}/resume`, {}))
  },
  /** The execution state of the mission's plan; changes nothing. */
  async execution(id: string, signal?: AbortSignal): Promise<MissionRunState> {
    return mapRunState(await http.get<unknown>(`/missions/${encodeURIComponent(id)}/execution`, { signal }))
  },
  async steps(id: string): Promise<MissionStep[]> {
    return arr(obj(await http.get<unknown>(`/missions/${encodeURIComponent(id)}/steps`)).items).map(mapStep)
  },
}
