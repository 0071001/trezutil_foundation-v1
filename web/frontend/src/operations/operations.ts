import { useCallback, useEffect, useRef, useState } from 'react'
import { toApiError, type ApiError } from '@/api/client'
import { missionsApi, type Mission, type MissionPlan, type MissionStatus, type MissionStep } from '@/api/missions'
import { conversationStore, type Conversation } from '@/conversation/store'
import { memoryAsPreferences } from '@/memory/memory'
import { newId } from '@/ui/kit'

/*
 * An operation is a mission of the backend (/api/v1/missions): a goal handed to Trezutil, with a plan of steps the
 * Commerce Action Runtime runs. Opening an operation records the goal; it searches, books, orders and spends nothing.
 * Running steps is always an explicit request, and the runtime alone checks and charges credits.
 */

export const OPERATION_TYPES: Record<string, string> = {
  EVENT: 'Événement', TRAVEL: 'Voyage', STAY: 'Séjour', MOVE: 'Déménagement', TRANSPORT: 'Transport',
  BUSINESS_TRIP: 'Voyage d’affaires', PROJECT: 'Projet', OTHER: 'Achat ou autre opération',
}

export const OPERATION_STATUS: Record<string, { label: string; tone: 'green' | 'amber' | 'neutral' | 'blue' | 'red' }> = {
  draft: { label: 'En préparation', tone: 'neutral' },
  active: { label: 'En cours', tone: 'blue' },
  paused: { label: 'En pause', tone: 'amber' },
  completed: { label: 'Terminée', tone: 'green' },
  cancelled: { label: 'Arrêtée', tone: 'red' },
}

/** The goal of a conversation in the person's words: what they asked first. */
export function objectiveOf(conv: Conversation): string {
  const first = conv.turns.find((x) => x.role === 'user')?.text ?? conv.title
  return first.trim().slice(0, 900)
}

/**
 * Opens the operation of a conversation. The key comes from the conversation, so a double click, a retry or a second
 * tab return the same mission. The commercial memory, when on, goes with it as the mission's preferences.
 */
export async function openOperationFor(conv: Conversation): Promise<Mission> {
  if (conv.operationId) return missionsApi.get(conv.operationId)
  const preferences = memoryAsPreferences()
  const mission = await missionsApi.create({ objective: objectiveOf(conv), title: conv.title, ...(Object.keys(preferences).length ? { preferences } : {}) }, `op-${conv.id}`)
  conversationStore.linkOperation(conv.id, mission.id)
  return mission
}

export interface OperationState {
  mission: Mission | null
  plan: MissionPlan | null
  loading: boolean
  error: ApiError | null
  /** What is under way: 'plan', 'run', 'resume', 'status', 'edit', or a step id. */
  busy: string | null
  actionError: ApiError | null
  reload: () => void
  preparePlan: () => Promise<void>
  run: (mode: 'run' | 'resume') => Promise<void>
  runStep: (step: MissionStep) => Promise<void>
  setStatus: (status: MissionStatus) => Promise<void>
  edit: (patch: { title?: string; objective?: string; location?: string }) => Promise<boolean>
}

const POLL_MS = 2500

export function useOperation(id: string | undefined): OperationState {
  const [mission, setMission] = useState<Mission | null>(null)
  const [plan, setPlan] = useState<MissionPlan | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [actionError, setActionError] = useState<ApiError | null>(null)
  const [tick, setTick] = useState(0)
  const planKey = useRef<string | null>(null)
  const stepKeys = useRef<Record<string, string>>({})

  useEffect(() => {
    if (!id) return
    let alive = true
    setLoading(true)
    setError(null)
    Promise.all([missionsApi.get(id), missionsApi.plan(id).catch(() => null)])
      .then(([m, p]) => { if (alive) { setMission(m); setPlan(p) } })
      .catch((err) => { if (alive) setError(toApiError(err)) })
      .finally(() => { if (alive) setLoading(false) })
    return () => { alive = false }
  }, [id, tick])

  // While a step runs (here or in another tab) the state is read again, so the flow moves by itself.
  const live = !!plan?.steps.some((s) => s.state === 'running')
  useEffect(() => {
    if (!id || !live) return
    const ctl = new AbortController()
    const timer = setInterval(() => {
      missionsApi.execution(id, ctl.signal)
        .then((st) => setPlan((p) => (p ? { ...p, steps: st.steps.length ? st.steps : p.steps } : p)))
        .catch(() => { /* asked again on the next tick */ })
    }, POLL_MS)
    return () => { ctl.abort(); clearInterval(timer) }
  }, [id, live])

  const reload = useCallback(() => setTick((n) => n + 1), [])

  const guard = async (label: string, work: () => Promise<void>): Promise<boolean> => {
    if (!id || busy) return false
    setBusy(label)
    setActionError(null)
    try { await work(); return true } catch (err) { setActionError(toApiError(err)); return false } finally { setBusy(null) }
  }

  return {
    mission, plan, loading, error, busy, actionError, reload,
    preparePlan: async () => {
      await guard('plan', async () => {
        planKey.current ??= newId()
        const next = await missionsApi.preparePlan(id!, planKey.current)
        planKey.current = null
        setPlan(next)
      })
    },
    run: async (mode) => {
      await guard(mode, async () => {
        const st = await (mode === 'run' ? missionsApi.run(id!) : missionsApi.resume(id!))
        setPlan((p) => (p ? { ...p, steps: st.steps.length ? st.steps : p.steps } : p))
        setMission((m) => (m && st.missionStatus ? { ...m, status: st.missionStatus } : m))
      })
    },
    runStep: async (step) => {
      await guard(step.id, async () => {
        stepKeys.current[step.id] ??= newId()
        try {
          const done = await missionsApi.executeStep(id!, step.id, stepKeys.current[step.id]!)
          delete stepKeys.current[step.id]
          setPlan((p) => (p ? { ...p, steps: p.steps.map((s) => (s.id === done.step.id ? done.step : s)) } : p))
        } catch (err) {
          const e = toApiError(err)
          if (e.status !== 0 && e.status < 500) delete stepKeys.current[step.id] // answered: the next attempt is new
          throw e
        }
      })
    },
    setStatus: async (status) => {
      await guard('status', async () => { setMission(await missionsApi.update(id!, { status })) })
    },
    edit: (patch) => guard('edit', async () => {
      setMission(await missionsApi.update(id!, patch))
      setPlan(await missionsApi.plan(id!).catch(() => null))
    }),
  }
}

/** Progress of a plan, from the state of its steps. */
export function progressOf(plan: MissionPlan | null): { done: number; total: number; percent: number } {
  const steps = plan?.steps.filter((s) => s.planVersion > 0 || s.state !== '') ?? []
  const done = steps.filter((s) => s.state === 'completed' || s.state === 'skipped').length
  return { done, total: steps.length, percent: steps.length ? Math.round((done / steps.length) * 100) : 0 }
}
