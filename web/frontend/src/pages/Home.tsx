import { Fragment, useEffect, type ReactNode } from 'react'
import type { HomeSectionKey } from '@/api/site'
import { BusinessSection, DiscoverySection, LoopSection, NeedsSection } from '@/components/home/sections'
import { HomeFrame } from '@/components/home/chrome'
import { HeroBackdrop } from '@/components/home/HeroBackdrop'
import { NeedComposer } from '@/components/home/NeedComposer'
import { WhatsAppSection } from '@/components/home/WhatsAppSection'
import { PartnersBand } from '@/components/partners'
import { TrustStrip } from '@/components/trust'
import { Heading } from '@/components/ui'
import { whatsappHref } from '@/lib/config'
import { DEFAULT_SECTIONS, useSiteContent } from '@/lib/useSiteContent'
import { t } from '@/lib/i18n'

/*
 * The front door. Trezutil is not a catalogue: a person arrives with a need,
 * says it, and Trezutil turns it into an action (buy, book, contact, request).
 * The page follows that experience (you, Trezutil, what exists, what you can
 * do, the businesses) and shows offers only as the supply layer. The reasoning,
 * the audit of the previous page and the open points are in
 * docs/web/HOME_PAGE_REDESIGN.md.
 */

/** The relief moment: "What do you need today?" and a place to say it. */
function Hero() {
  const wa = whatsappHref()
  return (
    <section className="home-hero" id="top" aria-labelledby="hero-title">
      <HeroBackdrop />
      <div className="home-hero-inner">
        <div className="home-hero-top">
          <Heading level={1} id="hero-title" className="home-title">{t('What do you need today?')}</Heading>
          <p className="home-lede">{t('Tell us what you need. We’ll help you find it.')}</p>
        </div>
        <NeedComposer />
        <p className="home-reassure">
          {t('You don’t have to search everywhere. Just tell Trezutil what you need.')}
          {wa && <> <a className="home-link" href={wa} target="_blank" rel="noopener noreferrer">{t('Prefer WhatsApp?')}</a></>}
        </p>
      </div>
    </section>
  )
}

/**
 * The blocks below the hero. Their order is the product narrative, fixed in
 * code (src/api/site.ts HOME_SECTIONS): administrators choose what shows, not
 * where, so the story cannot be scrambled. Until the managed settings arrive
 * (or if they cannot), everything shows.
 */
const SECTION_COMPONENTS: Record<HomeSectionKey, () => ReactNode> = {
  categories: () => <NeedsSection />,
  nearby: () => <DiscoverySection />,
  how: () => <LoopSection />,
  whatsapp: () => <WhatsAppSection />,
  trust: () => <TrustStrip />,
  seller: () => <BusinessSection />,
  partners: () => <PartnersBand />,
}

function HomeSections() {
  const { content } = useSiteContent()
  const visible = new Map((content?.sections ?? []).map((s) => [s.key, s.visible]))
  return <>{DEFAULT_SECTIONS.filter((s) => visible.get(s.key) !== false).map((s) => <Fragment key={s.key}>{SECTION_COMPONENTS[s.key]()}</Fragment>)}</>
}

export default function HomePage() {
  useEffect(() => { document.title = t('Trezutil — Tell us what you need') }, [])
  return (
    <HomeFrame>
      <Hero />
      <HomeSections />
    </HomeFrame>
  )
}
