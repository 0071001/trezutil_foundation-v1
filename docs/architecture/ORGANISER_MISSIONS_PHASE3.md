# Organiser — Mission step execution, phase 3

Phases 1 and 2 recorded a goal and planned it into steps. Phase 3 proves **single-step execution**: one step, on
request, through the existing Commerce Action Runtime.

```text
Mission → MissionStep → MissionStepExecutor → CommerceActionRuntime → existing action → result
                                                                          ↓
                                              mission_step_executions (durable) → step COMPLETED / FAILED
```

There is **no second commerce engine**. `internal/application/missionexec.Executor` is thin: it loads the mission and the
step, checks ownership and that the step can run, translates the step into a runtime request, calls the runtime and
records the outcome. It does not search, reserve, order, pay, or check inventory, authorization or credits: the runtime
does all of that, exactly as for any other action. Nothing runs a whole mission, and the multi-step orchestration
(dependencies, "execute all", retry loops, background work) is not started.

## The runtime, inspected first

`commerce.CommerceActionRuntime.Execute(ctx, action, authenticatedActor)` already provides: identity protection (the actor is
always the authenticated one), tenant resolution, action and parameter validation, a confirmation policy, an
idempotency store, the usage (credit) check, reservation, consumption or release, handler dispatch, and structured
results (`ExecuteResult` with a status and coded errors). The executor reuses all of it. Two facts shaped the design:

- The runtime's idempotency store is **in memory** and caches **successes only**. It cannot be the durable guarantee, so
  the mission layer keeps its own durable record of each attempt (below) and *also* gives the runtime a key.
- An action needs a `ConversationID`. A mission has no conversation, so the executor passes `mission:<mission id>` as the
  opaque correlation id the runtime asks for. This is the one place the mission layer bends to the runtime's shape; the
  runtime was not changed.

## The first executable action: `SEARCH_LISTINGS`

It is the cleanest existing action for orchestration: registered in the runtime, with a real handler, parameter
validation and tests; read-only (it changes nothing); no external dependency beyond the catalogue search the platform
already has; and a deterministic, small result (a count and listing ids). Phase 2 already named it as the action of
every `FIND_*` step. No action was invented.

A step is **executable** when it maps unambiguously onto a vertical of the existing vertical catalogue, so the request
is "search this vertical, in the mission's place if it has one" (the place is never made up):

| Step type | Mission type → vertical |
| --- | --- |
| `FIND_ACCOMMODATION` | `STAY`, `TRAVEL` → `HOTEL`; `MOVE` → `REAL_ESTATE` |
| `FIND_CATERING` | `EVENT` → `RESTAURANT` |
| `FIND_TRANSPORT` | `TRAVEL`, `EVENT` → `TRANSPORT` |
| `FIND_TRANSPORT_OPTION` | `TRANSPORT` → `TRANSPORT` |
| `FIND_LOCAL_TRANSPORT`, `FIND_MOVING_TRANSPORT` | `TRAVEL`, `MOVE` → `VEHICLE_RENTAL` |

Not executable yet, and answering `422 STEP_NOT_EXECUTABLE` rather than being run with a guess: `FIND_VENUE`,
`FIND_DECORATION`, `FIND_ENTERTAINMENT`, `FIND_PHOTOGRAPHY`, `FIND_ACTIVITIES`, `FIND_MOVERS`, `FIND_STORAGE` (no vertical of
their own: each would need search words or categories, a design of its own) and `CHECK_AVAILABILITY` (needs a listing and
dates chosen by an earlier step, i.e. multi-step orchestration). A step the runtime does not support, or a step added
by hand without an action, is also refused.

## Lifecycle

```text
PENDING ──execute──▶ IN_PROGRESS ──runtime SUCCESS──▶ COMPLETED   (final: never run again, never reset)
                          └───────runtime failure/timeout/panic──▶ FAILED ──execute again──▶ IN_PROGRESS …
```

- `failed → in_progress` is now a direct transition of the step lifecycle (a retry is the next attempt); `completed` and
  `skipped` steps are never run (`409 STEP_NOT_RUNNABLE`).
- A step is never left in progress: the executor always records an outcome (a runtime error, a timeout of 30 s or a
  panic end the attempt as `failed`), and does so even if the request was cancelled. If a process dies mid-run, the
  attempt is **given up on** after 2 minutes (`ABANDONED`) and the step can be run again; a late outcome of the
  abandoned attempt is refused.
- Steps of a plan that has been replaced are history and are not run; a paused or closed mission runs nothing
  (`409 MISSION_NOT_RUNNABLE`); a draft or active mission runs its steps. **Running a step never activates, completes or
  otherwise changes the mission** (no automatic completion yet).

## Durable result (`migrations/000029_mission_step_executions.sql`)

