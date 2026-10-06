# Organiser — Multi-step mission orchestration, phase 4

Phase 3 proved that **one** step can run through the Commerce Action Runtime. Phase 4 runs a **whole mission**: the steps of
its plan, in the order of what depends on what, with a safe reaction to failure and a safe way to resume.

```text
Mission → Plan → Step A → CommerceActionRuntime → Result ─┐
                                                           ├→ Step B unlocked → Runtime → Result → … → Mission progress
                                 (B depends on A: it takes A's listing by reference)
```

There is still **no second commerce engine** and **no new mission status**. `missionorch.Orchestrator` only *orders and
delegates*: it reads the state of the plan, asks the existing `missionexec.Executor` (Phase 3) to run the next ready step, and
looks again. It never talks to the runtime, never searches, reserves, orders or pays, and never reads or writes a credit
(a test scans its sources for credit and runtime identifiers).

## Architecture

| Layer | Package | Job |
|---|---|---|
| Domain | `internal/domain/mission_graph.go` | Dependencies, `ValidateGraph` (self, duplicate, unknown, cycles), `ComputeFlow` (the state of every step), `MissionFlow.Outcome/Progress`, the `MissionRunStore` and `MissionGraphRepository` ports. Pure: nothing runs here. |
| Planning | `missionplanning` (templates v2) | A step type may declare the earlier step it waits for. |
| Step executor | `missionexec` | Refuses a step whose dependencies are not completed; builds the runtime request from the mission **and** from what its dependencies produced; derives the state of the steps (`Flow`, `Annotate`). |
| Orchestrator | `missionorch` | Takes the run lock, runs ready steps one after another, applies the failure policy, activates and completes the mission. |
| Storage | `infrastructure/postgres/mission_graph.go`, migration `000030` | Dependencies stored with the plan; the run lock; compare-and-set of the mission status. |
| API | `internal/api/mission_orchestration_handlers.go` | `POST /missions/{id}/run`, `POST /missions/{id}/resume`, `GET /missions/{id}/execution`. |
| Frontend | `pages/Organize.tsx` | The progress view and the "Run everything" / "Resume" buttons. |

## Dependency model

A **step of a plan may depend on other steps of the same plan**: it cannot run until each of them has **completed**.

- Stored in `mission_step_dependencies(step_id, depends_on_step_id)`. Two composite foreign keys tie both steps to the **same
  mission and the same plan version**, so a dependency on a step of another mission, of another plan version or on a step that does not
  exist cannot be stored. A step cannot depend on itself (CHECK) and a dependency is stored once (primary key). A hand-added step
  (plan version 0) has none. Dependencies go with their steps (`ON DELETE CASCADE`).
