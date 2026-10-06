import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetSiteContent } from '@/lib/useSiteContent'
import { resetVerticals } from '@/lib/verticals'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

/*
 * Organiser: the question, the draft mission it creates, the mission shown back, and the promise that nothing is
 * searched, booked or ordered along the way.
 */

const MISSION_ID = '5b1f3c52-0a7d-4c1e-9f6e-3b2d4a8e9c10'
const SENTENCE = 'I want to organise a birthday for 40 people in Kinshasa'

const mission = (extra: Record<string, unknown> = {}) => ({
  id: MISSION_ID, type: 'EVENT', title: 'Birthday', objective: SENTENCE, status: 'draft', location: 'Kinshasa',
  start_date: null, end_date: null, participants: 40, budget_minor: 0, currency: '', preferences: {},
  created_at: '2026-10-06T09:00:00Z', updated_at: '2026-10-06T09:00:00Z', ...extra,
})

const plan = (extra: Record<string, unknown> = {}) => ({
  mission_id: MISSION_ID, mission_type: 'EVENT', plannable: true, needs_information: false, plan_version: 0, status: 'none', steps: [], versions: [], ...extra,
})

const planStep = (position: number, type: string, title: string, extra: Record<string, unknown> = {}) => ({
  id: `p${position}`, mission_id: MISSION_ID, type, title, description: '', status: 'pending', position, action_type: 'SEARCH_LISTINGS',
  action_support: 'supported', plan_version: 1, action_ref: '', result_ref: '', ...extra,
})

const EVENT_STEPS = [
  planStep(1, 'FIND_VENUE', 'Find a venue'), planStep(2, 'FIND_CATERING', 'Find catering'), planStep(3, 'FIND_DECORATION', 'Find decoration'),
  planStep(4, 'FIND_ENTERTAINMENT', 'Find entertainment'), planStep(5, 'FIND_PHOTOGRAPHY', 'Find a photographer'), planStep(6, 'FIND_TRANSPORT', 'Find transport'),
]

const currentPlan = (extra: Record<string, unknown> = {}) => plan({ plan_version: 1, status: 'current', plan_mission_type: 'EVENT', planner: 'rules', steps: EVENT_STEPS, versions: [1], ...extra })

const base = () => ({
  'GET /site/content': () => ({ body: { trust: [], partners: [], sections: [] } }),
  'GET /auth/users/me': () => ({ body: me }),
})

beforeEach(() => { resetVerticals(); resetSiteContent() })
afterEach(() => vi.unstubAllGlobals())

describe('Organiser: the question', () => {
  it('asks what you want to organise, conversation-first, with sentences to start from', async () => {
    mockApi(base())
    renderAt('/organize')
    expect(await screen.findByRole('heading', { level: 1, name: 'What do you want to organise?' })).toBeInTheDocument()
    expect(screen.getByLabelText('I want to organise…')).toBeInTheDocument()
    const ideas = screen.getByRole('list', { name: 'Where to start' })
    expect(within(ideas).getAllByRole('button').map((b) => b.textContent)).toEqual(['A trip', 'A party', 'A stay', 'A move', 'Transport', 'An event'])
    expect(document.title).toBe('Organise — Trezutil')
  })

  it('turns a suggestion into the start of your own sentence, not a search', async () => {
    const { calls } = mockApi(base())
    renderAt('/organize')
    await userEvent.click(await screen.findByRole('button', { name: 'A party' }))
    expect(screen.getByLabelText('I want to organise…')).toHaveValue('I want to organise a party for ')
    expect(calls.some((c) => c.path.startsWith('/search') || c.path.startsWith('/missions'))).toBe(false)
  })

  it('is reachable from the primary menu, which opens its entries, and from the legacy spelling', async () => {
    mockApi({ ...base(), 'POST /search': () => ({ body: { items: [], total: 0, has_next: false } }) })
    renderAt('/discover')
    const nav = await screen.findByRole('navigation', { name: 'Primary' })
    await userEvent.click(within(nav).getByRole('button', { name: 'Organise' }))
    await userEvent.click(within(nav).getByRole('link', { name: /Another project/ }))
    expect(await screen.findByRole('heading', { level: 1, name: 'What do you want to organise?' })).toBeInTheDocument()
  })

  it('accepts a sentence from the home page (?q=) and keeps it out of the address', async () => {
    mockApi(base())
    renderAt('/organise')
    expect(await screen.findByRole('heading', { level: 1, name: 'What do you want to organise?' })).toBeInTheDocument()
    renderAt(`/organize?q=${encodeURIComponent(SENTENCE)}`)
    await waitFor(() => expect(screen.getAllByLabelText('I want to organise…').some((f) => (f as HTMLTextAreaElement).value === SENTENCE)).toBe(true))
  })
})

