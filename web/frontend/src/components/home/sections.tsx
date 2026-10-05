import { useNavigate } from 'react-router'
import { searchApi } from '@/api/search'
import { ListingImage } from '@/components/cards'
import { NeedArt } from '@/components/home/art'
import { openConversation } from '@/components/home/NeedComposer'
import { ActionLink, Heading, Icon } from '@/components/ui'
import { detailPath, formatMoney, place } from '@/lib/format'
import { t, tx } from '@/lib/i18n'
import { NEED_GROUPS, type NeedGroup } from '@/lib/needs'
import { useAsync } from '@/lib/useAsync'
import type { SearchRequest, SearchResult } from '@/types'

/*
 * The home page below the hero, in the order of the product: what we can help
 * with, what exists around you, what happens from need to action, and what it
 * means for a business. Offers (listings) appear only as the supply layer: a
 * few rows in "Discover", never a catalogue. The presentation is typographic
 * (an index, hairlines, one dark band) rather than a grid of identical cards
 * with icons. See docs/web/HOME_PAGE_REDESIGN.md.
 */

export function SectionHeading({ id, title, copy }: { id: string; title: string; copy?: string }) {
  return (
    <div className="home-head">
      <Heading id={id}>{title}</Heading>
      {copy && <p>{copy}</p>}
    </div>
  )
}

/* ------------------------------------------------- what can we help with */

/** Whether any live offer exists for these verticals; null while unknown (and when the question cannot be answered). */
function useSupply(codes: string[] | undefined): boolean | null {
  const answer = useAsync(async () => {
    const totals = await Promise.all((codes ?? []).map((code) => searchApi.search({ text: '', vertical: code, page_size: 1 }).then((r) => r.items.length > 0 || r.total > 0)))
    return totals.some(Boolean)
  }, [(codes ?? []).join(',')], !!codes)
  if (!codes) return true
  if (answer.loading || answer.error) return null
  return answer.data ?? null
}

function GroupRow({ group }: { group: NeedGroup }) {
  const navigate = useNavigate()
  const supply = useSupply(group.requiresSupply)
  if (group.requiresSupply && supply !== true) return null // shown the day there is something to find
  return (
    <li className="need-group">
      <NeedArt id={group.id} />
      <h3>{t(group.title)}</h3>
      <p className="need-domains">
        {group.domains.map(([label, href], i) => (
          <span key={label}>{i > 0 && <i aria-hidden="true"> · </i>}<ActionLink href={href}>{t(label)}</ActionLink></span>
        ))}
      </p>
      <ul className="need-asks">
        {group.needs.map((need) => (
          <li key={need}>
            <button type="button" onClick={() => openConversation(navigate, t(need))}>
              <span>“{t(need)}”</span>
              <Icon name="arrow" size={15} />
            </button>
          </li>
        ))}
      </ul>
    </li>
  )
}

export function NeedsSection() {
  return (
    // a white band across the page (like "Discover"); the content keeps the page's width inside it
    <section className="home-needs" id="needs" aria-labelledby="needs-title">
      <div className="home-section">
        <SectionHeading id="needs-title" title={t('What can we help you with?')} copy={t('Pick a starting point, or just describe your need above.')} />
        <ul className="need-groups">
          {NEED_GROUPS.map((group) => <GroupRow key={group.id} group={group} />)}
        </ul>
      </div>
    </section>
  )
}

/* ------------------------------------------------------------ discovery */

interface Column {
  id: string
  title: string
  request: SearchRequest
  more: string
  moreLabel: string
}

const COLUMNS: Column[] = [
  { id: 'sleep', title: tx('To sleep'), request: { text: '', vertical: 'HOTEL', page_size: 3 }, more: '/search?vertical=HOTEL', moreLabel: tx('See more places to stay') },
  { id: 'eat', title: tx('To eat'), request: { text: '', vertical: 'RESTAURANT', page_size: 3 }, more: '/search?vertical=RESTAURANT', moreLabel: tx('See more places to eat') },
  { id: 'new', title: tx('New on Trezutil'), request: { text: '', page_size: 9, sort_field: 'newest' }, more: '/search?sort=newest', moreLabel: tx('See what’s new') },
]

function Row({ item }: { item: SearchResult }) {
  return (
    <li>
      <ActionLink className="disc-row" href={detailPath(item.listing_id, undefined, item.vertical)}>
        <span className="disc-thumb"><ListingImage src={item.primary_media_url} alt="" /></span>
        <span className="disc-main"><strong>{item.title}</strong><small>{place(item) || item.store?.name || t('Kinshasa')}</small></span>
        <span className="disc-price">{formatMoney(item.price_amount, item.price_currency)}</span>
      </ActionLink>
    </li>
  )
}

