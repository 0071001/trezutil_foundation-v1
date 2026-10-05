import { Icon, type IconName } from '@/components/ui'
import { integer } from '@/components/charts/scale'
import { TrustGlyph, type GlyphName } from '@/components/trustGlyphs'
import { getLang, t, tn, tx } from '@/lib/i18n'
import { useSiteContent } from '@/lib/useSiteContent'
import type { PublicTrustItem, SiteMetric } from '@/api/site'

/*
 * Trezutil in numbers: what the platform enables (conversations started,
 * customers and businesses brought together, what they did together, the
 * businesses that are here), never how many listings it holds. Every figure
 * is counted by the platform from its own tables (the partners, when staff
 * show them, from the partner band itself) and served by
 * GET /site/content; nothing here is written by hand, and a figure that has not
 * reached the minimum staff set for it is not shown. There is no marketing copy
 * and no promise the platform cannot keep: if the figures are unavailable, so is
 * the strip.
 */

interface MetricView {
  /** A small drawing made for the figure (the six that describe the business), or else a generic icon. */
  glyph?: GlyphName
  icon?: IconName
  tone: 'blue' | 'mint' | 'coral'
  one: string
  other: string
}

const METRICS: Record<SiteMetric, MetricView> = {
  conversations: { glyph: 'conversations', tone: 'blue', one: tx('{n} conversation started'), other: tx('{n} conversations started') },
  connections: { glyph: 'connections', tone: 'blue', one: tx('{n} customer–business connection'), other: tx('{n} customer–business connections') },
  registered_users: { glyph: 'users', tone: 'blue', one: tx('{n} user'), other: tx('{n} users') },
  merchants: { glyph: 'merchants', tone: 'blue', one: tx('{n} merchant'), other: tx('{n} merchants') },
  transactions: { glyph: 'transactions', tone: 'blue', one: tx('{n} transaction'), other: tx('{n} transactions') },
  partners: { glyph: 'partners', tone: 'blue', one: tx('{n} partner'), other: tx('{n} partners') },
  live_listings: { icon: 'shop', tone: 'blue', one: tx('{n} listing online'), other: tx('{n} listings online') },
  verified_sellers: { icon: 'shield', tone: 'mint', one: tx('{n} verified seller'), other: tx('{n} verified sellers') },
  deals_live: { icon: 'tag', tone: 'coral', one: tx('{n} listing on promotion'), other: tx('{n} listings on promotion') },
  cities_covered: { icon: 'location', tone: 'blue', one: tx('{n} city covered'), other: tx('{n} cities covered') },
  bookings_confirmed: { icon: 'calendar', tone: 'mint', one: tx('{n} confirmed booking'), other: tx('{n} confirmed bookings') },
  orders_completed: { icon: 'orders', tone: 'coral', one: tx('{n} order completed'), other: tx('{n} orders completed') },
  verticals_live: { icon: 'layers', tone: 'blue', one: tx('{n} sector with live listings'), other: tx('{n} sectors with live listings') },
}

/** The wording staff chose for the language, or the default. */
function label(item: PublicTrustItem, view: MetricView): string {
  const custom = getLang() === 'fr' ? item.labelFr : item.labelEn
  if (custom) return custom
  // The default wording is "{n} things": the number is shown apart, so only the words are kept.
  return tn(item.value, view.one, view.other, { n: '' }).replace(/\s+/g, ' ').trim()
}

/** The default wording of a metric, for the editor: "listings online". */
export function metricName(metric: SiteMetric): string {
  const view = METRICS[metric]
  return tn(2, view.one, view.other, { n: '' }).replace(/\s+/g, ' ').trim()
}

/** The row of live figures under the hero. */
export function TrustStrip() {
  const { content } = useSiteContent()
  return <TrustStripView items={content?.trust ?? []} />
}

/** The strip for the figures it is given; nothing at all when there are none. */
export function TrustStripView({ items }: { items: PublicTrustItem[] }) {
  if (items.length === 0) return null
  return (
    <section className="trust-strip" aria-label={t('Trezutil in numbers')}>
      <ul className="trust-list" data-count={items.length}>
        {items.map((item) => {
          const view = METRICS[item.metric]
          return (
            <li key={item.metric} className="trust-item" data-tone={view.tone} data-metric={item.metric}>
              <span className="trust-icon" aria-hidden="true">{view.glyph ? <TrustGlyph name={view.glyph} /> : <Icon name={view.icon ?? 'check'} size={22} />}</span>
              <div>
                <strong className="trust-value">{integer(item.value)}</strong>
                <span className="trust-copy">{label(item, view)}</span>
              </div>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
