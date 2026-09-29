import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError, toApiError } from '@/api/client'

export interface AsyncState<T> {
  data: T | undefined
  error: ApiError | null
  loading: boolean
  reload: () => void
  setData: (value: T) => void
}

/**
 * Loads data for a page and exposes loading/error/retry. Stale responses
 * (after deps change or unmount) are ignored.
 */
export function useAsync<T>(load: () => Promise<T>, deps: unknown[], enabled = true): AsyncState<T> {
  const [data, setData] = useState<T | undefined>(undefined)
  const [error, setError] = useState<ApiError | null>(null)
  const [loading, setLoading] = useState(enabled)
  const [nonce, setNonce] = useState(0)
  const loadRef = useRef(load)
  loadRef.current = load

  useEffect(() => {
    if (!enabled) { setLoading(false); return }
    let live = true
    setLoading(true)
    setError(null)
    loadRef.current()
      .then((value) => { if (live) setData(value) })
      .catch((err: unknown) => { if (live) setError(toApiError(err)) })
      .finally(() => { if (live) setLoading(false) })
    return () => { live = false }
  }, [...deps, nonce, enabled]) // eslint-disable-line react-hooks/exhaustive-deps

  const reload = useCallback(() => setNonce((n) => n + 1), [])
  return { data, error, loading, reload, setData }
}
