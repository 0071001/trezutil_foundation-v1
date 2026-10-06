import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { missionsApi, type Mission, type MissionOutcome, type MissionPlan, type MissionRunState, type MissionStep } from '@/api/missions'
import { AppShell } from '@/components/shell'
import { ErrorState, Loading } from '@/components/states'
import { ActionLink, Icon } from '@/components/ui'
import { formatMinor } from '@/lib/format'
import { getLang, t, tn, tx } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { ORGANISE_STARTS } from '@/lib/organise'
import { useVoiceInput } from '@/lib/voice'

/*
 * Organiser: hand a goal to Trezutil. The page is one question and a field to answer it in your own words; what
 * you write becomes a draft mission, shown back as what Trezutil understood. Preparing its plan is a separate,
 * explicit step that lists what needs doing. Nothing is searched, booked or ordered here and no credit is used: the
 * mission and its plan only record the goal and the intentions (docs/architecture/ORGANISER_MISSIONS_PHASE1.md and
 * ORGANISER_MISSIONS_PHASE2.md). The suggestions are shortcuts to a sentence, not categories.
 *
 * Once the plan exists, its parts can be run one at a time or all together ("Run everything"): the parts that are
 * ready, in the order of what depends on what, each through the Commerce OS. The page shows where every part stands
 * (done, in progress, waiting, blocked, failed), updates by itself while a run is under way, and never confirms,
 * books or orders anything for the person (docs/architecture/ORGANISER_MISSIONS_PHASE4.md).
 */

const DRAFT_KEY = 'trezutil_organise_draft'

/** Starts of a sentence, completed by the person. */
const IDEAS: { id: string; label: string; start: string }[] = [
  { id: 'trip', label: tx('A trip'), start: ORGANISE_STARTS.trip },
  { id: 'party', label: tx('A party'), start: ORGANISE_STARTS.party },
  { id: 'stay', label: tx('A stay'), start: ORGANISE_STARTS.stay },
  { id: 'move', label: tx('A move'), start: ORGANISE_STARTS.move },
  { id: 'transport', label: tx('Transport'), start: ORGANISE_STARTS.transport },
  { id: 'event', label: tx('An event'), start: ORGANISE_STARTS.event },
]

const TYPE_LABELS: Record<string, string> = {
  EVENT: tx('Event'), TRAVEL: tx('Trip'), STAY: tx('Stay'), MOVE: tx('Move'), TRANSPORT: tx('Transport'),
  BUSINESS_TRIP: tx('Business trip'), PROJECT: tx('Project'), OTHER: tx('Something else'),
}

const STATUS_LABELS: Record<string, string> = {
  draft: tx('Organisation in preparation'), active: tx('Under way'), paused: tx('On hold'), completed: tx('Done'), cancelled: tx('Cancelled'),
}

const STEP_LABELS: Record<string, string> = {
  pending: tx('To do'), in_progress: tx('In progress'), completed: tx('Done'), skipped: tx('Skipped'), failed: tx('Did not work out'),
}

const typeLabel = (type: string) => t(TYPE_LABELS[type] ?? TYPE_LABELS.OTHER!)

