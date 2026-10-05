import { useEffect, useState } from 'react'
import { siteApi, HOME_SECTIONS, type PublicSiteContent, type SectionSetting } from '@/api/site'

/*
 * The public site's managed content (trust strip figures, partners, section
 * order). One request is shared by every component on the page and reused for a
 * minute. If it cannot be loaded the site simply shows no figures and no
 * partners and keeps its default layout: nothing is ever filled in with
 * placeholders.
 */

const TTL_MS = 60_000

let cache: { at: number; value: PublicSiteContent } | null = null
let inflight: Promise<PublicSiteContent> | null = null

/** For tests: forget what was loaded. */
export function resetSiteContent(): void {
  cache = null
  inflight = null
}

function load(): Promise<PublicSiteContent> {
  inflight ??= siteApi.content()
    .then((value) => { cache = { at: Date.now(), value }; return value })
    .finally(() => { inflight = null })
  return inflight
}

export interface SiteContentState {
  /** null until it loads, and when it cannot be loaded. */
  content: PublicSiteContent | null
  loading: boolean
}

export function useSiteContent(): SiteContentState {
  const fresh = cache && Date.now() - cache.at < TTL_MS ? cache.value : null
  const [state, setState] = useState<SiteContentState>({ content: fresh ?? cache?.value ?? null, loading: !fresh })

  useEffect(() => {
    if (cache && Date.now() - cache.at < TTL_MS) { setState({ content: cache.value, loading: false }); return }
    let live = true
    load()
      .then((content) => { if (live) setState({ content, loading: false }) })
      .catch(() => { if (live) setState((s) => ({ content: s.content, loading: false })) }) // keep what was last known
    return () => { live = false }
  }, [])

  return state
}

/** The layout the home page uses until, or unless, the managed order is available. */
export const DEFAULT_SECTIONS: SectionSetting[] = HOME_SECTIONS.map((key) => ({ key, visible: true }))
