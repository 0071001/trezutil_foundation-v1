import { useEffect } from 'react'
import { Link, NavLink, useParams } from 'react-router'
import { isLegalDoc, LEGAL, LEGAL_DOCS, type LegalDoc } from '@/lib/legal'
import { LEGAL_CONTENT } from '@/pages/legalContent'
import { LEGAL_CONTENT_EN } from '@/pages/legalContentEn'
import { LanguageToggle, Logo } from '@/ui/kit'
import { getLang, locale, t } from '@/lib/i18n'

/*
 * The legal pages (/legal and /legal/<doc>): public, readable without an account, outside the app shell so they can
 * be linked from Google's consent screen, emails and the sign-in screen. Until a lawyer has reviewed the texts
 * (LEGAL.reviewed), each page says it is a draft.
 */

const updated = new Date(LEGAL.updated).toLocaleDateString(locale(), { day: 'numeric', month: 'long', year: 'numeric' })

export function Legal() {
  const { doc } = useParams()
  const current: LegalDoc | null = isLegalDoc(doc) ? doc : null
  const title = current ? LEGAL_DOCS[current] : t('Informations légales')
  useEffect(() => { document.title = `${title} — Trezutil`; window.scrollTo(0, 0) }, [title])

  return (
    <div className="legal-page">
      <header className="legal-head">
        <Link to="/" aria-label={t('Retour à Trezutil')}><Logo /></Link>
        <span className="legal-head-actions"><LanguageToggle /><Link to="/" className="secondary-btn small-btn">{t('Retour à Trezutil')}</Link></span>
      </header>
      <div className="legal-layout">
        <nav className="legal-nav" aria-label={t('Documents légaux')}>
          <NavLink to="/legal" end>{t('Informations légales')}</NavLink>
          {(Object.keys(LEGAL_DOCS) as LegalDoc[]).map((d) => <NavLink key={d} to={`/legal/${d}`}>{LEGAL_DOCS[d]}</NavLink>)}
        </nav>
        <article className="legal-article">
          {!LEGAL.reviewed && (
            <p className="legal-draft" role="note">
              {t('Ce texte est un projet en cours de validation juridique. Les éléments surlignés restent à compléter par la société qui exploite Trezutil.')}
            </p>
          )}
          <h1>{title}</h1>
          <p className="legal-updated">{t('Dernière mise à jour :')} {updated}</p>
          {getLang() === 'en' && <p className="legal-updated">English translation of the French text.</p>}
          {current ? (getLang() === 'en' ? LEGAL_CONTENT_EN : LEGAL_CONTENT)[current] : (
            <>
              <p className="legal-lede">{t('Tout ce qui encadre l’utilisation de Trezutil, en un seul endroit.')}</p>
              <ul className="legal-index">
                {(Object.keys(LEGAL_DOCS) as LegalDoc[]).map((d) => <li key={d}><Link to={`/legal/${d}`}>{LEGAL_DOCS[d]}</Link></li>)}
              </ul>
            </>
          )}
        </article>
      </div>
    </div>
  )
}