function newKey(): string {
  const c = globalThis.crypto
  return c?.randomUUID ? c.randomUUID() : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

function readDraft(): string {
  try { return sessionStorage.getItem(DRAFT_KEY) ?? '' } catch { return '' }
}
function writeDraft(text: string) {
  try { if (text) sessionStorage.setItem(DRAFT_KEY, text); else sessionStorage.removeItem(DRAFT_KEY) } catch { /* private mode */ }
}

/** A YYYY-MM-DD date, written in the person's language and never shifted by the time zone. */
function day(iso: string): string {
  const d = new Date(`${iso}T00:00:00Z`)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString(getLang() === 'fr' ? 'fr-FR' : 'en-GB', { timeZone: 'UTC', day: 'numeric', month: 'long', year: 'numeric' })
}

export default function OrganizePage() {
  const navigate = useNavigate()
  const { signedIn } = useSession()
  const [params, setParams] = useSearchParams()
  const [text, setText] = useState(() => params.get('q') ?? readDraft())
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const field = useRef<HTMLTextAreaElement>(null)
  const attempt = useRef<{ text: string; key: string } | null>(null)
  const fieldId = useId()
  const voice = useVoiceInput(setText)
  const recent = useAsync(() => missionsApi.list(), [signedIn], signedIn)

  useEffect(() => { document.title = t('Organise — Trezutil') }, [])
  // A sentence arriving as ?q= (from the home page) is kept in the field, not in the address.
  useEffect(() => {
    if (!params.has('q')) return
    setParams({}, { replace: true })
    // A sentence begun elsewhere (the menu): the cursor waits at its end, for the person to finish it.
    requestAnimationFrame(() => {
      const el = field.current
      if (!el) return
      el.focus()
      el.setSelectionRange(el.value.length, el.value.length)
    })
  }, [params, setParams])

  const start = (sentence: string) => {
    setText(t(sentence))
    setError(null)
    requestAnimationFrame(() => {
      const el = field.current
      if (!el) return
      el.focus()
      el.setSelectionRange(el.value.length, el.value.length)
    })
  }

  const submit = async () => {
    const objective = text.trim()
    if (!objective || busy) return
    if (!signedIn) {
      // A mission belongs to an account: keep the sentence while the person signs in, then bring them back to it.
      writeDraft(objective)
      navigate(`/login?returnTo=${encodeURIComponent('/organize')}`)
      return
    }
    // A retry of the same sentence reuses its key, so a double send or a flaky network never makes two missions.
    if (attempt.current?.text !== objective) attempt.current = { text: objective, key: newKey() }
    setBusy(true)
    setError(null)
    try {
      const mission = await missionsApi.create({ objective }, attempt.current.key)
      writeDraft('')
      attempt.current = null
      navigate(`/organize/${encodeURIComponent(mission.id)}`)
    } catch (err) {
      setError(toApiError(err))
      setBusy(false)
    }
  }

  const onSubmit = (e: FormEvent) => { e.preventDefault(); void submit() }
  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) { e.preventDefault(); void submit() }
  }

  return (
    <AppShell>
      <section className="organise" aria-labelledby="organise-title">
        <p className="organise-eyebrow">{t('Organiser')}</p>
        <h1 id="organise-title">{t('What do you want to organise?')}</h1>
        <p className="organise-lead">{t('A trip, an event, a move, a stay or something else. Just tell us what you want to do.')}</p>

        <form className="organise-ask" onSubmit={onSubmit}>
          <label htmlFor={fieldId}>{t('I want to organise…')}</label>
          <textarea
            id={fieldId} ref={field} rows={3} maxLength={2000} value={text}
            placeholder={t('a birthday for 40 people in Kinshasa')}
            onChange={(e) => { setText(e.target.value); if (error) setError(null) }} onKeyDown={onKey}
          />
          <div className="organise-ask-foot">
            {voice.supported && (
              <button type="button" className="organise-voice" aria-pressed={voice.listening} onClick={voice.toggle}>
                <Icon name="mic" size={17} />{voice.listening ? t('Listening… tap to stop') : t('Dictate')}
              </button>
            )}
            <button type="submit" className="organise-send" disabled={!text.trim() || busy} aria-busy={busy}>
              {busy ? t('Setting it up…') : t('Hand it to Trezutil')}<Icon name="arrow" size={17} />
            </button>
          </div>
        </form>
        {error && <ErrorState error={error} onRetry={() => void submit()} compact />}
        {!signedIn && <p className="organise-note">{t('You will sign in with your phone number first: a mission belongs to your account.')}</p>}

        <div className="organise-ideas">
          <p id="organise-ideas-label">{t('Where to start')}</p>
          <ul aria-labelledby="organise-ideas-label">
            {IDEAS.map((idea) => <li key={idea.id}><button type="button" onClick={() => start(idea.start)}>{t(idea.label)}</button></li>)}
          </ul>
        </div>

        {signedIn && (
          <div className="organise-recent" id="missions">
            <h2>{t('Your missions')}</h2>
            {recent.loading && !recent.data && <Loading label={t('Loading your missions…')} />}
            {recent.error && <ErrorState error={recent.error} onRetry={recent.reload} compact />}
            {recent.data && recent.data.items.length === 0 && <p className="organise-note">{t('Nothing organised yet. What you hand over will be kept here.')}</p>}
            {recent.data && recent.data.items.length > 0 && (
              <ul>
                {recent.data.items.map((m) => (
                  <li key={m.id}>
                    <ActionLink href={`/organize/${encodeURIComponent(m.id)}`}>
                      <strong>{m.title}</strong>
                      <span>{[m.location, m.participants ? tn(m.participants, '{n} person', '{n} people') : ''].filter(Boolean).join(' · ') || typeLabel(m.type)}</span>
                      <em>{t(STATUS_LABELS[m.status] ?? m.status)}</em>
                    </ActionLink>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </section>
    </AppShell>
  )
}

function Fact({ label, children }: { label: string; children: string }) {
  return <div><dt>{label}</dt><dd>{children}</dd></div>
}

/** What each planned step is called on the page: a short noun, calmer than the sentence the server sends. */
const STEP_NAMES: Record<string, string> = {
  FIND_VENUE: tx('Venue'), FIND_CATERING: tx('Catering'), FIND_DECORATION: tx('Decoration'), FIND_ENTERTAINMENT: tx('Entertainment'),
  FIND_PHOTOGRAPHY: tx('Photography'), FIND_TRANSPORT: tx('Transport'), FIND_ACCOMMODATION: tx('Accommodation'),
  FIND_LOCAL_TRANSPORT: tx('Local transport'), FIND_ACTIVITIES: tx('Activities'), FIND_MOVING_TRANSPORT: tx('Moving transport'),
  FIND_MOVERS: tx('Movers'), FIND_STORAGE: tx('Storage'), FIND_TRANSPORT_OPTION: tx('Transport option'), CHECK_AVAILABILITY: tx('Availability'),
}

/** The kinds of mission that can be planned today: the ones a person can pick when theirs could not be recognised. */
const PLANNABLE_TYPES = ['EVENT', 'TRAVEL', 'STAY', 'MOVE', 'TRANSPORT']

const stepName = (s: MissionStep) => (STEP_NAMES[s.type] ? t(STEP_NAMES[s.type]!) : s.title)

/**
 * What a step of the plan says about itself, and what it offers. A step that can be run says "Ready to start" with a
 * button; while it runs it says "Searching…"; when done it says "Done" and how many were found; when it failed it
 * says so and offers to try again; a step that cannot be run yet stays "To be defined". Nothing here is a dashboard.
 */
function stepView(s: MissionStep, running: boolean): { line: string; result?: string; action?: 'run' | 'retry' } {
  if (running || (s.status === 'in_progress' && !s.stale)) return { line: t('Searching…') }
  if (s.status === 'completed') {
    const found = s.execution?.status === 'completed' ? tn(s.execution.total, '{n} result found', '{n} results found') : undefined
    return { line: `✓ ${t('Done')}`, result: found }
  }
  if (s.status === 'failed' || (s.status === 'in_progress' && s.stale)) {
    return { line: t('Unable to complete this step.'), action: s.canExecute ? 'retry' : undefined }
  }
  if (s.executable && s.status === 'pending') return { line: t('Ready to start'), action: s.canExecute ? 'run' : undefined }
  return { line: stepState(s) }
}

/** A step that is still to be defined says so; anything else says where it stands. */
function stepState(s: MissionStep): string {
  return s.status === 'pending' ? t('To be defined') : t(STEP_LABELS[s.status] ?? s.status)
}

/** How often the page asks for the state of a mission while a run is under way. */
const POLL_MS = 1500

const BLOCKED_LINES: Record<string, string> = {
  dependency_failed: tx('Blocked: {name} did not work out.'),
  dependency_blocked: tx('Blocked: {name} is blocked.'),
  dependency_skipped: tx('Blocked: {name} was skipped.'),
}

const OUTCOME_LINES: Record<string, string> = {
  completed: tx('Everything is done.'), needs_attention: tx('Some parts need your attention.'),
  in_progress: tx('Your organisation is under way…'), waiting: tx('The rest is still to be defined.'),
}

interface PartView { line: string; mark?: string; result?: string; action?: 'run' | 'retry' }

/**
 * A part of the plan as the person reads it: ✓ done, ● in progress, ○ ready or waiting, ✕ did not work out, – blocked
 * or set aside. A blocked part says why, in words, and what it waits for is named, never an identifier.
 */
function flowView(s: MissionStep, names: Record<string, string>, running: boolean): PartView {
  const named = (ids: string[]) => ids.map((id) => names[id]).filter(Boolean).join(', ')
  switch (running ? 'running' : s.state) {
    case 'running': return { mark: '●', line: t('In progress…') }
    case 'completed': {
      const x = s.execution
      const result = x?.status !== 'completed' ? undefined
        : x.available === true ? t('Available') : x.available === false ? t('Not available') : tn(x.total, '{n} result found', '{n} results found')
      return { mark: '✓', line: t('Done'), result }
    }
    case 'ready': return { mark: '○', line: t('Ready to start'), action: s.canExecute ? 'run' : undefined }
    case 'failed': return { mark: '✕', line: t('Unable to complete this step.'), action: s.canExecute ? 'retry' : undefined }
    case 'skipped': return { mark: '–', line: t('Skipped') }
    case 'blocked': {
      if (s.reason === 'needs_input') return { mark: '–', line: s.detail === 'listing' ? t('The search found nothing to check.') : t('Add the dates to check availability.') }
      if (s.reason === 'needs_confirmation') return { mark: '–', line: t('Waiting for your confirmation.') }
      const line = BLOCKED_LINES[s.reason]
      return { mark: '–', line: line ? t(line, { name: named(s.blockedBy) }) : t('Blocked') }
    }
    default: // pending
      return { mark: '○', line: s.reason !== 'not_executable' && s.dependsOn.length ? t('Waiting for {name}', { name: named(s.dependsOn) }) : t('To be defined') }
  }
}

/** Where the whole organisation stands, from its parts (the same reading the server makes). */
function outcomeOf(steps: MissionStep[]): MissionOutcome {
  if (steps.length === 0) return 'empty'
  const n = (state: string) => steps.filter((s) => s.state === state).length
  if (n('completed') === steps.length) return 'completed'
  if (n('running') > 0) return 'in_progress'
  if (n('failed') + n('blocked') + n('skipped') > 0) return 'needs_attention'
  return n('ready') > 0 ? 'ready' : 'waiting'
}

/** The dates a check of availability needs: the only thing the person can add from here. */
function DatesForm({ mission, onSaved }: { mission: Mission; onSaved: () => void }) {
  const [start, setStart] = useState(mission.startDate)
  const [end, setEnd] = useState(mission.endDate)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const id = useId()
  const save = async (e: FormEvent) => {
    e.preventDefault()
    if (!start || busy) return
    setBusy(true)
    setError(null)
    try {
      await missionsApi.update(mission.id, { start_date: start, ...(end ? { end_date: end } : {}) })
      onSaved()
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <form className="mission-dates" onSubmit={(e) => void save(e)}>
      <label htmlFor={`${id}-start`}>{t('Start date')}</label>
      <input id={`${id}-start`} type="date" value={start} onChange={(e) => setStart(e.target.value)} required />
      <label htmlFor={`${id}-end`}>{t('End date')}</label>
      <input id={`${id}-end`} type="date" value={end} min={start || undefined} onChange={(e) => setEnd(e.target.value)} />
      <button type="submit" className="organise-secondary" disabled={!start || busy} aria-busy={busy}>{t('Save the dates')}</button>
      {error && <ErrorState error={error} compact />}
    </form>
  )
}

/**
 * Next step: prepare the plan (an explicit action, never automatic), or the plan itself once it exists. The plan is
 * a list of things to be defined, each one an intention: nothing is searched, booked or ordered by preparing it.
 */
function MissionPlanSection({ mission, plan, onPlan, onRun, onChanged }: {
  mission: Mission; plan: MissionPlan; onPlan: (p: MissionPlan) => void; onRun: (s: MissionRunState) => void; onChanged: () => void
}) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [kind, setKind] = useState('')
  const [running, setRunning] = useState<string | null>(null)
  const [orchestrating, setOrchestrating] = useState<'run' | 'resume' | null>(null)
  const [elsewhere, setElsewhere] = useState(false) // another run (another tab, a double click) holds the mission
  const epoch = useRef(0) // a read that began before a run ended must not overwrite what the run answered
  const onRunRef = useRef(onRun)
  onRunRef.current = onRun
  const attempt = useRef<{ replace: boolean; key: string } | null>(null)
  const runKeys = useRef<Record<string, string>>({})
  const kindId = useId()
  const hasPlan = plan.status === 'current' && plan.planVersion > 0
  // The mission was changed to another kind after its plan was prepared: say so, and offer a new plan.
  const stale = hasPlan && plan.planMissionType !== '' && plan.planMissionType !== mission.type
  // Parts of the current plan carry a state; a plan read from a server that does not send one is shown as before.
  const graph = hasPlan && plan.steps.some((s) => s.state !== '')
  const live = graph && (orchestrating !== null || elsewhere || plan.steps.some((s) => s.state === 'running'))

  // While a run is under way (here or elsewhere) the page asks for the state, so the parts turn from ○ to ● to ✓ by themselves.
  useEffect(() => {
    if (!live) return
    const ctl = new AbortController()
    const timer = setInterval(() => {
      const seen = epoch.current
      missionsApi.execution(mission.id, ctl.signal).then((st) => {
        if (ctl.signal.aborted || seen !== epoch.current) return
        onRunRef.current(st)
        setElsewhere(st.busy || st.steps.some((x) => x.state === 'running'))
      }).catch(() => { /* the next tick asks again */ })
    }, POLL_MS)
    return () => { ctl.abort(); clearInterval(timer) }
  }, [live, mission.id])

  const prepare = async (replace: boolean) => {
    if (busy) return
    // A retry of the same request reuses its key, so a double click or a flaky network never prepares two plans.
    if (attempt.current?.replace !== replace) attempt.current = { replace, key: newKey() }
    setBusy(true)
    setError(null)
    try {
      const next = await missionsApi.preparePlan(mission.id, attempt.current.key, replace)
      attempt.current = null
      onPlan(next)
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setBusy(false)
    }
  }

  /**
   * Runs one step, on request. A retry of a request that got no answer reuses its key, so the step is never run
   * twice; once the server has answered (done or failed) the next attempt is a new one with a new key.
   */
  const run = async (step: MissionStep) => {
    if (running) return
    runKeys.current[step.id] ??= newKey()
    setRunning(step.id)
    setError(null)
    try {
      const done = await missionsApi.executeStep(mission.id, step.id, runKeys.current[step.id]!)
      delete runKeys.current[step.id]
      onPlan({ ...plan, steps: plan.steps.map((s) => (s.id === done.step.id ? done.step : s)) })
    } catch (err) {
      const e = toApiError(err)
      if (e.status !== 0 && e.status < 500) delete runKeys.current[step.id] // answered: a retry is a new request
      setError(e)
      if (e.code === 'STEP_IN_PROGRESS' || e.code === 'STEP_NOT_RUNNABLE') onChanged() // what is stored has moved on: read it again
    } finally {
      setRunning(null)
    }
  }

  /**
   * Runs the whole mission, on request. Nothing is sent about what to run: the server decides from the plan. Repeating it
   * is safe (a completed part is never run again), so a double click or a retry after a network failure is harmless.
   */
  const orchestrate = async (mode: 'run' | 'resume') => {
    if (orchestrating || running) return
    setOrchestrating(mode)
    setError(null)
    try {
      const st = await (mode === 'run' ? missionsApi.run(mission.id) : missionsApi.resume(mission.id))
      epoch.current += 1
      onRun(st)
      setElsewhere(st.busy)
    } catch (err) {
      const e = toApiError(err)
      setError(e)
      if (e.code === 'MISSION_NOT_RUNNABLE' || e.code === 'PLAN_REQUIRED' || e.code === 'MISSION_GRAPH_INVALID') onChanged() // what is stored has moved on
    } finally {
      setOrchestrating(null)
    }
  }

  const chooseKind = async () => {
    if (!kind || busy) return
    setBusy(true)
    setError(null)
    try {
      await missionsApi.update(mission.id, { type: kind })
      onChanged()
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setBusy(false)
    }
  }

  if (hasPlan) {
    const names = Object.fromEntries(plan.steps.map((s) => [s.id, stepName(s)]))
    const done = plan.steps.filter((s) => s.state === 'completed').length
    const outcome = outcomeOf(plan.steps)
    const outcomeLine = OUTCOME_LINES[outcome]
    const bulk: 'run' | 'resume' | null = plan.steps.some((s) => s.state === 'failed' && s.canExecute) ? 'resume'
      : plan.steps.some((s) => s.state === 'ready' && s.canExecute) ? 'run' : null
    return (
      <section className="mission-next" aria-labelledby="mission-next-title">
        <h2 id="mission-next-title">{t('Your organisation')}</h2>
        {graph && (
          <div className="mission-progress">
            <p className="mission-progress-line" role="status">
              <span>{t('{done} of {total} done', { done, total: plan.steps.length })}</span>
              {outcomeLine && <strong>{t(outcomeLine)}</strong>}
            </p>
            <div className="mission-progress-bar" role="progressbar" aria-label={t('Progress')} aria-valuemin={0} aria-valuemax={plan.steps.length} aria-valuenow={done}>
              <span style={{ width: `${Math.round((done / plan.steps.length) * 100)}%` }} />
            </div>
            {(bulk || orchestrating) && (
              <button type="button" className="organise-send mission-run" disabled={!!orchestrating || !!running || elsewhere} aria-busy={!!orchestrating}
                onClick={() => void orchestrate(bulk ?? 'run')}>
                {orchestrating ? t('Running…') : bulk === 'resume' ? t('Resume') : t('Run everything')}
              </button>
            )}
            {elsewhere && !orchestrating && <p className="organise-note" role="status">{t('A run is already under way. This page updates by itself.')}</p>}
          </div>
        )}
        <ol className="mission-plan" aria-label={t('Your organisation')}>
          {plan.steps.map((s) => {
            const flow = s.state !== ''
            const view: PartView = flow ? flowView(s, names, running === s.id) : stepView(s, running === s.id)
            return (
              <li key={s.id} data-status={s.status} data-state={flow ? s.state : undefined}>
                {view.mark && <span className="mission-step-mark" aria-hidden="true">{view.mark}</span>}
                <strong>{stepName(s)}</strong>
                <em>{view.line}</em>
                {view.result && <span className="mission-step-result">{view.result}</span>}
                {s.actionSupport === 'not_yet_supported' && <small>{t('Not available yet')}</small>}
                {s.state === 'blocked' && s.reason === 'needs_input' && s.detail === 'dates' && <DatesForm mission={mission} onSaved={onChanged} />}
                {view.action && (
                  <button type="button" className="organise-secondary mission-step-action" disabled={!!running || !!orchestrating} onClick={() => void run(s)}>
                    {view.action === 'run' ? t('Execute') : t('Retry')}<span className="sr-only"> {stepName(s)}</span>
                  </button>
                )}
              </li>
            )
          })}
        </ol>
        <p className="organise-note">{plan.steps.some((s) => s.executable)
          ? (graph ? t('Run everything, or one part at a time: it only searches and checks availability. Nothing is booked or ordered, and nothing is confirmed without you.') : t('Execute a part when you are ready: it only searches, and nothing is booked or ordered.'))
          : t('Each part is still to be defined. Nothing is searched, booked or ordered yet.')}</p>
        {stale && (
          <div className="mission-stale">
            <p className="organise-note">{t('This plan was prepared for another kind of goal.')}</p>
            <button type="button" className="organise-secondary" disabled={busy} aria-busy={busy} onClick={() => void prepare(true)}>{busy ? t('Preparing…') : t('Prepare a new plan')}</button>
          </div>
        )}
        {error && <ErrorState error={error} compact />}
      </section>
    )
  }

  return (
    <section className="mission-next" aria-labelledby="mission-next-title">
      <h2 id="mission-next-title">{t('Next step')}</h2>
      {plan.plannable ? (
        <>
          <p className="mission-next-line">{t('I can help you organise this, step by step.')}</p>
          <p className="organise-note">{t('Preparing the plan only lists what needs doing. Nothing is searched, booked or ordered.')}</p>
          <button type="button" className="organise-send mission-prepare" disabled={busy} aria-busy={busy} onClick={() => void prepare(false)}>
            {busy ? t('Preparing…') : t('Prepare the plan')}<Icon name="arrow" size={17} />
          </button>
        </>
      ) : (
        <>
          <p className="mission-next-line">{t('I do not have a plan for this kind of goal yet.')}</p>
          <p className="organise-note">{t('Say what kind of organisation it is and I will prepare one.')}</p>
          <div className="mission-kind">
            <label htmlFor={kindId}>{t('What kind of organisation?')}</label>
            <select id={kindId} value={kind} onChange={(e) => setKind(e.target.value)}>
              <option value="">{t('Choose…')}</option>
              {PLANNABLE_TYPES.map((k) => <option key={k} value={k}>{typeLabel(k)}</option>)}
            </select>
            <button type="button" className="organise-secondary" disabled={!kind || busy} aria-busy={busy} onClick={() => void chooseKind()}>{t('Use this kind')}</button>
          </div>
        </>
      )}
      {error && <ErrorState error={error} onRetry={() => void (plan.plannable ? prepare(false) : chooseKind())} compact />}
    </section>
  )
}

function MissionView({ mission, plan, steps, onPlan, onRun, onChanged }: {
  mission: Mission; plan: MissionPlan; steps: MissionStep[]; onPlan: (p: MissionPlan) => void; onRun: (s: MissionRunState) => void; onChanged: () => void
}) {
  const where = [mission.location, mission.participants ? tn(mission.participants, '{n} person', '{n} people') : ''].filter(Boolean).join(' · ')
  const dates = mission.startDate && mission.endDate && mission.endDate !== mission.startDate
    ? t('{from} to {to}', { from: day(mission.startDate), to: day(mission.endDate) })
    : mission.startDate ? day(mission.startDate) : mission.endDate ? t('Until {date}', { date: day(mission.endDate) }) : ''
  const own = steps.filter((s) => s.planVersion === 0) // the steps added by hand; the plan's are in the plan
  return (
    <article className="mission" aria-labelledby="mission-title">
      <p className="organise-eyebrow">{typeLabel(mission.type)}</p>
      <h1 id="mission-title">{mission.title}</h1>
      {where && <p className="mission-where">{where}</p>}
      <p className="mission-status" data-status={mission.status}><span aria-hidden="true" />{t(STATUS_LABELS[mission.status] ?? mission.status)}</p>

      <section className="mission-understood" aria-labelledby="mission-understood-title">
        <h2 id="mission-understood-title">{t('Here is what I understood.')}</h2>
        <blockquote>{mission.objective}</blockquote>
        <dl>
          <Fact label={t('Kind')}>{typeLabel(mission.type)}</Fact>
          {mission.participants > 0 && <Fact label={t('People')}>{tn(mission.participants, '{n} person', '{n} people')}</Fact>}
          {mission.location && <Fact label={t('Place')}>{mission.location}</Fact>}
          {dates && <Fact label={t('When')}>{dates}</Fact>}
          {mission.budgetMinor > 0 && <Fact label={t('Budget')}>{formatMinor(mission.budgetMinor, mission.currency)}</Fact>}
        </dl>
      </section>

      <MissionPlanSection mission={mission} plan={plan} onPlan={onPlan} onRun={onRun} onChanged={onChanged} />

      {own.length > 0 && (
        <ol className="mission-steps" aria-label={t('The parts of this mission')}>
          {own.map((s) => (
            <li key={s.id}>
              <strong>{s.title}</strong>
              {s.description && <span>{s.description}</span>}
              <em>{t(STEP_LABELS[s.status] ?? s.status)}</em>
            </li>
          ))}
        </ol>
      )}

      <p className="mission-back"><ActionLink href="/organize"><Icon name="arrow" size={15} />{t('Organise something else')}</ActionLink></p>
    </article>
  )
}

export function MissionPage() {
  const { id = '' } = useParams()
  const data = useAsync(async () => {
    const mission = await missionsApi.get(id)
    // The plan and the steps added by hand are secondary: a failure to read them must not hide the mission.
    const [plan, steps] = await Promise.all([
      missionsApi.plan(id).catch(() => null),
      missionsApi.steps(id).catch(() => [] as MissionStep[]),
    ])
    return { mission, plan: plan ?? noPlan(mission), steps }
  }, [id])
  useEffect(() => { document.title = data.data ? `${data.data.mission.title} — Trezutil` : t('Organise — Trezutil') }, [data.data])
  const setPlan = (plan: MissionPlan) => { if (data.data) data.setData({ ...data.data, plan }) }
  // What a run (or a read of the execution) answers: the parts with their states, and the mission's own status.
  const applyRun = (run: MissionRunState) => {
    if (!data.data) return
    const { mission, plan } = data.data
    data.setData({ ...data.data, mission: run.missionStatus ? { ...mission, status: run.missionStatus } : mission, plan: { ...plan, steps: run.steps } })
  }
  return (
    <AppShell>
      <section className="organise">
        {data.loading && !data.data && <Loading label={t('Loading your mission…')} />}
        {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
        {data.data && <MissionView mission={data.data.mission} plan={data.data.plan} steps={data.data.steps} onPlan={setPlan} onRun={applyRun} onChanged={data.reload} />}
      </section>
    </AppShell>
  )
}

/** What to show when the plan could not be read: no plan, and the planning offer for the kinds that have one. */
function noPlan(mission: Mission): MissionPlan {
  return {
    missionId: mission.id, missionType: mission.type, planVersion: 0, status: 'none', plannable: PLANNABLE_TYPES.includes(mission.type),
    needsInformation: false, planMissionType: '', steps: [], versions: [],
  }
}
