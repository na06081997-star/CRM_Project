import clsx from 'clsx'
import { LogOut, Menu, Moon, Sun } from 'lucide-react'
import type { ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useAuth, useUser } from '@/lib/auth'
import { roleLabel } from '@/lib/labels'
import { usePrefs } from '@/lib/prefs'
import { Avatar } from '../ui'

export interface BottomNavItem {
  to: string
  label: string
  icon: ReactNode
  badge?: number
  end?: boolean
  /** Raised round button in the middle (main action). */
  primary?: boolean
}

/**
 * Phone navigation: the most used screens one thumb away, plus "more" for the
 * full menu. Hidden from the lg breakpoint, where the sidebar is visible.
 */
export function BottomNav({ items, onMore }: { items: BottomNavItem[]; onMore: () => void }) {
  return (
    <nav
      className="fixed inset-x-0 bottom-0 z-30 border-t border-line bg-surface/95 backdrop-blur lg:hidden"
      style={{ paddingBottom: 'env(safe-area-inset-bottom)' }}
      aria-label="ניווט ראשי"
    >
      <div className="mx-auto flex h-16 max-w-lg items-stretch">
        {items.map((i) =>
          i.primary ? (
            <NavLink key={i.to} to={i.to} className="flex flex-1 items-center justify-center" aria-label={i.label}>
              <span className="flex h-12 w-12 -translate-y-3 items-center justify-center rounded-2xl bg-brand text-white shadow-pop ring-4 ring-surface dark:text-slate-900 [&>svg]:h-6 [&>svg]:w-6">
                {i.icon}
              </span>
            </NavLink>
          ) : (
            <NavLink
              key={i.to}
              to={i.to}
              end={i.end}
              className={({ isActive }) =>
                clsx(
                  'relative flex flex-1 flex-col items-center justify-center gap-1 text-[11px] font-medium [&>svg]:h-5 [&>svg]:w-5',
                  isActive ? 'text-brand-strong' : 'text-muted',
                )
              }
            >
              {i.icon}
              <span className="max-w-full truncate px-0.5">{i.label}</span>
              {!!i.badge && (
                <span className="absolute end-[calc(50%-20px)] top-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-amber-500 px-1 text-[10px] font-bold text-white">
                  {i.badge}
                </span>
              )}
            </NavLink>
          ),
        )}
        <button onClick={onMore} className="flex flex-1 flex-col items-center justify-center gap-1 text-[11px] font-medium text-muted [&>svg]:h-5 [&>svg]:w-5">
          <Menu />
          <span>עוד</span>
        </button>
      </div>
    </nav>
  )
}

/** Account block at the bottom of the phone drawer (the header has no room). */
export function DrawerAccount() {
  const user = useUser()
  const logout = useAuth((s) => s.logout)
  const nav = useNavigate()
  const { theme, toggleTheme } = usePrefs()
  if (!user) return null
  return (
    <div className="flex items-center gap-2.5 border-t border-line pt-3 lg:hidden">
      <Avatar name={user.fullName} color={user.avatarColor} size={34} />
      <div className="min-w-0 flex-1 leading-tight">
        <p className="truncate text-sm font-semibold">{user.fullName}</p>
        <p className="text-xs text-muted">{roleLabel[user.role]}</p>
      </div>
      <button onClick={toggleTheme} className="flex h-10 w-10 items-center justify-center rounded-xl text-muted hover:bg-subtle" aria-label="ערכת צבעים">
        {theme === 'dark' ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
      </button>
      <button
        onClick={async () => {
          await logout()
          nav('/login')
        }}
        className="flex h-10 w-10 items-center justify-center rounded-xl text-muted hover:bg-subtle"
        aria-label="התנתקות"
      >
        <LogOut className="h-5 w-5" />
      </button>
    </div>
  )
}
