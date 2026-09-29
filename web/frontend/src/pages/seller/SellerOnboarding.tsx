import { useEffect } from 'react'
import { AppShell } from '@/components/shell'
import { ActionLink, Heading, Icon } from '@/components/ui'
import { useSession } from '@/lib/session'

/**
 * Seller onboarding is operator-assisted in the MVP (business verification,
 * organization, store and seller profile are created with the Trezutil team);
 * there is no self-serve onboarding API. This page explains the steps and is
 * shown to any signed-in account that is not yet a member of a business.
 */
export function SellerOnboardingPage() {
  const { user } = useSession()
  useEffect(() => { document.title = 'Sell on Trezutil' }, [])
  const steps = [
    ['Tell us about your business', 'Share your business name, what you sell and where. We verify every seller before they go live.'],
    ['We set up your workspace', 'The Trezutil team creates your business, store and seller profile, and adds your phone number as the owner.'],
    ['Create your listings', 'Add products, services or places from this workspace. They stay drafts until you publish.'],
    ['Sell and take bookings', 'Customers find you in search, in the app and on WhatsApp. Confirm orders and follow bookings here.'],
  ]
  return (
    <section className="page-section seller-onboarding">
      <div className="register-intro onboarding-intro">
        <span className="empty-mark"><Icon name="shop" /></span>
        <div className="section-eyebrow">For local businesses</div>
        <Heading level={1}>Your business.<br /><em>More discoverable.</em></Heading>
        <p>Put your products and services in front of people who are actually looking for them.</p>
      </div>
      <ol className="onboarding-steps">
        {steps.map(([title, copy], i) => <li key={title}><span>{i + 1}</span><div><strong>{title}</strong><p>{copy}</p></div></li>)}
      </ol>
      <div className="onboarding-cta">
        {user
          ? <p>Your account (<strong>{user.phone}</strong>) isn’t linked to a business yet. Contact the Trezutil team with this number to start — once you’re added, this workspace opens automatically.</p>
          : <p>Sign in with the phone number you want to use for your business.</p>}
        <div className="checkout-buttons">
          {!user && <ActionLink className="primary-button" href="/login?returnTo=%2Fseller">Sign in <Icon name="arrow" /></ActionLink>}
          <ActionLink className="outline-button" href="/conversation?q=I%20want%20to%20sell%20on%20Trezutil">Ask Trezutil</ActionLink>
        </div>
      </div>
    </section>
  )
}

/** Public route (/seller/onboarding) — the same page inside the app shell. */
export function SellerOnboardingRoute() {
  return <AppShell hideDock><SellerOnboardingPage /></AppShell>
}
