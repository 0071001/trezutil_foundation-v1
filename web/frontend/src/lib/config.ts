/**
 * Build-time public configuration (Vite `VITE_*` variables). Nothing here is
 * secret: values end up in the browser bundle.
 *
 *   VITE_API_BASE_URL     API base (default "/api/v1", same origin as the gateway)
 *   VITE_WHATSAPP_NUMBER  public WhatsApp Business number, digits only (e.g. 243810000000);
 *                         when unset, WhatsApp calls-to-action fall back to the web assistant.
 *   VITE_PLAY_STORE_URL   the public page of the mobile app on the Play Store (https), once it is published
 *   VITE_APP_STORE_URL    the public page of the mobile app on the App Store (https), once it is published
 *                         (a store that is not configured is simply not mentioned: nothing is claimed that does not exist)
 *   VITE_CHANNEL_PLACEHOLDERS  "true" shows placeholders for the channels that are not connected yet (the footer line
 *                         "Available on WhatsApp, PlayStore and AppStore", a QR code frame, the WhatsApp section), so the
 *                         layout can be seen before the real values exist. Staging and local work only: leave it unset in
 *                         production, where a channel that does not exist is never mentioned.
 *   VITE_APP_ENV          environment label shown in the administration header ("staging", "development"…);
 *                         empty or "production" shows nothing. A label only: it grants and decides nothing.
 */

/** The environment label to show in the console, or '' for production / unset. */
export function appEnvLabel(): string {
  const v = String(import.meta.env.VITE_APP_ENV ?? '').trim().toLowerCase()
  return v === 'production' || v === 'prod' ? '' : v.slice(0, 24)
}

export function whatsappHref(text?: string): string | null {
  const number = String(import.meta.env.VITE_WHATSAPP_NUMBER ?? '').replace(/\D/g, '')
  if (!number) return null
  return `https://wa.me/${number}${text ? `?text=${encodeURIComponent(text)}` : ''}`
}

/** An https address, or null (a mistyped, empty or non-https value is ignored rather than linked). */
function httpsUrl(value: unknown): string | null {
  try {
    const url = new URL(String(value ?? '').trim())
    return url.protocol === 'https:' ? url.toString() : null
  } catch {
    return null
  }
}

/** Where the mobile app can be installed, for the stores it is really published on. */
export function storeLinks(): { playStore: string | null; appStore: string | null } {
  return { playStore: httpsUrl(import.meta.env.VITE_PLAY_STORE_URL), appStore: httpsUrl(import.meta.env.VITE_APP_STORE_URL) }
}

/** Whether to show placeholders for channels that are not connected yet (see VITE_CHANNEL_PLACEHOLDERS). */
export function channelPlaceholders(): boolean {
  return String(import.meta.env.VITE_CHANNEL_PLACEHOLDERS ?? '').trim().toLowerCase() === 'true'
}
