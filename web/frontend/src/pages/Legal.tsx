import { useEffect } from 'react'
import { Navigate, useParams } from 'react-router'
import { HomeFrame } from '@/components/home/chrome'
import { LEGAL_TITLES } from '@/components/legal'
import { ActionLink, Heading } from '@/components/ui'
import { locale, t, useLang } from '@/lib/i18n'
import { LEGAL, LEGAL_DOCS, isLegalDoc } from '@/lib/legal'
import { legalText } from '@/pages/legalContent'

/*
 * One legal document at a time (/legal/terms, /legal/privacy, /legal/refunds,
 * /legal/notice), in the language of the site. The text is built from the
 * product's real behaviour and the company's facts (src/lib/legal.ts); while
 * it is not reviewed it says so in a banner, and every fact still missing is
 * highlighted.
 */

export default function LegalPage() {
  const { doc } = useParams()
  const lang = useLang()
  const valid = isLegalDoc(doc)
  const text = valid ? legalText(doc, lang === 'fr' ? 'fr' : 'en') : null
  useEffect(() => { if (text) document.title = `${text.title} — Trezutil` }, [text?.title]) // eslint-disable-line react-hooks/exhaustive-deps
  if (!valid || !text) return <Navigate to="/legal/terms" replace />
  const updated = new Date(`${LEGAL.updated}T12:00:00Z`).toLocaleDateString(locale(), { dateStyle: 'long', timeZone: 'UTC' })
  return (
    <HomeFrame>
      <div className="home-section legal-page">
        <nav className="legal-nav" aria-label={t('Legal documents')}>
          {LEGAL_DOCS.map((d) => <ActionLink key={d} href={`/legal/${d}`} nav>{t(LEGAL_TITLES[d])}</ActionLink>)}
        </nav>
        <article className="legal-doc">
          {!LEGAL.reviewed && <p className="legal-draft" role="note">{t('This text is a draft: a lawyer has not reviewed it yet and it is not final. Highlighted items are facts still to be filled in.')}</p>}
          <Heading level={1}>{text.title}</Heading>
          <p className="legal-updated">{t('Last updated {date}', { date: updated })}</p>
          <p className="legal-intro">{text.intro}</p>
          <nav className="legal-toc" aria-label={t('On this page')}>
            <ol>{text.sections.map((s) => <li key={s.id}><ActionLink href={`/legal/${doc}#${s.id}`}>{s.title}</ActionLink></li>)}</ol>
          </nav>
          {text.sections.map((s, i) => (
            <section key={s.id} id={s.id} aria-labelledby={`${s.id}-title`}>
              <h2 id={`${s.id}-title`}><span aria-hidden="true">{i + 1}.</span> {s.title}</h2>
              {s.body}
            </section>
          ))}
        </article>
      </div>
    </HomeFrame>
  )
}
