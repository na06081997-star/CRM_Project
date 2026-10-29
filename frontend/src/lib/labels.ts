import type { BookingStatus, BookingType, PaymentMethod, RequestStatus, Role, TaskStatus } from './types'

export const roleLabel: Record<Role, string> = {
  super_admin: 'מנהל־על',
  admin: 'מנהל/ת מתנ״ס',
  coordinator: 'רכז/ת',
  staff: 'צוות תפעול',
  user: 'משתמש/ת פורטל',
}

export const staffKindLabel: Record<string, string> = {
  cleaner: 'ניקיון',
  technician: 'טכנאי',
  maintenance: 'אחזקה',
  security: 'אבטחה',
}

export const bookingTypeLabel: Record<BookingType, string> = {
  activity: 'פעילות באמצעות רכז',
  rental: 'שכירות בתשלום',
  internal: 'הזמנה פנימית',
  maintenance: 'תחזוקה',
  closed: 'סגור / לא זמין',
}

export const bookingTypeShort: Record<BookingType, string> = {
  activity: 'פעילות',
  rental: 'שכירות',
  internal: 'פנימי',
  maintenance: 'תחזוקה',
  closed: 'סגור',
}

export const bookingStatusLabel: Record<BookingStatus, string> = {
  pending: 'ממתין לאישור',
  confirmed: 'מאושר',
  cancelled: 'בוטל',
  rejected: 'נדחה',
}

export const requestStatusLabel: Record<RequestStatus, string> = {
  pending: 'ממתינה',
  approved: 'אושרה',
  rejected: 'נדחתה',
  cancelled: 'בוטלה',
}

export const paymentMethodLabel: Record<PaymentMethod, string> = {
  bank_transfer: 'העברה בנקאית',
  check: 'צ׳ק',
  cash: 'מזומן',
  credit_card: 'כרטיס אשראי',
}

export const taskStatusLabel: Record<TaskStatus, string> = {
  open: 'פתוחה',
  in_progress: 'בביצוע',
  done: 'בוצע',
  cancelled: 'בוטלה',
}

export const taskKindLabel: Record<string, string> = {
  cleaning: 'ניקיון',
  setup: 'הקמה וסידור',
  repair: 'תיקון',
  inspection: 'בדיקה',
  other: 'אחר',
}

export const roomKindLabel: Record<string, string> = {
  event_hall: 'אולם אירועים',
  meeting_room: 'חדר ישיבות',
  sports_hall: 'אולם ספורט',
  music_room: 'חדר מוזיקה',
  studio: 'סטודיו',
  classroom: 'חדר חוגים',
  other: 'אחר',
}

export const customerKindLabel: Record<string, string> = {
  private: 'פרטי',
  organization: 'ארגון / עמותה',
  internal: 'פנימי',
}

export const weekdayShort = ['א׳', 'ב׳', 'ג׳', 'ד׳', 'ה׳', 'ו׳', 'ש׳']
export const weekdayLong = ['ראשון', 'שני', 'שלישי', 'רביעי', 'חמישי', 'שישי', 'שבת']

/** Tailwind classes per booking type: [soft background, accent border/text]. */
export const typeStyles: Record<BookingType, { bg: string; border: string; text: string; dot: string }> = {
  activity: { bg: 'bg-activity-soft', border: 'border-activity', text: 'text-activity', dot: 'bg-activity' },
  rental: { bg: 'bg-rental-soft', border: 'border-rental', text: 'text-rental', dot: 'bg-rental' },
  internal: { bg: 'bg-internal-soft', border: 'border-internal', text: 'text-internal', dot: 'bg-internal' },
  maintenance: { bg: 'bg-closed-soft hatch', border: 'border-closed', text: 'text-closed', dot: 'bg-closed' },
  closed: { bg: 'bg-closed-soft hatch', border: 'border-closed', text: 'text-closed', dot: 'bg-closed' },
}

export const requestRouteLabel: Record<'rental' | 'activity', string> = {
  rental: 'שכירות בתשלום',
  activity: 'פעילות באמצעות רכז',
}
