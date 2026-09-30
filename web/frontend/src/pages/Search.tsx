import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { searchApi } from '@/api/search'
import { ResultCard } from '@/components/cards'
import { SearchComposer } from '@/components/SearchComposer'
import { AppShell } from '@/components/shell'
import { EmptyState, ErrorState, SkeletonGrid } from '@/components/states'
import { ActionLink, Button, Icon, PageIntro } from '@/components/ui'
import { useAsync } from '@/lib/useAsync'
import { legacyTypeVertical, useVerticals } from '@/lib/verticals'
import type { SearchResponse, SearchResult, SortField } from '@/types'

const sorts: { value: SortField; label: string; direction?: 'asc' | 'desc' }[] = [
  { value: 'relevance', label: 'Recommended' },
  { value: 'newest', label: 'Newest first' },
  { value: 'price_asc', label: 'Price: low to high', direction: 'asc' },
  { value: 'price_desc', label: 'Price: high to low', direction: 'desc' },
]

export default function SearchPage() {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  // Vertical filter (backend catalogue); old ?type= links map onto verticals.
  const verticalParam = (params.get('vertical') || legacyTypeVertical[params.get('type') ?? ''] || '').toUpperCase()
  const where = params.get('where') ?? ''
  const categoryParam = params.get('category') ?? ''
  const sort = (params.get('sort') as SortField) || 'relevance'
  const [hideUnavailable, setHideUnavailable] = useState(false)
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [results, setResults] = useState<SearchResponse | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [slow, setSlow] = useState(false)
  const [nonce, setNonce] = useState(0)

  const categories = useAsync(() => searchApi.categories(), [])
  const { verticals, byCode } = useVerticals()
  const [whereDraft, setWhereDraft] = useState(where)
  useEffect(() => { setWhereDraft(where) }, [where])

  const request = useMemo(() => ({
    text: q, category_id: categoryParam || undefined, page_size: 12,
    vertical: verticalParam || undefined, location: where || undefined,
    sort_field: sort === 'relevance' ? undefined : sort,
    sort_direction: sorts.find((s) => s.value === sort)?.direction ?? 'desc',
  }), [q, categoryParam, verticalParam, where, sort])

  useEffect(() => { document.title = q ? `${q} — Trezutil search` : 'Search — Trezutil' }, [q])

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true); setError(null); setSlow(false)
    const slowTimer = setTimeout(() => setSlow(true), 3000)
    searchApi.search(request, { signal: controller.signal })
      .then(setResults)
      .catch((err: unknown) => { const e = toApiError(err); if (e.code !== 'ABORTED') { setResults(null); setError(e) } })
      .finally(() => { clearTimeout(slowTimer); if (!controller.signal.aborted) setLoading(false) })
    return () => { clearTimeout(slowTimer); controller.abort() }
  }, [request, nonce])

  const loadMore = async () => {
    if (!results?.next_cursor) return
    setLoadingMore(true)
    try {
      const next = await searchApi.search({ ...request, cursor: results.next_cursor })
      setResults({ ...next, items: [...results.items, ...next.items.filter((i) => !results.items.some((r) => r.listing_id === i.listing_id))] })
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setLoadingMore(false)
    }
  }

  const update = (changes: Record<string, string>) => {
    const next = new URLSearchParams(params)
    for (const [k, v] of Object.entries(changes)) { if (v) next.set(k, v); else next.delete(k) }
    setParams(next)
  }

  const items: SearchResult[] = (results?.items ?? []).filter((i) => !hideUnavailable || !['out_of_stock', 'unavailable'].includes(i.availability_status))
  const verticalName = byCode[verticalParam]?.name
  const label = `${q ? `“${q}”` : verticalName ?? 'everything'}${q && verticalName ? ` in ${verticalName}` : ''}${where ? ` around ${where}` : ''}`
  const refine = `/conversation?q=${encodeURIComponent(q ? `Help me choose: ${q}` : 'Help me find something')}`

  return (
    <AppShell dockPrompt={q ? `Help me choose: ${q}` : undefined}>
      <section className="page-section search-page">
        <PageIntro eyebrow="Search Trezutil" title="Find what you need." copy="Search naturally, then refine by category, price or availability." />
        <SearchComposer initial={q} onSubmit={(value) => update({ q: value })} />
        {results && !loading && (
          <div className="assistant-result" role="status">
            <span className="dock-logo"><Icon name="spark" /></span>
            <div><strong>{results.total > 0 ? `I found ${results.total} option${results.total === 1 ? '' : 's'} for ${label}.` : `Nothing matches ${label} yet.`}</strong><span>{results.total > 0 ? 'Ask me to compare them or narrow them down.' : 'Try other words, or tell me what you need in your own words.'}</span></div>
            <ActionLink href={refine}>Ask me to refine <Icon name="arrow" size={16} /></ActionLink>
          </div>
        )}
        <div className="search-layout">
          <aside className={`filter-panel ${filtersOpen ? 'open' : ''}`} aria-label="Filters">
            <div className="filter-head"><strong>Filters</strong><Button onClick={() => { setHideUnavailable(false); setParams(q ? { q } : {}) }}>Reset</Button></div>
            <fieldset className="filter-group"><legend>Type</legend>
              <label className="check-row"><input type="radio" name="vertical" checked={!verticalParam} onChange={() => update({ vertical: '', type: '' })} /><span>All</span></label>
              {verticals.map((v) => (
                <label className="check-row" key={v.code}><input type="radio" name="vertical" checked={verticalParam === v.code} onChange={() => update({ vertical: v.code, type: '' })} /><span>{v.name}</span></label>
              ))}
            </fieldset>
            <fieldset className="filter-group"><legend>Location</legend>
              <form className="filter-location" onSubmit={(e) => { e.preventDefault(); update({ where: whereDraft.trim() }) }}>
                <label className="sr-only" htmlFor="filter-where">Neighbourhood or city</label>
                <input id="filter-where" type="search" value={whereDraft} maxLength={100} placeholder="e.g. Gombe" onChange={(e) => setWhereDraft(e.target.value)} onBlur={() => { if (whereDraft.trim() !== where) update({ where: whereDraft.trim() }) }} />
              </form>
            </fieldset>
            {(categories.data?.length ?? 0) > 0 && (
              <fieldset className="filter-group"><legend>Category</legend>
                <label className="check-row"><input type="radio" name="category" checked={!categoryParam} onChange={() => update({ category: '' })} /><span>Any</span></label>
                {categories.data!.map((c) => (
                  <label className="check-row" key={c.id}><input type="radio" name="category" checked={categoryParam === c.id} onChange={() => update({ category: c.id, type: '' })} /><span>{c.name}</span></label>
                ))}
              </fieldset>
            )}
            <fieldset className="filter-group"><legend>Availability</legend>
              <label className="check-row"><input type="checkbox" checked={hideUnavailable} onChange={(e) => setHideUnavailable(e.target.checked)} /><span>Available now</span></label>
            </fieldset>
            <Button className="primary-button mobile-filter-apply" onClick={() => setFiltersOpen(false)}>Show results</Button>
          </aside>
          <div className="results-area">
            <div className="results-toolbar">
              <span>{results ? <><strong>{results.total}</strong> results</> : loading ? 'Searching…' : ''}</span>
              <Button className="outline-button mobile-filter-toggle" onClick={() => setFiltersOpen(true)}><Icon name="filter" size={16} />Filters</Button>
              <label>Sort by <select value={sort} onChange={(e) => update({ sort: e.target.value === 'relevance' ? '' : e.target.value })}>{sorts.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}</select></label>
            </div>
            {loading && <>{slow && <p className="slow-note" role="status">Still searching — this is taking longer than usual…</p>}<SkeletonGrid count={6} /></>}
            {!loading && error && <ErrorState error={error} onRetry={() => setNonce((n) => n + 1)} />}
            {!loading && !error && items.length === 0 && (
              <EmptyState icon="search" title="No results yet" copy={hideUnavailable ? 'Everything that matches is currently unavailable. Show unavailable listings or try other words.' : 'Try different words or a broader category — or describe what you need and Trezutil will look for you.'} action="Ask Trezutil" href={refine} />
            )}
            {!loading && !error && items.length > 0 && (
              <>
                <div className="results-grid">{items.map((item) => <ResultCard key={item.listing_id} item={item} />)}</div>
                {results?.has_next && <Button className="load-more" onClick={() => void loadMore()} busy={loadingMore}>{loadingMore ? 'Loading…' : 'Show more results'}</Button>}
              </>
            )}
          </div>
        </div>
      </section>
    </AppShell>
  )
}
