import clsx from 'clsx'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarClock, DoorOpen, LogIn, Maximize, Users, Wifi, WifiOff } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api, publicFileUrl } from '@/lib/api'
import { fmt, time, timeRange } from '@/lib/format'
import { useSocket } from '@/lib/realtime'
import type { SignageRoom, SignageView } from '@/lib/types'

const hebrewDate = new Intl.DateTimeFormat('he-u-ca-hebrew', { day: 'numeric', month: 'long', year: 'numeric' })

/** Public, login-free lobby display. Updates live over /ws/signage. */
export function SignagePage() {
  const { slug = '' } = useParams()
  const [params] = useSearchParams()
  const preview = params.has('preview')
  const qc = useQueryClient()
  const [now, setNow] = useState(new Date())
  const [slide, setSlide] = useState(0)

  const { data, isError } = useQuery({
    queryKey: ['signage', slug],
    queryFn: () => api.get<SignageView>('/public/signage', { screen: slug || undefined }),
    refetchInterval: 5 * 60_000, // safety net; live updates come over WebSocket
  })
  const connected = useSocket('/ws/signage', () => qc.invalidateQueries({ queryKey: ['signage', slug] }), !preview)

  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 1000)
    return () => clearInterval(t)
  }, [])
  const announcements = data?.announcements ?? []
  const rotate = (data?.screen?.rotateSeconds ?? 12) * 1000
  useEffect(() => {
    if (announcements.length < 2) return
    const t = setInterval(() => setSlide((s) => (s + 1) % announcements.length), rotate)
    return () => clearInterval(t)
  }, [announcements.length, rotate])

  const dark = (data?.screen?.theme ?? 'dark') === 'dark'
  const ann = announcements[slide % Math.max(1, announcements.length)]
  const rooms = data?.rooms ?? []
  const upcoming = (data?.upcoming ?? []).filter((e) => new Date(e.startsAt) > now)

  return (
    <div className={clsx('min-h-full select-none overflow-hidden', dark ? 'bg-[#070b12] text-slate-100' : 'bg-slate-100 text-slate-900')} style={{ fontSize: 'clamp(14px, 1.05vw, 22px)' }}>
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div className={clsx('absolute -start-40 -top-40 h-[40rem] w-[40rem] rounded-full blur-3xl', dark ? 'bg-teal-500/10' : 'bg-teal-300/30')} />
        <div className={clsx('absolute -bottom-40 -end-40 h-[40rem] w-[40rem] rounded-full blur-3xl', dark ? 'bg-violet-500/10' : 'bg-violet-300/30')} />
      </div>

      <div className="relative flex min-h-screen flex-col gap-[1.4em] p-[1.6em]">
        {/* Header */}
        <header className="flex items-center gap-[1.2em]">
          <div className="flex h-[3.2em] w-[3.2em] items-center justify-center rounded-2xl bg-teal-500 text-white shadow-lg shadow-teal-500/30">
            <svg viewBox="0 0 64 64" className="h-[1.9em] w-[1.9em]">
              <path d="M18 44 32 18l14 26" fill="none" stroke="currentColor" strokeWidth="6" strokeLinecap="round" strokeLinejoin="round" />
              <circle cx="32" cy="40" r="4" fill="currentColor" opacity=".7" />
            </svg>
          </div>
          <div>
            <h1 className="text-[2em] font-extrabold leading-tight">{data?.building?.name ?? 'קהילה+'}</h1>
            <p className={clsx('text-[1em]', dark ? 'text-slate-400' : 'text-slate-500')}>{data?.screen?.welcomeText || 'מה קורה עכשיו במתנ״ס'}</p>
          </div>
          <div className="ms-auto flex items-center gap-[2em]">
            {!slug && !preview && (
              <Link to="/login" className="flex items-center gap-2 rounded-2xl bg-teal-500 px-[1.1em] py-[0.6em] text-[0.95em] font-semibold text-white shadow-lg shadow-teal-500/30 hover:bg-teal-400">
                <LogIn className="h-[1.1em] w-[1.1em]" /> כניסה למערכת
              </Link>
            )}
            <div className="hidden gap-[1em] md:flex">
              <Pill dark={dark} label="פעילויות היום" value={data?.stats.today ?? 0} />
              <Pill dark={dark} label="חדרים תפוסים" value={data?.stats.occupied ?? 0} tone="rose" />
              <Pill dark={dark} label="חדרים פנויים" value={data?.stats.available ?? 0} tone="emerald" />
            </div>
            <div className="text-end">
              <p className="text-[3.2em] font-bold leading-none tabular-nums tracking-tight">{fmt(now, 'HH:mm')}</p>
              <p className={clsx('mt-1 text-[0.95em]', dark ? 'text-slate-400' : 'text-slate-500')}>
                {fmt(now, 'EEEE, d בMMMM')} · {hebrewDate.format(now)}
              </p>
            </div>
          </div>
        </header>

        {isError && <p className="rounded-2xl bg-rose-500/15 p-4 text-rose-300">לא ניתן לטעון את נתוני השילוט. מנסה שוב…</p>}

        <div className="grid flex-1 gap-[1.4em] xl:grid-cols-[1fr_28%]">
          {/* Rooms grid */}
          <section className="grid content-start gap-[1em] sm:grid-cols-2 2xl:grid-cols-3">
            {rooms.map((r) => (
              <RoomCard key={r.roomId} room={r} now={now} dark={dark} />
            ))}
            {rooms.length === 0 && data && <p className="text-slate-400">אין חדרים להצגה</p>}
          </section>

          {/* Side: announcements + upcoming */}
          <aside className="flex flex-col gap-[1.4em]">
            {ann && (
              <div key={ann.id} className={clsx('animate-fade-in overflow-hidden rounded-3xl', dark ? 'bg-gradient-to-br from-teal-600 to-teal-800' : 'bg-gradient-to-br from-teal-500 to-teal-700 text-white')}>
                {ann.imageKey && <img src={publicFileUrl(ann.imageKey)} alt="" className="aspect-video w-full object-cover" />}
                <div className="p-[1.3em] text-white">
                  <p className="text-[0.8em] font-semibold uppercase tracking-wide text-teal-100/80">הודעות ועדכונים</p>
                  <h2 className="mt-1 text-[1.6em] font-bold leading-tight">{ann.title}</h2>
                  {ann.body && <p className="mt-2 text-[1.05em] text-teal-50/90">{ann.body}</p>}
                  {announcements.length > 1 && (
                    <div className="mt-4 flex gap-1.5">
                      {announcements.map((a, i) => (
                        <span key={a.id} className={clsx('h-1.5 rounded-full transition-all', i === slide % announcements.length ? 'w-6 bg-white' : 'w-1.5 bg-white/40')} />
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}
            <div className={clsx('flex-1 rounded-3xl p-[1.3em]', dark ? 'bg-white/[0.04] ring-1 ring-white/10' : 'bg-white shadow-sm')}>
              <h2 className="mb-[0.8em] flex items-center gap-2 text-[1.2em] font-bold">
                <CalendarClock className="h-[1.2em] w-[1.2em] text-teal-400" /> בהמשך היום
              </h2>
              <ul className="space-y-[0.7em]">
                {upcoming.length === 0 && <li className={dark ? 'text-slate-500' : 'text-slate-400'}>אין אירועים נוספים היום</li>}
                {upcoming.slice(0, 7).map((e) => {
                  const mins = Math.round((new Date(e.startsAt).getTime() - now.getTime()) / 60000)
                  return (
                    <li key={e.bookingId} className="flex items-center gap-[0.8em]">
                      <span className="w-[3.4em] shrink-0 text-[1.1em] font-bold tabular-nums">{time(e.startsAt)}</span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-semibold">{e.title}</span>
                        <span className={clsx('block truncate text-[0.85em]', dark ? 'text-slate-400' : 'text-slate-500')}>{e.roomName}</span>
                      </span>
                      {mins <= 60 && <span className="shrink-0 rounded-full bg-amber-400/15 px-2 py-0.5 text-[0.8em] font-semibold text-amber-400">בעוד {mins} ד׳</span>}
                    </li>
                  )
                })}
              </ul>
            </div>
          </aside>
        </div>

        <footer className={clsx('flex items-center gap-3 text-[0.8em]', dark ? 'text-slate-500' : 'text-slate-400')}>
          {connected ? <Wifi className="h-4 w-4 text-emerald-400" /> : <WifiOff className="h-4 w-4 text-amber-400" />}
          {connected ? 'מתעדכן בזמן אמת' : preview ? 'תצוגה מקדימה' : 'מתחבר מחדש…'}
          <span className="ms-auto">קהילה+ · שילוט דיגיטלי</span>
          {!preview && slug && (
            <Link to="/login" className="flex items-center gap-1 rounded-lg px-2 py-1 hover:bg-white/10">
              <LogIn className="h-4 w-4" /> כניסה
            </Link>
          )}
          {!preview && (
            <button onClick={() => document.documentElement.requestFullscreen?.()} className="flex items-center gap-1 rounded-lg px-2 py-1 hover:bg-white/10" aria-label="מסך מלא">
              <Maximize className="h-4 w-4" /> מסך מלא
            </button>
          )}
        </footer>
      </div>
    </div>
  )
}

function Pill({ label, value, dark, tone = 'teal' }: { label: string; value: number; dark: boolean; tone?: 'teal' | 'rose' | 'emerald' }) {
  const c = { teal: 'text-teal-400', rose: 'text-rose-400', emerald: 'text-emerald-400' }[tone]
  return (
    <div className={clsx('rounded-2xl px-[1em] py-[0.5em] text-center', dark ? 'bg-white/[0.05] ring-1 ring-white/10' : 'bg-white shadow-sm')}>
      <p className={clsx('text-[1.6em] font-bold leading-none tabular-nums', c)}>{value}</p>
      <p className={clsx('mt-1 text-[0.75em]', dark ? 'text-slate-400' : 'text-slate-500')}>{label}</p>
    </div>
  )
}

const stateStyle = {
  occupied: { label: 'תפוס כעת', dot: 'bg-rose-500', chip: 'bg-rose-500/15 text-rose-400' },
  available: { label: 'פנוי', dot: 'bg-emerald-500', chip: 'bg-emerald-500/15 text-emerald-400' },
  soon: { label: 'מתחיל בקרוב', dot: 'bg-amber-400', chip: 'bg-amber-400/15 text-amber-400' },
  closed: { label: 'סגור', dot: 'bg-slate-500', chip: 'bg-slate-500/20 text-slate-400' },
} as const

function RoomCard({ room, now, dark }: { room: SignageRoom; now: Date; dark: boolean }) {
  // Recompute countdowns every second on the client for smooth timers.
  const cur = room.current
  const next = room.next
  const curEnd = cur ? new Date(cur.endsAt).getTime() : 0
  const live = cur && curEnd > now.getTime()
  const remaining = live ? Math.max(0, Math.ceil((curEnd - now.getTime()) / 60000)) : 0
  const progress = live ? Math.min(100, ((now.getTime() - new Date(cur!.startsAt).getTime()) / (curEnd - new Date(cur!.startsAt).getTime())) * 100) : 0
  const untilNext = next ? Math.max(0, Math.ceil((new Date(next.startsAt).getTime() - now.getTime()) / 60000)) : 0
  const state = live ? (room.state === 'closed' ? 'closed' : 'occupied') : next && untilNext <= 15 ? 'soon' : 'available'
  const st = stateStyle[state]

  return (
    <article className={clsx('relative overflow-hidden rounded-3xl p-[1.2em] transition', dark ? 'bg-white/[0.04] ring-1 ring-white/10' : 'bg-white shadow-sm')}>
      <span className="absolute inset-y-0 start-0 w-[0.35em]" style={{ background: room.color }} />
      <div className="flex items-center justify-between gap-2">
        <h3 className="flex items-center gap-2 text-[1.25em] font-bold">
          <DoorOpen className="h-[1em] w-[1em] opacity-60" /> {room.name}
        </h3>
        <span className={clsx('flex items-center gap-1.5 rounded-full px-[0.7em] py-[0.2em] text-[0.8em] font-semibold', st.chip)}>
          <span className={clsx('h-2 w-2 rounded-full', st.dot, state !== 'available' && 'animate-pulse2')} /> {st.label}
        </span>
      </div>

      {live && cur ? (
        <div className="mt-[0.8em]">
          <p className="truncate text-[1.45em] font-bold leading-tight">{cur.title}</p>
          <p className={clsx('mt-1 flex items-center gap-2 text-[0.9em]', dark ? 'text-slate-400' : 'text-slate-500')}>
            {timeRange(cur.startsAt, cur.endsAt)}
            {cur.host && (
              <>
                <span>·</span>
                <Users className="h-[0.9em] w-[0.9em]" /> {cur.host}
              </>
            )}
          </p>
          <div className={clsx('mt-[0.8em] h-[0.45em] overflow-hidden rounded-full', dark ? 'bg-white/10' : 'bg-slate-100')}>
            <div className="h-full rounded-full bg-gradient-to-l from-rose-400 to-rose-600 transition-all duration-1000" style={{ width: `${progress}%` }} />
          </div>
          <p className="mt-1.5 text-[0.85em] font-medium text-rose-400">נותרו {remaining >= 60 ? `${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, '0')} ש׳` : `${remaining} דקות`}</p>
        </div>
      ) : (
        <p className={clsx('mt-[0.8em] text-[1.3em] font-semibold', dark ? 'text-emerald-400/90' : 'text-emerald-600')}>{state === 'soon' ? 'מתארגנים לפעילות הבאה' : 'החדר פנוי'}</p>
      )}

      <div className={clsx('mt-[0.9em] border-t pt-[0.7em] text-[0.9em]', dark ? 'border-white/10 text-slate-400' : 'border-slate-100 text-slate-500')}>
        {next ? (
          <p className="truncate">
            <span className="font-semibold">הבא:</span> {next.title} · {time(next.startsAt)}
            <span className={clsx('ms-2', untilNext <= 15 && 'font-semibold text-amber-400')}>(בעוד {untilNext >= 60 ? `${Math.floor(untilNext / 60)} ש׳ ${untilNext % 60} ד׳` : `${untilNext} ד׳`})</span>
          </p>
        ) : (
          <p>אין פעילויות נוספות היום</p>
        )}
      </div>
    </article>
  )
}
