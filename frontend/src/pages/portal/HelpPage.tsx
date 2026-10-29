import { Clock, MapPin } from 'lucide-react'
import { Card, PageHeader } from '@/components/ui'
import { useBuildings } from '@/lib/queries'

const FAQ = [
  ['האם שליחת בקשה שומרת את החלל?', 'לא. בקשה ממתינה אינה שומרת את החלל. רק הזמנה מאושרת על ידי המתנ״ס או חסימת מנהל מונעות הזמנה נוספת.'],
  ['איך אדע שהבקשה אושרה?', 'הסטטוס באזור ״הבקשות שלי״ מתעדכן בזמן אמת, ותקבלו גם התראה במערכת.'],
  ['איך משלמים?', 'התשלום מתואם מול המזכירות: העברה בנקאית, צ׳ק או מזומן. המצב מוצג באזור ״תשלומים״.'],
  ['רוצה לבטל או לשנות מועד?', 'שלחו בקשת ביטול מתוך ההזמנה. עד לאישור המנהל ההזמנה נשארת בתוקף. לשינוי מועד – בקשו מועד חדש ובטלו את הקודם.'],
  ['רואים ביומן פעילות של רכז אחר?', 'כדי לתאם ביניכם מוצגים סוג הפעילות ושם הרכז בלבד – ללא פרטי לקוחות, מחירים או הערות.'],
]

export function HelpPage() {
  const buildings = useBuildings().data ?? []
  return (
    <div className="space-y-6">
      <PageHeader title="עזרה ויצירת קשר" subtitle="שאלות נפוצות ופרטי המתנ״ס" />
      <div className="grid gap-4 md:grid-cols-2">
        {buildings.map((b) => (
          <Card key={b.id} title={b.name}>
            <p className="flex items-center gap-2 text-sm">
              <MapPin className="h-4 w-4 text-muted" /> {b.address || '—'}
            </p>
            <p className="mt-2 flex items-center gap-2 text-sm">
              <Clock className="h-4 w-4 text-muted" /> שעות פעילות: {b.opensAt}–{b.closesAt}
            </p>
            <p className="mt-3 text-xs text-muted">לשאלות על הזמנה קיימת פנו למזכירות המתנ״ס עם מספר הבקשה (#).</p>
          </Card>
        ))}
      </div>
      <Card title="שאלות נפוצות" padded={false}>
        <ul className="divide-y divide-line">
          {FAQ.map(([q, a]) => (
            <li key={q} className="px-5 py-4">
              <p className="font-medium">{q}</p>
              <p className="mt-1 text-sm text-muted">{a}</p>
            </li>
          ))}
        </ul>
      </Card>
    </div>
  )
}
