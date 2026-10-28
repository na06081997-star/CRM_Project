import { lazy, Suspense, useEffect, type ReactNode } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { AdminLayout } from './components/layout/AdminLayout'
import { PageLoader } from './components/ui'
import { homeFor, useAuth } from './lib/auth'
import { LoginPage } from './pages/LoginPage'

const Dashboard = lazy(() => import('./pages/admin/DashboardPage').then((m) => ({ default: m.DashboardPage })))
const Calendar = lazy(() => import('./pages/calendar/CalendarPage').then((m) => ({ default: m.CalendarPage })))
const Bookings = lazy(() => import('./pages/admin/BookingsPage').then((m) => ({ default: m.BookingsPage })))
const Rooms = lazy(() => import('./pages/admin/RoomsPage').then((m) => ({ default: m.RoomsPage })))
const Activities = lazy(() => import('./pages/admin/ActivitiesPage').then((m) => ({ default: m.ActivitiesPage })))
const Customers = lazy(() => import('./pages/admin/CustomersPage').then((m) => ({ default: m.CustomersPage })))
const Coordinators = lazy(() => import('./pages/admin/CoordinatorsPage').then((m) => ({ default: m.CoordinatorsPage })))
const Payments = lazy(() => import('./pages/admin/PaymentsPage').then((m) => ({ default: m.PaymentsPage })))
const Reports = lazy(() => import('./pages/admin/ReportsPage').then((m) => ({ default: m.ReportsPage })))
const Users = lazy(() => import('./pages/admin/UsersPage').then((m) => ({ default: m.UsersPage })))
const Settings = lazy(() => import('./pages/admin/SettingsPage').then((m) => ({ default: m.SettingsPage })))
const Tasks = lazy(() => import('./pages/admin/TasksPage').then((m) => ({ default: m.TasksPage })))
const SignageAdmin = lazy(() => import('./pages/admin/SignageAdminPage').then((m) => ({ default: m.SignageAdminPage })))
const Portal = lazy(() => import('./pages/portal/PortalPage').then((m) => ({ default: m.PortalPage })))
const Staff = lazy(() => import('./pages/staff/StaffPage').then((m) => ({ default: m.StaffPage })))
const Signage = lazy(() => import('./pages/signage/SignagePage').then((m) => ({ default: m.SignagePage })))

function RequireAuth({ children, roles }: { children: ReactNode; roles?: string[] }) {
  const session = useAuth((s) => s.session)
  const loc = useLocation()
  if (!session) return <Navigate to="/login" replace state={{ from: loc.pathname + loc.search }} />
  if (roles && !roles.includes(session.user.role)) return <Navigate to={homeFor(session.user.role)} replace />
  return <>{children}</>
}

function Home() {
  const session = useAuth((s) => s.session)
  return <Navigate to={session ? homeFor(session.user.role) : '/signage'} replace />
}

export function App() {
  const { ready, init } = useAuth()
  useEffect(() => {
    init()
  }, [init])

  if (!ready)
    return (
      <div className="flex h-full items-center justify-center">
        <PageLoader />
      </div>
    )

  return (
    <Suspense fallback={<PageLoader />}>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/signage" element={<Signage />} />
        <Route path="/signage/:slug" element={<Signage />} />

        <Route
          path="/admin"
          element={
            <RequireAuth roles={['admin', 'coordinator']}>
              <AdminLayout />
            </RequireAuth>
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="calendar" element={<Calendar />} />
          <Route path="bookings" element={<Bookings />} />
          <Route path="rooms" element={<Rooms />} />
          <Route path="activities" element={<Activities />} />
          <Route path="customers" element={<Customers />} />
          <Route path="coordinators" element={<Coordinators />} />
          <Route path="payments" element={<Payments />} />
          <Route path="reports" element={<Reports />} />
          <Route path="users" element={<Users />} />
          <Route path="settings" element={<Settings />} />
          <Route path="tasks" element={<Tasks />} />
          <Route path="signage" element={<SignageAdmin />} />
        </Route>

        <Route
          path="/portal/*"
          element={
            <RequireAuth>
              <Portal />
            </RequireAuth>
          }
        />
        <Route
          path="/staff/*"
          element={
            <RequireAuth roles={['staff', 'admin', 'coordinator']}>
              <Staff />
            </RequireAuth>
          }
        />
        <Route path="*" element={<Home />} />
      </Routes>
    </Suspense>
  )
}
