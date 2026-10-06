# Organiser — Mission planning, phase 2

Phase 1 recorded a goal as a draft **Mission**. Phase 2 lets Trezutil understand that the goal needs several
commercial steps and write them down: **Mission → Mission Planner → Plan → Steps**.

```text
Conversation → Intent → Mission → MissionPlanner → Plan → Steps ┄┄▶ CommerceActionRuntime (not called yet)
```

**Planning does not execute anything.** It does not search, call OpenSearch, reserve, order, pay, use a credit, call a
provider or touch an order. The planner only proposes steps (intentions to act); the runtime stays the one place where
actions run, in a later phase. Nothing in Phase 2 calls it. A step is stored `pending`, with no `action_ref` and no
`result_ref`.

## The planner is behind an interface

```text
domain.MissionPlanner  { Name(); Version(); Plannable(type); Plan(ctx, mission) → PlanProposal }

                 Mission
                    ↓
              MissionPlanner
                    ↑
        ┌───────────┴───────────┐
  RuleBased (this phase)    AIMissionPlanner (later)
```

`internal/application/missionplanning` holds `RuleBased`, a template of steps per mission type. No model is used. A
later planner (for example one backed by a language model) is another implementation of the same interface: Mission,
MissionStep, the plan versions and the runtime do not change. A planner is pure: it reads the mission and answers.
The only thing `RuleBased` is given is a question it may ask — "does the runtime have a handler for this action?" —
wired to the runtime's handler registry (`registry.Has`); it never runs one.

## Rule-based templates

Templates are keyed by the mission types of Phase 1. The names used when this phase was described map onto them:
`ACCOMMODATION` is `STAY`, `MOVING` is `MOVE` (`NormalizeMissionType` accepts both spellings).

| Mission type | Steps (type → action) |
| --- | --- |
| `EVENT` | `FIND_VENUE`, `FIND_CATERING`, `FIND_DECORATION`, `FIND_ENTERTAINMENT`, `FIND_PHOTOGRAPHY`, `FIND_TRANSPORT` → `SEARCH_LISTINGS` |
| `TRAVEL` | `FIND_TRANSPORT`, `FIND_ACCOMMODATION`, `FIND_LOCAL_TRANSPORT`, `FIND_ACTIVITIES` → `SEARCH_LISTINGS` |
| `MOVE` | `FIND_ACCOMMODATION`, `FIND_MOVING_TRANSPORT`, `FIND_MOVERS`, `FIND_STORAGE` → `SEARCH_LISTINGS` |
| `TRANSPORT` | `FIND_TRANSPORT_OPTION` → `SEARCH_LISTINGS`, `CHECK_AVAILABILITY` → `CHECK_AVAILABILITY` |
| `STAY` | `FIND_ACCOMMODATION` → `SEARCH_LISTINGS`, `CHECK_AVAILABILITY` → `CHECK_AVAILABILITY` |
| `OTHER`, `BUSINESS_TRIP`, `PROJECT` | no template: the planner **asks for more information** and makes no step up |

- Steps are action-oriented and specific (`FIND_CATERING`, not "handle the event"), so each can be connected to a
  commerce action later. They only find and check: a plan never reserves, orders or pays.
- A step names its `action_type` and `action_support`: `supported` when the runtime has a handler for it today,
  `not_yet_supported` when it does not (the step keeps the action it will need, marked as unavailable, rather than a
  made-up execution). Today every template step is supported.
- Templates depend only on the mission type. They never state, and the planner never invents, a place, a head count, a
  date or a budget; planning leaves the mission untouched (it is not activated).
- Titles and descriptions are in English; the web client shows each step in the person's language from its type.

## Plan versioning and idempotency

A plan is a version: `mission_plans` (`version` from 1, `status` `current` or `superseded`, `mission_type`, `planner`,
`planner_version`, `fingerprint`, `idempotency_key`). A mission has at most one current plan.

- **The same request gives the same plan.** The fingerprint identifies a plan's content (planner, templates version,
  mission type, steps). Planning again with the same fingerprint returns the stored plan (200) and stores nothing.
  An `Idempotency-Key` replays the plan its first request stored. Concurrent requests are serialised on the mission
  row: exactly one stores the plan, the others get it back.
- **A different plan is never stored silently.** If the mission already has a different plan (its kind changed, or the
  templates changed), the answer is `409 PLAN_EXISTS` unless the request says `replace: true`.
- **A replacement is the next version and the old one stays**, `superseded`, with its own steps (`plan_version` on the
  steps), readable with `GET …/plan?version=N`. A plan with steps under way is not replaced (`PLAN_IN_PROGRESS`).
- Steps added by hand in Phase 1 are plan version 0 and keep their own positions and limit.

## API

```text
POST /api/v1/missions/{id}/plan        201 new version · 200 same plan or needs_information · body optional {"replace": true}
GET  /api/v1/missions/{id}/plan        current plan; ?version=N for a version; status "none" when there is no plan yet
GET  /api/v1/missions/{id}/steps       (Phase 1) now the hand-added steps and those of the current plan
```

Same authentication, ownership (403 for another person's mission, 404 for an unknown one), strict bodies and
`Idempotency-Key` as the rest of the mission API. Statuses stay lower case (`pending`), like Phase 1.

## Frontend (`/organize/:id`)

After a mission is created the page says "Voici ce que j'ai compris." and, as an explicit step, offers
"Préparer le plan" (never started for the person). The plan then reads as a calm numbered list ("01 Lieu — À définir")
under "Ton organisation", with no button that runs anything. A goal that could not be recognised shows one selector
("Quel type d'organisation ?"); a plan prepared for another kind than the mission's now offers "Préparer un nouveau
plan" (a replacement, asked for).

## Known limitations

- Templates are the same for every mission of a type: nothing adapts to what the person said beyond its type
  (no cake for a birthday, no dates). That is deliberate for now.
- `BUSINESS_TRIP` and `PROJECT` have no template.
- Steps are not executed, ordered by dependency or connected to the runtime; `action_ref` / `result_ref` stay empty.
- No endpoint moves a step through its lifecycle; there is no editing of a plan's steps.
- The language of step titles is client-side (French and English).
