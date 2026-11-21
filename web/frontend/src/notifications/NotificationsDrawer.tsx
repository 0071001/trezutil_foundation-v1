import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { markRead, type Notice } from '@/notifications/notifications'
import { Icon } from '@/ui/Icon'

/* The notification centre: what really waits for the person, read from their conversations, operations and orders. */

function ago(at: number): string {
  const s = Math.max(0, Math.round((Date.now() - at) / 1000))
  if (s < 60) return 'À l’instant'
  if (s < 3600) return `Il y a ${Math.round(s / 60)} min`
  if (s < 86400) return `Il y a ${Math.round(s / 3600)} h`
  return new Date(at).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })
}

export function NotificationsDrawer({ notices, read, onClose }: { notices: Notice[]; read: Record<string, true>; onClose: () => void }) {
  const navigate = useNavigate()
  const [filter, setFilter] = useState<'all' | 'todo'>('all')
  useEffect(() => {
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', esc)
    return () => document.removeEventListener('keydown', esc)
  }, [onClose])
  const todo = notices.filter((n) => n.kind === 'decision')
  const shown = filter === 'todo' ? todo : notices
  return (
    <>
      <div className="scrim" onClick={onClose} aria-hidden="true" />
      <aside className="notification-drawer" role="dialog" aria-modal="true" aria-label="Notifications">
        <div className="drawer-head"><h2>Notifications</h2><button type="button" className="icon-btn" onClick={onClose} aria-label="Fermer"><Icon name="x" /></button></div>
        <div className="drawer-tools">
          <button type="button" className={filter === 'all' ? 'on' : ''} aria-pressed={filter === 'all'} onClick={() => setFilter('all')}>Tout</button>
          <button type="button" className={filter === 'todo' ? 'on' : ''} aria-pressed={filter === 'todo'} onClick={() => setFilter('todo')}>À traiter {todo.length > 0 && <i>{todo.length}</i>}</button>
          <span />
          <button type="button" onClick={() => markRead(notices.map((n) => n.id))} disabled={!notices.some((n) => !read[n.id])}>Tout marquer comme lu</button>
        </div>
        <div className="notification-list">
          {shown.length === 0 ? (
            <div className="drawer-empty"><Icon name="bell" size={22} /><p><b>Rien à signaler</b><br />Les décisions à prendre, tes opérations et tes commandes en cours apparaîtront ici.</p></div>
          ) : shown.map((n) => (
            <button type="button" className="notification-item" key={n.id} onClick={() => { markRead([n.id]); onClose(); navigate(n.to) }}>
              <span className={`notification-symbol ${n.tone}`}><Icon name={n.kind === 'decision' ? 'shield' : n.kind === 'operation' ? 'flow' : 'truck'} /></span>
              <div><b>{n.title}</b><p>{n.text}</p><small>{ago(n.at)}</small></div>
              {!read[n.id] ? <i className="unread" aria-label="Non lue" /> : <span />}
            </button>
          ))}
        </div>
        <div className="drawer-foot"><Link to="/settings/notifications" onClick={onClose}>Gérer les préférences</Link></div>
      </aside>
    </>
  )
}
