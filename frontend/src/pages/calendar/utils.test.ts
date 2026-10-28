import { describe, expect, it } from 'vitest'
import type { Booking } from '@/lib/types'
import { byDay, layoutDay, snap, visibleRange } from './utils'

const day = new Date(2026, 9, 15) // 15 Oct 2026 local midnight

function b(id: string, sh: number, sm: number, eh: number, em: number): Booking {
  const s = new Date(day)
  s.setHours(sh, sm)
  const e = new Date(day)
  e.setHours(eh, em)
  return { id, startsAt: s.toISOString(), endsAt: e.toISOString() } as Booking
}

describe('layoutDay', () => {
  it('places non-overlapping events in a single lane', () => {
    const out = layoutDay([b('a', 9, 0, 10, 0), b('b', 10, 0, 11, 0)], day, 7 * 60, 23 * 60)
    expect(out.map((p) => [p.col, p.cols])).toEqual([
      [0, 1],
      [0, 1],
    ])
    expect(out[0].top).toBe(120) // 09:00 - 07:00
    expect(out[0].height).toBe(60)
  })

  it('splits overlapping events into lanes and reuses freed lanes', () => {
    const out = layoutDay([b('a', 9, 0, 11, 0), b('b', 9, 30, 10, 0), b('c', 10, 0, 10, 30)], day, 0, 24 * 60)
    const byId = Object.fromEntries(out.map((p) => [p.booking.id, p]))
    expect(byId.a.col).toBe(0)
    expect(byId.b.col).toBe(1)
    expect(byId.c.col).toBe(1) // lane 1 is free again at 10:00
    expect(out.every((p) => p.cols === 2)).toBe(true)
  })

  it('clips events to the visible hours', () => {
    const out = layoutDay([b('a', 5, 0, 8, 0)], day, 7 * 60, 23 * 60)
    expect(out[0].top).toBe(0)
    expect(out[0].height).toBe(60)
    expect(layoutDay([b('x', 1, 0, 2, 0)], day, 7 * 60, 23 * 60)).toHaveLength(0)
  })
})

describe('visibleRange', () => {
  it('week starts on Sunday', () => {
    const { from, to } = visibleRange('week', day) // Thursday
    expect(from.getDay()).toBe(0)
    expect((to.getTime() - from.getTime()) / 86400000).toBeCloseTo(7, 0)
  })
  it('month covers whole weeks', () => {
    const { from, to } = visibleRange('month', day)
    expect(from.getDay()).toBe(0)
    expect(Math.round((to.getTime() - from.getTime()) / 86400000) % 7).toBe(0)
  })
})

describe('helpers', () => {
  it('snaps to 15 minutes', () => {
    expect(snap(7)).toBe(0)
    expect(snap(8)).toBe(15)
    expect(snap(52)).toBe(45)
  })
  it('groups by day and sorts', () => {
    const m = byDay([b('late', 18, 0, 19, 0), b('early', 8, 0, 9, 0)])
    const list = [...m.values()][0]
    expect(list.map((x) => x.id)).toEqual(['early', 'late'])
  })
})
