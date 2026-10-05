import { useEffect, useState } from 'react'

/** Tracks a CSS media query; false where matchMedia does not exist (tests, old browsers). */
export function useMediaQuery(query: string): boolean {
  const supported = typeof window !== 'undefined' && typeof window.matchMedia === 'function'
  const [matches, setMatches] = useState(() => (supported ? window.matchMedia(query).matches : false))
  useEffect(() => {
    if (!supported) return
    const list = window.matchMedia(query)
    const update = () => setMatches(list.matches)
    update()
    list.addEventListener?.('change', update)
    return () => list.removeEventListener?.('change', update)
  }, [query, supported])
  return matches
}
