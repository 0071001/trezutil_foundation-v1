import { useEffect } from 'react'
import { useNavigate } from 'react-router'
import { isStaff } from '@/api/admin'
import { creditsApi } from '@/api/credits'
import { negotiationsApi } from '@/api/negotiations'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { useConversations } from '@/conversation/store'
import { useNotices } from '@/notifications/notifications'
import { useOrganizations } from '@/shell/profile'
import { useWorkspace } from '@/shell/workspace'
import { t, tn } from '@/lib/i18n'
import { Group, ListRow, Screen } from '@/mobile/ui'
import { initials } from '@/ui/kit'

/*
 * Profile (design → Profil): who is signed in, credits, missions and notifications with their real figures, settings,
 * help and sign-out. Purchases, bookings, negotiations and the supplier space, which the design does not draw, are in
 * their own group.
 */

export function Profile() {
  const { user, signedIn, signOut } = useSession()
  const navigate = useNavigate()
  const { conversations } = useConversations()
  const { missions, orders } = useWorkspace()
  const { unread } = useNotices(conversations, missions.data ?? [], orders.data ?? [])
  const { orgs } = useOrganizations(signedIn)
  const credits = useAsync(() => creditsApi.mine(), [signedIn], signedIn)
  const negotiations = useAsync(() => negotiationsApi.list(), [signedIn], signedIn)
  const toDecide = (negotiations.data ?? []).filter((n) => n.status === 'awaiting_decision').length
  useEffect(() => { document.title = `${t('Profil')} — Trezutil` }, [])
  const name = user?.display_name || user?.phone || ''
  const active = (missions.data ?? []).filter((m) => m.status === 'active').length
  const creditText = !signedIn ? undefined : credits.loading ? '…' : credits.data && (credits.data.opened || credits.data.meteringActive) ? t('{n} crédits disponibles', { n: credits.data.balance }) : t('Actions gratuites pour l’instant')

  return (
    <Screen title={t('Profil')}>
      {user ? (
        <div className="mx-profile-hero">
          <div className="mx-big-avatar">{initials(name)}</div>
          <h2>{name}</h2>
          <p>{user.email || user.phone}</p>
          {user.verified_at && <span>{t('Compte vérifié')}</span>}
        </div>
      ) : user === null ? (
        <div className="mx-profile-hero">
          <h2>{t('Connecte-toi à Trezutil')}</h2>
          <p>{t('Retrouve tes conversations, commandes et réservations sur tous tes appareils.')}</p>
          <button type="button" className="mx-primary" onClick={() => navigate('/login?returnTo=%2Fprofile')}>{t('Se connecter')}</button>
        </div>
      ) : null}

      {signedIn && (
        <>
          <Group>
            <ListRow icon="credit" accent title={t('Crédits personnels')} detail={creditText} onClick={() => navigate('/settings/credits')} />
            <ListRow icon="briefcase" title={t('Mes missions')} detail={missions.loading ? '…' : tn(active, '{n} en cours', '{n} en cours')} onClick={() => navigate('/operations')} />
            <ListRow icon="bell" title={t('Notifications')} detail={unread ? tn(unread, '{n} nouvelle', '{n} nouvelles') : t('Rien de nouveau')} onClick={() => navigate('/notifications')} />
          </Group>
          <Group>
            <ListRow icon="package" title={t('Commandes et réservations')} detail={orders.data ? tn(orders.data.length, '{n} commande', '{n} commandes') : undefined} onClick={() => navigate('/purchases')} />
            <ListRow icon="check" accent={toDecide > 0} title={t('Négociations')} detail={toDecide ? tn(toDecide, '{n} négociation à trancher', '{n} négociations à trancher') : undefined} onClick={() => navigate('/negotiations')} />
            {orgs.length > 0 ? (
              <>
                <ListRow icon="file" title={t('Demandes de prix')} onClick={() => navigate('/supplier/requests')} />
                <ListRow icon="package" title={t('Mes offres')} onClick={() => navigate('/supplier/offers')} />
              </>
            ) : <ListRow icon="briefcase" title={t('Devenir fournisseur')} onClick={() => navigate('/supplier')} />}
            {isStaff(user) && <ListRow icon="shield" title={t('Control Center')} onClick={() => navigate('/admin')} />}
          </Group>
        </>
      )}

      <Group>
        <ListRow icon="settings" title={t('Paramètres')} onClick={() => navigate('/settings')} />
        <ListRow icon="help" title={t('Aide et assistance')} onClick={() => navigate('/legal')} />
      </Group>
      {signedIn && <Group><ListRow icon="logout" title={t('Se déconnecter')} onClick={() => { signOut(); navigate('/') }} /></Group>}
      <p className="mx-version">{t('Trezutil pour mobile')}</p>
    </Screen>
  )
}
