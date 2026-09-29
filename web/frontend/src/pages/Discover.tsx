import { useEffect } from 'react'
import { searchApi } from '@/api/search'
import { ResultCard } from '@/components/cards'
import { SearchComposer } from '@/components/SearchComposer'
import { AppShell } from '@/components/shell'
import { EmptyState, ErrorState, SkeletonGrid } from '@/components/states'
import { ActionLink, Heading, Icon, type IconName } from '@/components/ui'
import { useAsync } from '@/lib/useAsync'
import type { SearchRequest } from '@/types'

const images = {
  city: 'https://images.unsplash.com/photo-1695071622344-75bb5863bc83?auto=format&fit=crop&w=1200&q=80',
  food: 'https://images.unsplash.com/photo-1601050690597-df0568f70950?auto=format&fit=crop&w=1200&q=80',
  service: 'https://images.unsplash.com/photo-1621905251189-08b45d6a269e?auto=format&fit=crop&w=1200&q=80',
}

const categoryItems: { label: string; icon: IconName; color: string; href: string }[] = [
  { label: 'Products', icon: 'shop', color: 'blue', href: '/search?type=products' },
  { label: 'Services', icon: 'service', color: 'navy', href: '/search?type=services' },
  { label: 'Food', icon: 'food', color: 'coral', href: '/search?type=food' },
  { label: 'Hotels', icon: 'building', color: 'mint', href: '/search?q=hotel' },
  { label: 'Real Estate', icon: 'home', color: 'soft', href: '/search?type=property' },
  { label: 'Transport', icon: 'car', color: 'line', href: '/search?type=vehicles' },
]

/** Figma Rail bound to a live search. */
function Rail({ title, copy, request, href }: { title: string; copy: string; request: SearchRequest; href: string }) {
  const rail = useAsync(() => searchApi.search(request), [JSON.stringify(request)])
  return (
    <section className="app-rail" aria-label={title}>
      <div className="rail-head"><div><Heading>{title}</Heading><p>{copy}</p></div><ActionLink href={href}>See all <Icon name="arrow" size={17} /></ActionLink></div>
      {rail.loading && <SkeletonGrid count={4} />}
      {rail.error && <ErrorState error={rail.error} onRetry={rail.reload} compact />}
      {rail.data && rail.data.items.length === 0 && <p className="rail-empty">Nothing here yet — new listings appear as sellers publish them.</p>}
      {rail.data && rail.data.items.length > 0 && <div className="results-grid">{rail.data.items.map((item) => <ResultCard item={item} key={item.listing_id} />)}</div>}
    </section>
  )
}

export default function DiscoverPage() {
  const total = useAsync(() => searchApi.search({ text: '', page_size: 1 }), [])
  useEffect(() => { document.title = 'Discover — Trezutil' }, [])
  return (
    <AppShell>
      <section className="discover-hero"><div className="page-section">
        <div className="discover-copy"><div className="section-eyebrow">Discover around you</div><Heading level={1}>What will you find today?</Heading><p>Useful products, places and people — listed by sellers around Kinshasa.</p><SearchComposer /></div>
        <div className="discover-collage"><img src={images.city} alt="Street commerce in Kinshasa" loading="eager" /><div><span>Listed on Trezutil</span><strong>{total.data ? `${total.data.total} places to discover` : 'Discover what’s near you'}</strong></div></div>
      </div></section>
      <section className="page-section discover-content">
        <nav className="category-strip" aria-label="Categories">{categoryItems.map((item) => <ActionLink href={item.href} className={`discover-category ${item.color}`} key={item.label}><Icon name={item.icon} /><strong>{item.label}</strong><Icon name="arrow" size={16} /></ActionLink>)}</nav>
        <Rail title="New around you" copy="The latest listings from sellers nearby" request={{ text: '', page_size: 4, sort_field: 'newest', sort_direction: 'desc' }} href="/search?sort=newest" />
        <div className="editorial-row">
          <ActionLink className="editorial-card food-feature" href="/search?type=food"><img src={images.food} alt="" loading="lazy" /><div><span>Eat nearby</span><Heading level={2}>Good food,<br />closer than you think.</Heading><small>Restaurants and delivery</small></div></ActionLink>
          <ActionLink className="editorial-card service-feature" href="/search?type=services"><img src={images.service} alt="" loading="lazy" /><div><span>Services</span><Heading level={2}>Get it done.</Heading><small>Local professionals you can book</small></div></ActionLink>
        </div>
        <Rail title="Homes & stays" copy="Apartments, houses and places to stay" request={{ text: 'apartment', page_size: 4 }} href="/search?type=property" />
        {total.data?.total === 0 && <EmptyState icon="shop" title="The marketplace is just opening" copy="Sellers are adding their first listings. Tell Trezutil what you need and we’ll look as soon as it’s listed." action="Ask Trezutil" href="/conversation" />}
      </section>
    </AppShell>
  )
}
