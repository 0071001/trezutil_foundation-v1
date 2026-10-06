import { ActionLink } from '@/components/ui'
import { LEGAL, LEGAL_DOCS, type LegalDoc } from '@/lib/legal'
import { channelPlaceholders } from '@/lib/config'
import { t, tx } from '@/lib/i18n'

/* Small pieces that point at the legal pages (see src/lib/legal.ts for when they show). */

export const LEGAL_TITLES: Record<LegalDoc, string> = {
  terms: tx('Terms of use'), privacy: tx('Privacy policy'), refunds: tx('Cancellations and refunds'), notice: tx('Legal notice'),
}

/** The legal pages are linked from the footer once they are reviewed, and on staging while they are drafts. */
export function legalLinksShown(): boolean {
  return LEGAL.reviewed || channelPlaceholders()
}

/**
 * "By continuing you accept the Terms of use and the Privacy policy": shown only once a lawyer has
 * reviewed them, so nobody is told they accept a text that is still a draft.
 */
export function LegalConsent() {
  if (!LEGAL.reviewed) return null
  const [before, rest] = t('By continuing you accept the {terms} and the {privacy}.').split('{terms}')
  const [between, after] = (rest ?? '').split('{privacy}')
  return (
    <p className="auth-switch legal-consent">
      {before}<ActionLink href="/legal/terms">{t(LEGAL_TITLES.terms)}</ActionLink>{between}<ActionLink href="/legal/privacy">{t(LEGAL_TITLES.privacy)}</ActionLink>{after}
    </p>
  )
}

/** The footer's row of legal links. */
export function LegalLinks() {
  if (!legalLinksShown()) return null
  return (
    <nav className="footer-legal" aria-label={t('Legal')}>
      {LEGAL_DOCS.map((doc) => <ActionLink key={doc} href={`/legal/${doc}`}>{t(LEGAL_TITLES[doc])}</ActionLink>)}
    </nav>
  )
}
