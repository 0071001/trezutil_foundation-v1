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
import type { Category, SearchResponse, SearchResult, SortField } from '@/types'

/**
 * Listing-type shortcuts. The search contract filters by category and text,
 * not by listing type, so a shortcut resolves to a matching operator category
 * when one exists and to search words otherwise.
 */
const typePresets: Record<string, { label: string; words: string[]; text: string }> = {
  products: { label: 'Products', words: ['product', 'electronic', 'phone', 'shop'], text: '' },
  services: { label: 'Services', words: ['service', 'repair', 'plumb', 'electric'], text: 'service' },
  property: { label: 'Real estate', words: ['real estate', 'apartment', 'house', 'propert', 'home'], text: 'apartment' },
  food: { label: 'Food', words: ['food', 'restaurant', 'dining'], text: 'restaurant' },
  vehicles: { label: 'Transport', words: ['vehicle', 'car', 'transport'], text: 'car' },
}

function matchCategory(categories: Category[], words: string[]): Category | undefined {
  return categories.find((c) => words.some((w) => `${c.name} ${c.slug}`.toLowerCase().includes(w)))
}

const sorts: { value: SortField; label: string; direction?: 'asc' | 'desc' }[] = [
  { value: 'relevance', label: 'Recommended' },
  { value: 'newest', label: 'Newest first' },
  { value: 'price_asc', label: 'Price: low to high', direction: 'asc' },
  { value: 'price_desc', label: 'Price: high to low', direction: 'desc' },
]

export default function SearchPage() {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const type = params.get('type') ?? ''
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
  const preset = typePresets[type]
  const presetCategory = preset && categories.data ? matchCategory(categories.data, preset.words) : undefined
  const categoryId = categoryParam || presetCategory?.id || ''
  const text = q || (preset && !presetCategory ? preset.text : '')
  const ready = !preset || categories.data !== undefined || categories.error !== null

  const request = useMemo(() => ({
    text, category_id: categoryId || undefined, page_size: 12,
    sort_field: sort === 'relevance' ? undefined : sort,
    sort_direction: sorts.find((s) => s.value === sort)?.direction ?? 'desc',
  }), [text, categoryId, sort])

  useEffect(() => { document.title = q ? `${q} — Trezutil search` : 'Search — Trezutil' }, [q])

  useEffect(() => {
    if (!ready) return
    const controller = new AbortController()
    setLoading(true); setError(null); setSlow(false)
    const slowTimer = setTimeout(() => setSlow(true), 3000)
    searchApi.search(request, { signal: controller.signal })
      .then(setResults)
      .catch((err: unknown) => { const e = toApiError(err); if (e.code !== 'ABORTED') { setResults(null); setError(e) } })
      .finally(() => { clearTimeout(slowTimer); if (!controller.signal.aborted) setLoading(false) })
    return () => { clearTimeout(slowTimer); controller.abort() }
  }, [request, ready, nonce])

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
  const label = q ? `“${q}”` : preset?.label ?? 'everything'
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
              <label className="check-row"><input type="radio" name="type" checked={!type} onChange={() => update({ type: '', category: '' })} /><span>All</span></label>
              {Object.entries(typePresets).map(([key, p]) => (
                <label className="check-row" key={key}><input type="radio" name="type" checked={type === key} onChange={() => update({ type: key, category: '' })} /><span>{p.label}</span></label>
              ))}
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
