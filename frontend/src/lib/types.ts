export type Role = 'super_admin' | 'admin' | 'coordinator' | 'staff' | 'user'
export type BookingType = 'activity' | 'rental' | 'internal' | 'maintenance' | 'closed'
export type BookingStatus = 'pending' | 'confirmed' | 'cancelled' | 'rejected'
export type RequestStatus = 'pending' | 'approved' | 'rejected' | 'cancelled'
export type PaymentMethod = 'bank_transfer' | 'check' | 'cash' | 'credit_card'
export type TaskStatus = 'open' | 'in_progress' | 'done' | 'cancelled'
export type ID = string

export interface User {
  id: ID
  email: string
  fullName: string
  phone: string | null
  role: Role
  staffKind: string | null
  telegramChatId: string | null
  avatarColor: string
  isActive: boolean
  createdAt: string
}

export interface Me {
  user: User
  permissions: string[]
}

export interface Building {
  id: ID
  name: string
  address: string
  opensAt: string
  closesAt: string
}

export interface Room {
  id: ID
  buildingId: ID
  name: string
  kind: string
  capacity: number
  color: string
  description: string
  rules: string
  equipment: string[]
  hourlyRate: number
  minHours: number
  bufferMinutes: number
  isBookable: boolean
  isActive: boolean
  imageKey: string | null
  sortOrder: number
}

export interface EquipmentItem {
  id: ID
  code: string
  label: string
  flatPrice: number
}

export interface Activity {
  id: ID
  name: string
  description: string
  coordinatorId: ID | null
  defaultRoomId: ID | null
  color: string
  audience: string
  isActive: boolean
  sessionsCount: number
}

export interface Customer {
  id: ID
  name: string
  kind: 'private' | 'organization' | 'internal'
  contactName: string | null
  phone: string | null
  email: string | null
  taxId: string | null
  notes: string
  userId: ID | null
  createdAt: string
  bookingsCount: number
  totalBilled: number
  totalPaid: number
}

export interface Booking {
  id: ID
  roomId: ID
  title: string
  type: BookingType
  status: BookingStatus
  startsAt: string
  endsAt: string
  bufferMinutes: number
  activityId: ID | null
  coordinatorId: ID | null
  customerId: ID | null
  requestId: ID | null
  seriesId: ID | null
  attendees: number
  equipment: string[]
  tablesCount: number
  chairsCount: number
  price: number
  notes: string
  internalNotes?: string
  createdBy: ID | null
  createdAt: string
  cancelRequestedAt: string | null
  cancelRequestedBy: ID | null
  cancelRequestReason: string
  roomName: string
  buildingId: ID
  coordinatorName: string | null
  customerName: string | null
  paidAmount: number
}

export interface BookingRequest {
  id: ID
  userId: ID
  customerId: ID | null
  roomId: ID
  route: 'rental' | 'activity'
  activityId: ID | null
  tablesCount: number
  chairsCount: number
  title: string
  eventKind: string
  startsAt: string
  endsAt: string
  attendees: number
  equipment: string[]
  notes: string
  status: RequestStatus
  quotedPrice: number
  finalPrice: number | null
  internalNotes?: string
  rejectionReason: string
  reviewedBy: ID | null
  reviewedAt: string | null
  bookingId: ID | null
  createdAt: string
  roomName: string
  requesterName: string
  requesterEmail: string
  requesterPhone: string | null
  requesterRole: Role
  activityName: string | null
  bookingStatus: BookingStatus | null
  cancelRequested: boolean
  hasConflict: boolean
}

export interface Payment {
  id: ID
  bookingId: ID
  amount: number
  method: PaymentMethod
  reference: string
  paidAt: string
  notes: string
  recordedBy: ID | null
  createdAt: string
  bookingTitle: string
  customerName: string | null
}

export interface PaymentSummary {
  totalBilled: number
  totalPaid: number
  outstanding: number
  unpaidCount: number
  byMethod: Partial<Record<PaymentMethod, number>>
  paidThisMonth: number
}

