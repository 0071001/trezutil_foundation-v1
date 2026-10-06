import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Outlet } from 'react-router'
import { MANAGER_ROLES, type SellerOrganization } from '@/api/seller'
import { PortalShell } from '@/components/portal/PortalShell'
import { PortalSwitcher } from '@/components/portal/PortalSwitcher'
import { ModuleHead, PlannedState, UnavailableState } from '@/components/portal/ModuleState'
import { AppShell, initials } from '@/components/shell'
import { ErrorState, Loading } from '@/components/states'
import { Icon } from '@/components/ui'
import { t } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { usePortals } from '@/portal/PortalsProvider'
import { allows, sellerGroups, sellerModules, shellGroups } from '@/portal/modules'
import { SellerOnboardingPage } from '@/pages/seller/SellerOnboarding'

export { Metric } from '@/components/portal/Metric'

/*
 * The seller portal ("run my business"). The workspace only calls platform
 * APIs that enforce organization membership and role server-side; hiding an
 * action here is a convenience, never the security boundary.
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

/** The four modules on the phone tab bar; "More" opens the full rail. */
const TAB_IDS = ['dashboard', 'orders', 'reservations', 'listings']

/** Organization roles, as literal keys so each has a French entry. */
function roleLabel(role: string): string {
  switch (role) {
    case 'owner': return t('Owner')
    case 'admin': return t('Admin')
    case 'manager': return t('Manager')
    case 'staff': return t('Staff')
    case 'accountant': return t('Accountant')
    case 'sales': return t('Sales')
    case 'operations': return t('Operations')
    default: return role ? `${role[0].toUpperCase()}${role.slice(1)}` : t('Seller')
  }
}

/** Neutral frame while the businesses load: no navigation yet, no customer header flash. */
function PortalLoading({ label }: { label: string }) {
  return <div className="app portal portal-seller tz-boot"><main id="main" tabIndex={-1} className="tz-content"><Loading label={label} /></main></div>
}

export function SellerLayout() {
  const portals = usePortals()
  const { user, signOut } = useSession()
  const [selected, setSelected] = useState(storedOrg)
  useEffect(() => { document.title = t('Seller workspace — Trezutil') }, [])

  const { orgs, orgsStatus, orgsError } = portals
  const org = orgs.find((o) => o.organization_id === selected) ?? orgs[0]
  const canManage = !!org && MANAGER_ROLES.has(org.role)
  const groups = useMemo(() => shellGroups('seller', { canManage }), [canManage])

  if (orgsStatus === 'idle' || (orgsStatus === 'loading' && orgs.length === 0)) return <PortalLoading label={t('Loading your business…')} />
  if (orgsStatus === 'error') return <AppShell><div className="seller-state"><ErrorState error={orgsError ?? t('Could not load')} onRetry={portals.reload} /></div></AppShell>
  // A signed-in account that belongs to no business has no workspace yet: the onboarding page explains the steps.
  if (!org) return <AppShell><SellerOnboardingPage /></AppShell>

  const selectOrg = (id: string) => { setSelected(id); try { localStorage.setItem(ORG_KEY, id) } catch { /* ignore */ } }
  return (
    <SellerContext.Provider value={{ orgs, org, canManage, selectOrg, reload: portals.reload }}>
      <PortalShell
        kind="seller"
        groups={groups}
        tabIds={TAB_IDS}
        identity={<div className="tz-rail-identity"><span aria-hidden="true">{initials(org.display_name)}</span><div><strong>{org.display_name}</strong><small>{roleLabel(org.role)}</small></div></div>}
        railTop={orgs.length > 1 ? (
          <label className="field seller-org-picker"><span>{t('Business')}</span>
            <select value={org.organization_id} onChange={(e) => selectOrg(e.target.value)}>{orgs.map((o) => <option key={o.organization_id} value={o.organization_id}>{o.display_name}</option>)}</select>
          </label>
        ) : undefined}
        user={{ name: user?.display_name || user?.phone || '', detail: roleLabel(org.role) }}
        onSignOut={signOut}
        switcher={<PortalSwitcher current="seller" portals={portals.portals} />}
      >
        <Outlet key={org.organization_id} />
      </PortalShell>
    </SellerContext.Provider>
  )
}

/** A seller module that is planned or unavailable: what it is for, and why it is not here yet. */
export function SellerModulePage({ id }: { id: string }) {
  const { canManage } = useSeller()
  const mod = sellerModules.find((m) => m.id === id)
  useEffect(() => { if (mod) document.title = `${t(mod.label)} — Trezutil` }, [mod])
  if (!mod) return null
  const group = sellerGroups.find((g) => g.id === mod.group)
  return (
    <div className="tz-module">
      <ModuleHead eyebrow={group ? t(group.label) : ''} title={t(mod.label)} copy={t(mod.purpose)} />
      {!allows(mod.requires, { canManage }) && <ViewOnly />}
      {mod.status.kind === 'unavailable' && <UnavailableState gapIds={mod.status.gaps} note={mod.status.note} link={mod.status.link} />}
      {mod.status.kind === 'planned' && <PlannedState endpoints={mod.status.endpoints} limits={mod.status.limits} />}
    </div>
  )
}

export function SellerHead({ eyebrow, title, copy, action }: { eyebrow: string; title: string; copy: string; action?: ReactNode }) {
  return <div className="seller-page-head"><div><div className="section-eyebrow">{eyebrow}</div><h1 className="heading heading-1">{title}</h1><p>{copy}</p></div>{action}</div>
}

export function ViewOnly() {
  return <p className="inline-notice info" role="note"><Icon name="user" size={16} /><span>{t('You can view this business. Only its owner, admins and managers can make changes.')}</span></p>
}
