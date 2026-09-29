import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { Outlet } from 'react-router'
import { MANAGER_ROLES, sellerApi, type SellerOrganization } from '@/api/seller'
import { AppShell, initials } from '@/components/shell'
import { ErrorState, Loading } from '@/components/states'
import { ActionLink, Icon, type IconName } from '@/components/ui'
import { useAsync } from '@/lib/useAsync'
import { SellerOnboardingPage } from '@/pages/seller/SellerOnboarding'

/*
 * Figma SellerShell. The workspace only calls platform APIs that enforce
 * organization membership and role server-side; hiding an action here is a
 * convenience, never the security boundary.
 */

interface SellerCtx {
  orgs: SellerOrganization[]
  org: SellerOrganization
  canManage: boolean
  selectOrg: (id: string) => void
  reload: () => void
}

const SellerContext = createContext<SellerCtx | null>(null)

export function useSeller(): SellerCtx {
  const ctx = useContext(SellerContext)
  if (!ctx) throw new Error('useSeller outside SellerLayout')
  return ctx
}

const ORG_KEY = 'trezutil_seller_org'
function storedOrg(): string {
  try { return localStorage.getItem(ORG_KEY) ?? '' } catch { return '' }
}

const links: [string, string, IconName, boolean?][] = [
  ['Overview', '/seller', 'home', true],
  ['Listings', '/seller/listings', 'shop'],
  ['Orders', '/seller/orders', 'orders'],
  ['Reservations', '/seller/reservations', 'calendar'],
  ['Inventory', '/seller/inventory', 'briefcase'],
  ['Business', '/seller/business', 'building'],
]

function SellerNav({ org, orgs, onSelect }: { org: SellerOrganization; orgs: SellerOrganization[]; onSelect: (id: string) => void }) {
  return (
    <aside className="seller-nav" aria-label="Seller workspace">
      <div><span className="seller-store-logo" aria-hidden="true">{initials(org.display_name)}</span><div><strong>{org.display_name}</strong><small>{org.role ? `${org.role[0].toUpperCase()}${org.role.slice(1)}` : 'Seller account'}</small></div></div>
      {orgs.length > 1 && (
        <label className="field seller-org-picker"><span>Business</span>
          <select value={org.organization_id} onChange={(e) => onSelect(e.target.value)}>{orgs.map((o) => <option key={o.organization_id} value={o.organization_id}>{o.display_name}</option>)}</select>
        </label>
      )}
      <nav>{links.map(([label, href, icon, end]) => <ActionLink nav end={end} href={href} key={label}><Icon name={icon} />{label}</ActionLink>)}</nav>
      <ActionLink className="seller-back" href="/"><Icon name="chevron" />Back to Trezutil</ActionLink>
    </aside>
  )
}

/** Horizontal workspace tabs for small screens (the Figma sidebar is hidden there). */
function SellerMobileTabs() {
  return <nav className="seller-mobile-tabs" aria-label="Seller workspace">{links.map(([label, href, , end]) => <ActionLink nav end={end} href={href} key={label}>{label}</ActionLink>)}</nav>
}

export function SellerLayout() {
  const context = useAsync(() => sellerApi.context(), [])
  const [selected, setSelected] = useState(storedOrg)
  useEffect(() => { document.title = 'Seller workspace — Trezutil' }, [])

  let body: ReactNode
  if (context.loading) body = <Loading label="Loading your business…" />
  else if (context.error) body = <div className="seller-state"><ErrorState error={context.error} onRetry={context.reload} /></div>
  else if (!context.data?.length) body = <SellerOnboardingPage />
  else {
    const orgs = context.data
    const org = orgs.find((o) => o.organization_id === selected) ?? orgs[0]
    const selectOrg = (id: string) => { setSelected(id); try { localStorage.setItem(ORG_KEY, id) } catch { /* ignore */ } }
    return (
      <AppShell hideDock>
        <SellerContext.Provider value={{ orgs, org, canManage: MANAGER_ROLES.has(org.role), selectOrg, reload: context.reload }}>
          <div className="seller-app">
            <SellerNav org={org} orgs={orgs} onSelect={selectOrg} />
            <div className="seller-main"><SellerMobileTabs /><Outlet key={org.organization_id} /></div>
          </div>
        </SellerContext.Provider>
      </AppShell>
    )
  }
  return <AppShell hideDock>{body}</AppShell>
}

export function Metric({ label, value, note, icon }: { label: string; value: string; note: string; icon: IconName }) {
  return <div className="metric"><span><Icon name={icon} /></span><small>{label}</small><strong>{value}</strong><p>{note}</p></div>
}

export function SellerHead({ eyebrow, title, copy, action }: { eyebrow: string; title: string; copy: string; action?: ReactNode }) {
  return <div className="seller-page-head"><div><div className="section-eyebrow">{eyebrow}</div><h1 className="heading heading-1">{title}</h1><p>{copy}</p></div>{action}</div>
}

export function ViewOnly() {
  return <p className="inline-notice info" role="note"><Icon name="user" size={16} /><span>You can view this business. Only its owner, admins and managers can make changes.</span></p>
}
