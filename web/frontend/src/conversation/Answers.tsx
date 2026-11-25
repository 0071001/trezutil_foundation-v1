import { useState, type FormEvent } from 'react'
import { Icon } from '@/ui/Icon'
import { t } from '@/lib/i18n'
import { asDateField, fieldDate, inDays, nextSaturday } from '@/lib/dates'

/*
 * Answering the assistant without typing: a date picker when it asks for a date, a counter when it asks for a number.
 * Each sends exactly what the person would have typed ("2026-12-01 14:00", "3"), which the runtime already reads.
 */

const DATE_PARAMS = ['start_at', 'end_at']
const COUNT_PARAMS = ['nights', 'party_size', 'quantity']

export const asksForDate = (name: string) => DATE_PARAMS.includes(name)
export const asksForCount = (name: string) => COUNT_PARAMS.includes(name)

export function DateAnswer({ param, after, disabled, onSend }: { param: string; after?: string; disabled: boolean; onSend: (text: string) => void }) {
  const end = param === 'end_at'
  const hour = end ? 11 : 14
  const start = end && after ? new Date(after) : null
  const min = start ? after! : fieldDate(new Date(), new Date().getHours())
  const [value, setValue] = useState(() => fieldDate(inDays(1, start ?? new Date()), hour))
  const choices: [string, string][] = start
    ? [[t('1 nuit'), fieldDate(inDays(1, start), hour)], [t('2 nuits'), fieldDate(inDays(2, start), hour)], [t('1 semaine'), fieldDate(inDays(7, start), hour)]]
    : [[t('Demain'), fieldDate(inDays(1), hour)], [t('Ce week-end'), fieldDate(nextSaturday(), hour)], [t('Dans une semaine'), fieldDate(inDays(7), hour)]]
  const valid = asDateField(value) !== '' && value >= min
  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (valid && !disabled) onSend(value.replace('T', ' '))
  }
  return (
    <form className="answer-box" onSubmit={submit} noValidate aria-label={end ? t('Choisir la date de fin') : t('Choisir la date de début')}>
      <label className="answer-label">
        <span><Icon name="calendar" size={15} />{end ? t('Date et heure de fin') : t('Date et heure de début')}</span>
        <input type="datetime-local" value={value} min={min} onChange={(e) => setValue(e.target.value)} />
      </label>
      <div className="answer-chips" role="group" aria-label={t('Dates rapides')}>
        {choices.map(([label, v]) => <button type="button" key={label} aria-pressed={value === v} onClick={() => setValue(v)}>{label}</button>)}
      </div>
      {!valid && value && <p className="form-error" role="alert">{start ? t('La fin doit venir après le début.') : t('Choisis une date à venir.')}</p>}
      <button type="submit" className="cta primary" disabled={!valid || disabled}>{t('Valider la date')} <Icon name="arrow" size={15} /></button>
    </form>
  )
}

export function CountAnswer({ param, disabled, onSend }: { param: string; disabled: boolean; onSend: (text: string) => void }) {
  const [value, setValue] = useState(param === 'party_size' ? 2 : 1)
  const label = param === 'nights' ? t('Nombre de nuits') : param === 'party_size' ? t('Nombre de personnes') : t('Quantité')
  return (
    <form className="answer-box row" aria-label={label} onSubmit={(e) => { e.preventDefault(); if (!disabled) onSend(String(value)) }}>
      <span className="answer-label">{label}</span>
      <div className="stepper">
        <button type="button" onClick={() => setValue((v) => Math.max(1, v - 1))} disabled={value <= 1} aria-label={t('Retirer un')}>−</button>
        <output aria-live="polite">{value}</output>
        <button type="button" onClick={() => setValue((v) => Math.min(99, v + 1))} disabled={value >= 99} aria-label={t('Ajouter un')}>+</button>
      </div>
      <button type="submit" className="cta primary" disabled={disabled}>{t('Valider')} <Icon name="arrow" size={15} /></button>
    </form>
  )
}
