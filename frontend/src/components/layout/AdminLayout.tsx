import clsx from 'clsx'
import { useQuery } from '@tanstack/react-query'
import {
  BarChart3,
  Bell,
  Building2,
  CalendarDays,
  ClipboardList,
  CreditCard,
  LayoutGrid,
  ListChecks,
  LogOut,
  Menu,
  Monitor,
  Moon,
  Search,
  Settings2,
  ShieldCheck,
  Sparkles,
  Sun,
  UserCog,
  Users,
  X,
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { api } from '@/lib/api'
import { useAuth, useCan, useUser } from '@/lib/auth'
import { relative } from '@/lib/format'
import { roleLabel } from '@/lib/labels'
import { usePrefs } from '@/lib/prefs'
import { useBuildings } from '@/lib/queries'
import { useLiveInvalidation } from '@/lib/realtime'
import type { BookingRequest, Notification, PaymentSummary } from '@/lib/types'
import { Avatar, IconButton, Select } from '../ui'
import { CommandPalette } from './CommandPalette'
import { Logo } from './Logo'

interface NavItem {
  to: string
  label: string
  icon: ReactNode
  perm?: string
  badge?: number
  end?: boolean
}

export function AdminLayout() {
  const can = useCan()
  const user = useUser()
  const logout = useAuth((s) => s.logout)
  const nav = useNavigate()
  const location = useLocation()
  const { sidebarOpen, setSidebar, buildingId, setBuilding, theme, toggleTheme } = usePrefs()
  const buildings = useBuildings().data ?? []
  const live = useLiveInvalidation(true)
  const [palette, setPalette] = useState(false)

  const pending = useQuery({
    queryKey: ['requests', 'pending'],
    queryFn: () => api.get<BookingRequest[]>('/requests', { status: 'pending' }),
    enabled: can('requests:review'),
  })
  const pay = useQuery({
    queryKey: ['payments', 'summary'],
    queryFn: () => api.get<PaymentSummary>('/payments/summary'),
    enabled: can('payments:view'),
  })

  useEffect(() => setSidebar(false), [location.pathname, setSidebar])
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPalette(true)
      }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [])

  const main: NavItem[] = [
    { to: '/admin', label: 'לוח בקרה', icon: <LayoutGrid />, perm: 'dashboard:view', end: true },
    { to: '/admin/calendar', label: 'יומן הזמנות', icon: <CalendarDays />, perm: 'calendar:view' },
    { to: '/admin/bookings', label: 'הזמנות ובקשות', icon: <ClipboardList />, perm: 'requests:review', badge: pending.data?.length },
    { to: '/admin/rooms', label: 'חדרים ואולמות', icon: <Building2 />, perm: 'rooms:manage' },
    { to: '/admin/activities', label: 'פעילויות', icon: <Sparkles />, perm: 'activities:manage' },
    { to: '/admin/customers', label: 'לקוחות', icon: <Users />, perm: 'customers:manage' },
    { to: '/admin/coordinators', label: 'רכזים', icon: <UserCog />, perm: 'bookings:manage' },
    { to: '/admin/payments', label: 'תשלומים', icon: <CreditCard />, perm: 'payments:view', badge: pay.data?.unpaidCount },
    { to: '/admin/reports', label: 'דוחות', icon: <BarChart3 />, perm: 'reports:view' },
    { to: '/admin/users', label: 'משתמשים והרשאות', icon: <ShieldCheck />, perm: 'users:manage' },
    { to: '/admin/settings', label: 'הגדרות מערכת', icon: <Settings2 />, perm: 'settings:manage' },
  ]
  const extra: NavItem[] = [
    { to: '/admin/tasks', label: 'משימות תפעול', icon: <ListChecks />, perm: 'tasks:manage' },
    { to: '/admin/signage', label: 'שילוט דיגיטלי', icon: <Monitor />, perm: 'signage:manage' },
    { to: '/portal', label: 'פורטל רכזים (הדגמה)', icon: <UserCog />, perm: 'requests:review' },
    { to: '/staff', label: 'אפליקציית צוות', icon: <ListChecks />, perm: 'tasks:manage' },
  ]

  const renderNav = (items: NavItem[]) =>
    items
      .filter((i) => !i.perm || can(i.perm))
      .map((i) => (
        <NavLink
          key={i.to}
          to={i.to}
          end={i.end}
          className={({ isActive }) =>
            clsx(
              'group flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition [&>svg]:h-[18px] [&>svg]:w-[18px]',
              isActive ? 'bg-brand-soft text-brand-strong' : 'text-ink/80 hover:bg-subtle hover:text-ink',
            )
          }
        >
          {i.icon}
          <span className="flex-1">{i.label}</span>
          {!!i.badge && <span className="rounded-full bg-amber-100 px-1.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-900 dark:text-amber-200">{i.badge}</span>}
        </NavLink>
      ))

  const sidebar = (
    <div className="flex h-full flex-col gap-5 p-4">
      <div className="flex items-center justify-between">
        <Logo />
        <IconButton label="סגירה" className="lg:hidden" onClick={() => setSidebar(false)}>
          <X className="h-4 w-4" />
        </IconButton>
      </div>
      {buildings.length > 1 && (
        <div>
          <span className="label">מתנ״ס פעיל</span>
          <Select value={buildingId ?? ''} onChange={(e) => setBuilding(e.target.value || null)}>
            <option value="">כל המבנים</option>
            {buildings.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </Select>
        </div>
      )}
      <nav className="scroll-thin -mx-1 flex flex-1 flex-col gap-0.5 overflow-y-auto px-1">
        {renderNav(main)}
        <p className="mb-1 mt-5 px-3 text-[11px] font-semibold uppercase tracking-wide text-muted">מודולים נוספים</p>
        {renderNav(extra)}
        <a href="/signage/lobby" target="_blank" rel="noreferrer" className="mt-1 flex items-center gap-3 rounded-xl px-3 py-2 text-xs text-muted hover:bg-subtle">
          <span className={clsx('h-2 w-2 rounded-full', live ? 'bg-ok animate-pulse2' : 'bg-line')} />
          {live ? 'מחובר בזמן אמת' : 'מתחבר…'} · פתיחת מסך לובי
        </a>
      </nav>
    </div>
  )

  return (
    <div className="flex h-full">
      {/* Sidebar: right side in RTL */}
      <aside className="hidden w-64 shrink-0 border-e border-line bg-surface lg:block">{sidebar}</aside>
      {sidebarOpen && (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setSidebar(false)} />
          <aside className="absolute inset-y-0 start-0 w-72 animate-slide-up bg-surface shadow-pop">{sidebar}</aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-line bg-surface/85 px-4 backdrop-blur lg:px-6">
          <IconButton label="תפריט" className="lg:hidden" onClick={() => setSidebar(true)}>
            <Menu className="h-5 w-5" />
          </IconButton>
          <button
            onClick={() => setPalette(true)}
            className="flex h-10 max-w-md flex-1 items-center gap-2 rounded-xl border border-line bg-subtle px-3 text-sm text-muted transition hover:border-brand/40"
          >
            <Search className="h-4 w-4" />
            <span className="flex-1 truncate text-start">חיפוש הזמנה, לקוח, חדר או טלפון…</span>
            <kbd className="hidden rounded-md border border-line bg-surface px-1.5 text-[10px] sm:inline">Ctrl K</kbd>
          </button>
          <div className="ms-auto flex items-center gap-1">
            <IconButton label={theme === 'dark' ? 'מצב בהיר' : 'מצב כהה'} onClick={toggleTheme}>
              {theme === 'dark' ? <Sun className="h-[18px] w-[18px]" /> : <Moon className="h-[18px] w-[18px]" />}
            </IconButton>
            <NotificationsBell />
            <div className="mx-2 hidden h-8 w-px bg-line sm:block" />
            {user && (
              <div className="flex items-center gap-2.5">
                <div className="hidden text-end leading-tight sm:block">
                  <p className="text-sm font-semibold">{user.fullName}</p>
                  <p className="text-xs text-muted">{roleLabel[user.role]}</p>
                </div>
                <Avatar name={user.fullName} color={user.avatarColor} />
                <IconButton
                  label="התנתקות"
                  onClick={async () => {
                    await logout()
                    nav('/login')
                  }}
                >
                  <LogOut className="h-[18px] w-[18px]" />
                </IconButton>
              </div>
            )}
          </div>
        </header>
        <main className="scroll-thin flex-1 overflow-y-auto">
          <div className="mx-auto w-full max-w-[1600px] p-4 lg:p-6">
            <Outlet />
          </div>
        </main>
      </div>
      <CommandPalette open={palette} onClose={() => setPalette(false)} />
    </div>
  )
}

export function NotificationsBell() {
  const [open, setOpen] = useState(false)
  const { data = [], refetch } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => api.get<Notification[]>('/notifications', { limit: 20 }),
    refetchInterval: 60_000,
  })
  const unread = data.filter((n) => !n.readAt).length
  return (
    <div className="relative">
      <IconButton
        label="התראות"
        onClick={async () => {
          setOpen((o) => !o)
          if (unread) {
            await api.post('/notifications/read')
            refetch()
          }
        }}
      >
        <Bell className="h-[18px] w-[18px]" />
        {unread > 0 && <span className="absolute end-1.5 top-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-danger px-1 text-[10px] font-bold text-white">{unread}</span>}
      </IconButton>
      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className="absolute end-0 top-11 z-50 w-80 animate-slide-up overflow-hidden rounded-2xl border border-line bg-surface shadow-pop">
            <p className="border-b border-line px-4 py-3 text-sm font-semibold">התראות</p>
            <div className="scroll-thin max-h-96 overflow-y-auto">
              {data.length === 0 && <p className="p-6 text-center text-sm text-muted">אין התראות חדשות</p>}
              {data.map((n) => (
                <div key={n.id} className="border-b border-line/60 px-4 py-3 last:border-0">
                  <p className="text-sm font-medium">{n.subject || 'הודעה'}</p>
                  <p className="mt-0.5 line-clamp-2 whitespace-pre-line text-xs text-muted">{n.body}</p>
                  <p className="mt-1 text-[11px] text-muted/80">{relative(n.createdAt)}</p>
                </div>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
