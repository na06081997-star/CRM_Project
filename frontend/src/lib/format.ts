import { format, formatDistanceToNowStrict, isSameDay, parseISO } from 'date-fns'
import { he } from 'date-fns/locale'

export const toDate = (v: string | Date) => (typeof v === 'string' ? parseISO(v) : v)

export const fmt = (v: string | Date, pattern: string) => format(toDate(v), pattern, { locale: he })

export const time = (v: string | Date) => fmt(v, 'HH:mm')
export const dateShort = (v: string | Date) => fmt(v, 'dd.MM')
export const dateLong = (v: string | Date) => fmt(v, 'EEEE, d בMMMM yyyy')
export const dateTime = (v: string | Date) => fmt(v, 'dd.MM.yy HH:mm')

export function timeRange(start: string | Date, end: string | Date) {
  const s = toDate(start)
  const e = toDate(end)
  return isSameDay(s, e) ? `${time(s)}–${time(e)}` : `${fmt(s, 'dd.MM HH:mm')} – ${fmt(e, 'dd.MM HH:mm')}`
}

export const relative = (v: string | Date) => formatDistanceToNowStrict(toDate(v), { locale: he, addSuffix: true })

const ils = new Intl.NumberFormat('he-IL', { style: 'currency', currency: 'ILS', maximumFractionDigits: 0 })
const ils2 = new Intl.NumberFormat('he-IL', { style: 'currency', currency: 'ILS', minimumFractionDigits: 2 })

export const money = (n: number, cents = false) => (cents ? ils2 : ils).format(n || 0)
export const num = (n: number) => new Intl.NumberFormat('he-IL').format(n || 0)

export function minutesLabel(m: number) {
  if (m < 60) return `${m} דק׳`
  const h = Math.floor(m / 60)
  const r = m % 60
  return r ? `${h} ש׳ ${r} דק׳` : `${h} ש׳`
}

export function bytes(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

/** "YYYY-MM-DDTHH:mm" for <input type="datetime-local"> in local time. */
export const toLocalInput = (v: string | Date) => fmt(v, "yyyy-MM-dd'T'HH:mm")
export const fromLocalInput = (v: string) => new Date(v).toISOString()

export function initials(name: string) {
  const parts = name.trim().split(/\s+/)
  return (parts[0]?.[0] || '') + (parts[1]?.[0] || '')
}
