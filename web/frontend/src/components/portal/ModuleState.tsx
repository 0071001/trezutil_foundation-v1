import { ActionLink, Heading, Icon } from '@/components/ui'
import { gaps, type GapId } from '@/portal/gaps'
import { t } from '@/lib/i18n'

function GapList({ ids }: { ids: GapId[] }) {
  return (
    <ul className="tz-gaps">
      {ids.map((id) => (
        <li key={id}>
          <strong>{t(gaps[id].title)} <span className="tz-ref">{id}</span></strong>
          <span>{t(gaps[id].needs)}</span>
        </li>
      ))}
    </ul>
  )
}

/**
 * A module the platform cannot serve yet. It says so, says what is missing and
 * cites the contract (docs/web/PORTALS_BACKEND_GAP_ANALYSIS.md). It never shows
 * sample data, a zero, or a control that does nothing.
 */
export function UnavailableState({ gapIds, note, link }: { gapIds: GapId[]; note?: string; link?: { to: string; label: string } }) {
  return (
    <section className="tz-state" aria-labelledby="unavailable-title" data-state="unavailable">
      <p className="tz-state-title" id="unavailable-title"><Icon name="lock" size={18} />{t('Not available yet')}</p>
      <p>{t('This screen needs a capability the Trezutil platform has not built yet. Nothing is shown here rather than figures that would not be real.')}</p>
      {note && <p>{t(note)}</p>}
      <div>
        <p className="tz-label">{t('What is missing')}</p>
        <GapList ids={gapIds} />
      </div>
      {link && <ActionLink className="tz-link-action" href={link.to}>{t(link.label)} <Icon name="arrow" size={14} /></ActionLink>}
    </section>
  )
}

/** A module whose data exists but whose screen is not built yet. */
export function PlannedState({ endpoints, limits }: { endpoints: string[]; limits?: GapId[] }) {
  return (
    <section className="tz-state" aria-labelledby="planned-title" data-state="planned">
      <p className="tz-state-title" id="planned-title"><Icon name="clock" size={18} />{t('Coming soon')}</p>
      <p>{t('The data this screen needs already exists on the platform. The screen itself has not been built yet.')}</p>
      <div>
        <p className="tz-label">{t('It will use')}</p>
        <ul className="tz-endpoints">{endpoints.map((e) => <li key={e}>{e}</li>)}</ul>
      </div>
      {limits && limits.length > 0 && (
        <div>
          <p className="tz-label">{t('Known limits')}</p>
          <GapList ids={limits} />
        </div>
      )}
    </section>
  )
}

/** The page head of a console module. */
export function ModuleHead({ eyebrow, title, copy }: { eyebrow: string; title: string; copy: string }) {
  return (
    <header className="tz-module-head">
      <div className="section-eyebrow">{eyebrow}</div>
      <Heading level={1} className="tz-title">{title}</Heading>
      <p>{copy}</p>
    </header>
  )
}
