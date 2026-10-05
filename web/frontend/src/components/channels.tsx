import { QrCode } from '@/components/QrCode'
import { channelPlaceholders, storeLinks, whatsappHref } from '@/lib/config'
import { t } from '@/lib/i18n'

/*
 * The channels Trezutil can be reached on, and what is real about each. WhatsApp
 * is real when its number is configured, a store when its public page is
 * (src/lib/config.ts). A channel that is not real is shown only as a
 * placeholder, and only when placeholders are switched on (staging): that lets
 * the layout be seen before the values exist, and a production build without
 * them never mentions an app or a number that does not exist.
 * See docs/web/OUR_COMPANY.md.
 */

export interface Channel {
  name: 'WhatsApp' | 'PlayStore' | 'AppStore'
  /** Where it leads; null for a placeholder, which is text and not a link. */
  href: string | null
}

export interface Reach {
  /** The channels to name: the real ones, plus the placeholders when those are on. */
  channels: Channel[]
  /** The WhatsApp chat address, when the number is configured. */
  whatsapp: string | null
  /** What to draw for the WhatsApp QR code: the real one, a placeholder frame, or nothing. */
  qr: 'live' | 'placeholder' | null
}

export function reach(): Reach {
  const placeholders = channelPlaceholders()
  const whatsapp = whatsappHref()
  const { playStore, appStore } = storeLinks()
  const all: Channel[] = [{ name: 'WhatsApp', href: whatsapp }, { name: 'PlayStore', href: playStore }, { name: 'AppStore', href: appStore }]
  return { channels: all.filter((c) => c.href !== null || placeholders), whatsapp, qr: whatsapp ? 'live' : placeholders ? 'placeholder' : null }
}

/**
 * The frame of a QR code, with no data in it: three finder squares and a speech
 * bubble, in grey. It cannot be scanned (it holds nothing), which is the point:
 * it shows where the code will go without sending anyone to a number nobody set.
 */
export function QrPlaceholder() {
  const finder = (x: number, y: number) => (
    <g key={`${x}-${y}`}>
      <path fillRule="evenodd" d={`M${x} ${y}h7v7h-7zM${x + 1} ${y + 1}v5h5v-5z`} />
      <rect x={x + 2} y={y + 2} width="3" height="3" />
    </g>
  )
  return (
    <svg className="qr qr-placeholder" viewBox="0 0 33 33" role="img" aria-label={t('QR code coming soon')} shapeRendering="crispEdges">
      <rect width="33" height="33" fill="#ffffff" />
      <g fill="#aab6c2">
        {finder(4, 4)}{finder(22, 4)}{finder(4, 22)}
        <path d="M13 12h7a1.5 1.5 0 0 1 1.5 1.5v4.5a1.5 1.5 0 0 1-1.5 1.5h-3.5l-2.5 2.5v-2.5h-1a1.5 1.5 0 0 1-1.5-1.5v-4.5a1.5 1.5 0 0 1 1.5-1.5z" />
      </g>
    </svg>
  )
}

/** The WhatsApp QR code: the real one when the number is set, otherwise its placeholder frame. */
export function ChannelQr({ whatsapp }: { whatsapp: string | null }) {
  return whatsapp
    ? <QrCode text={whatsapp} label={t('QR code that opens the WhatsApp chat with Trezutil')} />
    : <QrPlaceholder />
}
