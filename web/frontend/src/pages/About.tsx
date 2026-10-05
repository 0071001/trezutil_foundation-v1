import { useEffect } from 'react'
import type { HomeSectionKey } from '@/api/site'
import { HomeFrame } from '@/components/home/chrome'
import { LoopSection, SectionHeading } from '@/components/home/sections'
import { PartnersBand } from '@/components/partners'
import { TrustStrip } from '@/components/trust'
import { ActionLink, Heading, Icon } from '@/components/ui'
import { whatsappHref } from '@/lib/config'
import { t, tx } from '@/lib/i18n'
import { useSiteContent } from '@/lib/useSiteContent'

/*
 * "Our company": what Trezutil is, in the words the rest of the site already
 * uses, with nothing that cannot be checked (no team, dates or addresses are
 * made up). The five steps, the live figures and the partners are the home
 * page's own blocks, so they say the same thing in both places and follow the
 * administrators' choices of what to show. See docs/web/OUR_COMPANY.md.
 */

const POINTS: { title: string; copy: string }[] = [
  { title: tx('You start from a need, not a catalogue'), copy: tx('Most sites ask you to search through lists. Trezutil starts from what you say you need, and asks only what it must to find the right thing.') },
  { title: tx('Offers come from real businesses'), copy: tx('Every offer you see is published by a business on Trezutil, with its price and, where it applies, its availability.') },
  { title: tx('You can act on it'), copy: tx('Buy, book or pay online without leaving the conversation. Contacting a seller and asking for a quote are still on their way.') },
]

export default function AboutPage() {
  const { content } = useSiteContent()
  const wa = whatsappHref()
  // the blocks an administrator hides from the home page stay hidden here too
  const shown = (key: HomeSectionKey) => (content?.sections ?? []).find((s) => s.key === key)?.visible !== false
  useEffect(() => { document.title = `${t('About Trezutil')} — Trezutil` }, [])
  return (
    <HomeFrame>
      <section className="home-section about-hero" aria-labelledby="about-title">
        <p className="about-eyebrow">{t('Our company')}</p>
        <Heading level={1} id="about-title" className="about-title">{t('Commerce through conversation.')}</Heading>
        <p className="about-lede">{t('Trezutil helps people find what they need and act on it: a place to live, something to buy, someone to call, a table for tonight. Trezutil currently serves Kinshasa.')}</p>
        <ul className="about-points">
          {POINTS.map((point) => <li key={point.title}><h2>{t(point.title)}</h2><p>{t(point.copy)}</p></li>)}
        </ul>
      </section>
      <LoopSection />
      {shown('trust') && <TrustStrip />}
      {shown('partners') && <PartnersBand />}
      <section className="home-section about-contact" id="contact" aria-labelledby="contact-title">
        <SectionHeading id="contact-title" title={t('Contact us')} copy={t('The quickest way to reach Trezutil is to write to it. Ask a question, or describe what you need, in your own words.')} />
        <div className="about-actions">
          <ActionLink className="home-button" href="/conversation">{t('Make a request')}<Icon name="arrow" size={18} /></ActionLink>
          {wa && <ActionLink className="home-link" href={wa}>{t('Find us on WhatsApp')}</ActionLink>}
        </div>
        <p className="about-business">{t('Have something to sell or offer?')} <ActionLink className="home-link" href="/seller/onboarding">{t('Start selling on Trezutil')}</ActionLink></p>
      </section>
    </HomeFrame>
  )
}
