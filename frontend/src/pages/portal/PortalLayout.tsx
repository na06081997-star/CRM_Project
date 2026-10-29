import clsx from 'clsx'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarDays, CreditCard, HelpCircle, LayoutDashboard, LayoutGrid, ListChecks, LogOut, Menu, Moon, Plus, Search, Sparkles, Sun, X } from 'lucide-react'
import { useEffect, type ReactNode } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { NotificationsBell } from '@/components/layout/AdminLayout'
import { Logo, PoweredBy } from '@/components/layout/Logo'
import { toast } from '@/components/toast'
import { Avatar, Button, IconButton } from '@/components/ui'
import { api } from '@/lib/api'
import { useAuth, useCan, useUser } from '@/lib/auth'
import { roleLabel } from '@/lib/labels'
import { usePrefs } from '@/lib/prefs'
import { useSocket } from '@/lib/realtime'
import type { Booking, BookingRequest } from '@/lib/types'

interface NavItem {
  to: string
  label: string
  icon: ReactNode
  badge?: number
  end?: boolean
  show?: boolean
}

/** Shared queries for the portal (cached across pages). */
export const useMyRequests = () => useQuery({ queryKey: ['requests', 'mine'], queryFn: () => api.get<BookingRequest[]>('/requests') })
export const useMyBalances = () => useQuery({ queryKey: ['balances', 'mine'], queryFn: () => api.get<Booking[]>('/payments/balances') })

/**
 * CRM workspace for coordinators (רכז) and portal users. Same look as the
 * admin interface, but every list is scoped to the signed-in user.
 */
export function PortalLayout() {
  const user = useUser()
  const can = useCan()
  const logout = useAuth((s) => s.logout)
  const nav = useNavigate()
  const qc = useQueryClient()
  const location = useLocation()
  const { sidebarOpen, setSidebar, theme, toggleTheme } = usePrefs()
  const requests = useMyRequests().data ?? []
  const balances = useMyBalances().data ?? []
  const coordinator = can('requests:activity')

  useEffect(() => setSidebar(false), [location.pathname, setSidebar])
  const live = useSocket('/ws', (ev) => {
    if (ev.type === 'booking.changed') {
      qc.invalidateQueries({ queryKey: ['blocks'] })
      qc.invalidateQueries({ queryKey: ['bookings'] })
      qc.invalidateQueries({ queryKey: ['availability'] })
    }
    if (ev.type === 'request.changed') {
      qc.invalidateQueries({ queryKey: ['requests'] })
      qc.invalidateQueries({ queryKey: ['bookings'] })
    }
    if (ev.type === 'payment.changed') qc.invalidateQueries({ queryKey: ['balances'] })
    if (ev.type === 'notification') {
      qc.invalidateQueries({ queryKey: ['notifications'] })
      const d = ev.data as { subject?: string }
      if (d?.subject) toast.info(d.subject)
    }
  })

  const pending = requests.filter((r) => r.status === 'pending').length
  const items: NavItem[] = [
    { to: '/portal', label: 'לוח בקרה', icon: <LayoutGrid />, end: true },
    { to: '/portal/calendar', label: 'יומן זמינות', icon: <CalendarDays />, show: can('availability:view') },
    { to: '/portal/search', label: 'חיפוש חלל', icon: <Search /> },
    { to: '/portal/requests', label: 'הבקשות שלי', icon: <ListChecks />, badge: pending },
    { to: '/portal/bookings', label: coordinator ? 'הפעילויות שלי' : 'ההזמנות שלי', icon: <Sparkles /> },
    { to: '/portal/payments', label: 'תשלומים', icon: <CreditCard />, badge: balances.length },
    { to: '/portal/help', label: 'עזרה ויצירת קשר', icon: <HelpCircle /> },
  ]

  const sidebar = (
    <div className="flex h-full flex-col gap-5 p-4">
      <div className="flex items-center justify-between">
        <Logo subtitle={coordinator ? 'אזור רכז' : 'פורטל הזמנות'} />
        <IconButton label="סגירה" className="lg:hidden" onClick={() => setSidebar(false)}>
          <X className="h-4 w-4" />
        </IconButton>
      </div>
      <Button icon={<Plus className="h-4 w-4" />} onClick={() => nav('/portal/new')}>
        בקשה חדשה
      </Button>
      <nav className="scroll-thin -mx-1 flex flex-1 flex-col gap-0.5 overflow-y-auto px-1">
        {items
          .filter((i) => i.show !== false)
          .map((i) => (
            <NavLink
              key={i.to}
              to={i.to}
              end={i.end}
              className={({ isActive }) =>
                clsx(
                  'flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition [&>svg]:h-[18px] [&>svg]:w-[18px]',
                  isActive ? 'bg-brand-soft text-brand-strong' : 'text-ink/80 hover:bg-subtle hover:text-ink',
                )
              }
            >
              {i.icon}
              <span className="flex-1">{i.label}</span>
              {!!i.badge && <span className="rounded-full bg-amber-100 px-1.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-900 dark:text-amber-200">{i.badge}</span>}
            </NavLink>
          ))}
        <p className="mt-auto flex items-center gap-2 px-3 pt-4 text-xs text-muted">
          <span className={clsx('h-2 w-2 rounded-full', live ? 'animate-pulse2 bg-ok' : 'bg-line')} />
          {live ? 'מחובר בזמן אמת' : 'מתחבר…'}
        </p>
        <p className="px-3 text-[11px] leading-relaxed text-muted">מוצגים רק הבקשות, הפעילויות והתשלומים שלך. ביומן מוצג לכולם רק פנוי / תפוס.</p>
      </nav>
      <PoweredBy className="border-t border-line px-3 pt-3" />
    </div>
  )

  return (
    <div className="flex h-full">
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
          <Link to="/portal/search" className="flex h-10 max-w-md flex-1 items-center gap-2 rounded-xl border border-line bg-subtle px-3 text-sm text-muted transition hover:border-brand/40">
            <Search className="h-4 w-4" />
            <span className="truncate">מתי פנוי, ואיזה חלל מתאים לי?</span>
          </Link>
          <div className="ms-auto flex items-center gap-1">
            {can('dashboard:view') && (
              <Link to="/admin" className="me-1 hidden sm:block">
                <Button size="sm" variant="secondary" icon={<LayoutDashboard className="h-4 w-4" />}>
                  לממשק הניהול
                </Button>
              </Link>
            )}
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
          <div className="mx-auto w-full max-w-[1500px] p-4 lg:p-6">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
