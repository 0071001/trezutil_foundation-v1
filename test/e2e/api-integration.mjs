#!/usr/bin/env node
// Real-stack integration journey for the web client's API surface (no mocks, no extra dependency).
//
//   node test/e2e/api-integration.mjs [gateway URL, default http://127.0.0.1:8080]
//
// Needs the local stack (deployments/docker/docker-compose.app.yml) with APP_ENV=development: sign-in codes are read
// from the identity-service log. It creates two test accounts and their data in that stack's database; never point it
// at production.
import { execSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'

const BASE = (process.argv[2] || 'http://127.0.0.1:8080').replace(/\/$/, '')
const COMPOSE = 'docker compose -f deployments/docker/docker-compose.app.yml --env-file deployments/docker/app.env'
const results = []

function record(name, ok, detail = '') {
  results.push({ name, ok, detail })
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${name}${detail ? ` — ${detail}` : ''}`)
}

async function api(method, path, { token, body, headers = {} } = {}) {
  const res = await fetch(`${BASE}/api/v1${path}`, {
    method,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...headers },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  let data = null
  try { data = await res.json() } catch { /* empty body */ }
  return { status: res.status, data }
}

async function check(name, fn) {
  try {
    const detail = await fn()
    record(name, true, typeof detail === 'string' ? detail : '')
  } catch (err) {
    record(name, false, err instanceof Error ? err.message : String(err))
  }
}

function expect(cond, message) { if (!cond) throw new Error(message) }

function otpFor(phone) {
  const log = execSync(`${COMPOSE} logs --tail 200 identity-service`, { encoding: 'utf8' })
  const codes = [...log.matchAll(new RegExp(`"phone":"\\${phone}","otp_code":"(\\d+)"`, 'g'))].map((m) => m[1])
  return codes.at(-1)
}

async function signIn(phone) {
  const challenge = await api('POST', '/auth/request-otp', { body: { phone } })
  expect(challenge.status === 200, `request-otp ${challenge.status}`)
  await new Promise((r) => setTimeout(r, 1200))
  const code = otpFor(phone)
  expect(code, 'no code in the identity-service log (APP_ENV=development?)')
  const session = await api('POST', '/auth/verify-otp', { body: { request_id: challenge.data.request_id, code } })
  expect(session.status === 200, `verify-otp ${session.status}`)
  return { token: session.data.tokens.access_token, refresh: session.data.tokens.refresh_token, user: session.data.user }
}

const run = async () => {
  const health = await fetch(`${BASE}/healthz`).then((r) => r.json()).catch(() => null)
  if (health?.status !== 'ok') { console.error(`Gateway not healthy at ${BASE}`); process.exit(2) }

  let a, b
  await check('1. Inscription et connexion (code à usage unique, compte créé au premier code)', async () => {
    // Fresh numbers on each run: the backend limits code requests per number (a 429 is the protection working).
    const base = 810000000 + Math.floor(Math.random() * 89999999)
    a = await signIn(`+243${base}`)
    b = await signIn(`+243${base + 1}`)
    expect(a.token && b.token && a.user.id !== b.user.id, 'two distinct sessions')
    return `utilisateurs ${a.user.id.slice(0, 8)}…, ${b.user.id.slice(0, 8)}…`
  })
  if (!a || !b) return

  await check('1b. Profil : lecture et mise à jour (PATCH /auth/users/me)', async () => {
    const upd = await api('PATCH', '/auth/users/me', { token: a.token, body: { display_name: 'Acheteur Test' } })
    expect(upd.status === 200 && upd.data.display_name === 'Acheteur Test', `PATCH ${upd.status}`)
    const me = await api('GET', '/auth/users/me', { token: a.token })
    expect(me.data.display_name === 'Acheteur Test', 'name not saved')
  })

  await check('1c. Rafraîchissement de session (POST /auth/refresh)', async () => {
    const r = await api('POST', '/auth/refresh', { body: { refresh_token: a.refresh } })
    expect(r.status === 200 && r.data.tokens?.access_token, `refresh ${r.status}`)
  })

  const sender = `web-${randomUUID()}`
  let convId = ''
  await check('2-3. Création d’une conversation et envoi d’un message', async () => {
    const r = await api('POST', '/conversations/message', { token: a.token, body: { message_text: 'Je cherche des hôtels à Gombe', channel: 'web', sender_external_id: sender, message_idempotency_key: randomUUID(), locale: 'fr' } })
    expect(r.status === 200 && r.data.conversation_id, `message ${r.status}`)
    convId = r.data.conversation_id
    return `réponse : « ${String(r.data.response?.text ?? '').slice(0, 60)}… »`
  })

  await check('2b. Message rejoué avec la même clé : exécuté une seule fois', async () => {
    const key = randomUUID()
    const body = { message_text: 'Des restaurants à proximité', channel: 'web', sender_external_id: sender, message_idempotency_key: key, locale: 'fr' }
    const first = await api('POST', '/conversations/message', { token: a.token, body })
    const again = await api('POST', '/conversations/message', { token: a.token, body })
    expect(first.status === 200 && again.status === 200, 'both answered')
    expect(again.data.response?.metadata?.duplicate === true, 'the replay must be flagged as a duplicate')
  })

  await check('4. Récupération de l’historique (GET /conversations, /messages)', async () => {
    const list = await api('GET', '/conversations', { token: a.token })
    expect(list.status === 200, `list ${list.status}`)
    const mine = list.data.items.find((c) => c.id === convId)
    expect(mine && mine.external_id === sender && mine.title === 'Je cherche des hôtels à Gombe', 'conversation listed with its title')
    const msgs = await api('GET', `/conversations/${convId}/messages`, { token: a.token })
    expect(msgs.status === 200 && msgs.data.items.length >= 2, `messages ${msgs.status}`)
    expect(msgs.data.items[0].direction === 'inbound', 'question first')
    return `${msgs.data.items.length} messages`
  })

  await check('4b. Isolation : un autre compte ne lit ni ne poursuit cette conversation', async () => {
    const read = await api('GET', `/conversations/${convId}/messages`, { token: b.token })
    expect(read.status === 403, `read by another user: ${read.status}`)
    const list = await api('GET', '/conversations', { token: b.token })
    expect(!list.data.items.some((c) => c.id === convId), 'listed for another user')
    const cont = await api('POST', '/conversations/message', { token: b.token, body: { message_text: 'Bonjour', channel: 'web', sender_external_id: sender, message_idempotency_key: randomUUID() } })
    expect(cont.status === 403, `continued by another user: ${cont.status}`)
  })

  await check('4c. Historique refusé sans session (401)', async () => {
    const r = await api('GET', '/conversations')
    expect(r.status === 401, `guest list ${r.status}`)
    const bad = await api('GET', '/conversations', { token: 'not-a-token' })
    expect(bad.status === 401, `invalid token ${bad.status}`)
  })

  await check('5-6. Recherche commerciale (POST /search, public)', async () => {
    const r = await api('POST', '/search', { body: { text: 'hôtel', page_size: 5 } })
    expect(r.status === 200, `search ${r.status}`)
    const items = r.data.items ?? r.data.Items ?? []
    return `${items.length} résultat(s) dans le catalogue local`
  })

  let missionId = ''
  await check('12. Création et suivi d’une mission (plan, démarrage, pause, reprise, arrêt)', async () => {
    const created = await api('POST', '/missions', { token: a.token, headers: { 'Idempotency-Key': randomUUID() }, body: { objective: 'Organiser une conférence de 80 personnes à Kinshasa', type: 'EVENT', title: 'Conférence', location: 'Kinshasa' } })
    expect(created.status === 201, `create ${created.status}`)
    missionId = created.data.id
    const plan = await api('POST', `/missions/${missionId}/plan`, { token: a.token, headers: { 'Idempotency-Key': randomUUID() }, body: {} })
    expect(plan.status === 201 || plan.status === 200, `plan ${plan.status}`)
    for (const status of ['active', 'paused', 'active', 'cancelled']) {
      const r = await api('PATCH', `/missions/${missionId}`, { token: a.token, body: { status } })
      expect(r.status === 200 && r.data.status === status, `→ ${status}: ${r.status}`)
    }
    const after = await api('PATCH', `/missions/${missionId}`, { token: a.token, body: { status: 'active' } })
    expect(after.status >= 400, 'a cancelled mission must not restart')
    return `${(plan.data.steps ?? []).length} étapes planifiées`
  })

  await check('12b. Isolation des missions', async () => {
    const r = await api('GET', `/missions/${missionId}`, { token: b.token })
    expect(r.status === 403 || r.status === 404, `other user: ${r.status}`)
  })

  await check('11. Commandes et réservations du compte', async () => {
    const o = await api('GET', '/orders', { token: a.token })
    const r = await api('GET', '/reservations', { token: a.token })
    expect(o.status === 200 && r.status === 200, `orders ${o.status}, reservations ${r.status}`)
    const guest = await api('GET', '/orders')
    expect(guest.status === 401, `guest orders ${guest.status}`)
  })

  await check('13. Crédits : réservés aux gestionnaires d’une organisation', async () => {
    const r = await api('GET', `/sellers/usage?organization_id=${randomUUID()}`, { token: a.token })
    expect(r.status === 403 || r.status === 404, `usage of a foreign organization: ${r.status}`)
  })

  await check('15. Déconnexion et expiration : un jeton invalide est refusé partout', async () => {
    for (const path of ['/auth/users/me', '/missions', '/orders']) {
      const r = await api('GET', path, { token: 'expired.token.value' })
      expect(r.status === 401, `${path}: ${r.status}`)
    }
  })

  const failed = results.filter((r) => !r.ok)
  console.log(`\n${results.length - failed.length}/${results.length} vérifications réussies`)
  process.exit(failed.length ? 1 : 0)
}

run().catch((err) => { console.error(err); process.exit(1) })
