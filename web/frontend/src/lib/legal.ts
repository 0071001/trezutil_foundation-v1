/*
 * The facts the legal pages need and the repository cannot know: who the company
 * is, where it is registered, how to reach it, how long data is kept. Fill them
 * in, have the texts reviewed by a lawyer, then set `reviewed` to true.
 *
 * Until then:
 *   - every missing fact shows on the page as a highlighted "[To complete: …]",
 *     never as an invented value;
 *   - the pages carry a banner saying the text is a draft;
 *   - the footer links to them only on staging (placeholders on), and the
 *     sign-in form does not tell people they accept them.
 * See docs/legal/LEGAL_CHECKLIST.md.
 */

export interface LegalEntity {
  /** The company that operates Trezutil. */
  name: string
  /** Its legal form (SARL, SA…). */
  form: string
  /** Its registered address. */
  address: string
  /** Its registration numbers (trade register, national id, tax id). */
  registration: string
  /** The person responsible for the publication of the site. */
  director: string
  /** Where people write for anything legal or about their data. */
  email: string
  phone: string
  /** Who hosts the service (company and country). */
  hosting: string
  /** The law that governs the terms and the courts that hear disputes. */
  jurisdiction: string
}

export interface LegalConfig {
  /** True only once a lawyer has reviewed the texts and every fact below is filled in. */
  reviewed: boolean
  /** The day the texts were last changed (ISO). */
  updated: string
  entity: LegalEntity
  /** The minimum age to hold an account. */
  minimumAge: string
  /** How long conversations, orders and accounts are kept after the last activity. */
  retention: string
  /** The authority people may complain to about their data. */
  authority: string
  /** How long a customer has to ask for a refund, and how long a refund takes. */
  refundWindow: string
  refundDelay: string
  /** What may not be sold on Trezutil. */
  prohibitedItems: string
}

export const LEGAL: LegalConfig = {
  reviewed: false,
  updated: '2026-10-05',
  entity: { name: '', form: '', address: '', registration: '', director: '', email: '', phone: '', hosting: '', jurisdiction: '' },
  minimumAge: '',
  retention: '',
  authority: '',
  refundWindow: '',
  refundDelay: '',
  prohibitedItems: '',
}

/** The legal pages, in the order they are listed. */
export const LEGAL_DOCS = ['terms', 'privacy', 'refunds', 'notice'] as const
export type LegalDoc = (typeof LEGAL_DOCS)[number]
export const isLegalDoc = (value: string | undefined): value is LegalDoc => (LEGAL_DOCS as readonly string[]).includes(value ?? '')
