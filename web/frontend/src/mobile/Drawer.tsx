import { useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { creditsApi } from '@/api/credits'
import { conversationStore, useConversations } from '@/conversation/store'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { t } from '@/lib/i18n'
import { MIcon } from '@/mobile/icons'
import { MLogo, useMobileFrame } from '@/mobile/ui'
import { dayLabel, initials } from '@/ui/kit'

/* The drawer of the design: new conversation, search, the history by day (rename, archive, delete), the account. */

export function Drawer({ close }: { close: () => void }) {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { user, signedIn } = useSession()
  const { conversations } = useConversations()
  const { openCart, cartCount } = useMobileFrame()
  const [menuFor, setMenuFor] = useState('')
  const credits = useAsync(() => creditsApi.mine(), [signedIn], signedIn)
  const go = (to: string) => { close(); navigate(to) }

  useEffect(() => {
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') close() }
    document.addEventListener('keydown', esc)
    return () => document.removeEventListener('keydown', esc)
  }, [close])

  const list = conversations.filter((c) => !c.archived).sort((a, b) => b.updatedAt - a.updatedAt).slice(0, 40)
  const groups: { label: string; items: typeof list }[] = []
  for (const c of list) {
    const label = dayLabel(c.updatedAt)
    const last = groups[groups.length - 1]
    if (last && last.label === label) last.items.push(c)
    else groups.push({ label, items: [c] })
  }
  const name = user?.display_name || user?.phone || ''
  const balance = credits.data && (credits.data.opened || credits.data.meteringActive) ? t('{n} crédits', { n: credits.data.balance }) : t('Profil personnel')

  return (
    <div className="mx-layer">
      <button type="button" className="mx-scrim" onClick={close} aria-label={t('Fermer')} />
      <aside className="mx-drawer" role="dialog" aria-modal="true" aria-label={t('Menu')}>
        <div className="mx-drawer-top"><MLogo small /><button type="button" className="mx-icon-btn" onClick={close} aria-label={t('Fermer')}><MIcon name="close" /></button></div>
        <button type="button" className="mx-new-chat" onClick={() => go('/')}><MIcon name="edit" size={19} />{t('Nouvelle conversation')}</button>
        <button type="button" className="mx-drawer-search" onClick={() => go('/search')}><MIcon name="search" size={19} /><span>{t('Rechercher')}</span></button>
        {cartCount > 0 && <button type="button" className="mx-drawer-search" onClick={() => { close(); openCart() }}><MIcon name="package" size={19} /><span>{t('Panier ({n})', { n: cartCount })}</span></button>}
        <div className="mx-history">
          {groups.length === 0 && <p className="mx-history-empty">{t('Tes conversations apparaîtront ici.')}</p>}
          {groups.map((g) => (
            <div key={g.label}>
              <p className="mx-history-label">{g.label}</p>
              {g.items.map((c) => (
                <div className={`mx-history-item${pathname === `/c/${c.id}` ? ' active' : ''}`} key={c.id}>
                  <button type="button" onClick={() => go(`/c/${c.id}`)}>{c.title}</button>
                  <button type="button" aria-label={t('Actions pour {title}', { title: c.title })} aria-haspopup="menu" aria-expanded={menuFor === c.id} onClick={() => setMenuFor((m) => (m === c.id ? '' : c.id))}><MIcon name="more" size={18} /></button>
                  {menuFor === c.id && (
                    <div className="mx-item-menu" role="menu">
                      <button type="button" role="menuitem" onClick={() => { setMenuFor(''); const next = window.prompt(t('Nouveau titre'), c.title); if (next) conversationStore.rename(c.id, next) }}><MIcon name="edit" size={16} />{t('Renommer')}</button>
                      <button type="button" role="menuitem" onClick={() => { setMenuFor(''); conversationStore.archive(c.id); if (pathname === `/c/${c.id}`) go('/') }}><MIcon name="archive" size={16} />{t('Archiver')}</button>
                      <button type="button" role="menuitem" className="danger" onClick={() => {
                        setMenuFor('')
                        if (window.confirm(t('Supprimer cette conversation ? Ses messages sont effacés de Trezutil ; les commandes et réservations qu’elle a créées restent.'))) { conversationStore.remove(c.id); if (pathname === `/c/${c.id}`) go('/') }
                      }}><MIcon name="trash" size={16} />{t('Supprimer')}</button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          ))}
        </div>
        <div className="mx-drawer-bottom">
          {user ? (
            <button type="button" onClick={() => go('/profile')} aria-label={t('Profil : {name}', { name })}>
              <span className="mx-avatar">{initials(name)}</span>
              <span className="mx-row-text"><strong>{name}</strong><small>{balance}</small></span>
              <MIcon name="more" size={19} />
            </button>
          ) : user === null ? (
            <button type="button" onClick={() => go('/login?returnTo=%2F')}>
              <span className="mx-avatar"><MIcon name="user" size={17} /></span>
              <span className="mx-row-text"><strong>{t('Se connecter')}</strong><small>{t('Retrouve tes conversations et tes commandes')}</small></span>
              <MIcon name="chevron" size={17} />
            </button>
          ) : null}
        </div>
      </aside>
    </div>
  )
}
