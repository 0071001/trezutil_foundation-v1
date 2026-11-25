import { useEffect } from 'react'
import { useNavigate } from 'react-router'
import { creditsApi } from '@/api/credits'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { locale, t } from '@/lib/i18n'
import { MIcon } from '@/mobile/icons'
import { Group, Screen } from '@/mobile/ui'
import { ErrorNote, Loading } from '@/ui/kit'

/*
 * Credits (design → Crédits): the person's real balance and movements (GET /me/credits). There is no way to buy
 * credits yet, so the design's "add credits" button is not offered.
 */

export function Credits() {
  const { signedIn } = useSession()
  const navigate = useNavigate()
  const credits = useAsync(() => creditsApi.mine(), [signedIn], signedIn)
  useEffect(() => { document.title = `${t('Crédits')} — Trezutil` }, [])
  const c = credits.data
  return (
    <Screen title={t('Crédits')} className="mx-credits">
      {!signedIn ? (
        <button type="button" className="mx-primary" onClick={() => navigate('/login?returnTo=%2Fsettings%2Fcredits')}>{t('Se connecter')}</button>
      ) : credits.loading && !c ? <Loading /> : credits.error ? <ErrorNote error={credits.error} onRetry={credits.reload} /> : c ? (
        <>
          <div className="mx-credit-card">
            <p>{t('Solde disponible')}</p>
            <h2>{c.balance.toLocaleString(locale())}</h2>
            <span>{t('crédits personnels')}</span>
          </div>
          <div className="mx-credit-note"><MIcon name="shield" size={18} />
            <p>{c.meteringActive ? t('Les crédits servent aux opérations commerciales, jamais à la conversation.') : t('Les actions ne sont pas facturées pour l’instant : la conversation et les actions sont gratuites.')}</p>
          </div>
          <h3>{t('Activité récente')}</h3>
          {c.ledger.length === 0 ? <p className="mx-empty">{t('Aucun mouvement sur la période.')}</p> : (
            <div className="mx-group mx-transactions">
              {c.ledger.slice(0, 20).map((e) => (
                <div key={e.id}>
                  <span className="mx-transaction-icon">{e.amount > 0 ? '+' : <MIcon name="search" size={16} />}</span>
                  <p><strong>{e.actionType || e.kind}</strong><small>{new Date(e.createdAt).toLocaleDateString(locale(), { day: 'numeric', month: 'long', year: 'numeric' })}</small></p>
                  <b className={e.amount > 0 ? 'positive' : ''}>{e.amount > 0 ? `+ ${e.amount}` : `− ${Math.abs(e.amount)}`}</b>
                </div>
              ))}
            </div>
          )}
          <Group><div className="mx-setting-row"><span className="mx-row-text"><strong>{t('Période')}</strong><small>{c.periodId || '—'}</small></span></div></Group>
        </>
      ) : null}
    </Screen>
  )
}
