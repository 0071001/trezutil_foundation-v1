import { useEffect } from 'react'
import { useNavigate } from 'react-router'
import { useConversations } from '@/conversation/store'
import { markRead, useNotices } from '@/notifications/notifications'
import { timeAgo } from '@/lib/format'
import { t } from '@/lib/i18n'
import { useWorkspace } from '@/shell/workspace'
import { MIcon } from '@/mobile/icons'
import { Screen } from '@/mobile/ui'

/* Notifications (design → Notifications): what really waits for the person, read from their data. */

export function Notifications() {
  const navigate = useNavigate()
  const { conversations } = useConversations()
  const { missions, orders } = useWorkspace()
  const { notices, prefs } = useNotices(conversations, missions.data ?? [], orders.data ?? [])
  useEffect(() => { document.title = `${t('Notifications')} — Trezutil` }, [])
  return (
    <Screen title={t('Notifications')}>
      {!notices.length ? <p className="mx-empty">{t('Les décisions à prendre, tes missions et tes commandes en cours apparaîtront ici.')}</p> : (
        <div className="mx-notifications">
          {notices.map((n) => {
            const unread = !prefs.read[n.id]
            return (
              <button type="button" key={n.id} onClick={() => { markRead([n.id]); navigate(n.to) }}>
                <span className={`mx-notification-icon${unread ? ' unread' : ''}`}><MIcon name={n.tone === 'green' ? 'check' : 'bell'} size={18} /></span>
                <span><strong>{n.title}</strong><p>{n.text}</p><small>{timeAgo(n.at / 1000)}</small></span>
              </button>
            )
          })}
        </div>
      )}
    </Screen>
  )
}
