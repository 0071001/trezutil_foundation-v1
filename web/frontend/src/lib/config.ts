/**
 * Build-time public configuration (Vite `VITE_*` variables). Nothing here is
 * secret: values end up in the browser bundle.
 *
 *   VITE_API_BASE_URL     API base (default "/api/v1", same origin as the gateway)
 *   VITE_WHATSAPP_NUMBER  public WhatsApp Business number, digits only (e.g. 243810000000);
 *                         when unset, WhatsApp calls-to-action fall back to the web assistant.
 */

export function whatsappHref(text?: string): string | null {
  const number = String(import.meta.env.VITE_WHATSAPP_NUMBER ?? '').replace(/\D/g, '')
  if (!number) return null
  return `https://wa.me/${number}${text ? `?text=${encodeURIComponent(text)}` : ''}`
}
