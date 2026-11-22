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

/** A whole streamed answer may take this long (the server's budget is 90 s). */
const STREAM_TIMEOUT_MS = 100_000
/** The server sends a keep-alive every 15 s: this long without a byte means the connection is gone. */
const STREAM_IDLE_MS = 40_000

/**
 * POSTs a JSON body and reads the answer as Server-Sent Events, calling onEvent(name, data) for each event as it
 * arrives (data parsed from JSON). Same token, single refresh on 401 and error envelope as the other calls. Resolves
 * when the server closes the stream; rejects with an ApiError: ABORTED (the caller's signal), TIMEOUT, or a network
 * code (status 0) when the connection broke, so the caller can ask again with the same idempotency key.
 */
async function stream(path: string, body: unknown, onEvent: (name: string, data: unknown) => void, signal?: AbortSignal, retried = false): Promise<void> {
  const headers: Record<string, string> = { Accept: 'text/event-stream', 'Content-Type': 'application/json' }
  const token = tokens.access()
  if (token) headers.Authorization = `Bearer ${token}`

  const control = new AbortController()
  let reason: 'timeout' | 'idle' | null = null
  const total = setTimeout(() => { reason = 'timeout'; control.abort() }, STREAM_TIMEOUT_MS)
  let idle = setTimeout(() => { reason = 'idle'; control.abort() }, STREAM_IDLE_MS)
  const touch = () => { clearTimeout(idle); idle = setTimeout(() => { reason = 'idle'; control.abort() }, STREAM_IDLE_MS) }
  const onAbort = () => control.abort()
  signal?.addEventListener('abort', onAbort)
  const broken = () => {
    if (signal?.aborted) return new ApiError(t('Request cancelled'), 0, 'ABORTED')
    if (reason === 'timeout') return new ApiError(t('The server took too long to answer. Please try again.'), 0, 'TIMEOUT')
    return new ApiError(t('You appear to be offline. Check your connection and try again.'), 0, 'STREAM_INTERRUPTED')
  }

  try {
    let response: Response
    try {
      response = await fetch(`${API_BASE}${path}`, { method: 'POST', headers, body: JSON.stringify(body), credentials: 'same-origin', signal: control.signal })
    } catch {
      throw broken()
    }
    if (response.status === 401 && token && !retried) {
      if (await refreshSession()) return stream(path, body, onEvent, signal, true)
      tokens.clear()
      window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT))
    }
    if (!response.ok || !response.body) {
      const data = (await response.json().catch(() => null)) as unknown
      const envelope = (data ?? {}) as ErrorEnvelope
      throw new ApiError(envelope.error?.message ?? defaultMessage(response.status), response.status,
        envelope.error?.code ?? t('HTTP_{status}', { status: response.status }), data)
    }
    const reader = response.body.getReader()
    // Stopping (or a timeout) ends the read at once, whatever the transport does with the aborted request.
    const cancel = () => { void reader.cancel().catch(() => { /* already closed */ }) }
    control.signal.addEventListener('abort', cancel)
    const decoder = new TextDecoder()
    let buffer = ''
    for (;;) {
      let chunk: ReadableStreamReadResult<Uint8Array>
      try {
        chunk = await reader.read()
      } catch {
        throw broken()
      }
      if (control.signal.aborted) throw broken()
      if (chunk.done) break
      touch()
      buffer += decoder.decode(chunk.value, { stream: true }).replace(/\r\n/g, '\n')
      let end = buffer.indexOf('\n\n')
      while (end >= 0) {
        const frame = buffer.slice(0, end)
        buffer = buffer.slice(end + 2)
        let name = 'message'
        const data: string[] = []
        for (const line of frame.split('\n')) {
          if (line.startsWith(':')) continue // keep-alive comment
          if (line.startsWith('event:')) name = line.slice(6).trim()
          else if (line.startsWith('data:')) data.push(line.slice(5).replace(/^ /, ''))
        }
        if (data.length) {
          let parsed: unknown = null
          try { parsed = JSON.parse(data.join('\n')) } catch { parsed = data.join('\n') }
          onEvent(name, parsed)
        }
        end = buffer.indexOf('\n\n')
      }
    }
  } finally {
    clearTimeout(total)
    clearTimeout(idle)
    signal?.removeEventListener('abort', onAbort)
  }
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
  /** POSTs and reads the answer as Server-Sent Events (see stream). */
  stream,
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
