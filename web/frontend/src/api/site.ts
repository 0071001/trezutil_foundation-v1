import { http } from '@/api/client'
import { arr, num, obj, str } from '@/api/json'

/*
 * The public site's managed content: the trust strip (live figures), the
 * partner band and the order of the home page's sections. Staff edit it in the
 * administration console; everyone reads it from GET /site/content. A figure is
 * never stored or typed: the server computes every number.
 */

export const SITE_METRICS = [
  'conversations', 'connections', 'transactions', 'merchants', 'registered_users', 'partners',
  'live_listings', 'verified_sellers', 'deals_live', 'cities_covered', 'bookings_confirmed', 'orders_completed', 'verticals_live',
] as const
export type SiteMetric = (typeof SITE_METRICS)[number]

export const PARTNER_KINDS = ['bank', 'mobile_money', 'shop', 'hotel', 'hub', 'tech', 'other'] as const
export type PartnerKind = (typeof PARTNER_KINDS)[number]

/**
 * The blocks of the home page staff can show or hide, **in the order of the
 * product narrative** (what we can help with, what exists around you, from need
 * to action, the WhatsApp invitation, figures, businesses, partners). The hero
 * is always first. Staff choose what shows, not where. The server also stores
 * older keys (showcase, services, properties, reservations) from before the
 * redesign; they are ignored here. `whatsapp` is one of them that came back as a
 * small block: the server already knew the key, so nothing changed there.
 */
export const HOME_SECTIONS = ['categories', 'nearby', 'how', 'whatsapp', 'trust', 'seller', 'partners'] as const
export type HomeSectionKey = (typeof HOME_SECTIONS)[number]

export interface SectionSetting { key: HomeSectionKey; visible: boolean }

export interface PublicTrustItem { metric: SiteMetric; value: number; labelFr: string; labelEn: string }
export interface PublicPartner { id: string; name: string; kind: PartnerKind; logoUrl: string; linkUrl: string }
export interface PublicSiteContent {
  trust: PublicTrustItem[]
  partners: PublicPartner[]
  sections: SectionSetting[]
}

export interface TrustSetting { metric: SiteMetric; enabled: boolean; minValue: number; labelFr: string; labelEn: string }
export interface PartnerSetting { id: string; name: string; kind: PartnerKind; logoUrl: string; linkUrl: string; visible: boolean }
export interface SiteDocument { trust: TrustSetting[]; partners: PartnerSetting[]; sections: SectionSetting[] }

export interface SiteAdminView {
  content: SiteDocument
  /** The version the editor read; a save must name it (0 when nothing was ever saved). */
  version: number
  stored: boolean
  updatedAt: string
  updatedByName: string
  /** The live figure behind every metric, so an editor sees what each one shows today. */
  stats: Record<string, number>
  limits: { maxTrustItems: number; maxPartners: number; maxPartnerName: number; maxTrustLabel: number; maxAssetBytes: number }
}

const isMetric = (v: string): v is SiteMetric => (SITE_METRICS as readonly string[]).includes(v)
const isKind = (v: string): v is PartnerKind => (PARTNER_KINDS as readonly string[]).includes(v)
const isSection = (v: string): v is HomeSectionKey => (HOME_SECTIONS as readonly string[]).includes(v)

function sections(value: unknown): SectionSetting[] {
  return arr(value).map((v) => ({ key: str(obj(v).key), visible: obj(v).visible !== false })).filter((s): s is SectionSetting => isSection(s.key))
}

export function mapPublicContent(value: unknown): PublicSiteContent {
  const raw = obj(value)
  return {
    trust: arr(raw.trust).map((v) => {
      const r = obj(v)
      return { metric: str(r.metric), value: num(r.value), labelFr: str(r.label_fr), labelEn: str(r.label_en) }
    }).filter((i): i is PublicTrustItem => isMetric(i.metric)),
    partners: arr(raw.partners).map((v) => {
      const r = obj(v)
      return { id: str(r.id), name: str(r.name), kind: str(r.kind), logoUrl: str(r.logo_url), linkUrl: str(r.link_url) }
    }).filter((p): p is PublicPartner => isKind(p.kind) && !!p.logoUrl),
    sections: sections(raw.sections),
  }
}

export function mapAdminView(value: unknown): SiteAdminView {
  const raw = obj(value)
  const content = obj(raw.content)
  const limits = obj(raw.limits)
  return {
    content: {
      trust: arr(content.trust).map((v) => {
        const r = obj(v)
        return { metric: str(r.metric), enabled: r.enabled === true, minValue: num(r.min_value), labelFr: str(r.label_fr), labelEn: str(r.label_en) }
      }).filter((i): i is TrustSetting => isMetric(i.metric)),
      partners: arr(content.partners).map((v) => {
        const r = obj(v)
        return { id: str(r.id), name: str(r.name), kind: str(r.kind), logoUrl: str(r.logo_url), linkUrl: str(r.link_url), visible: r.visible === true }
      }).filter((p): p is PartnerSetting => isKind(p.kind)),
      sections: sections(content.sections),
    },
    version: num(raw.version), stored: raw.stored === true, updatedAt: str(raw.updated_at), updatedByName: str(raw.updated_by_name),
    stats: Object.fromEntries(Object.entries(obj(raw.stats)).map(([k, v]) => [k, num(v)])),
    limits: {
      maxTrustItems: num(limits.max_trust_items) || 8, maxPartners: num(limits.max_partners) || 40, maxPartnerName: num(limits.max_partner_name) || 60,
      maxTrustLabel: num(limits.max_trust_label) || 40, maxAssetBytes: num(limits.max_asset_bytes) || 512 * 1024,
    },
  }
}

/** The document in the shape the server accepts. */
export function toRequest(doc: SiteDocument, expectedVersion: number): Record<string, unknown> {
  return {
    expected_version: expectedVersion,
    trust: doc.trust.map((t) => ({ metric: t.metric, enabled: t.enabled, min_value: t.minValue, label_fr: t.labelFr, label_en: t.labelEn })),
    partners: doc.partners.map((p) => ({ id: p.id, name: p.name, kind: p.kind, logo_url: p.logoUrl, link_url: p.linkUrl, visible: p.visible })),
    sections: doc.sections.map((s) => ({ key: s.key, visible: s.visible })),
  }
}

export const siteApi = {
  /** Public: what the home page shows. */
  async content(): Promise<PublicSiteContent> {
    return mapPublicContent(await http.get<unknown>('/site/content'))
  },
  /** Administrators: the editable document and the live figures. */
  async adminContent(): Promise<SiteAdminView> {
    return mapAdminView(await http.get<unknown>('/admin/site/content'))
  },
  /** Saves the document; fails with 409 if somebody saved since `expectedVersion`. */
  async save(doc: SiteDocument, expectedVersion: number): Promise<SiteAdminView> {
    return mapAdminView(await http.put<unknown>('/admin/site/content', toRequest(doc, expectedVersion)))
  },
  /** Uploads a PNG, JPEG or WebP logo and returns the address to store in a partner. */
  async uploadLogo(file: File): Promise<{ url: string }> {
    const raw = obj(await http.upload<unknown>('/admin/site/assets', 'file', file))
    return { url: str(raw.url) }
  },
}