describe('Organiser: creating a draft mission', () => {
  it('asks a guest to sign in first and keeps the sentence for after', async () => {
    const { calls } = mockApi(base())
    renderAt('/organize')
    await userEvent.type(await screen.findByLabelText('I want to organise…'), SENTENCE)
    await userEvent.click(screen.getByRole('button', { name: /Hand it to Trezutil/ }))
    expect(await screen.findByRole('heading', { name: /Sign in|Welcome|Continue/i })).toBeInTheDocument()
    expect(sessionStorage.getItem('trezutil_organise_draft')).toBe(SENTENCE)
    expect(calls.some((c) => c.path.startsWith('/missions'))).toBe(false)
  })

  it('creates a draft, shows what was understood and starts nothing', async () => {
    signIn()
    const { calls } = mockApi({
      ...base(),
      'GET /missions': () => ({ body: { items: [], total: 0, page: 1, page_size: 20 } }),
      'POST /missions': () => ({ status: 201, body: mission() }),
      [`GET /missions/${MISSION_ID}`]: () => ({ body: mission() }),
      [`GET /missions/${MISSION_ID}/plan`]: () => ({ body: plan() }),
      [`GET /missions/${MISSION_ID}/steps`]: () => ({ body: { items: [], total: 0 } }),
    })
    renderAt('/organize')
    expect(await screen.findByText('Nothing organised yet. What you hand over will be kept here.')).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('I want to organise…'), SENTENCE)
    await userEvent.click(screen.getByRole('button', { name: /Hand it to Trezutil/ }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Birthday' })).toBeInTheDocument()
    expect(screen.getByText('Kinshasa · 40 people', { selector: '.mission-where' })).toBeInTheDocument()
    expect(screen.getByText('Organisation in preparation')).toBeInTheDocument()
    const understood = screen.getByRole('region', { name: 'Here is what I understood.' })
    expect(within(understood).getByText(SENTENCE)).toBeInTheDocument()
    expect(within(understood).getByText('Kinshasa')).toBeInTheDocument()
    expect(within(understood).getByText('40 people')).toBeInTheDocument()
    expect(within(understood).getByText('Event')).toBeInTheDocument()
    expect(screen.getByText('I can help you organise this, step by step.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Prepare the plan/ })).toBeInTheDocument() // offered, never started for you
    expect(screen.queryByRole('list', { name: 'Your organisation' })).not.toBeInTheDocument()
    expect(screen.queryByRole('list', { name: 'The parts of this mission' })).not.toBeInTheDocument()

    const post = calls.find((c) => c.method === 'POST' && c.path === '/missions')!
    expect(post.body).toEqual({ objective: SENTENCE })
    expect(post.headers['Idempotency-Key']).toBeTruthy()
    // Nothing else ran: no search, no order, no reservation, no payment, no conversation action.
    expect(calls.filter((c) => c.method !== 'GET' && c.path !== '/missions')).toEqual([])
    expect(calls.some((c) => /^\/(search|orders|reservations|payments|conversations|inventory)/.test(c.path))).toBe(false)
  })

  it('shows the error, keeps the sentence and retries with the same key so one mission results', async () => {
    signIn()
    let attempts = 0
    const { calls } = mockApi({
      ...base(),
      'GET /missions': () => ({ body: { items: [], total: 0 } }),
      'POST /missions': () => (++attempts === 1 ? { status: 500, body: { error: { code: 'INTERNAL_ERROR', message: 'boom' } } } : { status: 201, body: mission() }),
      [`GET /missions/${MISSION_ID}`]: () => ({ body: mission() }),
      [`GET /missions/${MISSION_ID}/plan`]: () => ({ body: plan() }),
      [`GET /missions/${MISSION_ID}/steps`]: () => ({ body: { items: [], total: 0 } }),
    })
    renderAt('/organize')
    await userEvent.type(await screen.findByLabelText('I want to organise…'), SENTENCE)
    await userEvent.click(screen.getByRole('button', { name: /Hand it to Trezutil/ }))
    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText('Something went wrong on our side')).toBeInTheDocument()
    expect(screen.getByLabelText('I want to organise…')).toHaveValue(SENTENCE)

    await userEvent.click(within(alert).getByRole('button', { name: /Try again/ }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Birthday' })).toBeInTheDocument()
    const posts = calls.filter((c) => c.method === 'POST' && c.path === '/missions')
    expect(posts).toHaveLength(2)
    expect(posts[0]!.headers['Idempotency-Key']).toBe(posts[1]!.headers['Idempotency-Key'])
  })

  it('lists your missions and cannot send an empty sentence', async () => {
    signIn()
    mockApi({
      ...base(),
      'GET /missions': () => ({ body: { items: [mission(), mission({ id: 'b2', title: 'Trip', type: 'TRAVEL', status: 'active', location: '', participants: 0 })], total: 2 } }),
    })
    renderAt('/organize')
    const list = await screen.findByRole('heading', { name: 'Your missions' })
    await waitFor(() => expect(within(list.parentElement!).getAllByRole('link')).toHaveLength(2)) // the heading shows while the list loads
    const links = within(list.parentElement!).getAllByRole('link')
    expect(links[0]).toHaveAttribute('href', `/organize/${MISSION_ID}`)
    expect(within(list.parentElement!).getByText('Under way')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Hand it to Trezutil/ })).toBeDisabled()
  })
})

describe('Organiser: the mission page', () => {
  const served = (extra: Record<string, unknown> = {}, steps: unknown[] = []) => {
    signIn()
    return mockApi({
      ...base(),
      [`GET /missions/${MISSION_ID}`]: () => ({ body: mission(extra) }),
      [`GET /missions/${MISSION_ID}/plan`]: () => ({ body: plan() }),
      [`GET /missions/${MISSION_ID}/steps`]: () => ({ body: { items: steps, total: steps.length } }),
    })
  }

  it('survives a reload: the mission is read from the server by its address', async () => {
    served({ start_date: '2027-03-05', end_date: '2027-03-06', budget_minor: 250000, currency: 'USD' })
    renderAt(`/organize/${MISSION_ID}`)
    expect(await screen.findByRole('heading', { level: 1, name: 'Birthday' })).toBeInTheDocument()
    const understood = screen.getByRole('region', { name: 'Here is what I understood.' })
    expect(within(understood).getByText(/5 March 2027 to 6 March 2027/)).toBeInTheDocument()
    expect(within(understood).getByText(/\$2,500|2,500/)).toBeInTheDocument()
  })

  it('shows the recorded parts in order, without a button that runs anything', async () => {
    served({ status: 'active' }, [
      { id: 's1', mission_id: MISSION_ID, type: 'VENUE', title: 'Find a venue', description: 'For 40 people', status: 'pending', position: 1, action_type: '' },
      { id: 's2', mission_id: MISSION_ID, type: 'CATERING', title: 'Choose a caterer', description: '', status: 'pending', position: 2, action_type: '' },
    ])
    renderAt(`/organize/${MISSION_ID}`)
    const steps = await screen.findByRole('list', { name: 'The parts of this mission' })
    expect(within(steps).getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)).toEqual(['Find a venue', 'Choose a caterer'])
    expect(within(steps).queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByText('Under way')).toBeInTheDocument()
  })

  it('still shows the mission when its parts cannot be read', async () => {
    signIn()
    mockApi({
      ...base(),
      [`GET /missions/${MISSION_ID}`]: () => ({ body: mission() }),
      [`GET /missions/${MISSION_ID}/plan`]: () => ({ status: 500, body: { error: { code: 'X', message: 'x' } } }),
      [`GET /missions/${MISSION_ID}/steps`]: () => ({ status: 500, body: { error: { code: 'X', message: 'x' } } }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    expect(await screen.findByRole('heading', { level: 1, name: 'Birthday' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Prepare the plan/ })).toBeInTheDocument() // an event can still be planned
  })

  it('says so when the mission belongs to somebody else or does not exist', async () => {
    signIn()
    mockApi({ ...base(), [`GET /missions/${MISSION_ID}`]: () => ({ status: 403, body: { error: { code: 'FORBIDDEN', message: 'no' } } }) })
    renderAt(`/organize/${MISSION_ID}`)
    expect(await screen.findByText('You don’t have access')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { level: 1, name: 'Birthday' })).not.toBeInTheDocument()
  })

  it('offers a way back when it is not found', async () => {
    signIn()
    mockApi({ ...base(), [`GET /missions/${MISSION_ID}`]: () => ({ status: 404, body: { error: { code: 'NOT_FOUND', message: 'Mission not found' } } }) })
    renderAt(`/organize/${MISSION_ID}`)
    expect(await screen.findByText('We couldn’t find that')).toBeInTheDocument()
  })

  it('asks a guest to sign in rather than showing an empty page', async () => {
    mockApi(base())
    renderAt(`/organize/${MISSION_ID}`)
    expect((await screen.findAllByText(/Sign in/)).length).toBeGreaterThan(0) // sent to the sign-in page, which says so
    expect(screen.queryByRole('heading', { level: 1, name: 'Birthday' })).not.toBeInTheDocument()
  })

  it('links back to organise something else', async () => {
    served()
    renderAt(`/organize/${MISSION_ID}`)
    expect(await screen.findByRole('link', { name: /Organise something else/ })).toHaveAttribute('href', '/organize')
  })
})

describe('Organiser: the plan', () => {
  const PLAN_PATH = `/missions/${MISSION_ID}/plan`
  const serve = (routes: Parameters<typeof mockApi>[0] = {}, missionExtra: Record<string, unknown> = {}) => {
    signIn()
    return mockApi({
      ...base(),
      [`GET /missions/${MISSION_ID}`]: () => ({ body: mission(missionExtra) }),
      [`GET /missions/${MISSION_ID}/steps`]: () => ({ body: { items: [], total: 0 } }),
      ...routes,
    })
  }

  it('prepares the plan only when asked, and shows the parts to be defined', async () => {
    let planned = false
    const { calls } = serve({
      [`GET ${PLAN_PATH}`]: () => ({ body: planned ? currentPlan() : plan() }),
      [`POST ${PLAN_PATH}`]: () => { planned = true; return { status: 201, body: currentPlan() } },
    })
    renderAt(`/organize/${MISSION_ID}`)
    const button = await screen.findByRole('button', { name: /Prepare the plan/ })
    expect(calls.some((c) => c.method === 'POST')).toBe(false) // opening the mission does not plan it
    await userEvent.click(button)

    const list = await screen.findByRole('list', { name: 'Your organisation' })
    const items = within(list).getAllByRole('listitem')
    expect(items.map((li) => li.querySelector('strong')?.textContent)).toEqual(['Venue', 'Catering', 'Decoration', 'Entertainment', 'Photography', 'Transport'])
    expect(items.every((li) => within(li).getByText('To be defined'))).toBe(true)
    expect(screen.getByRole('heading', { level: 2, name: 'Your organisation' })).toBeInTheDocument()
    expect(screen.getByText('Each part is still to be defined. Nothing is searched, booked or ordered yet.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Prepare the plan/ })).not.toBeInTheDocument()
    expect(within(list).queryByRole('button')).not.toBeInTheDocument() // nothing here runs anything

    const post = calls.find((c) => c.method === 'POST')!
    expect(post.path).toBe(PLAN_PATH)
    expect(post.body).toEqual({})
    expect(post.headers['Idempotency-Key']).toBeTruthy()
    // Planning ran no search, order, reservation, payment or conversation action.
    expect(calls.some((c) => /^\/(search|orders|reservations|payments|conversations|inventory|availability)/.test(c.path))).toBe(false)
  })

  it('shows a plan that already exists, without preparing it again, after a reload', async () => {
    const { calls } = serve({ [`GET ${PLAN_PATH}`]: () => ({ body: currentPlan() }) })
    renderAt(`/organize/${MISSION_ID}`)
    expect(await screen.findByRole('list', { name: 'Your organisation' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Prepare/ })).not.toBeInTheDocument()
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
  })

  it('writes the plan in French, from the type of each step', async () => {
    const { setLang } = await import('@/lib/i18n')
    setLang('fr', false)
    serve({ [`GET ${PLAN_PATH}`]: () => ({ body: currentPlan() }) })
    renderAt(`/organize/${MISSION_ID}`)
    const list = await screen.findByRole('list', { name: 'Ton organisation' })
    expect(within(list).getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)).toEqual(['Lieu', 'Traiteur', 'Décoration', 'Animation', 'Photographie', 'Transport'])
    expect(within(list).getAllByText('À définir')).toHaveLength(6)
    expect(screen.getByRole('heading', { level: 2, name: 'Ton organisation' })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Voici ce que j’ai compris.' })).toBeInTheDocument()
  })

  it('names a step the server sent that the page does not know by its own title', async () => {
    serve({ [`GET ${PLAN_PATH}`]: () => ({ body: currentPlan({ steps: [planStep(1, 'FIND_SOMETHING_NEW', 'Find a pastry chef')] }) }) })
    renderAt(`/organize/${MISSION_ID}`)
    expect(await screen.findByText('Find a pastry chef')).toBeInTheDocument()
  })

  it('says so when a step needs an action the Commerce OS does not have yet', async () => {
    serve({ [`GET ${PLAN_PATH}`]: () => ({ body: currentPlan({ steps: [planStep(1, 'FIND_VENUE', 'Find a venue', { action_support: 'not_yet_supported' }), planStep(2, 'FIND_CATERING', 'Find catering')] }) }) })
    renderAt(`/organize/${MISSION_ID}`)
    const list = await screen.findByRole('list', { name: 'Your organisation' })
    const [venue, catering] = within(list).getAllByRole('listitem')
    expect(within(venue!).getByText('Not available yet')).toBeInTheDocument()
    expect(within(catering!).queryByText('Not available yet')).not.toBeInTheDocument()
  })

  it('shows the error, keeps the offer, and retries with the same key so one plan results', async () => {
    let attempts = 0
    const { calls } = serve({
      [`GET ${PLAN_PATH}`]: () => ({ body: plan() }),
      [`POST ${PLAN_PATH}`]: () => (++attempts === 1 ? { status: 500, body: { error: { code: 'INTERNAL_ERROR', message: 'boom' } } } : { status: 201, body: currentPlan() }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: /Prepare the plan/ }))
    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText('Something went wrong on our side')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Prepare the plan/ })).toBeEnabled()
    await userEvent.click(within(alert).getByRole('button', { name: /Try again/ }))
    expect(await screen.findByRole('list', { name: 'Your organisation' })).toBeInTheDocument()
    const posts = calls.filter((c) => c.method === 'POST')
    expect(posts).toHaveLength(2)
    expect(posts[0]!.headers['Idempotency-Key']).toBe(posts[1]!.headers['Idempotency-Key'])
  })

  it('asks for the kind when the goal could not be recognised, then offers the plan', async () => {
    let kindSet = false
    const { calls } = serve({
      [`GET /missions/${MISSION_ID}`]: () => ({ body: mission({ type: kindSet ? 'TRAVEL' : 'OTHER', title: 'Something', location: '', participants: 0 }) }),
      [`GET ${PLAN_PATH}`]: () => ({ body: plan({ mission_type: kindSet ? 'TRAVEL' : 'OTHER', plannable: kindSet }) }),
      [`PATCH /missions/${MISSION_ID}`]: () => { kindSet = true; return { body: mission({ type: 'TRAVEL', title: 'Something', location: '', participants: 0 }) } },
    }, { type: 'OTHER', title: 'Something', location: '', participants: 0 })
    renderAt(`/organize/${MISSION_ID}`)
    expect(await screen.findByText('I do not have a plan for this kind of goal yet.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Prepare the plan/ })).not.toBeInTheDocument()
    const use = screen.getByRole('button', { name: 'Use this kind' })
    expect(use).toBeDisabled()
    const select = screen.getByLabelText('What kind of organisation?')
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual(['Choose…', 'Event', 'Trip', 'Stay', 'Move', 'Transport'])
    await userEvent.selectOptions(select, 'TRAVEL')
    await userEvent.click(use)
    expect(await screen.findByRole('button', { name: /Prepare the plan/ })).toBeInTheDocument()
    expect(calls.find((c) => c.method === 'PATCH')!.body).toEqual({ type: 'TRAVEL' })
    expect(calls.some((c) => c.method === 'POST')).toBe(false) // choosing a kind does not plan either
  })

  it('offers a new plan when the mission changed kind after it was planned, and asks to replace', async () => {
    const { calls } = serve({
      [`GET ${PLAN_PATH}`]: () => ({ body: currentPlan({ mission_type: 'TRAVEL', plan_mission_type: 'EVENT' }) }),
      [`POST ${PLAN_PATH}`]: () => ({ status: 201, body: currentPlan({ plan_version: 2, versions: [1, 2], mission_type: 'TRAVEL', plan_mission_type: 'TRAVEL', steps: [planStep(1, 'FIND_TRANSPORT', 'Find transport', { plan_version: 2 })] }) }),
    }, { type: 'TRAVEL' })
    renderAt(`/organize/${MISSION_ID}`)
    expect(await screen.findByText('This plan was prepared for another kind of goal.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Prepare a new plan' }))
    await waitFor(() => expect(screen.queryByText('This plan was prepared for another kind of goal.')).not.toBeInTheDocument())
    expect(calls.find((c) => c.method === 'POST')!.body).toEqual({ replace: true })
    expect(within(screen.getByRole('list', { name: 'Your organisation' })).getAllByRole('listitem')).toHaveLength(1)
  })

  it('does not offer a new plan while the plan still matches the mission', async () => {
    serve({ [`GET ${PLAN_PATH}`]: () => ({ body: currentPlan() }) })
    renderAt(`/organize/${MISSION_ID}`)
    await screen.findByRole('list', { name: 'Your organisation' })
    expect(screen.queryByRole('button', { name: 'Prepare a new plan' })).not.toBeInTheDocument()
  })

  it('shows what the server refused, in the server’s words', async () => {
    serve({
      [`GET ${PLAN_PATH}`]: () => ({ body: plan() }),
      [`POST ${PLAN_PATH}`]: () => ({ status: 409, body: { error: { code: 'MISSION_CLOSED', message: 'The mission is closed' } } }),
    }, { status: 'cancelled' })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: /Prepare the plan/ }))
    expect(await screen.findByText('The mission is closed')).toBeInTheDocument()
  })

  it('keeps the steps added by hand apart from the plan', async () => {
    serve({
      [`GET ${PLAN_PATH}`]: () => ({ body: currentPlan() }),
      [`GET /missions/${MISSION_ID}/steps`]: () => ({ body: { items: [
        { id: 'h1', mission_id: MISSION_ID, type: 'OTHER', title: 'My own step', description: '', status: 'pending', position: 1, action_type: '', action_support: '', plan_version: 0 },
        ...EVENT_STEPS,
      ], total: 7 } }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    const plan = await screen.findByRole('list', { name: 'Your organisation' })
    expect(within(plan).getAllByRole('listitem')).toHaveLength(6)
    const own = screen.getByRole('list', { name: 'The parts of this mission' })
    expect(within(own).getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)).toEqual(['My own step'])
  })
})

describe('Organiser: running a step', () => {
  const PLAN_PATH = `/missions/${MISSION_ID}/plan`
  const runnable = (position: number, extra: Record<string, unknown> = {}) =>
    planStep(position, 'FIND_ACCOMMODATION', 'Find accommodation', { id: `p${position}`, executable: true, can_execute: true, stale: false, ...extra })
  const quiet = () => planStep(9, 'FIND_ACTIVITIES', 'Find activities', { id: 'p9', executable: false, can_execute: false, stale: false })
  const found = (total: number, id = 'e1') => ({ id, attempt: 1, status: 'completed', action_type: 'SEARCH_LISTINGS', started_at: '2026-10-06T09:00:00Z', result: { reference: id, total, listing_ids: ['a', 'b', 'c'].slice(0, total) } })
  const notFound = { id: 'e0', attempt: 1, status: 'failed', action_type: 'SEARCH_LISTINGS', error: { code: 'ACTION_FAILED' } }
  const answer = (stepId: string, execution: Record<string, unknown>, stepExtra: Record<string, unknown> = {}, extra: Record<string, unknown> = {}) => ({
    ...execution, mission_id: MISSION_ID, step_id: stepId, execution_id: execution.id, replayed: false, step_status: execution.status === 'completed' ? 'completed' : 'failed',
    step: runnable(1, { id: stepId, status: execution.status === 'completed' ? 'completed' : 'failed', can_execute: execution.status !== 'completed', execution, ...stepExtra }), ...extra,
  })
  const serve = (steps: unknown[], routes: Parameters<typeof mockApi>[0] = {}, missionExtra: Record<string, unknown> = {}) => {
    signIn()
    return mockApi({
      ...base(),
      [`GET /missions/${MISSION_ID}`]: () => ({ body: mission(missionExtra) }),
      [`GET /missions/${MISSION_ID}/steps`]: () => ({ body: { items: [], total: 0 } }),
      [`GET ${PLAN_PATH}`]: () => ({ body: currentPlan({ steps }) }),
      ...routes,
    })
  }

  it('offers to run a step that can be run, and only that one', async () => {
    serve([runnable(1), quiet()])
    renderAt(`/organize/${MISSION_ID}`)
    const list = await screen.findByRole('list', { name: 'Your organisation' })
    const [stay, activities] = within(list).getAllByRole('listitem')
    expect(within(stay!).getByText('Ready to start')).toBeInTheDocument()
    expect(within(stay!).getByRole('button', { name: /Execute/ })).toBeEnabled()
    expect(within(activities!).getByText('To be defined')).toBeInTheDocument()
    expect(within(activities!).queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByText('Execute a part when you are ready: it only searches, and nothing is booked or ordered.')).toBeInTheDocument()
  })

  it('runs one step on request: searching, then done with how many were found', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    const { calls } = serve([runnable(1), quiet()], {
      [`POST /missions/${MISSION_ID}/steps/p1/execute`]: async () => { await gate; return { status: 201, body: answer('p1', found(3)) } },
    })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: /Execute/ }))
    expect(await screen.findByText('Searching…')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Execute/ })).not.toBeInTheDocument()
    release()
    expect(await screen.findByText('✓ Done')).toBeInTheDocument()
    expect(screen.getByText('3 results found')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Execute|Retry/ })).not.toBeInTheDocument()

    const writes = calls.filter((c) => c.method !== 'GET')
    expect(writes).toHaveLength(1)
    expect(writes[0]!.path).toBe(`/missions/${MISSION_ID}/steps/p1/execute`)
    expect(writes[0]!.headers['Idempotency-Key']).toBeTruthy()
    expect(writes[0]!.body).toEqual({}) // the page says nothing about what to run: the server decides
    // Nothing else was asked of the platform: no search, order, reservation or payment from the page.
    expect(calls.some((c) => /^\/(search|orders|reservations|payments|conversations|inventory|availability)/.test(c.path))).toBe(false)
    // The mission is not changed by running a step.
    expect(calls.some((c) => c.method === 'PATCH')).toBe(false)
    expect(screen.getByText('Organisation in preparation')).toBeInTheDocument()
  })

  it('says when nothing was found', async () => {
    serve([runnable(1)], { [`POST /missions/${MISSION_ID}/steps/p1/execute`]: () => ({ status: 201, body: answer('p1', found(0)) }) })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: /Execute/ }))
    expect(await screen.findByText('✓ Done')).toBeInTheDocument()
    expect(screen.getByText('0 results found')).toBeInTheDocument()
  })

  it('runs one step at a time', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    serve([runnable(1), runnable(2)], {
      [`POST /missions/${MISSION_ID}/steps/p1/execute`]: async () => { await gate; return { status: 201, body: answer('p1', found(1)) } },
    })
    renderAt(`/organize/${MISSION_ID}`)
    const buttons = await screen.findAllByRole('button', { name: /Execute/ })
    await userEvent.click(buttons[0]!)
    await screen.findByText('Searching…')
    expect(screen.getByRole('button', { name: /Execute/ })).toBeDisabled() // the other step waits
    release()
    await screen.findByText('✓ Done')
    expect(screen.getByRole('button', { name: /Execute/ })).toBeEnabled()
  })

  it('says a step could not be completed and retries it with a new request', async () => {
    let n = 0
    const { calls } = serve([runnable(1)], {
      [`POST /missions/${MISSION_ID}/steps/p1/execute`]: () => (++n === 1 ? { status: 201, body: answer('p1', notFound) } : { status: 201, body: answer('p1', found(2, 'e2')) }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: /Execute/ }))
    expect(await screen.findByText('Unable to complete this step.')).toBeInTheDocument()
    expect(screen.queryByText('✓ Done')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Retry/ }))
    expect(await screen.findByText('✓ Done')).toBeInTheDocument()
    expect(screen.getByText('2 results found')).toBeInTheDocument()
    const posts = calls.filter((c) => c.method === 'POST')
    expect(posts).toHaveLength(2)
    // The server answered the first request, so the retry is a new execution, not a replay of the failed one.
    expect(posts[0]!.headers['Idempotency-Key']).not.toBe(posts[1]!.headers['Idempotency-Key'])
  })

  it('retries a request that got no answer with the same key, so the step cannot run twice', async () => {
    let n = 0
    const { calls } = serve([runnable(1)], {
      [`POST /missions/${MISSION_ID}/steps/p1/execute`]: () => (++n === 1 ? { status: 500, body: { error: { code: 'INTERNAL_ERROR', message: 'boom' } } } : { status: 200, body: answer('p1', found(3), {}, { replayed: true }) }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: /Execute/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong on our side')
    expect(screen.getByRole('button', { name: /Execute/ })).toBeEnabled()
    await userEvent.click(screen.getByRole('button', { name: /Execute/ }))
    expect(await screen.findByText('✓ Done')).toBeInTheDocument()
    const posts = calls.filter((c) => c.method === 'POST')
    expect(posts).toHaveLength(2)
    expect(posts[0]!.headers['Idempotency-Key']).toBe(posts[1]!.headers['Idempotency-Key'])
  })

  it('starts over with a new key after the server refused the request', async () => {
    let n = 0
    const { calls } = serve([runnable(1)], {
      [`POST /missions/${MISSION_ID}/steps/p1/execute`]: () => (++n === 1 ? { status: 422, body: { error: { code: 'STEP_NOT_EXECUTABLE', message: 'This step cannot be executed yet' } } } : { status: 201, body: answer('p1', found(1)) }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: /Execute/ }))
    expect(await screen.findByText('This step cannot be executed yet')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Execute/ }))
    await screen.findByText('✓ Done')
    const posts = calls.filter((c) => c.method === 'POST')
    expect(posts[0]!.headers['Idempotency-Key']).not.toBe(posts[1]!.headers['Idempotency-Key'])
  })

  it('reads the stored state again when the server says the step is already running', async () => {
    const { calls } = serve([runnable(1)], {
      [`POST /missions/${MISSION_ID}/steps/p1/execute`]: () => ({ status: 409, body: { error: { code: 'STEP_IN_PROGRESS', message: 'The step is already being executed' } } }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: /Execute/ }))
    expect(await screen.findByText('The step is already being executed')).toBeInTheDocument()
    await waitFor(() => expect(calls.filter((c) => c.method === 'GET' && c.path === PLAN_PATH).length).toBeGreaterThan(1))
  })

  it('shows what is stored when the page is opened: done, failed, running, stuck', async () => {
    serve([
      runnable(1, { status: 'completed', can_execute: false, execution: found(3) }),
      runnable(2, { status: 'failed', can_execute: true, execution: notFound }),
      runnable(3, { status: 'in_progress', can_execute: false, stale: false, execution: { id: 'e3', attempt: 1, status: 'in_progress' } }),
      runnable(4, { status: 'in_progress', can_execute: true, stale: true, execution: { id: 'e4', attempt: 1, status: 'in_progress' } }),
    ])
    renderAt(`/organize/${MISSION_ID}`)
    const list = await screen.findByRole('list', { name: 'Your organisation' })
    const [done, failedStep, running, stuck] = within(list).getAllByRole('listitem')
    expect(within(done!).getByText('✓ Done')).toBeInTheDocument()
    expect(within(done!).getByText('3 results found')).toBeInTheDocument()
    expect(within(done!).queryByRole('button')).not.toBeInTheDocument()
    expect(within(failedStep!).getByText('Unable to complete this step.')).toBeInTheDocument()
    expect(within(failedStep!).getByRole('button', { name: /Retry/ })).toBeInTheDocument()
    expect(within(running!).getByText('Searching…')).toBeInTheDocument()
    expect(within(running!).queryByRole('button')).not.toBeInTheDocument()
    expect(within(stuck!).getByText('Unable to complete this step.')).toBeInTheDocument()
    expect(within(stuck!).getByRole('button', { name: /Retry/ })).toBeInTheDocument()
  })

  it('offers nothing to run while the mission does not allow it', async () => {
    serve([runnable(1, { can_execute: false })], {}, { status: 'paused' })
    renderAt(`/organize/${MISSION_ID}`)
    const list = await screen.findByRole('list', { name: 'Your organisation' })
    expect(within(list).getByText('Ready to start')).toBeInTheDocument()
    expect(within(list).queryByRole('button')).not.toBeInTheDocument()
  })

  it('writes the lifecycle in French', async () => {
    const { setLang } = await import('@/lib/i18n')
    setLang('fr', false)
    serve([runnable(1), runnable(2, { status: 'completed', can_execute: false, execution: found(3) }), runnable(3, { status: 'failed', can_execute: true, execution: notFound }), quiet()])
    renderAt(`/organize/${MISSION_ID}`)
    const list = await screen.findByRole('list', { name: 'Ton organisation' })
    const [ready, done, failedStep, quietStep] = within(list).getAllByRole('listitem')
    expect(within(ready!).getByText('Prêt à commencer')).toBeInTheDocument()
    expect(within(ready!).getByRole('button', { name: /Exécuter/ })).toBeInTheDocument()
    expect(within(done!).getByText('✓ Terminé')).toBeInTheDocument()
    expect(within(done!).getByText('3 résultats trouvés')).toBeInTheDocument()
    expect(within(failedStep!).getByText('Impossible de terminer cette étape.')).toBeInTheDocument()
    expect(within(failedStep!).getByRole('button', { name: /Réessayer/ })).toBeInTheDocument()
    expect(within(quietStep!).getByText('À définir')).toBeInTheDocument()
  })
})

