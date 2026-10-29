import { lazy, Suspense, useEffect, type ReactNode } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { AdminLayout } from './components/layout/AdminLayout'
import { PageLoader } from './components/ui'
import { homeFor, useAuth } from './lib/auth'
import { useDocumentBranding } from './lib/brand'
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
const PortalLayout = lazy(() => import('./pages/portal/PortalLayout').then((m) => ({ default: m.PortalLayout })))
const PortalHome = lazy(() => import('./pages/portal/HomePage').then((m) => ({ default: m.HomePage })))
const PortalSearch = lazy(() => import('./pages/portal/SearchPage').then((m) => ({ default: m.SearchPage })))
const PortalRoom = lazy(() => import('./pages/portal/RoomPage').then((m) => ({ default: m.RoomPage })))
const PortalNewRequest = lazy(() => import('./pages/portal/NewRequestPage').then((m) => ({ default: m.NewRequestPage })))
const PortalRequests = lazy(() => import('./pages/portal/RequestsPage').then((m) => ({ default: m.RequestsPage })))
const PortalBookings = lazy(() => import('./pages/portal/BookingsPage').then((m) => ({ default: m.BookingsPage })))
const PortalPayments = lazy(() => import('./pages/portal/PaymentsPage').then((m) => ({ default: m.PaymentsPage })))
const PortalHelp = lazy(() => import('./pages/portal/HelpPage').then((m) => ({ default: m.HelpPage })))
const PortalCalendar = lazy(() => import('./pages/calendar/CalendarPage').then((m) => ({ default: () => <m.CalendarPage mode="portal" /> })))
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
  useDocumentBranding()
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
            <RequireAuth roles={['super_admin', 'admin']}>
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

        <Route path="/rakaz/*" element={<Navigate to="/portal" replace />} />
        <Route
          path="/portal"
          element={
            <RequireAuth roles={['coordinator', 'user', 'admin', 'super_admin']}>
              <PortalLayout />
            </RequireAuth>
          }
        >
          <Route index element={<PortalHome />} />
          <Route path="calendar" element={<PortalCalendar />} />
          <Route path="search" element={<PortalSearch />} />
          <Route path="rooms/:id" element={<PortalRoom />} />
          <Route path="new" element={<PortalNewRequest />} />
          <Route path="requests" element={<PortalRequests />} />
          <Route path="bookings" element={<PortalBookings />} />
          <Route path="payments" element={<PortalPayments />} />
          <Route path="help" element={<PortalHelp />} />
        </Route>
        <Route
          path="/staff/*"
          element={
            <RequireAuth roles={['staff', 'admin', 'super_admin']}>
              <Staff />
            </RequireAuth>
          }
        />
        <Route path="*" element={<Home />} />
      </Routes>
    </Suspense>
  )
}