- **Cycles** cannot be excluded by a constraint, so the domain rejects them twice: when a plan is built (`NewMissionPlan` only accepts a
  dependency on an *earlier* step, then calls `ValidateGraph`) and again **before every read or run** (`ComputeFlow` validates first). A
  mission whose stored graph has a cycle (inserted behind the application's back) answers `409 MISSION_GRAPH_INVALID` and runs nothing.
- Templates v2 (`TemplatesVersion = "2"`) carry only **real data dependencies**: the availability check of a STAY or TRANSPORT plan waits for
  the search that found what it checks. Nothing else waits for anything (no invented workflow). The fingerprint includes the
  dependencies, so a plan prepared with templates v1 re-planned with v2 is a different plan (`PLAN_EXISTS` until replaced with `replace: true`).

### States are derived, never stored

READY and BLOCKED are **not** stored. The five stored step statuses stay the only source of truth (`pending, in_progress, completed,
skipped, failed`), and `ComputeFlow` derives, from them and the dependencies:

| State | Meaning |
|---|---|
| `pending` | Waiting (for steps it depends on), or nothing executable behind it (`reason`: `waiting`, `not_executable`) |
| `ready` | Every dependency is completed, its inputs are known, and it can run now |
| `running` | An attempt is in progress |
| `completed` | Done; final |
| `failed` | The latest attempt failed, or went silent (`reason: abandoned`); it can be retried on request |
| `blocked` | It cannot run until something outside it is resolved. `reason`: `dependency_failed`, `dependency_blocked`, `dependency_skipped`, `needs_input` (`detail`: `dates` or `listing`), `needs_confirmation` |
| `skipped` | Set aside on purpose; **its dependents stay blocked** (only COMPLETED unlocks) |

The mission's **outcome** is derived the same way (`empty, completed, in_progress, needs_attention, ready, waiting`). It is a reading of the steps,
not a stored status: the mission keeps `draft → active → completed` (and paused/cancelled). **Completed only when every step is completed**; all steps
are required (optional steps are future work), so a skipped step leaves the mission in `needs_attention`.

## Result propagation (by reference only)

A step takes what it needs from its dependencies as **references**, never as a copy of commerce data. The only new executable action is
`CHECK_AVAILABILITY` (already in the runtime, read-only): it needs a listing and dates.

- **listing**: the first `listing_ids` entry of the completed search it depends on. That is the runtime's own top result, so it is deterministic.
- **dates**: the mission's `start_date` (and `end_date`; a mission with only a start date is a one-day window), plus `party_size` from its participants.

A step that lacks either is **not run with a guess**: it is `blocked` with `needs_input` (`dates` / `listing`) and the person can add the dates
from the mission page. The availability execution keeps a small summary (`available`, `capacity`, `remaining`, the dates) and the checked
listing as its `result_reference`.

## Execution flow

```text
POST /missions/{id}/run
  ├─ load mission, ownership (403 / 404)            ├─ Flow(): no plan → 409 PLAN_REQUIRED, bad graph → 409 MISSION_GRAPH_INVALID
  ├─ mission completed → 200 with its state          ├─ paused / cancelled → 409 MISSION_NOT_RUNNABLE
  ├─ AcquireRun (lease)  ── not acquired → 202 {busy:true}, nothing run
  └─ loop: Flow → first READY step (position order) → [draft → active] → ExtendRun → Executor.Execute → record
        stop at the first failure (default) or carry on with independent steps
     finish: every step completed → mission completed;   ReleaseRun
```

- **Sequential, in position order** among the ready steps; the dependency order always wins (a check listed first still runs after its search).
  Independent steps could run side by side, but sequential is the safe default and a person waiting gains little from saving a second.
- Each step's attempt key is `orch:<run token>:<step id>`; the client sends no idempotency key.
- A step is attempted **at most once per orchestration**, so a failure cannot loop.
- A draft mission becomes **active** when its first step is run; a mission whose steps were completed one by one by hand is completed by the next run.
- The loop re-reads the state after every step: what changed meanwhile (a paused mission, a step run elsewhere) is respected.

## Failure and resume behaviour

| Situation | Behaviour |
|---|---|
| A step fails | Default (`on_failure: stop`): the run stops. What depends on it is **blocked** and never run; what completed **stays completed** (no rollback). |
| `on_failure: continue` | Carries on with the steps that do not depend on the failed one; its dependents stay blocked. |
| `run` again | **Never retries a failed step.** It runs what is still ready. |
| `resume` | A run that also retries the steps whose latest attempt failed or went silent, **once each per call**. A completed step is never run again. |
| Needs confirmation | The runtime's `REQUIRES_CONFIRMATION` becomes the execution error `CONFIRMATION_REQUIRED` (it replaces Phase 3's `UNEXPECTED_RESULT` for this case). The step is **blocked for the person**; neither `run` nor `resume` retries it, and nothing is ever confirmed on their behalf. |
| A step goes silent (process died) | After two minutes it is `failed` (`abandoned`); `resume` gives up on that attempt and starts a new one. |
| The process dies mid-run | The lock's lease (5 minutes, renewed before each step) expires by itself; the next run takes over. |

No retry is automatic or unbounded: a failed step is retried only when the person presses **Resume**, and once per press.

## Concurrency strategy

1. **Mission-level lock** (`mission_run_locks`): one atomic `INSERT … ON CONFLICT DO UPDATE … WHERE expires_at <= now RETURNING`. Of ten
   simultaneous orchestrations exactly one takes it; the others answer `202 {busy:true}` and run nothing. The holder renews the lease before every
   step; if it ever finds the lock is no longer its own it stops. The lock is released at the end (also after a failure, and even if the request
   was cancelled).
2. **Step claim** (Phase 3): `BeginExecution` locks the step row, so a step has at most one attempt in progress. This is the second line of
   defence: with the lock removed, six orchestrations still run each step once (tested).
3. **Compare-and-set** on the mission status (`TransitionMission … WHERE status = $from`): a stale read cannot overwrite a change made meanwhile
   (ten simultaneous `draft → active` apply once).

A busy answer can read the steps after the holder has finished: it then says `busy: true`, `ran: []` and `outcome: completed`. That is accurate
(nothing was run by this request, and everything is done) and not an error.

## Idempotency

Running is idempotent by construction, not by a client key: **a completed step is never run again**, so `run` ×N and `resume` ×N on a finished mission
run nothing, charge nothing, ask the runtime nothing, and answer `200` with the state. A mission already completed answers `200` too. The
Phase 3 single-step endpoint keeps its `Idempotency-Key` behaviour.

## Credits

**Untouched by the mission layer.** Credits are checked, reserved, consumed or released by the Commerce Action Runtime exactly as for any
other action. Tests through the **real** runtime (with a counting credit store) show: a metered search is charged once on success, a failed
search keeps the credits (reserve then release), a resume charges only its success, insufficient credits fail the step with
`INSUFFICIENT_CREDITS` and the dependent is blocked, repeating or ten simultaneous orchestrations charge once.

## Database changes (`migrations/000030_mission_orchestration.sql`)

- `mission_steps`: `UNIQUE (id, mission_id, plan_version)` (`uq_mission_steps_identity`), the target of the composite keys.
- `mission_step_dependencies(mission_id, plan_version, step_id, depends_on_step_id, created_at)`: see *Dependency model*.
- `mission_run_locks(mission_id PK → missions ON DELETE CASCADE, token UUID, acquired_at, expires_at CHECK expires_at > acquired_at)`.
- Down removes all three and is repeatable (tested: down, up again, down twice). Existing plans have no dependencies and keep working.

