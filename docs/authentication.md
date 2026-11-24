# Authentication: password + e-mail code

Production and development instances use `AUTH_MODE=password`:

1. The user types their **username or e-mail** and their **password**.
2. The server e-mails a **6-digit code** (valid 10 minutes, 5 tries).
3. The code opens the session (7 days, `AUTH_SESSION_TTL`).

Every new interactive login asks for a new code, super administrators included. Demo instances keep `AUTH_MODE=dev` (user picker behind `DEMO_ACCESS_CODE`); nothing below applies to them.

Accounts are never self-created: the super administrator **invites** them.

## Environment variables

Required with `AUTH_MODE=password` (see `.env.example`):

| Variable | Example | Notes |
|---|---|---|
| `PUBLIC_URL` | `https://maaleramot.kehilaplus.com` | Origin used to build the links in e-mails. Must be `https://` when `APP_ENV=production`. Defaults to the first `CORS_ORIGINS` entry. |
| `JWT_SECRET` | `openssl rand -hex 32` | Signs session tokens. |
| `AUTH_CODE_SECRET` | `openssl rand -hex 32` | HMAC key for the stored codes. It must differ from `JWT_SECRET` and is never stored in the database. |
| `SMTP_HOST` | `smtp.hostinger.com` | |
| `SMTP_PORT` | `465` | |
| `SMTP_TLS` | `ssl` | `ssl` = implicit TLS (465), `starttls` = mandatory STARTTLS (587), `none` = local test servers only (refused in production). |
| `SMTP_USER` | `noreply@kehilaplus.com` | The Hostinger mailbox. |
| `SMTP_PASSWORD` | *(the mailbox password)* | Only in the env file or Dokploy. Never in git, never in chat. |
| `MAIL_FROM` | `קהילה+ <noreply@kehilaplus.com>` | Sender shown to users. `SMTP_FROM` is still read as a fallback. |
| `BOOTSTRAP_SUPER_ADMIN_EMAIL` | `owner@client.org.il` | Initial super administrator of a **new** instance (see below). It may be the same mailbox as the sender. |

Optional tuning (Go durations or integers). The defaults follow the specification:

| Variable | Default |
|---|---|
| `AUTH_INVITE_TTL` | `24h` |
| `AUTH_RESET_TTL` | `1h` |
| `AUTH_CODE_TTL` | `10m` |
| `AUTH_CODE_MAX_ATTEMPTS` | `5` (per code) |
| `AUTH_RESEND_COOLDOWN` | `60s` |
| `AUTH_CHALLENGE_TTL` | `30m` (the whole "enter your code" step) |
| `AUTH_MAX_SENDS_PER_FLOW` | `5` |
| `AUTH_RATE_WINDOW` | `15m` (the sliding window for the per-account and per-IP limits) |
| `AUTH_MAX_LOGIN_FAILS_PER_ACCOUNT` / `_PER_IP` | `10` / `50` |
| `AUTH_MAX_CODE_FAILS_PER_ACCOUNT` / `_PER_IP` | `10` / `50` (these keep counting across resends) |
| `AUTH_MAX_SENDS_PER_ACCOUNT` / `_PER_IP` | `8` / `40` |
| `AUTH_SESSION_TTL` | `168h` |

## First start of a new instance

1. Fill the env file, including `BOOTSTRAP_SUPER_ADMIN_EMAIL`, and deploy.
2. On first start the server migrates the database and creates **one pending super administrator**. Its username is the local part of the e-mail address. The server then e-mails the invitation from `MAIL_FROM`. The log says `bootstrap: invitation e-mailed…`.
3. The owner opens the link, chooses and confirms a password, then types the 6-digit code from the second e-mail. That opens the first session.

The bootstrap runs **once per instance**. The instance records it in `system_state`. Later restarts:
- never create a second account;
- never touch the password;
- never re-enable a disabled account;
- never resend the invitation.

Changing `BOOTSTRAP_SUPER_ADMIN_EMAIL` later does nothing; privileges never follow the variable. If the database already contains a super administrator (an existing instance), the bootstrap does nothing either.

### If the first invitation was not delivered

When SMTP refuses the first invitation, the server logs `bootstrap: invitation e-mail failed …`. Fix the SMTP variables, then run, before anyone has logged in:

```sh
docker exec -it <backend-container> /app/server bootstrap-resend
```

The command e-mails a new link and revokes the previous ones. It only works while the initial account is still pending.

To test the SMTP settings on their own:

```sh
docker exec -it <backend-container> /app/server mail-test -to you@example.org
```

## Inviting users

Go to **ניהול → משתמשים → הזמנת משתמש**. Fill in the username, e-mail, full name and role. Only roles you may grant are listed; only a super administrator can create another super administrator.

The server checks permissions and case-insensitive uniqueness of the username and the e-mail. It then creates a **pending** account and e-mails a personal link, valid 24 hours. The user:
1. chooses a password;
2. receives a code at **their own** address;
3. lands in the CRM with their role.

If they close the page after choosing the password, they simply log in with username + password + code: the first verified code activates the account. The invitation link itself stays valid until a password is submitted, so a mail scanner opening it does no harm.

