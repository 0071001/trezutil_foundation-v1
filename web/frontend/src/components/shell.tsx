import type { ReactNode } from 'react'
import { MobileNav, SiteHeader } from '@/components/header'
import { OfflineBanner } from '@/components/states'
import { usePrefs } from '@/lib/prefs'

export { LanguageSwitch, initials } from '@/components/header'

/**
 * The frame of a product page: the header, the page, the phone's bottom bar. There is no "Ask Trezutil about this
 * page" bar across the page any more: the header's request button is the one way into the conversation, and the pages
 * that have a question of their own (a listing, an order, a booking, a search) offer it inline.
 */
export function AppShell({ children, compact = false }: { children: ReactNode; compact?: boolean }) {
  const { theme } = usePrefs()
  return (
    <div className={`app product-app ${theme === 'dark' ? 'dark' : ''}`}>
      <SiteHeader />
      <OfflineBanner />
      <main id="main" tabIndex={-1} className={compact ? 'app-content compact-content' : 'app-content'}>{children}</main>
      <MobileNav />
    </div>
  )
}
