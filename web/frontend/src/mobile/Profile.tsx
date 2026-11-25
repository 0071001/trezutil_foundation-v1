import { useEffect, type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router'
import { isStaff } from '@/api/admin'
import { reservationsApi } from '@/api/commerce'
import { useConversations } from '@/conversation/store'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { isDark, setTheme, useTheme } from '@/lib/theme'
import { getLang, LANGUAGES, switchLanguage, t } from '@/lib/i18n'
import { useWorkspace } from '@/shell/workspace'
import { Icon, type IconName } from '@/ui/Icon'
import { initials } from '@/ui/kit'

/*
 * Profile (Design Trezutil Mobile App → Profil): who is signed in, what they did with Trezutil (real counts), and the
 * settings, each leading to its real page. A guest sees how to sign in instead of a profile.
 */

function Row({ icon, label, value, to, onClick, danger, trailing, pressed }: { icon: IconName; label: string; value?: string; to?: string; onClick?: () => void; danger?: boolean; trailing?: ReactNode; pressed?: boolean }) {
  const body = (
    <>
      <span className="m-row-icon"><Icon name={icon} size={16} /></span>
      <span className="m-row-label">{label}</span>
      {value && <span className="m-row-value">{value}</span>}
      {trailing ?? <Icon name="chevron" size={16} />}
    </>
  )
  const cls = `m-setting${danger ? ' danger' : ''}`
  return to ? <Link className={cls} to={to}>{body}</Link> : <button type="button" className={cls} onClick={onClick} aria-pressed={pressed}>{body}</button>
}

function Group({ title, children }: { title?: string; children: ReactNode }) {
  return <section className="m-section">{title && <p className="m-eyebrow">{title}</p>}<div className="m-group">{children}</div></section>
}

export function Profile() {
  const { user, signedIn, signOut } = useSession()
  const navigate = useNavigate()
  const { conversations } = useConversations()
  const { orders } = useWorkspace()
  const bookings = useAsync(() => reservationsApi.list(), [signedIn], signedIn)
  useTheme()
  const dark = isDark()
  const lang = getLang()
  const other = LANGUAGES.find((l) => l.code !== lang)!
  useEffect(() => { document.title = `${t('Profil')} — Trezutil` }, [])
  const name = user?.display_name || user?.phone || ''
  const stats: [string, number | undefined][] = [
    [t('Conversations'), conversations.filter((c) => !c.archived && c.turns.length > 0).length],
    [t('Réservations'), bookings.data?.length],
    [t('Achats'), orders.data?.length],
  ]

  return (
    <div className="m-screen m-profile">
      <header className="m-page-head"><p className="m-eyebrow">{t('Compte')}</p></header>
      <div className="m-scroll">
        {user ? (
          <div className="m-profile-card">
            <div className="m-profile-id">
              <span className="m-profile-avatar">{initials(name)}</span>
              <div>
                <h1>{name}</h1>
                {user.phone && <small>{user.phone}</small>}
                {user.email && <small>{user.email}</small>}
              </div>
            </div>
            <div className="m-stats">
              {stats.map(([label, value]) => <div key={label}><b>{value ?? '–'}</b><small>{label}</small></div>)}
            </div>
          </div>
        ) : user === null ? (
          <div className="m-profile-card guest">
            <b>{t('Connecte-toi à Trezutil')}</b>
            <p>{t('Retrouve tes conversations, commandes et réservations sur tous tes appareils.')}</p>
            <Link className="primary-btn" to="/login?returnTo=%2Fprofile">{t('Se connecter')}</Link>
          </div>
        ) : null}

        <Group title={t('Préférences')}>
          <Row icon="globe" label={t('Langue')} value={LANGUAGES.find((l) => l.code === lang)!.label} onClick={() => switchLanguage(other.code)} trailing={<span className="m-row-action">{other.short}</span>} />
          {signedIn && <Row icon="bell" label={t('Notifications')} to="/settings/notifications" />}
          <Row icon={dark ? 'sun' : 'moon'} label={dark ? t('Mode clair') : t('Mode sombre')} onClick={() => setTheme(dark ? 'light' : 'dark')} pressed={dark}
            trailing={<span className={`m-switch${dark ? ' on' : ''}`} aria-hidden="true"><i /></span>} />
          {signedIn && <Row icon="settings" label={t('Tous les paramètres')} to="/settings/preferences" />}
        </Group>

        {signedIn && (
          <Group title={t('Compte')}>
            <Row icon="user" label={t('Profil personnel')} to="/settings/profile" />
            <Row icon="heart" label={t('Mémoire commerciale')} to="/settings/memory" />
            <Row icon="wallet" label={t('Achats et paiements')} to="/purchases" />
            <Row icon="star" label={t('Crédits et usage')} to="/settings/credits" />
            <Row icon="shield" label={t('Confidentialité et sécurité')} to="/settings/privacy" />
            <Row icon="building" label={t('Devenir fournisseur')} to="/supplier" />
            {isStaff(user) && <Row icon="settings" label={t('Control Center')} onClick={() => navigate('/admin')} />}
          </Group>
        )}

        <Group title={t('Support')}>
          <Row icon="help" label={t('Conditions et confidentialité')} to="/legal" />
        </Group>

        {signedIn && (
          <Group>
            <Row icon="logout" label={t('Se déconnecter')} danger onClick={() => { signOut(); navigate('/') }} />
          </Group>
        )}
        <p className="m-version">Trezutil</p>
      </div>
    </div>
  )
}
