# Notifications & reminders

All notifications are sent **by e-mail** from the CRM's mailbox (`MAIL_FROM`, Hostinger SMTP — see [authentication.md](authentication.md) for the SMTP variables):

- login codes, invitations and password resets;
- contract signing links;
- request approved / rejected, cancellations;
- staff reminders before each task (`REMINDER_LEAD_MINUTES`, default 60).

Every attempt is stored in the `notifications` table and shown in the bell menu.

Without `SMTP_HOST` (local development), messages are only written to the backend log.

## Paid channels

The basic pack has no paid channel. SMS (Twilio, 019…), WhatsApp Business or Telegram bots are not part of it. A client who wants one gets a separate quote that covers the provider's price per message and the integration work; the channel is then added back to `internal/notify` for that client.

The "שליחה בוואטסאפ" button next to a signing link only opens WhatsApp on the manager's own phone with the message ready. The CRM sends nothing itself and it costs nothing.
