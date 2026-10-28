# Notifications & reminders

Staff receive reminders before each task (`REMINDER_LEAD_MINUTES`, default 60); portal users are told when their request is approved or rejected. Every attempt is stored in the `notifications` table and shown in the bell menu.

Enable channels with `NOTIFY_CHANNELS` (comma-separated). A message is sent on every enabled channel for which the person has an address.

| Channel | Cost | Setup |
| --- | --- | --- |
| `console` | free | Default. Writes to the backend log — good for development. |
| `telegram` | **free** | Create a bot with [@BotFather](https://t.me/BotFather) → `TELEGRAM_BOT_TOKEN`. Each worker opens the bot and sends `/start`; get their chat id (e.g. via `https://api.telegram.org/bot<TOKEN>/getUpdates`) and paste it in **משתמשים והרשאות → Telegram chat ID**. |
| `email` | **free** (Gmail: 500/day) | Gmail → Security → 2-Step → **App passwords**. `SMTP_HOST=smtp.gmail.com`, `SMTP_PORT=587`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM`. |
| `twilio` | paid (~$0.07 / SMS to Israel) | Trial credit available. `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_FROM`. |
| `sms019` | paid (Israeli, cheaper) | 019 SMS account → `SMS019_USERNAME`, `SMS019_TOKEN`, `SMS019_SOURCE` (approved sender name). Verify the payload against your 019 API version before production. |

**Real SMS is never free in Israel.** The free route that works best for staff is Telegram (instant push on the phone) plus e-mail. Switch to SMS later by adding `twilio` or `sms019` to `NOTIFY_CHANNELS`; phone numbers like `050-1234567` are normalised automatically.

Example: `NOTIFY_CHANNELS=console,telegram,email`
