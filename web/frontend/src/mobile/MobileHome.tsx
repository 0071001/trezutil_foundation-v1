import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { siteApi } from '@/api/site'
import { conversationStore, type TurnAttachment } from '@/conversation/store'
import { Composer, type ComposerHandle } from '@/conversation/Composer'
import { useAsync } from '@/lib/useAsync'
import { safeImage } from '@/lib/format'
import { t, tx } from '@/lib/i18n'
import { MIcon, type MIconName } from '@/mobile/icons'
import { AppHeader } from '@/mobile/ui'
import { Trezy } from '@/ui/Trezy'

/*
 * The home of the mobile design: Trezy, the question, three suggestions, the partners the team published on the
 * site (nothing is shown when there are none), and the composer at the bottom.
 */

const SUGGESTIONS: [MIconName, string, string][] = [
  ['search', tx('Trouver un fournisseur'), tx('Trouve-moi un fournisseur fiable')],
  ['briefcase', tx('Comparer des offres'), tx('Je souhaite comparer plusieurs offres')],
  ['credit', tx('Négocier un prix'), tx('Aide-moi à négocier un meilleur prix')],
]

export function MobileHome() {
  const navigate = useNavigate()
  const [draft, setDraft] = useState('')
  const composer = useRef<ComposerHandle>(null)
  const site = useAsync(() => siteApi.content(), [])
  const partners = (site.data?.partners ?? []).filter((p) => p.name).slice(0, 4)
  useEffect(() => { document.title = 'Trezutil' }, [])

  const start = (text: string, attachments: TurnAttachment[] = []) => {
    const id = conversationStore.send(undefined, text, attachments)
    if (id) navigate(`/c/${id}`)
  }

  return (
    <div className="mx-screen">
      <AppHeader onNew={() => { setDraft(''); composer.current?.focus() }} />
      <div className="mx-home">
        <div className="mx-welcome">
          <Trezy pose="accueil" size={148} />
          <h1>{t('Que souhaites-tu acheter,')}<br />{t('vendre ou faire réaliser ?')}</h1>
          <p>{t('Exprime simplement ton besoin.')}<br />{t('Trezutil s’occupe du reste.')}</p>
        </div>
        <div className="mx-suggestions" role="group" aria-label={t('Pour commencer')}>
          {SUGGESTIONS.map(([icon, label, message]) => (
            <button type="button" key={label} onClick={() => start(t(message))}><MIcon name={icon} size={17} />{t(label)}</button>
          ))}
        </div>
        {partners.length > 0 && (
          <div className="mx-partners" aria-label={t('Partenaires')}>
            {partners.map((p) => {
              const logo = safeImage(p.logoUrl)
              return logo ? <img key={p.id} src={logo} alt={p.name} loading="lazy" /> : <span key={p.id}>{p.name}</span>
            })}
          </div>
        )}
      </div>
      <Composer ref={composer} value={draft} onChange={setDraft} onSubmit={start} onVoice={() => navigate('/voice')} />
    </div>
  )
}
