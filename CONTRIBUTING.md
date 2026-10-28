# Contributing

## Branches

| Branch | Purpose |
| --- | --- |
| `main` | Production. Always green, auto-deployed by Dokploy. Protected: PR + passing CI required. |
| `feat/<topic>` | New features, branched from `main`. |
| `fix/<topic>` | Bug fixes. |
| `chore/<topic>`, `docs/<topic>` | Tooling and documentation. |

Open a PR into `main`; squash-merge with a Conventional Commit title.

## Commit messages — Conventional Commits

```
feat(calendar): drag bookings between rooms
fix(payments): reject check payments without a number
docs: explain Dokploy domains
chore(deps): bump pgx
feat!: rename booking statuses        ← "!" = breaking change
```

## Releases — semantic versioning

`release-please` watches `main`:

- `fix:` → patch (`0.1.0 → 0.1.1`)
- `feat:` → minor (`0.1.1 → 0.2.0`)
- `feat!:` / `BREAKING CHANGE:` → major (pre-1.0 it bumps minor)

It keeps a "release PR" open with the next version and CHANGELOG. Merging it creates the `vX.Y.Z` tag and GitHub release.

## Checklist before a PR

```bash
cd backend && gofmt -l . && go vet ./... && go test ./...
cd frontend && npm run lint && npm test && npm run build
```

Never commit `.env` or any credential — configuration comes only from environment variables (see `.env.example`).
