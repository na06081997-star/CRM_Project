# Test-driven development

Every change starts with a test that fails for the right reason:

1. **Red.** Write the smallest test that shows the missing behaviour or the bug. Run it and watch it fail.
2. **Green.** Write the simplest code that makes it pass.
3. **Refactor.** Clean up while the suite stays green.
4. Commit the test **together with** the code. A bug fix without a test that reproduces the bug is not done.

Example in the history: the rooms page spun forever on an instance without buildings. `RoomsPage.test.tsx` was written first, failed on "עדיין לא הוגדר מבנה", then the page was fixed.

## Commands

```bash
scripts/test.sh            # everything, like CI (starts a test PostgreSQL in Docker if needed)
scripts/test.sh backend    # Go: gofmt, vet, unit + integration tests
scripts/test.sh frontend   # React: type check + Vitest
scripts/test.sh watch      # Vitest watch mode: rerun on every save
```

A single Go test:

```bash
cd backend
TEST_DATABASE_URL='postgres://postgres:pg@localhost:55433/crm_test?sslmode=disable' \
  go test -tags integration -run TestInvitationFirstLoginAndNextLogins ./internal/service/
```

A single React test file: `cd frontend && npx vitest run src/pages/admin/RoomsPage.test.tsx`.

## Where tests go

| What | Where | Tooling |
|---|---|---|
| Pure rules (conflicts, pricing, recurrence…) | `backend/internal/domain/*_test.go` | plain Go tests, no database |
| Use cases, permissions, SQL | `backend/internal/service/*_integration_test.go` (`//go:build integration`) | `setup(t)` gives a migrated, seeded database. `setupAuth(t, …)` adds password mode and a capturing SMTP server. |
| HTTP contract (routes, status codes, what is public) | `backend/internal/api/*_integration_test.go` | `httptest` server over the real service |
| Migrations on existing data | `backend/internal/db/*_integration_test.go` | `db.MigrateTo` up to a version, then insert legacy rows, then migrate the rest |
| E-mails | any of the above | `mailtest.Start(t)`: an SMTP server that captures messages; follow the real links and codes |
| Screens | `frontend/src/**/X.test.tsx` next to the page | `renderPage(<Page />, { role })` and `mockApi({ '/rooms': [...] })` from `src/test/render.tsx` |
| Helpers (dates, formatting…) | `frontend/src/**/*.test.ts` | Vitest |

Rules of thumb:
- Test behaviour the user or the API sees: texts, statuses, HTTP codes, e-mails sent. Not private functions.
- Every role has its own test when permissions differ ("a coordinator cannot …").
- Every screen that lists data is also tested **empty**. Production starts with an empty database.
- No real network in tests. Use `mockApi` in React. In Go, `mailtest` stands in for SMTP and `storage.NewS3("", …)` turns file storage off.

## CI

GitHub Actions runs the same commands on every push to `develop` / `main` and on pull requests: gofmt, vet, unit tests, integration tests on PostgreSQL 16, type check, Vitest, build, and a Docker smoke test. A red CI blocks the merge.
