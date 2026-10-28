import { LogOut, Moon, Sun } from 'lucide-react'
import type { ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth, useUser } from '@/lib/auth'
import { usePrefs } from '@/lib/prefs'
import { Avatar, IconButton } from '../ui'
import { NotificationsBell } from './AdminLayout'
import { Logo } from './Logo'

/** Top bar for the portal and staff apps. */
export function SimpleHeader({ subtitle, children }: { subtitle: string; children?: ReactNode }) {
  const user = useUser()
  const logout = useAuth((s) => s.logout)
  const nav = useNavigate()
  const { theme, toggleTheme } = usePrefs()
  return (
    <header className="sticky top-0 z-30 border-b border-line bg-surface/90 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center gap-3 px-4">
        <Logo subtitle={subtitle} />
        <div className="ms-auto flex items-center gap-1">
          {children}
          <IconButton label="ערכת צבעים" onClick={toggleTheme}>
            {theme === 'dark' ? <Sun className="h-[18px] w-[18px]" /> : <Moon className="h-[18px] w-[18px]" />}
          </IconButton>
          <NotificationsBell />
          {user && <Avatar name={user.fullName} color={user.avatarColor} size={32} />}
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
      </div>
    </header>
  )
}
