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
import { t, tx, tn } from '@/lib/i18n'

const sorts: { value: SortField; label: string; direction?: 'asc' | 'desc' }[] = [
  { value: 'relevance', label: tx('Recommended') },
  { value: 'newest', label: tx('Newest first') },
  { value: 'price_asc', label: tx('Price: low to high'), direction: 'asc' },
  { value: 'price_desc', label: tx('Price: high to low'), direction: 'desc' },
  { value: 'discount', label: tx('Biggest discount'), direction: 'desc' },
]

export default function SearchPage() {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  // Vertical filter (backend catalogue); old ?type= links map onto verticals.
  const verticalParam = (params.get('vertical') || legacyTypeVertical[params.get('type') ?? ''] || '').toUpperCase()
  const where = params.get('where') ?? ''
  const categoryParam = params.get('category') ?? ''
  // ?promo=1: only listings currently on sale.
  const promo = params.get('promo') === '1'
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
    vertical: verticalParam || undefined, location: where || undefined, on_promotion: promo || undefined,
    sort_field: sort === 'relevance' ? undefined : sort,
    sort_direction: sorts.find((s) => s.value === sort)?.direction ?? 'desc',
  }), [q, categoryParam, verticalParam, where, sort, promo])

  useEffect(() => { document.title = q ? t('{q} — Trezutil search', { q }) : t('Search — Trezutil') }, [q])

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

  const items: SearchResult[] = (results?.items ?? []).filter((i) => (!hideUnavailable || !['out_of_stock', 'unavailable'].includes(i.availability_status)) && (!promo || i.discount_percent > 0))
  const verticalName = byCode[verticalParam]?.name
  const subject = q ? (verticalName ? t('“{q}” in {vertical}', { q, vertical: verticalName }) : `“${q}”`) : (verticalName ?? t('everything'))
  const base = promo ? (q || verticalName ? t('{subject} on promotion', { subject }) : t('deals')) : subject
  const label = where ? t('{label} around {where}', { label: base, where }) : base
  const refine = `/conversation?q=${encodeURIComponent(q ? t('Help me choose: {q}', { q }) : t('Help me find something'))}`

  return (
    <AppShell dockPrompt={q ? t('Help me choose: {q}', { q }) : undefined}>
      <section className="page-section search-page">
        <PageIntro eyebrow={t('Search Trezutil')} title={t('Find what you need.')} copy={t('Search naturally, then refine by category, price or availability.')} />
        <SearchComposer initial={q} onSubmit={(value) => update({ q: value })} />
        {results && !loading && (
          <div className="assistant-result" role="status">
            <span className="dock-logo"><Icon name="spark" /></span>
            <div><strong>{results.total > 0 ? tn(results.total, 'I found {n} option for {label}.', 'I found {n} options for {label}.', { label }) : t('Nothing matches {label} yet.', { label })}</strong><span>{results.total > 0 ? t('Ask me to compare them or narrow them down.') : t('Try other words, or tell me what you need in your own words.')}</span></div>
            <ActionLink href={refine}>{t('Ask me to refine')} <Icon name="arrow" size={16} /></ActionLink>
          </div>
        )}
        <div className="search-layout">
          <aside className={`filter-panel ${filtersOpen ? 'open' : ''}`} aria-label={t('Filters')}>
            <div className="filter-head"><strong>{t('Filters')}</strong><Button onClick={() => { setHideUnavailable(false); setParams(q ? { q } : {}) }}>{t('Reset')}</Button></div>
            <fieldset className="filter-group"><legend>{t('Type')}</legend>
              <label className="check-row"><input type="radio" name="vertical" checked={!verticalParam} onChange={() => update({ vertical: '', type: '' })} /><span>{t('All')}</span></label>
              {verticals.map((v) => (
                <label className="check-row" key={v.code}><input type="radio" name="vertical" checked={verticalParam === v.code} onChange={() => update({ vertical: v.code, type: '' })} /><span>{v.name}</span></label>
              ))}
            </fieldset>
            <fieldset className="filter-group"><legend>{t('Location')}</legend>
              <form className="filter-location" onSubmit={(e) => { e.preventDefault(); update({ where: whereDraft.trim() }) }}>
                <label className="sr-only" htmlFor="filter-where">{t('Neighbourhood or city')}</label>
                <input id="filter-where" type="search" value={whereDraft} maxLength={100} placeholder={t('e.g. Gombe')} onChange={(e) => setWhereDraft(e.target.value)} onBlur={() => { if (whereDraft.trim() !== where) update({ where: whereDraft.trim() }) }} />
              </form>
            </fieldset>
            {(categories.data?.length ?? 0) > 0 && (
              <fieldset className="filter-group"><legend>{t('Category')}</legend>
                <label className="check-row"><input type="radio" name="category" checked={!categoryParam} onChange={() => update({ category: '' })} /><span>{t('Any')}</span></label>
                {categories.data!.map((c) => (
                  <label className="check-row" key={c.id}><input type="radio" name="category" checked={categoryParam === c.id} onChange={() => update({ category: c.id, type: '' })} /><span>{c.name}</span></label>
                ))}
              </fieldset>
            )}
            <fieldset className="filter-group"><legend>{t('Offers')}</legend>
              <label className="check-row"><input type="checkbox" checked={promo} onChange={(e) => update({ promo: e.target.checked ? '1' : '' })} /><span>{t('On promotion')}</span></label>
            </fieldset>
            <fieldset className="filter-group"><legend>{t('Availability')}</legend>
              <label className="check-row"><input type="checkbox" checked={hideUnavailable} onChange={(e) => setHideUnavailable(e.target.checked)} /><span>{t('Available now')}</span></label>
            </fieldset>
            <Button className="primary-button mobile-filter-apply" onClick={() => setFiltersOpen(false)}>{t('Show results')}</Button>
          </aside>
          <div className="results-area">
            <div className="results-toolbar">
              <span>{results ? <><strong>{results.total}</strong> {t('results')}</> : loading ? t('Searching…') : ''}</span>
              <Button className="outline-button mobile-filter-toggle" onClick={() => setFiltersOpen(true)}><Icon name="filter" size={16} />{t('Filters')}</Button>
              <label>{t('Sort by')} <select value={sort} onChange={(e) => update({ sort: e.target.value === 'relevance' ? '' : e.target.value })}>{sorts.map((s) => <option key={s.value} value={s.value}>{t(s.label)}</option>)}</select></label>
            </div>
            {loading && <>{slow && <p className="slow-note" role="status">{t('Still searching — this is taking longer than usual…')}</p>}<SkeletonGrid count={6} /></>}
            {!loading && error && <ErrorState error={error} onRetry={() => setNonce((n) => n + 1)} />}
            {!loading && !error && items.length === 0 && (
              <EmptyState icon="search" title={promo ? t('No deals right now') : t('No results yet')} copy={hideUnavailable ? t('Everything that matches is currently unavailable. Show unavailable listings or try other words.') : promo ? t('Sellers add offers all the time. Check back soon, or describe what you need and Trezutil will look for you.') : t('Try different words or a broader category — or describe what you need and Trezutil will look for you.')} action={t('Ask Trezutil')} href={refine} />
            )}
            {!loading && !error && items.length > 0 && (
              <>
                <div className="results-grid">{items.map((item) => <ResultCard key={item.listing_id} item={item} />)}</div>
                {results?.has_next && <Button className="load-more" onClick={() => void loadMore()} busy={loadingMore}>{loadingMore ? t('Loading…') : t('Show more results')}</Button>}
              </>
            )}
          </div>
        </div>
      </section>
    </AppShell>
  )
}
