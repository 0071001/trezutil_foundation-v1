import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { useSession } from '@/lib/session'
import { timeAgo } from '@/lib/format'
import { t } from '@/lib/i18n'
import { OPERATION_STATUS } from '@/operations/operations'
import { useWorkspace } from '@/shell/workspace'
import { Group, ListRow, Screen } from '@/mobile/ui'
import { ErrorNote, Loading } from '@/ui/kit'

/* Missions (design → Missions): the account's operations, filtered by All, Under way and Finished. */

type Filter = 'all' | 'running' | 'done'
const FINISHED = new Set(['completed', 'cancelled'])

export function Missions() {
  const { signedIn } = useSession()
  const navigate = useNavigate()
  const { missions } = useWorkspace()
  const [filter, setFilter] = useState<Filter>('all')
  useEffect(() => { document.title = `${t('Missions')} — Trezutil` }, [])
  const list = (missions.data ?? []).filter((m) => filter === 'all' || (filter === 'done' ? FINISHED.has(m.status) : !FINISHED.has(m.status)))
  return (
    <Screen title={t('Missions')}>
      {!signedIn ? (
        <button type="button" className="mx-primary" onClick={() => navigate('/login?returnTo=%2Foperations')}>{t('Se connecter')}</button>
      ) : (
        <>
          <div className="mx-filter-tabs" role="group" aria-label={t('Filtrer les missions')}>
            {([['all', t('Toutes')], ['running', t('En cours')], ['done', t('Terminées')]] as [Filter, string][]).map(([id, label]) => (
              <button type="button" key={id} aria-pressed={filter === id} className={filter === id ? 'active' : ''} onClick={() => setFilter(id)}>{label}</button>
            ))}
          </div>
          {missions.loading && !missions.data ? <Loading /> : missions.error ? <ErrorNote error={missions.error} onRetry={missions.reload} />
            : !list.length ? <p className="mx-empty">{t('Aucune mission ici. Dans une conversation, choisis « Suivre en mission » après une recherche.')}</p> : (
              <Group>
                {list.map((m) => {
                  const status = OPERATION_STATUS[m.status]?.label ?? m.status
                  const at = Date.parse(m.updatedAt)
                  return <ListRow key={m.id} icon={m.status === 'completed' ? 'check' : 'briefcase'} title={m.title || m.objective}
                    detail={`${status}${Number.isFinite(at) ? ` · ${timeAgo(at / 1000)}` : ''}`} onClick={() => navigate(`/operations/${m.id}`)} />
                })}
              </Group>
            )}
        </>
      )}
    </Screen>
  )
}
