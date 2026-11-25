import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { useConversations } from '@/conversation/store'
import { t } from '@/lib/i18n'
import { MIcon } from '@/mobile/icons'
import { Screen, SectionLabel } from '@/mobile/ui'
import { dayLabel } from '@/ui/kit'

/* Search (design → Rechercher): the conversations of the account, by title and by what was written in them. */

const norm = (s: string) => s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')

export function Search() {
  const navigate = useNavigate()
  const { conversations } = useConversations()
  const [query, setQuery] = useState('')
  useEffect(() => { document.title = `${t('Rechercher')} — Trezutil` }, [])
  const q = norm(query.trim())
  const list = conversations
    .filter((c) => !q || norm(c.title).includes(q) || c.turns.some((x) => norm(x.text).includes(q)))
    .sort((a, b) => b.updatedAt - a.updatedAt)
    .slice(0, 40)
  return (
    <Screen title={t('Rechercher')} className="mx-search">
      <div className="mx-search-field">
        <MIcon name="search" size={19} />
        <input autoFocus type="search" placeholder={t('Rechercher dans les conversations')} aria-label={t('Rechercher dans les conversations')} value={query} onChange={(e) => setQuery(e.target.value)} />
        {query && <button type="button" onClick={() => setQuery('')} aria-label={t('Effacer')}><MIcon name="close" size={16} /></button>}
      </div>
      <SectionLabel>{q ? t('Résultats') : t('Récents')}</SectionLabel>
      <div className="mx-list">
        {list.map((c) => (
          <button type="button" className="mx-conversation-row" key={c.id} onClick={() => navigate(`/c/${c.id}`)}>
            <span className="mx-row-icon"><MIcon name="edit" size={18} /></span>
            <span className="mx-row-text"><strong>{c.title}</strong><small>{dayLabel(c.updatedAt)}{c.archived ? ` · ${t('Archivée')}` : ''}</small></span>
            <MIcon name="chevron" size={17} />
          </button>
        ))}
        {!list.length && <p className="mx-empty">{q ? t('Aucune conversation ne correspond.') : t('Tes conversations apparaîtront ici.')}</p>}
      </div>
    </Screen>
  )
}