One table owned by the Mission domain, `mission_step_executions`: `step_id` and `mission_id` foreign keys with
`ON DELETE CASCADE` (so a deleted step, mission or owner takes its attempts with it), `attempt` (unique per step),
`status`, `action_type`, `idempotency_key`, `result_reference`, `result_payload` (JSONB), `runtime_status`, `error_code`,
`started_at`, `completed_at`, `created_at`. Indexes: unique `(step_id, idempotency_key)`, unique `(step_id)` where in
progress (one running attempt per step), and `(mission_id, step_id, attempt DESC)`. CHECKs keep the record coherent
(`failed` has an error code; `completed_at` exists exactly when the attempt has ended).

It stores a **reference and a small summary**, not commerce data: for a search, `{"total": 3, "listing_ids": [≤10 ids],
"has_more": false, "vertical": …, "location": …}`; `result_reference` is the execution itself (a result set is not a
commerce entity; an action that creates an order or a reservation will reference that entity). `mission_steps.action_ref`
and `result_ref` point at the attempt. `error_code` is a **stable classification**: the runtime's own code when it gave
one in the expected form (`INSUFFICIENT_CREDITS`, `INVALID_PARAM`, `ACTION_FAILED`…), else `RUNTIME_ERROR`,
`RUNTIME_TIMEOUT`, `ABANDONED`, `UNEXPECTED_RESULT` (for example a request for confirmation, which steps do not handle
yet). Runtime messages are never stored, so no internal detail leaks.

## Idempotency

- **Same step + same `Idempotency-Key` = the same execution.** The key is unique per step in the database; a repeated
  request (any state, even after the mission was closed, for its owner) returns the stored attempt (`200`,
  `replayed: true`) and runs nothing. A failed attempt's key replays the failure: a retry uses a new key.
- **Concurrent requests run once.** Starting an attempt locks the step row, so of N simultaneous requests one starts and the
  others get `409 STEP_IN_PROGRESS`; N simultaneous requests with the same key give one `201` and replays. Tested on
  PostgreSQL.
- The runtime also gets a key (`mission-step:<step>:<key or execution id>`), so its own idempotent cache and its logs line up.
  The runtime's action id **is** the execution id, which also ties its credit reservation to the record.

## Credits

Unchanged and entirely the runtime's. The executor does not import, read or write the usage service, accounts,
reservations or the ledger (a test guards the package source). Verified against the real runtime with a counting credit
store: a success reserves once and consumes once; a failed run reserves and **releases**, so the credits are kept; a
retry reserves again and consumes once (net: charged for the success only); insufficient credits is the runtime's own
answer (`INSUFFICIENT_CREDITS`) and fails the step without searching; a replay or a completed step charges nothing.

## API

```text
POST /api/v1/missions/{id}/steps/{step_id}/execute     no body; Idempotency-Key optional
   201 an attempt was run (completed or failed)   200 replay of an Idempotency-Key
   400 body or key invalid   401   403 another person's mission   404 unknown mission/step (or a step of another mission)
   409 STEP_IN_PROGRESS · STEP_NOT_RUNNABLE · MISSION_NOT_RUNNABLE    422 STEP_NOT_EXECUTABLE
```

The request carries no parameters: what the runtime is asked is decided server-side from the mission and the step, never by
the client. The answer is `{mission_id, step_id, execution_id, status, attempt, action_type, result:{reference, total,
listing_ids…} | error:{code}, replayed, step_status, step}`. A failure of the runtime is a *recorded outcome* (`201` with
`status: failed`), not an HTTP error. `GET …/steps`, `GET …/plan` now also give each step `executable`, `can_execute`,
`stale` and its latest `execution`. Logs carry `mission_id`, `step_id`, `execution_id`, `action_type` (existing zerolog).

## Frontend (`/organize/:id`)

Only the steps of the plan change. A step that can be run reads "Prêt à commencer" with **[Exécuter]**; while it runs,
"Recherche en cours…"; when done, "✓ Terminé" and "3 résultats trouvés"; on failure "Impossible de terminer cette étape." with
**[Réessayer]**. A step that cannot be run stays "À définir". One step runs at a time; a request that got no answer is retried
with the same key, a request the server answered gets a new one. No dashboard, no progress meter, no redesign.

## Known limitations (for later phases)

- Only `SEARCH_LISTINGS`, and only the steps in the table above. Steps that need search words, categories, a chosen
  listing, dates, a reservation, an order or a payment are not executable yet.
- Results are counts and listing ids; the page does not show the listings found, and a result cannot yet be chosen.
- No dependency between steps, no "execute all", no automatic retry, no background execution, no mission completion
  from its steps, no confirmation handling for steps that would need it.
- The runtime's idempotency store is in memory; the durable guarantee is the mission record. A crash between the runtime
  finishing and the outcome being recorded leaves the step in progress until it is given up on (2 minutes); for a
  read-only search this only means searching again, but a future write action will need a durable runtime key.
- `ConversationID` is the opaque `mission:<id>`; the runtime has no notion of a mission context of its own.
