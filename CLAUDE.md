# CRM קהילה+ — working rules

- **TDD, always** (docs/tdd.md). Write a failing test before the code for every feature and bug fix, then make it pass, then refactor. Commit the test with the fix. `scripts/test.sh` must be green before a push.
- **Every screen is tested empty.** Production and dev start without demo data.
- **Mobile first.** Every screen must work at 390px without horizontal scroll. Managers and coordinators mostly use a phone.
- Hebrew RTL UI. Code, comments and docs in English. Talk to the owner in French.
- Branches: work on `develop`, keep `demo/<client>` equal to `develop`. `main` (production) only after the owner validates. Deploys are manual in Dokploy.
- Never commit `.env*` files or secrets. Secrets live in `../envs/<client>/*.env` and in Dokploy.
- Notifications are e-mail only in the basic pack (docs/notifications.md). Paid channels are quoted per client.
- Auth: password + e-mail code (docs/authentication.md). The demo instance uses the user picker (`AUTH_MODE=dev`).
