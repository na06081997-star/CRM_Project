# Deploying on Dokploy

Dokploy runs its own Traefik, so the bundled one is disabled there.

1. **Dokploy → Projects → Create → Compose**
   - Provider: GitHub, repository `na06081997-star/CRM_Project`, branch `main`
   - Compose path: `./docker-compose.yml`
   - Enable **Auto Deploy** (deploys on every push to `main`).
2. **Environment**: paste the contents of `.env.example` and change at least:
   ```env
   APP_ENV=production
   COMPOSE_PROFILES=               # empty → bundled Traefik not started
   DOMAIN=crm.example.co.il
   CORS_ORIGINS=https://crm.example.co.il
   POSTGRES_PASSWORD=<openssl rand -hex 24>
   REDIS_PASSWORD=<openssl rand -hex 24>
   JWT_SECRET=<openssl rand -hex 32>
   GARAGE_RPC_SECRET=<openssl rand -hex 32>
   GARAGE_ADMIN_TOKEN=<openssl rand -base64 32>
   S3_ACCESS_KEY=GK<24 hex chars>  # e.g. "GK" + openssl rand -hex 12
   S3_SECRET_KEY=<openssl rand -hex 32>
   AUTH_MODE=firebase              # see firebase-setup.md
   ```
   `APP_ENV=production` refuses `AUTH_MODE=dev` and skips demo data. To get demo data once in production, add `SEED_DEMO=true` on an empty database.
3. **Domains** → add `crm.example.co.il` → service **frontend**, port **80**, HTTPS on (Let's Encrypt).
4. **Deploy**. First boot runs migrations automatically.

Signage screens: open `https://crm.example.co.il/signage/lobby` on the lobby TV (kiosk / full screen). No login needed; it updates live.

## Backups

- PostgreSQL volume `postgres-data` — schedule `pg_dump` in Dokploy (**Backups**) or a cron.
- Garage volumes `garage-meta` + `garage-data` hold uploaded files.

## Without Dokploy (any VPS)

```bash
git clone https://github.com/na06081997-star/CRM_Project.git && cd CRM_Project
cp .env.example .env && nano .env      # keep COMPOSE_PROFILES=standalone
docker compose up -d --build
```

For HTTPS on a plain VPS, put Caddy/Cloudflare in front or extend the Traefik service with a Let's Encrypt resolver.
