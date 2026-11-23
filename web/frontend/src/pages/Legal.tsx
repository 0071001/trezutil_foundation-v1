import { useEffect } from 'react'
import { Link, NavLink, useParams } from 'react-router'
import { isLegalDoc, LEGAL, LEGAL_DOCS, type LegalDoc } from '@/lib/legal'
import { LEGAL_CONTENT } from '@/pages/legalContent'
import { Logo } from '@/ui/kit'

/*
 * The legal pages (/legal and /legal/<doc>): public, readable without an account, outside the app shell so they can
 * be linked from Google's consent screen, emails and the sign-in screen. Until a lawyer has reviewed the texts
 * (LEGAL.reviewed), each page says it is a draft.
 */

const updated = new Date(LEGAL.updated).toLocaleDateString('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })

export function Legal() {
  const { doc } = useParams()
  const current: LegalDoc | null = isLegalDoc(doc) ? doc : null
  const title = current ? LEGAL_DOCS[current] : 'Informations légales'
  useEffect(() => { document.title = `${title} — Trezutil`; window.scrollTo(0, 0) }, [title])

  return (
    <div className="legal-page">
      <header className="legal-head">
        <Link to="/" aria-label="Retour à Trezutil"><Logo /></Link>
        <Link to="/" className="secondary-btn small-btn">Retour à Trezutil</Link>
      </header>
      <div className="legal-layout">
        <nav className="legal-nav" aria-label="Documents légaux">
          <NavLink to="/legal" end>Informations légales</NavLink>
          {(Object.keys(LEGAL_DOCS) as LegalDoc[]).map((d) => <NavLink key={d} to={`/legal/${d}`}>{LEGAL_DOCS[d]}</NavLink>)}
        </nav>
        <article className="legal-article">
          {!LEGAL.reviewed && (
            <p className="legal-draft" role="note">
              Ce texte est un projet en cours de validation juridique. Les éléments surlignés restent à compléter par la société qui exploite Trezutil.
            </p>
          )}
          <h1>{title}</h1>
          <p className="legal-updated">Dernière mise à jour : {updated}</p>
          {current ? LEGAL_CONTENT[current] : (
            <>
              <p className="legal-lede">Tout ce qui encadre l’utilisation de Trezutil, en un seul endroit.</p>
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
