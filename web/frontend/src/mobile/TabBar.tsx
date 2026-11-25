import { useLocation, useNavigate } from 'react-router'
import { useConversations } from '@/conversation/store'
import { Icon, type IconName } from '@/ui/Icon'
import { t } from '@/lib/i18n'

/*
 * The bottom navigation of the mobile layout: Home, Discover, Ask (the conversation), Activity, Profile. "Ask" goes back
 * to the conversation in progress, or to the home composer when there is none.
 */

type Tab = 'home' | 'discover' | 'ask' | 'activity' | 'profile'

export function activeTab(pathname: string): Tab | null {
  if (pathname === '/') return 'home'
  if (pathname.startsWith('/discover')) return 'discover'
  if (pathname.startsWith('/c/')) return 'ask'
  if (pathname.startsWith('/activity') || pathname.startsWith('/purchases') || pathname.startsWith('/operations')) return 'activity'
  if (pathname.startsWith('/profile') || pathname.startsWith('/settings') || pathname === '/supplier' || pathname.startsWith('/suppliers') || pathname.startsWith('/overview')) return 'profile'
  return null
}

export function TabBar() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const { conversations } = useConversations()
  const active = activeTab(pathname)
  const latest = conversations.filter((c) => !c.archived).sort((a, b) => b.updatedAt - a.updatedAt)[0]
  const tabs: [Tab, IconName, string, () => void][] = [
    ['home', 'home', t('Accueil'), () => navigate('/')],
    ['discover', 'compass', t('Découvrir'), () => navigate('/discover')],
    ['ask', 'message', t('Demander'), () => navigate(latest ? `/c/${latest.id}` : '/', latest ? undefined : { state: { focus: true } })],
    ['activity', 'clock', t('Activité'), () => navigate('/activity')],
    ['profile', 'user', t('Profil'), () => navigate('/profile')],
  ]
  return (
    <nav className="m-tabbar" aria-label={t('Navigation principale')}>
      {tabs.map(([id, icon, label, go]) => (
        <button type="button" key={id} className={`${id === 'ask' ? 'ask' : ''}${active === id ? ' active' : ''}`} aria-current={active === id ? 'page' : undefined} onClick={go}>
          <span className="m-tab-icon"><Icon name={icon} size={id === 'ask' ? 18 : 20} /></span>
          <span>{label}</span>
        </button>
      ))}
    </nav>
  )
}
