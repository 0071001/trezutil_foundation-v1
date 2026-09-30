// Shared helpers for the browser journeys and captures.
import { readFileSync } from 'node:fs'

export const APP = process.env.E2E_APP ?? 'http://localhost:8080'
export const API = `${APP}/api/v1`
export const IDENTITY_LOG = process.env.E2E_IDENTITY_LOG

export const viewports = {
  mobile: { width: 390, height: 844 },
  tablet: { width: 820, height: 1180 },
  desktop: { width: 1440, height: 900 },
}

async function post(path, body, token) {
  const res = await fetch(`${API}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify(body),
  })
  return { status: res.status, body: await res.json().catch(() => null) }
}

/** Signed-in GET against the real API. */
export async function apiGet(path, token) {
  const res = await fetch(`${API}${path}`, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
  return res.json().catch(() => ({}))
}

/** Latest OTP code logged by identity-service (APP_ENV=development only). */
export function latestOtp() {
  if (!IDENTITY_LOG) throw new Error('E2E_IDENTITY_LOG is required')
  const lines = readFileSync(IDENTITY_LOG, 'utf8').split('\n').filter((l) => l.includes('"otp_code"'))
  return JSON.parse(lines[lines.length - 1]).otp_code
}

/** Signs in through the real OTP API and returns the session tokens. */
export async function apiLogin(phone) {
  const req = await post('/auth/request-otp', { phone })
  if (req.status !== 200) throw new Error(`request-otp ${req.status} ${JSON.stringify(req.body)}`)
  await new Promise((r) => setTimeout(r, 300))
  const res = await post('/auth/verify-otp', { request_id: req.body.request_id, code: latestOtp() })
  if (res.status !== 200) throw new Error(`verify-otp ${res.status}`)
  return res.body.tokens
}

/** Makes a browser context signed in (tokens in storage, as the app stores them). */
export async function signInContext(context, tokens) {
  await context.addInitScript(([a, r]) => {
    localStorage.setItem('trezutil_access_token', a)
    localStorage.setItem('trezutil_refresh_token', r)
  }, [tokens.access_token, tokens.refresh_token])
}

export async function findListing(text = 'Riverside apartment', filters = {}) {
  const res = await post('/search', { text, page_size: 1, sort_field: 'newest', sort_direction: 'desc', ...filters })
  const item = res.body?.items?.[0]
  if (!item) throw new Error('no published listing found — run test/e2e/journeys.py first')
  return item.ListingID
}
