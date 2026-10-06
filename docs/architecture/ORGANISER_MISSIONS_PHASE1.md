# Organiser — Missions, phase 1

Organiser answers "What do you want to organise?". It is **not a new commerce engine**: it is an orchestration
layer over the Commerce Core. The new concept is the **Mission**, a goal a person delegates to Trezutil that may
need several commerce actions.

```text
Conversation → Intent → Mission (when several actions are needed)
             → Commerce Action Runtime → Search / Availability / Reserve / Order
             → Transaction → Fulfillment
```

Phase 1 builds only the foundation: the Mission and its ordered steps (domain, storage, API) and the Organiser
entry in the product (menu, page, draft flow). **Steps are recorded, not executed.** Nothing is searched, booked or
ordered when a mission is created, and no credit is used.

## Mission integration points

```text
Mission integration points:
- Intent:               Not changed. domain.ConversationIntent / CommerceActionType stay as they are. A step names
                        the action it will run (MissionStep.ActionType, a CommerceActionType code) but nothing
                        reads it yet. The sentence a person writes is read by application.UnderstandObjective, a
                        keyword reading (FR/EN) that returns a kind, title, place and head count. It is not an
                        AI classifier and never invents a field.
- CommerceActionRuntime: Not touched. Missions never call it in phase 1. A later phase runs a step by building a
                        domain.CommerceAction and submitting it to the existing runtime, then stores the action id
                        and the result on the step (MissionStep.ActionRef, ResultRef: empty today).
- Orders:               Not touched. A step's ResultRef will point at an order the runtime created.
- Reservations:         Not touched. Same: ResultRef will point at a reservation.
- Credits:              Not touched. No usage policy, ledger or reservation is read or written. "Credits power
                        action, not conversation": when steps run, each action is metered by the existing usage
                        policies; a mission itself is never priced.
- API:                  /api/v1/missions (below), in the conversation service, behind the gateway. Same bearer
                        authentication (auth.RequireUser), error envelope, Idempotency-Key header and paging as
                        the rest of the API. Documented in api/openapi/openapi.yaml.
- Frontend:             /organize (public question, sign-in needed to hand it over) and /organize/:id (the mission).
                        "Organise something" is one more need in src/lib/needs.ts, so it appears in the primary menu,
                        the mobile drawer and the home page index with no separate list to drift.
```

## Domain (`internal/domain/mission.go`)

| Concept | Notes |
| --- | --- |
| `Mission` | id, owner, type, title, objective, status, location, start/end date, participants, budget (minor units + currency), preferences, timestamps. Created as a **draft**; owned by its creator. |
| `MissionType` | Open code like verticals: `EVENT, TRAVEL, STAY, MOVE, TRANSPORT, BUSINESS_TRIP, PROJECT, OTHER`. |
| `MissionStatus` | Lower case like order statuses: `draft → active/cancelled`, `active → paused/completed/cancelled`, `paused → active/cancelled`; `completed` and `cancelled` are final and read-only. |
| `MissionStep` | id, mission, type code, title, description, status (`pending, in_progress, completed, skipped, failed`), position, `ActionType`, `ActionRef`, `ResultRef`. |
| Step position | Assigned by the repository (next free from 1) inside a transaction that locks the mission row, so concurrent additions never share a position or pass the limit (30 steps). |
| Errors | `ErrMissionNotFound` (404), `ErrMissionForbidden` (403), `ErrMissionInvalid` (400), `ErrInvalidMissionTransition` / `ErrMissionClosed` / `ErrMissionStepLimit` / `ErrIdempotencyConflict` (409). |

The step lifecycle exists in the domain and is tested, but no endpoint moves a step: nothing runs yet.

## Persistence (`migrations/000027_missions.sql`)

`missions` (owner FK to `users`, `ON DELETE CASCADE`; CHECKs on type code, status, budget, dates; indexes
`(owner_id, updated_at DESC)`, `(owner_id, status)`; unique `(owner_id, idempotency_key)` where set) and
`mission_steps` (FK to `missions`, `ON DELETE CASCADE`; unique `(mission_id, position)`; CHECK `position > 0`).

## API

```text
POST  /api/v1/missions                    201 draft (200 on an Idempotency-Key replay)
GET   /api/v1/missions?status=&page=&page_size=
GET   /api/v1/missions/{id}
PATCH /api/v1/missions/{id}               fields and/or status (lifecycle-checked)
POST  /api/v1/missions/{id}/steps         recorded as pending
GET   /api/v1/missions/{id}/steps
```

Every route needs a signed-in user and only touches that user's missions. The owner comes from the verified
token, never from the body (an `owner_id` in a body is rejected: bodies are strict). Another person's mission
answers 403, an unknown or malformed id 404.

## Frontend

- `/organize`: "Qu'est-ce que tu veux organiser ?", one field ("Je veux organiser…"), and "Où commencer": six sentence
  starts (a trip, a party, a stay, a move, transport, an event) that fill the field, not categories. A guest's
  sentence is kept (session storage) while they sign in. A retry of the same sentence reuses its Idempotency-Key.
- `/organize/:id`: the mission as Trezutil understood it ("Ce que nous avons compris"), its status
  ("Organisation en préparation" for a draft), and "Prochaine étape — Définir ce dont tu as besoin." Recorded steps,
  when there are any, are listed in order with no control that runs anything.
- Styles: `src/styles/organise.css`, scoped to `.organise`; paper, one accent, hairlines; no gradient, glow or card grid.

## Deliberately not done (next phases)

Running steps through the Commerce Action Runtime; AI decomposition of a goal into steps; step endpoints; workflows
for travel, event, wedding; merchant-side mission views; any mission pricing. Mission titles are not translated:
they are read from the person's own sentence, in their language.
