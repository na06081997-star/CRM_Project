# קהילה+ — CRM למתנ״סים

מערכת ניהול מלאה למתנ״ס: יומן הזמנות חכם, אישור בקשות מהפורטל, ניהול חדרים ותמחור, תשלומים ויתרות, משימות לצוות התפעול עם תזכורות, ושילוט דיגיטלי בזמן אמת.

> Hebrew-first (RTL), responsive (desktop / tablet / mobile), real-time over WebSockets.

![calendar mockup](docs/specs/calendar-week-mockup.jpeg)

## Modules

| Area | What it does |
| --- | --- |
| **לוח בקרה** | Today's schedule, live "happening now", pending requests, occupancy, revenue & balances, open tasks |
| **יומן הזמנות** | Google-Calendar-style **Year / Month / Week / Day** views + **by room** and **by coordinator** resource views. Filters: building, rooms, coordinator, booking type, status, "view as". Drag to create, drag to move (across days or rooms), resize, live conflict check, weekly recurrence, print |
| **הזמנות ובקשות** | Portal requests review: availability re-checked on approval, final price, internal notes, approve / reject with reason (requester notified). Full bookings table |
| **חדרים ואולמות** | Rooms per building, capacity, hourly rate, minimum hours, turnover buffer, equipment, rules, live status |
| **תשלומים** | Balances per booking, bank transfer / check / cash / credit card, CSV export, full audit log |
| **פורטל משתמש** | Search free rooms by date/time/capacity, free-slot suggestions, price estimate, submit & track requests live |
| **צוות תפעול** | Mobile app for cleaners/technicians: today's tasks, "התחלתי" / "בוצע" with note, recurring schedules, automatic reminders |
| **שילוט דיגיטלי** | Public lobby screen `/signage/<slug>`: each room's current activity, countdown, next event, upcoming list, rotating announcements with images — pushed live |

Also: customers (with history & balances), coordinators workload, activities (חוגים), reports (by month / room / type / payment method), users & roles, settings, dark mode, Ctrl+K global search.

## Roles

| Role | Home | Can |
| --- | --- | --- |
| **מנהל־על** (`super_admin`) | `/admin` | Everything, incl. users & permissions and system settings (buildings, equipment) |
| **מנהל מתנ״ס** (`admin`) | `/admin` | Approves / rejects every coordinator's and customer's request, global calendar, rooms & pricing, all payments, cancellations, staff tasks, signage |
| **רכז** (`coordinator`) | `/rakaz` | Availability calendar (free / busy — others only show type + coordinator, never customers, prices or notes), requests slots for their activities ("פעילות באמצעות רכז"), sees only their own requests & activities, asks for cancellations |
| **צוות** (`staff`) | `/staff` | Own tasks, "בוצע" with note, reminders |
| **משתמש** (`user`) | `/portal` | Search free rooms, paid rental requests, own requests, cancellation requests |

A pending request never blocks a room; only approved bookings and maintenance blocks do. A cancellation request keeps the room blocked until the manager approves it. The public lobby screen (`/` → `/signage`) has a "כניסה למערכת" button; each user signs in with their own account and lands on their own area.

## Architecture

```
Browser ─► Traefik ─► frontend (nginx: React SPA, proxies /api + /ws) ─► backend (Go)
                                                                          ├─► PostgreSQL 16
                                                                          ├─► Redis 7 (pub/sub for WebSocket fan-out)
                                                                          └─► Garage (S3 object storage)
```

Only Traefik publishes a port; the backend, database, Redis and Garage live on the private Docker network.

| Layer | Stack |
| --- | --- |
| Frontend | React 18, TypeScript, Vite, Tailwind (RTL), TanStack Query, Zustand, date-fns (he) |
| Backend | Go 1.23, chi, pgx, gorilla/websocket, go-redis, minio-go — `cmd/` → `api` (handlers) → `service` (use cases) → `store` (repositories), pure rules in `domain` |
| Data | PostgreSQL with a `btree_gist` **exclusion constraint** that makes overlapping bookings impossible at the database level (buffer included) |
| Auth | Firebase / Google sign-in (ID tokens verified against Google's public keys) or a demo login in dev |
| Notifications | console (dev), Telegram & e-mail (free), Twilio / 019 SMS (paid) — see [docs/notifications.md](docs/notifications.md) |

## Quick start (Docker)

```bash
cp .env.example .env        # then edit secrets
docker compose up -d --build
open http://localhost        # admin: login page → pick "מאיה שגיא"
open http://localhost/signage/lobby
```

In development the database is seeded with a realistic demo matnas (rooms, coordinators, weekly programs, rentals, payments, tasks, signage). Set `APP_ENV=production` to disable the demo login and seeding.

## Local development (without Docker)

```bash
# backend (needs PostgreSQL; Redis/Garage optional)
cd backend
DATABASE_URL=postgres://crm:crm@localhost:5432/crm?sslmode=disable \
JWT_SECRET=dev-secret-dev-secret-dev-secret-dev HTTP_ADDR=:8080 go run ./cmd/server

# frontend (proxies /api and /ws to :8080)
cd frontend && npm install && npm run dev   # http://localhost:5173
```

## Tests

```bash
cd backend  && go test ./...                                    # unit: conflicts, pricing, permissions, signage, auth, notify
cd backend  && TEST_DATABASE_URL=postgres://…/crm_test go test -tags integration ./internal/service/...
cd frontend && npm test && npm run lint && npm run build
```

CI (GitHub Actions) runs all of the above plus a full `docker compose` smoke test on every push and PR.

## Git workflow & versioning

See [CONTRIBUTING.md](CONTRIBUTING.md): `main` is always deployable (Dokploy auto-deploys it), work happens on `feat/*` / `fix/*` branches, commits follow **Conventional Commits**, and **release-please** turns them into semantic versions (`v0.2.0`, `v1.0.0`…) with a generated CHANGELOG.

## Docs

- [Firebase / Google login setup](docs/firebase-setup.md)
- [Deploying on Dokploy](docs/deployment-dokploy.md)
- [Notifications & SMS options](docs/notifications.md)
- Original specs: [docs/specs](docs/specs)