describe('Organiser: running the whole mission', () => {
  const PLAN_PATH = `/missions/${MISSION_ID}/plan`
  const RUN = `POST /missions/${MISSION_ID}/run`
  const RESUME = `POST /missions/${MISSION_ID}/resume`
  const EXECUTION = `GET /missions/${MISSION_ID}/execution`

  // A stay: a search for accommodation, then a check of availability that waits for it.
  const search = (extra: Record<string, unknown> = {}) => planStep(1, 'FIND_ACCOMMODATION', 'Find accommodation', {
    id: 's1', executable: true, can_execute: true, stale: false, state: 'ready', depends_on: [], blocked_by: [], ...extra,
  })
  const check = (extra: Record<string, unknown> = {}) => planStep(2, 'CHECK_AVAILABILITY', 'Check availability', {
    id: 's2', action_type: 'CHECK_AVAILABILITY', executable: true, can_execute: false, stale: false, state: 'pending', reason: 'waiting', depends_on: ['s1'], blocked_by: [], ...extra,
  })
  const found = { id: 'e1', attempt: 1, status: 'completed', action_type: 'SEARCH_LISTINGS', started_at: '2026-10-06T09:00:00Z', result: { reference: 'e1', total: 3, listing_ids: ['a', 'b', 'c'] } }
  const checked = { id: 'e2', attempt: 1, status: 'completed', action_type: 'CHECK_AVAILABILITY', started_at: '2026-10-06T09:00:01Z', result: { reference: 'a', listing_id: 'a', available: true } }
  const failedAttempt = { id: 'e0', attempt: 1, status: 'failed', action_type: 'SEARCH_LISTINGS', error: { code: 'ACTION_FAILED' } }
  const done = (extra: Record<string, unknown> = {}) => search({ status: 'completed', state: 'completed', can_execute: false, execution: found, ...extra })
  const checkedDone = (extra: Record<string, unknown> = {}) => check({ status: 'completed', state: 'completed', reason: undefined, can_execute: false, execution: checked, ...extra })

  const state = (steps: unknown[], extra: Record<string, unknown> = {}) => ({
    mission_id: MISSION_ID, plan_version: 1, mission_status: 'active', outcome: 'needs_attention', busy: false,
    progress: { completed: (steps as { state: string }[]).filter((s) => s.state === 'completed').length, skipped: 0, total: steps.length }, ran: [], steps, ...extra,
  })

  const serve = (steps: unknown[], routes: Parameters<typeof mockApi>[0] = {}, missionExtra: Record<string, unknown> = {}) => {
    signIn()
    return mockApi({
      ...base(),
      [`GET /missions/${MISSION_ID}`]: () => ({ body: mission({ type: 'STAY', ...missionExtra }) }),
      [`GET /missions/${MISSION_ID}/steps`]: () => ({ body: { items: [], total: 0 } }),
      [`GET ${PLAN_PATH}`]: () => ({ body: currentPlan({ mission_type: 'STAY', plan_mission_type: 'STAY', steps }) }),
      ...routes,
    })
  }
  const items = async () => within(await screen.findByRole('list', { name: 'Your organisation' })).getAllByRole('listitem')

  it('shows where every part stands, what a waiting part waits for, and one button to run what is ready', async () => {
    serve([search(), check()])
    renderAt(`/organize/${MISSION_ID}`)
    const [first, second] = await items()
    expect(within(first!).getByText('Ready to start')).toBeInTheDocument()
    expect(within(second!).getByText('Waiting for Accommodation')).toBeInTheDocument() // named, never an identifier
    expect(within(second!).queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByText('0 of 2 done')).toBeInTheDocument()
    expect(screen.getByRole('progressbar', { name: 'Progress' })).toHaveAttribute('aria-valuenow', '0')
    expect(screen.getByRole('button', { name: 'Run everything' })).toBeEnabled()
    expect(screen.getByText(/nothing is confirmed without you/)).toBeInTheDocument()
    expect(first!.querySelector('.mission-step-mark')).toHaveTextContent('○')
  })

  it('runs everything on request, shows the progress and completes the mission', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    const { calls } = serve([search(), check()], {
      [RUN]: async () => { await gate; return { body: state([done(), checkedDone()], { mission_status: 'completed', outcome: 'completed', ran: [{ step_id: 's1' }, { step_id: 's2' }] }) } },
      [EXECUTION]: () => ({ body: state([done(), check({ state: 'ready', can_execute: true })]) }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: 'Run everything' }))
    expect(await screen.findByRole('button', { name: 'Running…' })).toBeDisabled()
    release()
    expect(await screen.findByText('Everything is done.')).toBeInTheDocument()
    expect(screen.getByText('2 of 2 done')).toBeInTheDocument()
    const [first, second] = await items()
    expect(first!.querySelector('.mission-step-mark')).toHaveTextContent('✓')
    expect(within(first!).getByText('3 results found')).toBeInTheDocument()
    expect(within(second!).getByText('Available')).toBeInTheDocument() // the check says whether it is free, not a count
    expect(screen.getByText('Done', { selector: '.mission-status' })).toBeInTheDocument() // the mission's own status follows
    expect(screen.queryByRole('button', { name: /Run everything|Resume|Execute|Retry/ })).not.toBeInTheDocument()

    const writes = calls.filter((c) => c.method !== 'GET')
    expect(writes.map((c) => `${c.method} ${c.path}`)).toEqual([`POST /missions/${MISSION_ID}/run`])
    expect(writes[0]!.body).toEqual({}) // the page says nothing about what to run: the server decides
    expect(calls.some((c) => /^\/(search|orders|reservations|payments|conversations|inventory|availability)/.test(c.path))).toBe(false)
  })

  it('says what failed, what it blocks, and resumes only on request', async () => {
    const failedSearch = search({ status: 'failed', state: 'failed', can_execute: true, execution: failedAttempt })
    const blocked = check({ state: 'blocked', reason: 'dependency_failed', blocked_by: ['s1'] })
    const { calls } = serve([failedSearch, blocked], {
      [RESUME]: () => ({ body: state([done({ id: 's1' }), checkedDone()], { mission_status: 'completed', outcome: 'completed' }) }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    const [first, second] = await items()
    expect(within(first!).getByText('Unable to complete this step.')).toBeInTheDocument()
    expect(first!.querySelector('.mission-step-mark')).toHaveTextContent('✕')
    expect(within(second!).getByText('Blocked: Accommodation did not work out.')).toBeInTheDocument()
    expect(within(second!).queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByText('Some parts need your attention.')).toBeInTheDocument()
    // A failed part is retried only when asked: the button says Resume, nothing ran by itself.
    expect(calls.filter((c) => c.method !== 'GET')).toHaveLength(0)
    expect(screen.queryByRole('button', { name: 'Run everything' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Resume' }))
    expect(await screen.findByText('Everything is done.')).toBeInTheDocument()
    expect(calls.filter((c) => c.method !== 'GET').map((c) => c.path)).toEqual([`/missions/${MISSION_ID}/resume`])
  })

  it('says why a part is blocked: a skipped part, a part waiting for the person, and nothing found to check', async () => {
    serve([
      search({ status: 'skipped', state: 'skipped', can_execute: false }),
      check({ id: 's2', state: 'blocked', reason: 'dependency_skipped', blocked_by: ['s1'] }),
      check({ id: 's3', position: 3, state: 'blocked', reason: 'needs_confirmation', depends_on: [] }),
      check({ id: 's4', position: 4, state: 'blocked', reason: 'needs_input', detail: 'listing', depends_on: [] }),
    ])
    renderAt(`/organize/${MISSION_ID}`)
    const [skipped, dependent, confirm, nothing] = await items()
    expect(within(skipped!).getByText('Skipped')).toBeInTheDocument()
    expect(within(dependent!).getByText('Blocked: Accommodation was skipped.')).toBeInTheDocument()
    expect(within(confirm!).getByText('Waiting for your confirmation.')).toBeInTheDocument()
    expect(within(confirm!).queryByRole('button')).not.toBeInTheDocument() // never confirmed on the person's behalf
    expect(within(nothing!).getByText('The search found nothing to check.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Run everything|Resume/ })).not.toBeInTheDocument()
  })

  it('asks for the dates a check needs, and saves them without running anything', async () => {
    const { calls } = serve([done(), check({ state: 'blocked', reason: 'needs_input', detail: 'dates', can_execute: false })], {
      [`PATCH /missions/${MISSION_ID}`]: () => ({ body: mission({ type: 'STAY', start_date: '2026-11-10', end_date: '2026-11-13' }) }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    const [, second] = await items()
    expect(within(second!).getByText('Add the dates to check availability.')).toBeInTheDocument()
    const save = within(second!).getByRole('button', { name: 'Save the dates' })
    expect(save).toBeDisabled() // nothing to save before a start date
    await userEvent.type(within(second!).getByLabelText('Start date'), '2026-11-10')
    await userEvent.type(within(second!).getByLabelText('End date'), '2026-11-13')
    await userEvent.click(save)
    await waitFor(() => expect(calls.some((c) => c.method === 'PATCH')).toBe(true))
    const patch = calls.find((c) => c.method === 'PATCH')!
    expect(patch.body).toEqual({ start_date: '2026-11-10', end_date: '2026-11-13' })
    expect(calls.some((c) => c.path.endsWith('/run') || c.path.endsWith('/resume') || c.path.endsWith('/execute'))).toBe(false)
  })

  it('tells the person when another run holds the mission, and follows it until it ends', async () => {
    let reads = 0
    const { calls } = serve([search(), check()], {
      [RUN]: () => ({ status: 202, body: state([search({ state: 'running', can_execute: false, status: 'in_progress' }), check()], { busy: true, outcome: 'in_progress' }) }),
      [EXECUTION]: () => {
        reads += 1
        return { body: reads < 2 ? state([search({ state: 'running', can_execute: false, status: 'in_progress' }), check()], { busy: true, outcome: 'in_progress' }) : state([done(), checkedDone()], { mission_status: 'completed', outcome: 'completed' }) }
      },
    })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: 'Run everything' }))
    expect(await screen.findByText('A run is already under way. This page updates by itself.')).toBeInTheDocument()
    expect(screen.getByText('In progress…')).toBeInTheDocument()
    expect(screen.getByText('Your organisation is under way…')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Run everything' })).not.toBeInTheDocument() // nothing to start while it runs
    // The page asks for the state by itself (and only reads), until the other run is over.
    expect(await screen.findByText('Everything is done.', undefined, { timeout: 6000 })).toBeInTheDocument()
    expect(screen.queryByText('A run is already under way. This page updates by itself.')).not.toBeInTheDocument()
    const writes = calls.filter((c) => c.method !== 'GET')
    expect(writes).toHaveLength(1) // the single request that was refused as busy; the polling never writes
    const settled = calls.length
    await new Promise((resolve) => setTimeout(resolve, 1800))
    expect(calls.length).toBe(settled) // and it stops asking once nothing is running
  }, 15000)

  it('shows what the server refused, and reads the stored state again when the mission has moved on', async () => {
    const { calls } = serve([search(), check()], {
      [RUN]: () => ({ status: 409, body: { error: { code: 'MISSION_NOT_RUNNABLE', message: 'The mission does not allow running steps now' } } }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    await userEvent.click(await screen.findByRole('button', { name: 'Run everything' }))
    expect(await screen.findByText('The mission does not allow running steps now')).toBeInTheDocument()
    await waitFor(() => expect(calls.filter((c) => c.method === 'GET' && c.path === PLAN_PATH).length).toBeGreaterThan(1))
  })

  it('offers nothing to run while the mission does not allow it', async () => {
    serve([search({ can_execute: false }), check()], {}, { status: 'paused' })
    renderAt(`/organize/${MISSION_ID}`)
    await items()
    expect(screen.queryByRole('button', { name: /Run everything|Resume|Execute|Retry/ })).not.toBeInTheDocument()
  })

  it('still runs a single part, one at a time, with a key of its own', async () => {
    const { calls } = serve([search(), check()], {
      [`POST /missions/${MISSION_ID}/steps/s1/execute`]: () => ({
        status: 201, body: { ...found, mission_id: MISSION_ID, step_id: 's1', execution_id: 'e1', replayed: false, step: search({ status: 'completed', state: 'completed', can_execute: false, execution: found }) },
      }),
    })
    renderAt(`/organize/${MISSION_ID}`)
    const [first] = await items()
    await userEvent.click(within(first!).getByRole('button', { name: /Execute/ }))
    expect(await screen.findByText('3 results found')).toBeInTheDocument()
    const writes = calls.filter((c) => c.method !== 'GET')
    expect(writes).toHaveLength(1)
    expect(writes[0]!.headers['Idempotency-Key']).toBeTruthy()
  })

  it('writes the progress in French', async () => {
    const { setLang } = await import('@/lib/i18n')
    setLang('fr', false)
    serve([
      search({ status: 'failed', state: 'failed', can_execute: true, execution: failedAttempt }),
      check({ state: 'blocked', reason: 'dependency_failed', blocked_by: ['s1'] }),
      check({ id: 's3', position: 3, depends_on: ['s1'] }),
    ])
    renderAt(`/organize/${MISSION_ID}`)
    const list = await screen.findByRole('list', { name: 'Ton organisation' })
    const [failedStep, blocked, waiting] = within(list).getAllByRole('listitem')
    expect(within(failedStep!).getByText('Impossible de terminer cette étape.')).toBeInTheDocument()
    expect(within(blocked!).getByText('Bloqué : « Hébergement » n’a pas abouti.')).toBeInTheDocument()
    expect(within(waiting!).getByText('En attente de « Hébergement »')).toBeInTheDocument()
    expect(screen.getByText('0 sur 3 terminées')).toBeInTheDocument()
    expect(screen.getByText('Certaines étapes demandent ton attention.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reprendre' })).toBeInTheDocument()
    expect(screen.getByRole('progressbar', { name: 'Progression' })).toBeInTheDocument()
  })

  it('leaves a plan from a server that sends no states exactly as it was', async () => {
    serve([planStep(1, 'FIND_ACCOMMODATION', 'Find accommodation', { executable: true, can_execute: true })])
    renderAt(`/organize/${MISSION_ID}`)
    const [first] = await items()
    expect(within(first!).getByText('Ready to start')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Run everything|Resume/ })).not.toBeInTheDocument()
  })
})

describe('Organiser: layout', () => {
  const css = readFileSync(join(process.cwd(), 'src/styles/organise.css'), 'utf8')

  it('is scoped to Organiser and uses the app tokens, so dark mode follows', () => {
    const selectors = [...css.matchAll(/^([^\s@/}{][^{]*)\{/gm)].map((m) => m[1]!.trim())
    expect(selectors.filter((s) => !/^\.(organise|mission)/.test(s))).toEqual([])
    expect(css).toMatch(/var\(--text\)/)
    expect(css.replace(/\/\*[\s\S]*?\*\//g, '')).not.toMatch(/gradient|backdrop-filter|box-shadow/)
  })

  it('adapts to a phone: the send button takes the full width and lists stack', () => {
    expect(css).toMatch(/@media \(max-width: 560px\)[\s\S]*\.organise-send \{ flex: 1 1 100%/)
    expect(css).toMatch(/prefers-reduced-motion/)
  })
})
