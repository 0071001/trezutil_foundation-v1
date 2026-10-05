import { useEffect, useMemo } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router'
import { canSeeOverview, isAdmin, isStaff } from '@/api/admin'
import { ModuleHead, PlannedState, UnavailableState } from '@/components/portal/ModuleState'
import { PortalShell } from '@/components/portal/PortalShell'
import { PortalSwitcher } from '@/components/portal/PortalSwitcher'
import { EmptyState, Loading } from '@/components/states'
import { Button, Icon } from '@/components/ui'
import { t } from '@/lib/i18n'
import { usePrefs } from '@/lib/prefs'
import { useSession } from '@/lib/session'
import { usePortals } from '@/portal/PortalsProvider'
import { adminGroups, adminModules, shellGroups } from '@/portal/modules'

/*
 * The administration portal ("run Trezutil"). Only staff accounts get past the
 * guard; the server checks the role again on every request, so a screen that
 * is reached some other way still receives a 403 and shows it.
 */

export function AdminLayout() {
  const { user, signOut } = useSession()
  const portals = usePortals(isStaff(user))
  const { theme } = usePrefs()
  const location = useLocation()
  const roles = user?.roles ?? []
  const groups = useMemo(() => shellGroups('admin', { roles }), [roles.join(',')]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { document.title = t('Administration — Trezutil') }, [])

  if (user === undefined) return <Loading label={t('Checking your session…')} />
  if (!user) return <Navigate to={`/admin/login?returnTo=${encodeURIComponent(`${location.pathname}${location.search}`)}`} replace />
  if (!isStaff(user)) {
    return (
      <div className={theme === 'dark' ? 'app dark' : 'app'}>
        <main id="main" tabIndex={-1} className="admin-denied">
          <EmptyState icon="user" title={t('You don’t have access')} copy={t('This area is reserved for the Trezutil team. Sign in with an administration account.')} action={t('Back to Trezutil')} href="/" />
          <Button onClick={signOut}>{t('Sign out')}</Button>
        </main>
      </div>
    )
  }

  const roleLabel = isAdmin(user) ? t('Administrator') : user.roles.includes('operator') ? t('Operator') : t('Moderator')
  return (
    <PortalShell
      kind="admin"
      groups={groups}
      identity={<div className="tz-rail-identity"><span aria-hidden="true"><Icon name="shield" size={18} /></span><div><strong>{t('Administration')}</strong><small>{roleLabel}</small></div></div>}
      user={{ name: user.display_name || user.email || '', detail: `${roleLabel}${user.email ? ` · ${user.email}` : ''}` }}
      onSignOut={signOut}
      switcher={<PortalSwitcher current="admin" portals={portals.portals.length ? portals.portals : ['customer', 'admin']} />}
    >
      {canSeeOverview(user) ? <Outlet /> : (
        <EmptyState icon="user" title={t('Nothing to show yet')} copy={t('Your role does not have a page in the administration yet.')} />
      )}
    </PortalShell>
  )
}

/** An administration module that is planned or unavailable. */
export function AdminModulePage({ id }: { id: string }) {
  const mod = adminModules.find((m) => m.id === id)
  useEffect(() => { if (mod) document.title = `${t(mod.label)} — Trezutil` }, [mod])
  if (!mod) return null
  const group = adminGroups.find((g) => g.id === mod.group)
  return (
    <div className="tz-module">
      <ModuleHead eyebrow={group ? t(group.label) : ''} title={t(mod.label)} copy={t(mod.purpose)} />
      {mod.status.kind === 'unavailable' && <UnavailableState gapIds={mod.status.gaps} note={mod.status.note} link={mod.status.link} />}
      {mod.status.kind === 'planned' && <PlannedState endpoints={mod.status.endpoints} limits={mod.status.limits} />}
    </div>
  )
}
