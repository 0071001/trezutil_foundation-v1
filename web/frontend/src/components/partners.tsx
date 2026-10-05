import { useState } from 'react'
import type { PublicPartner } from '@/api/site'
import { Icon } from '@/components/ui'
import { t } from '@/lib/i18n'
import { useSiteContent } from '@/lib/useSiteContent'

/*
 * The partner band. The partners and their logos are managed by staff in the
 * administration console (Site content); this only draws what the server sends.
 * A logo is stored with the platform, never loaded from a partner's own site.
 */

/** One logo. A tall or square mark is given more height so it reads at the same weight as a wide one. */
function Logo({ partner, copy }: { partner: PublicPartner; copy: boolean }) {
  const [tall, setTall] = useState(false)
  const image = (
    <img
      src={partner.logoUrl}
      alt={copy ? '' : partner.name}
      decoding="async"
      draggable={false}
      onLoad={(e) => setTall(e.currentTarget.naturalWidth > 0 && e.currentTarget.naturalHeight / e.currentTarget.naturalWidth > 0.45)}
    />
  )
  return (
    <li className="partner" data-kind={partner.kind} data-tall={tall || undefined}>
      {partner.linkUrl
        ? <a href={partner.linkUrl} target="_blank" rel="noopener noreferrer" tabIndex={copy ? -1 : undefined} aria-label={copy ? undefined : t('{name} (opens its website)', { name: partner.name })}>{image}</a>
        : image}
    </li>
  )
}

/** One loop of logos. The second loop is a visual copy for the seamless scroll and is hidden from assistive technology. */
function Track({ partners, copy = false }: { partners: PublicPartner[]; copy?: boolean }) {
  return (
    <ul className="partners-track" aria-hidden={copy || undefined}>
      {partners.map((p) => <Logo key={p.id} partner={p} copy={copy} />)}
    </ul>
  )
}

/**
 * A band of partner logos that scrolls on its own. It pauses on hover and
 * keyboard focus, has a pause button (moving content must be stoppable), and
 * holds still, wrapped, for people who ask their system to reduce motion.
 * With no partners to show, there is no band.
 */
export function PartnersBand() {
  const { content } = useSiteContent()
  return <PartnersBandView partners={content?.partners ?? []} />
}

/** The band for the partners it is given (also the editor's preview); nothing at all when there are none. */
export function PartnersBandView({ partners }: { partners: PublicPartner[] }) {
  const [paused, setPaused] = useState(false)
  if (partners.length === 0) return null
  return (
    <section className="partners-band" aria-labelledby="partners-title">
      <div className="partners-head">
        <div>
          <p className="section-eyebrow" id="partners-title">{t('Our partners')}</p>
          <p className="partners-sub">{t('Banks, mobile money, shops, hotels and innovation')}</p>
        </div>
        <button type="button" className="partners-pause" aria-pressed={paused} aria-label={paused ? t('Resume scrolling') : t('Pause scrolling')} onClick={() => setPaused((p) => !p)}>
          <Icon name={paused ? 'play' : 'pause'} size={14} />
        </button>
      </div>
      <div className="partners-marquee" data-paused={paused || undefined} style={{ ['--partners-duration' as string]: `${Math.max(20, partners.length * 3.4)}s` }}>
        <Track partners={partners} />
        <Track partners={partners} copy />
      </div>
    </section>
  )
}
