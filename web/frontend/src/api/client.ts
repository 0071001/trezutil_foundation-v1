/**
 * HTTP client for the Trezutil gateway (/api/v1). It attaches the access
 * token, refreshes it once on 401, turns backend error envelopes into
 * ApiError, de-duplicates identical in-flight GETs and reports network
 * failures distinctly from API errors.
 *
 * Identity is never sent in bodies: the backend derives it from the token.
 */
import type { ErrorEnvelope, Session } from '@/types'
import { t } from '@/lib/i18n'

export const API_BASE = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api/v1'

const ACCESS_KEY = 'trezutil_access_token'
const REFRESH_KEY = 'trezutil_refresh_token'
const REQUEST_TIMEOUT_MS = 30_000

/** Dispatched on window when a signed-in session can no longer be renewed. */
export const SESSION_EXPIRED_EVENT = 'trezutil:session-expired'
/** Dispatched on window whenever the stored session changes (sign-in/out). */
export const SESSION_CHANGED_EVENT = 'trezutil:session-changed'

export class ApiError extends Error {
  constructor(message: string, public status: number, public code: string, public body?: unknown) {
    super(message)
    this.name = 'ApiError'
  }

  get isNetwork() { return this.status === 0 }
}

export function toApiError(err: unknown): ApiError {
  if (err instanceof ApiError) return err
  return new ApiError(err instanceof Error ? err.message : t('Unexpected error'), 0, 'UNKNOWN')
}

function storage(): Storage | null {
  try { return typeof window === 'undefined' ? null : window.localStorage } catch { return null }
}

export const tokens = {
  access(): string | null { return storage()?.getItem(ACCESS_KEY) ?? null },
  refresh(): string | null { return storage()?.getItem(REFRESH_KEY) ?? null },
  save(session: Pick<Session, 'tokens'>) {
    storage()?.setItem(ACCESS_KEY, session.tokens.access_token)
    if (session.tokens.refresh_token) storage()?.setItem(REFRESH_KEY, session.tokens.refresh_token)
    window.dispatchEvent(new Event(SESSION_CHANGED_EVENT))
  },
  clear() {
    storage()?.removeItem(ACCESS_KEY)
    storage()?.removeItem(REFRESH_KEY)
    window.dispatchEvent(new Event(SESSION_CHANGED_EVENT))
  },
}

let refreshInFlight: Promise<boolean> | null = null

/** Exchanges the refresh token for a new session (single flight). */
export function refreshSession(): Promise<boolean> {
  const refreshToken = tokens.refresh()
  if (!refreshToken) return Promise.resolve(false)
  refreshInFlight ??= fetch(`${API_BASE}/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
    credentials: 'same-origin',
  })
    .then(async (res) => {
      if (!res.ok) return false
      const data = (await res.json()) as Partial<Session>
      if (!data.tokens?.access_token) return false
      tokens.save(data as Session)
      return true
    })
    .catch(() => false)
    .finally(() => { refreshInFlight = null })
  return refreshInFlight
}

export interface RequestOptions {
  idempotencyKey?: string
  signal?: AbortSignal
}

const inflightGets = new Map<string, Promise<unknown>>()

/** The calls that open a session. They go out without a token: an access token is short-lived, and one left in the
 *  browser by an earlier visit makes the server answer 401 to the sign-in itself, which looked like a wrong password. */
const SIGN_IN_PATHS = new Set(['/auth/request-otp', '/auth/verify-otp', '/auth/staff/login'])

async function send<T>(method: string, path: string, body: unknown, options: RequestOptions, retried: boolean): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  // A file upload sets its own multipart boundary; everything else is JSON.
  const isForm = typeof FormData !== 'undefined' && body instanceof FormData
  if (body !== undefined && !isForm) headers['Content-Type'] = 'application/json'
  const token = SIGN_IN_PATHS.has(path) ? null : tokens.access()
  if (token) headers.Authorization = `Bearer ${token}`
  if (options.idempotencyKey) headers['Idempotency-Key'] = options.idempotencyKey

  const timeout = new AbortController()
  const timer = setTimeout(() => timeout.abort(), REQUEST_TIMEOUT_MS)
  const onAbort = () => timeout.abort()
  options.signal?.addEventListener('abort', onAbort)

  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : isForm ? (body as FormData) : JSON.stringify(body),
      credentials: 'same-origin',
      signal: timeout.signal,
    })
  } catch {
    if (options.signal?.aborted) throw new ApiError(t('Request cancelled'), 0, 'ABORTED')
    throw new ApiError(
      timeout.signal.aborted ? t('The server took too long to answer. Please try again.') : t('You appear to be offline. Check your connection and try again.'),
      0, timeout.signal.aborted ? 'TIMEOUT' : t('NETWORK_ERROR'))
  } finally {
    clearTimeout(timer)
    options.signal?.removeEventListener('abort', onAbort)
  }

  const data = (await response.json().catch(() => null)) as unknown

  // Access tokens are short-lived: refresh once and replay the request.
  if (response.status === 401 && token && !retried && !path.startsWith('/auth/')) {
    if (await refreshSession()) return send<T>(method, path, body, options, true)
    tokens.clear()
    window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT))
  }

  if (!response.ok) {
    const envelope = (data ?? {}) as ErrorEnvelope
    throw new ApiError(
      envelope.error?.message ?? defaultMessage(response.status),
      response.status,
      envelope.error?.code ?? t('HTTP_{status}', { status: response.status }),
      data,
    )
  }
  return data as T
}

function defaultMessage(status: number): string {
  if (status === 401) return t('Please sign in to continue.')
  if (status === 403) return t('You do not have access to this.')
  if (status === 404) return t('We could not find that.')
  if (status === 429) return t('Too many attempts. Please wait a moment and try again.')
  if (status >= 500) return t('Something went wrong on our side. Please try again.')
  return t('Request failed ({status})', { status })
}

export const http = {
  /** Identical concurrent GETs share one network request. */
  get<T>(path: string, options: RequestOptions = {}): Promise<T> {
    if (options.signal) return send<T>('GET', path, undefined, options, false)
    const key = `${tokens.access() ?? ''}|${path}`
    const existing = inflightGets.get(key)
    if (existing) return existing as Promise<T>
    const p = send<T>('GET', path, undefined, options, false).finally(() => inflightGets.delete(key))
    inflightGets.set(key, p)
    return p
  },
  post<T>(path: string, body: unknown = {}, options: RequestOptions = {}): Promise<T> {
    return send<T>('POST', path, body, options, false)
  },
  put<T>(path: string, body: unknown = {}, options: RequestOptions = {}): Promise<T> {
    return send<T>('PUT', path, body, options, false)
  },
  patch<T>(path: string, body: unknown = {}, options: RequestOptions = {}): Promise<T> {
    return send<T>('PATCH', path, body, options, false)
  },
  del<T>(path: string, options: RequestOptions = {}): Promise<T> {
    return send<T>('DELETE', path, undefined, options, false)
  },
  /** Sends one file as multipart form data. */
  upload<T>(path: string, field: string, file: File, options: RequestOptions = {}): Promise<T> {
    const form = new FormData()
    form.append(field, file)
    return send<T>('POST', path, form, options, false)
  },
}

export function query(params: Record<string, string | number | undefined | null>): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== null && v !== '') q.set(k, String(v))
  const s = q.toString()
  return s ? `?${s}` : ''
}
