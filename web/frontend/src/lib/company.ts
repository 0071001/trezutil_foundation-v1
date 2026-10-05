import { tx } from '@/lib/i18n'

/*
 * "Our company": the footer column about Trezutil itself, as opposed to the needs
 * it helps with (src/lib/needs.ts) which make up the primary menu. It is kept out
 * of the primary menu on purpose: that menu is for what a person came to do.
 * Every entry leads to something that exists today; entries for pages that do
 * not exist yet (team, careers, press) are not shown. See docs/web/OUR_COMPANY.md.
 */

export const COMPANY_LABEL = tx('Our company')

export const COMPANY_LINKS: [label: string, href: string][] = [
  [tx('About Trezutil'), '/about'],
  [tx('How it works'), '/about#how'],
  [tx('Contact us'), '/about#contact'],
]