export function DiscoverySection() {
  const sleep = useAsync(() => searchApi.search(COLUMNS[0]!.request), [])
  const eat = useAsync(() => searchApi.search(COLUMNS[1]!.request), [])
  const fresh = useAsync(() => searchApi.search(COLUMNS[2]!.request), [])
  const pending = sleep.loading || eat.loading || fresh.loading

  const hotels = sleep.error ? [] : sleep.data?.items ?? []
  const tables = eat.error ? [] : eat.data?.items ?? []
  const taken = new Set([...hotels, ...tables].map((i) => i.listing_id))
  // The "new" list never repeats what the other two already show.
  const latest = (fresh.error ? [] : fresh.data?.items ?? []).filter((i) => !taken.has(i.listing_id)).slice(0, 3)
  const lists: [Column, SearchResult[]][] = [[COLUMNS[0]!, hotels], [COLUMNS[1]!, tables], [COLUMNS[2]!, latest]]
  const shown = lists.filter(([, items]) => items.length > 0)

  if (!pending && shown.length === 0) return null // nothing to discover yet: no empty section
  return (
    // a white band across the page; the content keeps the page's width inside it
    <section className="home-discover" id="discover" aria-labelledby="discover-title">
      <div className="home-section">
        <SectionHeading id="discover-title" title={t('Discover what exists around you')} copy={t('Real offers from businesses nearby, a few at a time.')} />
        {pending ? (
          <div className="disc-cols" aria-hidden="true">
            {COLUMNS.map((c) => <div className="disc-col" key={c.id}>{[0, 1, 2].map((i) => <div className="disc-skeleton skeleton" key={i} />)}</div>)}
          </div>
        ) : (
          <div className="disc-cols">
            {shown.map(([column, items]) => (
              <div className="disc-col" key={column.id}>
                <h3>{t(column.title)}</h3>
                <ul>{items.map((item) => <Row item={item} key={item.listing_id} />)}</ul>
                <ActionLink className="disc-more" href={column.more}><span>{t(column.moreLabel)}</span><Icon name="arrow" size={16} /></ActionLink>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  )
}

/* ------------------------------------------------- need to action: the loop */

/**
 * Two parties, one loop: one list down the page, the steps you take in ink and
 * what Trezutil and the business do in blue (`data-side`, drawn in CSS).
 */
const STEPS: { who: string; title: string; copy: string; side: 'you' | 'us' }[] = [
  { who: tx('You'), title: tx('You say what you need'), copy: tx('In your own words, written or spoken.'), side: 'you' },
  { who: tx('Trezutil'), title: tx('We understand it'), copy: tx('Budget, place, date: we ask only what we need to find the right thing.'), side: 'us' },
  { who: tx('The offers'), title: tx('You see what fits'), copy: tx('Products, services, places and providers that match, with their availability.'), side: 'us' },
  { who: tx('You'), title: tx('You act'), copy: tx('Buy, book or pay online, from the conversation.'), side: 'you' },
  { who: tx('The business'), title: tx('It confirms'), copy: tx('The seller receives the order or the booking, confirms it and follows it from its own space.'), side: 'us' },
]

const ACTIONS: { label: string; soon?: boolean }[] = [
  { label: tx('Buy') }, { label: tx('Book') }, { label: tx('Pay online') },
  { label: tx('Contact the seller'), soon: true }, { label: tx('Request a quote'), soon: true },
]

export function LoopSection() {
  return (
    <section className="home-section home-loop" id="how" aria-labelledby="how-title">
      <SectionHeading id="how-title" title={t('From your need to the action')} copy={t('You don’t have to search everywhere. This is what happens when you ask.')} />
      <ol className="loop">
        {STEPS.map((step, i) => (
          <li key={step.title} data-side={step.side}>
            <div className="loop-card">
              <span className="loop-who">{t(step.who)}</span>
              <h3>{t(step.title)}</h3>
              <p>{t(step.copy)}</p>
            </div>
            <span className="loop-axis" aria-hidden="true"><b>{i + 1}</b></span>
          </li>
        ))}
      </ol>
      <div className="loop-actions">
        <p id="loop-actions-label">{t('Then you can')}</p>
        <ul aria-labelledby="loop-actions-label">
          {ACTIONS.map((a) => <li key={a.label} className={a.soon ? 'soon' : undefined}><span>{t(a.label)}</span>{a.soon && <small>{t('Soon')}</small>}</li>)}
        </ul>
      </div>
    </section>
  )
}

/* ------------------------------------------------------------ businesses */

const CAPABILITIES: { label: string; copy: string }[] = [
  { label: tx('Products'), copy: tx('Show what you sell.') },
  { label: tx('Services'), copy: tx('Offer what you do.') },
  { label: tx('Orders'), copy: tx('Receive them, confirm them.') },
  { label: tx('Bookings'), copy: tx('Rooms, tables, appointments.') },
  { label: tx('Availability'), copy: tx('Keep your calendar true.') },
  { label: tx('Statistics'), copy: tx('See how your business is doing.') },
]

/** Bundled with the site (public/images), so the page never loads a picture from a third party. Source and licence: docs/web/HOME_IMAGES.md. */
const MERCHANT = '/images/merchant-1100.webp'
const MERCHANT_SET = '/images/merchant-640.webp 640w, /images/merchant-1100.webp 1100w'

export function BusinessSection() {
  return (
    <section className="home-business" id="business" aria-labelledby="business-title">
      <div className="home-business-inner">
        <div className="business-copy">
          <Heading id="business-title">{t('Have something to sell or offer?')}</Heading>
          <p>{t('Bring your products, services and availability to customers who are already looking for them.')}</p>
          <div className="business-actions">
            <ActionLink className="home-button" href="/seller">{t('Start selling on Trezutil')}<Icon name="arrow" size={18} /></ActionLink>
            <ActionLink className="home-link" href="/seller/onboarding">{t('See how it works')}</ActionLink>
          </div>
        </div>
        <img className="business-photo" src={MERCHANT} srcSet={MERCHANT_SET} sizes="(max-width: 1000px) 100vw, 42vw" width={1100} height={1100} alt={t('A shop owner at work')} loading="lazy" decoding="async" />
        <ul className="business-caps">
          {CAPABILITIES.map((c) => (
            <li key={c.label}><strong>{t(c.label)}</strong><small>{t(c.copy)}</small></li>
          ))}
        </ul>
      </div>
    </section>
  )
}
