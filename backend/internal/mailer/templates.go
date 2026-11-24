package mailer

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// Brand is what every e-mail shows: the CRM product and the client's name.
type Brand struct {
	AppName string // קהילה+
	OrgName string // מינהל קהילתי מעלה רמות
}

func (b Brand) title() string {
	if b.OrgName == "" {
		return b.AppName
	}
	return b.OrgName + " · " + b.AppName
}

// Invitation asks a new user to choose a password.
func Invitation(b Brand, to, fullName, username, link string, valid time.Duration) Mail {
	subject := "הזמנה למערכת " + b.AppName + " – " + orDefault(b.OrgName, b.AppName)
	lines := []string{
		"שלום " + fullName + ",",
		"נפתח עבורך חשבון במערכת " + b.AppName + " של " + orDefault(b.OrgName, "הארגון") + ".",
		"שם המשתמש שלך: " + username,
		"כדי להפעיל את החשבון, בחרו סיסמה בקישור הבא. לאחר מכן יישלח אליכם קוד אימות בן 6 ספרות לכתובת זו.",
	}
	return build(b, to, subject, lines, "בחירת סיסמה והפעלת החשבון", link,
		"הקישור אישי ובתוקף "+Duration(valid)+". אם לא ציפיתם להזמנה זו, אפשר להתעלם מהאימייל.")
}

// Code carries the 6-digit verification code.
func Code(b Brand, to, fullName, code string, valid time.Duration) Mail {
	subject := "קוד האימות שלך: " + code + " – " + b.AppName
	text := strings.Join([]string{
		"שלום " + fullName + ",",
		"",
		"קוד האימות לכניסה ל" + b.AppName + ":",
		"",
		"    " + code,
		"",
		"הקוד בתוקף " + Duration(valid) + " ולשימוש חד־פעמי. אל תמסרו אותו לאף אחד – צוות המערכת לעולם לא יבקש אותו.",
		"אם לא ניסיתם להתחבר, מומלץ להחליף סיסמה.",
		"",
		"— " + b.title(),
	}, "\n")
	body := p("שלום "+fullName+",") +
		p("קוד האימות לכניסה ל"+b.AppName+":") +
		`<p style="margin:20px 0;text-align:center"><span dir="ltr" style="display:inline-block;font-size:34px;font-weight:700;letter-spacing:10px;padding:12px 18px;background:#f1f5f9;border-radius:12px;color:#0f172a;font-family:Consolas,Menlo,monospace">` +
		html.EscapeString(code) + `</span></p>` +
		small("הקוד בתוקף "+Duration(valid)+" ולשימוש חד־פעמי. אל תמסרו אותו לאף אחד – צוות המערכת לעולם לא יבקש אותו. אם לא ניסיתם להתחבר, מומלץ להחליף סיסמה.")
	return Mail{To: to, Subject: subject, Text: text, HTML: layout(b, body)}
}

// PasswordReset links to the "choose a new password" page.
func PasswordReset(b Brand, to, fullName, link string, valid time.Duration) Mail {
	lines := []string{
		"שלום " + fullName + ",",
		"התקבלה בקשה לאיפוס הסיסמה שלך במערכת " + b.AppName + ".",
		"לבחירת סיסמה חדשה לחצו על הקישור. לאחר השינוי כל החיבורים הקיימים ינותקו, והכניסה הבאה תדרוש קוד אימות שיישלח לאימייל.",
	}
	return build(b, to, "איפוס סיסמה – "+b.AppName, lines, "בחירת סיסמה חדשה", link,
		"הקישור אישי, חד־פעמי ובתוקף "+Duration(valid)+". אם לא ביקשתם איפוס – התעלמו מהאימייל, הסיסמה לא תשתנה.")
}

// Notice wraps a plain notification (reminder, contract link…) in the same layout.
func Notice(b Brand, to, subject, body string) Mail {
	var parts []string
	for _, para := range strings.Split(body, "\n") {
		parts = append(parts, p(para))
	}
	return Mail{To: to, Subject: subject, Text: body + "\n\n— " + b.title(), HTML: layout(b, strings.Join(parts, ""))}
}

func build(b Brand, to, subject string, lines []string, action, link, note string) Mail {
	text := strings.Join(lines, "\n\n") + "\n\n" + action + ":\n" + link + "\n\n" + note + "\n\n— " + b.title()
	var body strings.Builder
	for _, l := range lines {
		body.WriteString(p(l))
	}
	body.WriteString(`<p style="margin:24px 0;text-align:center"><a href="` + html.EscapeString(link) +
		`" style="display:inline-block;background:#0f766e;color:#ffffff;text-decoration:none;font-weight:700;font-size:16px;padding:14px 26px;border-radius:12px">` +
		html.EscapeString(action) + `</a></p>`)
	body.WriteString(small("אם הכפתור לא נפתח, העתיקו את הקישור לדפדפן:") +
		`<p dir="ltr" style="margin:4px 0 16px;font-size:12px;word-break:break-all;color:#475569">` + html.EscapeString(link) + `</p>`)
	body.WriteString(small(note))
	return Mail{To: to, Subject: subject, Text: text, HTML: layout(b, body.String())}
}

func layout(b Brand, inner string) string {
	return `<!doctype html><html lang="he" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>` +
		`<body style="margin:0;padding:0;background:#f1f5f9">` +
		`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f1f5f9"><tr><td align="center" style="padding:16px">` +
		`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px;background:#ffffff;border-radius:16px;overflow:hidden">` +
		`<tr><td dir="rtl" style="background:#0f766e;padding:16px 20px;color:#ffffff;font-family:Arial,Helvetica,sans-serif;text-align:right">` +
		`<div style="font-size:20px;font-weight:700">` + html.EscapeString(b.AppName) + `</div>` +
		`<div style="font-size:13px;opacity:.85">` + html.EscapeString(b.OrgName) + `</div></td></tr>` +
		`<tr><td dir="rtl" style="padding:20px;font-family:Arial,Helvetica,sans-serif;font-size:15px;line-height:1.6;color:#1e293b;text-align:right">` +
		inner + `</td></tr></table></td></tr></table></body></html>`
}

func p(s string) string {
	if s == "" {
		return ""
	}
	return `<p style="margin:0 0 12px">` + html.EscapeString(s) + `</p>`
}

func small(s string) string {
	return `<p style="margin:0 0 8px;font-size:13px;color:#64748b">` + html.EscapeString(s) + `</p>`
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// Duration writes a validity period in Hebrew ("24 שעות", "10 דקות").
func Duration(d time.Duration) string {
	switch {
	case d >= 48*time.Hour && d%(24*time.Hour) == 0:
		return fmt.Sprintf("%d ימים", int(d/(24*time.Hour)))
	case d == 24*time.Hour:
		return "24 שעות"
	case d == time.Hour:
		return "שעה אחת"
	case d >= 2*time.Hour && d%time.Hour == 0:
		return fmt.Sprintf("%d שעות", int(d/time.Hour))
	case d == time.Minute:
		return "דקה אחת"
	default:
		return fmt.Sprintf("%d דקות", int(d.Round(time.Minute)/time.Minute))
	}
}
