import { t } from '@/lib/i18n'
/*
 * The facts the legal pages need that the code cannot know: who operates Trezutil, where it is registered, how to
 * reach it, how long data is kept. Nothing here is invented: an empty fact shows on the page as a highlighted
 * "[À compléter : …]". Fill them in, have a lawyer review the texts, then set `reviewed` to true (the draft banner
 * goes and the sign-in screen states that continuing accepts the terms). See docs/legal/LEGAL_CHECKLIST.md.
 */

export interface LegalEntity {
  /** The company that operates Trezutil, and its legal form (SARL, SA…). */
  name: string
  form: string
  /** Registered office. */
  address: string
  /** RCCM, national identification number, tax number (NIF). */
  registration: string
  /** The person responsible for the publication of the site. */
  director: string
  /** Where people write about anything legal or about their personal data. */
  email: string
  phone: string
  /** The law that governs the terms and the courts that hear disputes. */
  jurisdiction: string
}

export interface LegalConfig {
  /** True only once a lawyer has reviewed the texts and every fact below is filled in. */
  reviewed: boolean
  /** Last change of the texts (ISO date). */
  updated: string
  entity: LegalEntity
  /** Hosting: known from the server itself. */
  hosting: string
  minimumAge: string
  /** How long accounts, conversations, orders and logs are kept. */
  retention: string
  /** The data-protection authority people may complain to. */
  authority: string
  /** Time to ask for a refund, and time before the money arrives. */
  refundWindow: string
  refundDelay: string
  /** What may not be offered on Trezutil. */
  prohibitedItems: string
  /** What Trezutil charges suppliers (commission, subscription), if anything. */
  supplierFees: string
}

export const LEGAL: LegalConfig = {
  reviewed: false,
  updated: '2026-11-23',
  entity: { name: '', form: '', address: '', registration: '', director: '', email: '', phone: '', jurisdiction: '' },
  // Verified on the production server (2026-11-23): a virtual server rented from Contabo GmbH, located in France.
  hosting: t('Contabo GmbH (Allemagne) — serveur situé à Lauterbourg, en France (Union européenne)'),
  minimumAge: '',
  retention: '',
  authority: '',
  refundWindow: '',
  refundDelay: '',
  prohibitedItems: '',
  supplierFees: '',
}

/** The legal pages, in the order they are listed (slug → title). */
export const LEGAL_DOCS = {
  conditions: t('Conditions générales d’utilisation'),
  confidentialite: t('Politique de confidentialité'),
  cookies: t('Cookies et stockage local'),
  remboursements: t('Annulations et remboursements'),
  fournisseurs: t('Conditions fournisseurs'),
  mentions: t('Mentions légales'),
} as const
export type LegalDoc = keyof typeof LEGAL_DOCS
export const isLegalDoc = (value: string | undefined): value is LegalDoc => !!value && value in LEGAL_DOCS
