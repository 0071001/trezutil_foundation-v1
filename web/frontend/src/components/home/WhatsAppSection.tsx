import { ChannelQr, reach } from '@/components/channels'
import { ActionLink, Heading, Icon } from '@/components/ui'
import { whatsappHref } from '@/lib/config'
import { t, tx } from '@/lib/i18n'

/*
 * A small invitation to write to Trezutil on WhatsApp. It says only what the
 * conversation does (it understands a free-text need, finds and compares
 * offers, checks availability) and nothing about speed, price or support the
 * platform does not promise. It exists only for a channel that exists: with the
 * WhatsApp number set it is the real thing; with placeholders on (staging) it
 * is the same layout with a disabled button and a QR frame; otherwise it is not
 * drawn. Administrators can hide it with the other home blocks (Site content).
 */

const POINTS = [tx('Nothing to install'), tx('No sign-up form: your number is enough'), tx('In French or English')]

export function WhatsAppSection() {
  const { qr } = reach()
  if (qr === null) return null
  const chat = whatsappHref(t('Hello Trezutil! I need some help.'))
  return (
    <section className="home-section home-whatsapp" id="whatsapp" aria-labelledby="whatsapp-title">
      <div className="invite-panel">
        <div className="invite-copy">
          <p className="invite-eyebrow"><Icon name="whatsapp" size={18} />{t('On WhatsApp')}</p>
          <Heading id="whatsapp-title">{t('Trezutil, right in your WhatsApp.')}</Heading>
          <p>{t('Say what you need in a message, the way you would to a friend. Trezutil finds what fits, compares it and checks availability, right in the chat.')}</p>
          <ul className="invite-points">
            {POINTS.map((point) => <li key={point}><Icon name="check" size={16} />{t(point)}</li>)}
          </ul>
          <div className="invite-actions">
            {chat
              ? <ActionLink className="home-button invite-button" href={chat}><Icon name="whatsapp" size={18} />{t('Write to Trezutil on WhatsApp')}</ActionLink>
              : <>
                  <button type="button" className="home-button invite-button" disabled aria-describedby="whatsapp-soon"><Icon name="whatsapp" size={18} />{t('Write to Trezutil on WhatsApp')}</button>
                  <small id="whatsapp-soon">{t('The WhatsApp number is coming soon.')}</small>
                </>}
          </div>
        </div>
        <figure className="invite-qr">
          <ChannelQr whatsapp={chat ? whatsappHref() : null} />
          <figcaption>{chat ? t('Scan to chat with Trezutil on WhatsApp') : t('QR code coming soon')}</figcaption>
        </figure>
      </div>
    </section>
  )
}
