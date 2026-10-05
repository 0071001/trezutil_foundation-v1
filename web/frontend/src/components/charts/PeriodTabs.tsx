import { t, tn } from '@/lib/i18n'

/** The windows a dashboard offers, as a row of toggle buttons. */
export function PeriodTabs({ value, onChange, options = [7, 30, 90] }: { value: number; onChange: (days: number) => void; options?: readonly number[] }) {
  return (
    <div className="tzc-period" role="group" aria-label={t('Period')}>
      {options.map((days) => (
        <button key={days} type="button" aria-pressed={value === days} onClick={() => onChange(days)}>{tn(days, '{n} day', '{n} days')}</button>
      ))}
    </div>
  )
}