Coordinators and customers created from the contracts flow get their invitation automatically, in two cases:
- once their account opens, that is, after the contract is signed;
- right away when there is no contract.

### Tracking invitations

The users page shows for every account:
- username, e-mail and role;
- **account state**: pending, active or disabled;
- **invitation state**: sent, delivery failed (with the SMTP answer), expired, used, or replaced;
- **first and last successful login**.

A sent invitation is not a login: the login dates only move when a code is verified. Accounts that existed before this system show *היסטוריה לא זמינה* ("history unavailable") until their first new login.

**שליחת הזמנה מחדש** ("resend invitation") works for any pending account, even after expiry. It always issues a new link and revokes the previous ones.

## Forgot password

**שכחתי סיסמה** ("forgot password") on the login page always answers the same message, whether the account exists or not. The e-mail leaves in the background, so the response time does not tell either.

If the account exists, its owner receives a personal, single-use link, valid 1 hour. After a new password is chosen:
- every existing session is revoked (the session tokens carry a version that is bumped);
- every pending link and code is cancelled;
- the next login requires a new e-mail code.

## Security notes

- **Invitation and reset links.** Each one carries 32 random bytes. The database stores only their SHA-256, with the account, purpose and expiry. The token sits in the URL **fragment** (`/activate#…`), so it never reaches Traefik, nginx or backend logs. The page sends it in a POST body.
- **Consuming a link.** Opening the page does not consume it; the valid submission does. A row lock plus a conditional update guarantees single use, even with two simultaneous submissions. A test covers this.
- **Login codes.** The server draws a uniform random number with exactly 6 digits, leading zeros included. It stores the code as HMAC-SHA256 with `AUTH_CODE_SECRET`, bound to the account, the flow (activation or login), the attempt and its generation. A resend replaces the code, so the previous one stops working.
- **Code attempts.** Each try is reserved atomically before the comparison, so concurrent guesses can never exceed 5 per code. The account-wide and per-IP limits keep counting after resends.
- **After the password step.** Once the password is accepted, the browser only holds an opaque challenge handle. It can verify or resend a code and nothing else; using it as a bearer token gets `401` everywhere.
- **Passwords.** The policy is enforced server-side:
  - at least 10 characters (72 bytes at most, a bcrypt limit), with letters and digits;
  - not a common password;
  - must not contain the username or the e-mail.

  Passwords are hashed with bcrypt, cost 12. A failed lookup still runs a bcrypt comparison, so timing does not reveal accounts.
- **Client IP.** The rate limits use only `X-Real-Ip`, which Traefik overwrites. Forgeable forwarding headers are ignored.
- **Logs.** Passwords, codes, tokens, full activation links and SMTP secrets never appear in logs. A test checks the log output.
- **Session-opening routes.** Only `/api/auth/verify` opens a session, plus `/api/auth/dev-login` on demo instances; that route does not exist in password mode. Firebase sign-in was removed.

## Migrating an existing instance (Google sign-in → password)

The migration (`0006_password_auth`) runs automatically at start-up.

It keeps the following for every account:
- ids;
- e-mails;
- roles;
- `firebase_uid`;
- business data.

It changes these:
- it gives every account a **username**, derived from the e-mail and made unique with a number (`dana`, `dana2`, …);
- it marks active accounts as activated;
- it marks their login history as unknown.

Former Google accounts have **no password**. Their owners set one through an invitation from the users page, or through *שכחתי סיסמה*. From then on they log in with a code like everyone else.

**Before deploying**, list the accounts that need attention:

```sh
docker exec -it <backend-container> /app/server auth-report
```

The report lists, each with how to fix it:
- accounts without a deliverable e-mail (`.local`, malformed…);
- e-mails shared by several accounts, differing only by case;
- active accounts without a password;
- pending invitations.

On the users page:
- give each account its own valid address;
- disable duplicates;
- then send the invitations.

While an e-mail is ambiguous, login with that e-mail is refused, but the username still works.

## Recovery: the only super administrator lost their mailbox

This is an operator procedure on the server, never exposed as a public route:

```sh
docker exec -it <backend-container> /app/server recover-super-admin \
  -user <username or current e-mail> -email <new mailbox> -yes
```

The command:
1. replaces the account's e-mail;
2. clears its password;
3. revokes every session, link and code;
4. records an audit entry;
5. e-mails a new invitation to the new address.

The account keeps its id and role. Only use it after confirming the person's identity by another channel.

## Local testing

The integration tests use an in-process SMTP server (`internal/mailer/mailtest`) that captures messages. They follow the real links and type the real codes from the captured e-mails:

```sh
TEST_DATABASE_URL='postgres://…/crm_test?sslmode=disable' go test -tags integration -p 1 ./...
```

To click through the screens locally, run Mailpit (`docker run -p 1025:1025 -p 8025:8025 axllent/mailpit`). Then start the backend with:
- `AUTH_MODE=password`, `SMTP_HOST=localhost`, `SMTP_PORT=1025`, `SMTP_TLS=none`;
- `PUBLIC_URL=http://localhost:5173`;
- `SEED_DEMO=false`, to test the bootstrap on an empty database.

Read the e-mails at http://localhost:8025.
