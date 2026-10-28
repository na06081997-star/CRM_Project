import {
  addDays,
  addMonths,
  addWeeks,
  addYears,
  endOfMonth,
  endOfWeek,
  endOfYear,
  startOfDay,
  startOfMonth,
  startOfWeek,
  startOfYear,
} from 'date-fns'
import type { Booking } from '@/lib/types'

export type View = 'year' | 'month' | 'week' | 'day' | 'rooms' | 'coordinators'

export const WEEK = { weekStartsOn: 0 as const } // Sunday

export function visibleRange(view: View, anchor: Date): { from: Date; to: Date } {
  switch (view) {
    case 'year':
      return { from: startOfYear(anchor), to: addDays(endOfYear(anchor), 1) }
    case 'month': {
      // Full weeks around the month, like Google Calendar.
      const from = startOfWeek(startOfMonth(anchor), WEEK)
      const to = addDays(endOfWeek(endOfMonth(anchor), WEEK), 1)
      return { from: startOfDay(from), to: startOfDay(to) }
    }
    case 'week':
      return { from: startOfWeek(anchor, WEEK), to: addDays(startOfWeek(anchor, WEEK), 7) }
    default:
      return { from: startOfDay(anchor), to: addDays(startOfDay(anchor), 1) }
  }
}

export function step(view: View, anchor: Date, dir: 1 | -1): Date {
  switch (view) {
    case 'year':
      return addYears(anchor, dir)
    case 'month':
      return addMonths(anchor, dir)
    case 'week':
      return addWeeks(anchor, dir)
    default:
      return addDays(anchor, dir)
  }
}

export interface Placed {
  booking: Booking
  top: number // minutes from grid start
  height: number // minutes
  col: number
  cols: number
}

/**
 * Lays out the bookings of one column (day or resource): clusters of
 * mutually overlapping events are split into side-by-side lanes.
 */
export function layoutDay(bookings: Booking[], dayStart: Date, gridStartMin: number, gridEndMin: number): Placed[] {
  const items = bookings
    .map((b) => {
      const s = (new Date(b.startsAt).getTime() - dayStart.getTime()) / 60000
      const e = (new Date(b.endsAt).getTime() - dayStart.getTime()) / 60000
      return { b, s: Math.max(s, gridStartMin), e: Math.min(e, gridEndMin) }
    })
    .filter((x) => x.e > x.s)
    .sort((a, b) => a.s - b.s || b.e - a.e)

  const out: Placed[] = []
  let cluster: { item: (typeof items)[number]; col: number }[] = []
  let lanes: number[] = [] // end minute of the last event in each lane
  let clusterEnd = -Infinity

  const flush = () => {
    const cols = lanes.length
    for (const c of cluster) {
      out.push({ booking: c.item.b, top: c.item.s - gridStartMin, height: c.item.e - c.item.s, col: c.col, cols })
    }
    cluster = []
    lanes = []
  }

  for (const it of items) {
    if (it.s >= clusterEnd) {
      flush()
      clusterEnd = -Infinity
    }
    let lane = lanes.findIndex((end) => end <= it.s)
    if (lane === -1) {
      lane = lanes.length
      lanes.push(it.e)
    } else lanes[lane] = it.e
    cluster.push({ item: it, col: lane })
    clusterEnd = Math.max(clusterEnd, it.e)
  }
  flush()
  return out
}

export const SNAP = 15

export const snap = (min: number, to = SNAP) => Math.round(min / to) * to

export function minutesToDate(dayStart: Date, minutes: number) {
  return new Date(dayStart.getTime() + minutes * 60000)
}

/** Groups bookings by local calendar day key (yyyy-mm-dd). */
export function byDay(bookings: Booking[]) {
  const m = new Map<string, Booking[]>()
  for (const b of bookings) {
    const d = new Date(b.startsAt)
    const k = dayKey(d)
    const arr = m.get(k)
    if (arr) arr.push(b)
    else m.set(k, [b])
  }
  for (const arr of m.values()) arr.sort((a, b) => a.startsAt.localeCompare(b.startsAt))
  return m
}

export const dayKey = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