## API

| Route | Notes |
|---|---|
| `POST /missions/{id}/run` | Optional body `{"on_failure": "stop"\|"continue"}`. `200` state; `202` state when busy; `409 PLAN_REQUIRED`, `MISSION_GRAPH_INVALID`, `MISSION_NOT_RUNNABLE`; `403`/`404` |
| `POST /missions/{id}/resume` | Same, plus retries failed/silent steps once each |
| `GET /missions/{id}/execution` | Read-only state |
| `POST /missions/{id}/steps/{step_id}/execute` (Phase 3) | Now also `409 STEP_WAITING` (a dependency has not completed) and `409 STEP_NEEDS_INPUT` (dates / listing) |

Response: `{ mission_id, plan_version, mission_status, outcome, busy, progress: {completed, skipped, total}, ran: [{step_id, step_type, action_type, execution_id, attempt, status, error_code?}], steps: [<step> + state, depends_on, blocked_by, reason?, detail?] }`.
Ownership follows Phases 1–3: someone else's mission is `403`, an unknown or malformed id `404`. See `api/openapi/openapi.yaml`.

## Observability

Every orchestration and step logs `mission_id`, `plan_version`, `step_id`, `execution_id`, `action_type` (plus `status` and `error_code`); the executor adds
`attempt`. Skipped, busy and lost-lock cases are logged at info/warn with the same fields. No log line carries a runtime message or a credit figure.

## Frontend (`/organize/:id`)

The plan list shows a mark per part (✓ done, ● in progress, ○ ready or waiting, ✕ did not work out, – blocked or set aside), a line in words (what it waits for, named; why it
is blocked), a progress bar with "n of m done" and the outcome sentence. One primary button: **Run everything** when something is ready,
**Resume** when something failed. While a run is under way (here, or in another tab) the page reads `/execution` every 1.5 s so the marks move
by themselves, and stops when nothing is running. A part blocked for lack of dates shows a two-field form that saves them (a `PATCH` of the mission, nothing is run). Single-part
buttons remain. A server that sends no states (Phase 3) is shown exactly as before. French strings use the existing register.

## Tests

- Domain: graph validation (self, duplicate, unknown, cycles of 2 and 3, behind a valid step), flow states of chains/diamonds/failed/skipped/blocked propagation, 2 000-step chain, outcome table.
- Step level: dependency refusal, input propagation, no-guess blocking, confirmation, silent attempts, invalid graphs, hand-added steps unchanged.
- Orchestrator (real orchestrator + real executor + in-memory store with the Postgres semantics): single step, dependent chain, dependency beats position, failure stop / continue, resume, plain run never retries, repeats, **10 simultaneous orchestrations**, no-lock claim test (30 repeats), busy + lease takeover, lock release after failure, mission status, ownership/404/no plan/cycle/self dependency, confirmation, silent step, only the plan's steps, source guard.
- Through the **real runtime** with credits: charged once, failed search keeps credits, resume charges its success, insufficient credits, ten simultaneous.
- HTTP handlers: auth, state shape, policy body, 202, error mapping, malformed ids.
- **PostgreSQL** (`infrastructure/postgres/integration/mission_orchestration_test.go`): whole mission on the real schema over HTTP, failure/resume, missing input, manual steps vs graph, ownership, a cycle inserted by raw SQL, every constraint (cross-mission, cross-version, self, duplicate, unknown, plan version 0), cascade, plans keep their own graph across versions, **10 simultaneous runs** (and 10 missions × 2), the step claim without the lock, the lock itself (20 racers, extend, release, takeover, constraints), compare-and-set, migration down/up/down.
- Frontend: states and marks, run, failure and resume, blocked reasons, dates form, busy + polling, refusal, paused, single step, French, backward compatibility.

## Known limitations

- **Sequential only.** Independent steps are not run in parallel.
- **All steps are required**; optional steps and a "skip" action are not part of this phase (a step set aside blocks its dependents and the mission's completion).
- **Two executable actions** (`SEARCH_LISTINGS`, `CHECK_AVAILABILITY`). Steps without a vertical (venue, decoration, entertainment, photography, activities, movers, storage) stay "to be defined". Nothing reserves, orders or pays; a reservation step is a later phase.
- **No confirmation flow** for a step that needs one: it is blocked for the person; there is no UI to confirm it from the mission page yet.
- **A run blocks the HTTP request** until it ends (bounded by the number of steps × the 30 s runtime timeout); the page polls meanwhile. A crashed process leaves the mission "busy" for up to the 5-minute lease.
- **Plans prepared with templates v1** have no dependencies: their check step is a plain pending step. Prepare a new plan (`replace: true`) to get the graph.
- The orchestrator does not retry on its own and has no scheduler: it only runs when asked.
- The committed `api/openapi/openapi.yaml` already has one misindented line near the top (the `users-me-roles` response, line 95, also in `HEAD`) that strict YAML parsers reject; it is not part of this phase and was left alone.