export interface Task {
  id: ID
  title: string
  description: string
  kind: string
  status: TaskStatus
  roomId: ID | null
  bookingId: ID | null
  scheduleId: ID | null
  assigneeId: ID | null
  dueAt: string
  completedAt: string | null
  completionNote: string
  reminderSentAt: string | null
  createdAt: string
  roomName: string | null
  assigneeName: string | null
}

export interface TaskSchedule {
  id: ID
  title: string
  kind: string
  roomId: ID | null
  assigneeId: ID | null
  weekdays: number[]
  timeOfDay: string
  durationMinutes: number
  isActive: boolean
}

export interface Conflict {
  bookingId: ID
  title: string
  startsAt: string
  endsAt: string
  reason: 'overlap' | 'buffer' | 'maintenance'
}

export interface PriceQuote {
  hours: number
  billableHours: number
  base: number
  extras: number
  total: number
  lines: { label: string; amount: number }[] | null
}

export interface SignageScreen {
  id: ID
  slug: string
  name: string
  buildingId: ID | null
  theme: 'dark' | 'light'
  welcomeText: string
  rotateSeconds: number
  roomIds: ID[]
  isActive: boolean
}

export interface Announcement {
  id: ID
  title: string
  body: string
  imageKey: string | null
  startsAt: string
  endsAt: string | null
  priority: number
  screenId: ID | null
  createdAt: string
}

export interface SignageEvent {
  bookingId: ID
  title: string
  type: BookingType
  startsAt: string
  endsAt: string
  roomId: ID
  roomName: string
  host: string | null
}

export interface SignageRoom {
  roomId: ID
  name: string
  color: string
  state: 'available' | 'occupied' | 'soon' | 'closed'
  current: SignageEvent | null
  next: SignageEvent | null
  minutesRemaining: number
  minutesUntilNext: number
  progressPercent: number
}

export interface SignageView {
  screen: SignageScreen | null
  building: Building | null
  generatedAt: string
  rooms: SignageRoom[]
  upcoming: SignageEvent[] | null
  announcements: Announcement[] | null
  stats: { occupied: number; available: number; today: number }
}

export interface Dashboard {
  todayBookings: number
  ongoingNow: number
  pendingRequests: number
  occupancyToday: number
  occupancyWeek: number
  openTasks: number
  payments: PaymentSummary
  today: Booking[]
  latestRequests: BookingRequest[]
  roomsWeek: { roomId: ID; name: string; color: string; hours: number; percent: number }[] | null
  week: { date: string; count: number; revenue: number }[]
  byType: Partial<Record<BookingType, number>>
  topUnpaid: Booking[]
}

export interface ReportRow {
  key: string
  label: string
  bookings: number
  hours: number
  revenue: number
  paid: number
}

export interface Report {
  from: string
  to: string
  byMonth: ReportRow[]
  byRoom: ReportRow[]
  byType: ReportRow[]
  totals: ReportRow
  paymentsByMethod: ReportRow[]
}

export interface Notification {
  id: ID
  channel: string
  subject: string
  body: string
  status: string
  readAt: string | null
  createdAt: string
}

export interface AuditEntry {
  id: number
  actorName: string | null
  entity: string
  entityId: ID | null
  action: string
  data: Record<string, unknown>
  createdAt: string
}

export interface Attachment {
  id: ID
  entity: string
  entityId: ID
  filename: string
  contentType: string
  sizeBytes: number
  createdAt: string
}

export interface RoomAvailability {
  room: Room
  freeSlots: { start: string; end: string }[] | null
  windowFree: boolean | null
  estimatedCost: number | null
}

export interface PublicConfig {
  appName: string
  authMode: 'dev' | 'firebase'
  timezone: string
  version: string
  firebase: { apiKey: string; authDomain: string; projectId: string; appId: string }
}

/** Occupied period in the coordinator availability calendar. */
export interface Block {
  id: string
  roomId: ID
  roomName: string
  buildingId: ID
  startsAt: string
  endsAt: string
  type: BookingType
  status: BookingStatus
  title: string
  coordinatorName: string | null
  mine: boolean
  request: boolean
  seriesId: ID | null
  cancelRequested: boolean
}
