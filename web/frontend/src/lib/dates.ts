/* The values of date-and-time fields ("2026-12-01T14:00"), in the person's own time. */

const pad = (n: number) => String(n).padStart(2, '0')

export const fieldDate = (d: Date, hour: number) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(hour)}:00`

export function inDays(days: number, from = new Date()): Date {
  const d = new Date(from)
  d.setDate(d.getDate() + days)
  return d
}

/** The coming Saturday (next week's when today is one). */
export function nextSaturday(from = new Date()): Date {
  return inDays((6 - from.getDay() + 7) % 7 || 7, from)
}

/** A date typed or picked ("2026-12-01 14:00"), as a field value; '' for anything else. */
export function asDateField(text: string): string {
  const m = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2})$/.exec(text.trim())
  return m ? `${m[1]}T${m[2]}` : ''
}
