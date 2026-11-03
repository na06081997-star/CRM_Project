# Deploying on a VPS with Dokploy

## Architecture

```
Internet ─► VPS (Ubuntu, ufw + fail2ban, key-only SSH)
             └─ Docker
                 ├─ Dokploy (panel) + its Traefik ── HTTPS (Let's Encrypt) for every app
                 ├─ Project "maaleramot"        (one project per client)
                 │     ├─ production  ◄── maaleramot.kehilaplus.com        (branch main)
                 │     ├─ development ◄── dev.maaleramot.kehilaplus.com    (branch develop)
                 │     └─ demo        ◄── demo.maaleramot.kehilaplus.com   (branch demo/maaleramot)
                 │        each: frontend ◄── domain   backend · postgres · redis · garage (private network)
                 ├─ Project "<next client>"     → same repo, its own env files, domains, databases, files
                 └─ Project "<any other app>"
```

- One Dokploy **project per client** (and per other app). Each compose stack gets its own private network and volumes, so data never mixes. Only Dokploy's Traefik publishes ports 80/443.
- `docker-compose.yml` is the core stack and publishes nothing. Locally, `docker-compose.standalone.yml` adds a bundled Traefik (enabled through `COMPOSE_FILE` in `.env`); Dokploy does not use it.
- **Deploys are manual**: nothing redeploys on a git push (no GitHub webhook, Auto Deploy off). Deploy an environment from Dokploy (the compose service → **Deploy**) once its branch holds what should go live.

## Clients, environments and domains

Domain: `kehilaplus.com` (DNS at Hostinger). Each client gets a short slug (`maaleramot`, later e.g. `talpiotjerusalem`) and the same three environments:

| Dokploy environment | Branch | URL | Data / login |
| --- | --- | --- | --- |
| `production` | `main` | `<client>.kehilaplus.com` | real data, Google login (`AUTH_MODE=firebase`) |
| `development` | `develop` | `dev.<client>.kehilaplus.com` | empty, Google login, same setup as production (the last check before production) |
| `demo` | `demo/<client>` | `demo.<client>.kehilaplus.com` | demo data, demo login behind `DEMO_ACCESS_CODE` |

The Dokploy panel is `panel.kehilaplus.com`.

Flow: work lands on `develop` → deploy **development** and check it → merge `develop` into `main` → deploy **production**. A `demo/<client>` branch is a frozen version shown to that client; update it only on purpose.

The code is the same for every client; what differs lives in each environment's variables: `DOMAIN`, `CORS_ORIGINS`, `ORG_*` (name, logo, color), secrets and databases.

### Adding a client

1. **DNS** (Hostinger → Domains → kehilaplus.com → DNS): three `A` records to the VPS IP: `<client>`, `dev.<client>`, `demo.<client>`.
2. **Logo**: add `frontend/public/brand/clients/<client>.png` and `<client>-mark.png` (or upload them later in הגדרות מערכת).
3. **Dokploy**: create the project `<client>` with the environments `production`, `development` and `demo`, one compose service in each (step 3 below), with fresh secrets for each.
4. **Firebase** → Authentication → Settings → Authorized domains: add `<client>.kehilaplus.com` and `dev.<client>.kehilaplus.com`.
5. Optional demo: `git branch demo/<client> develop && git push -u origin demo/<client>`.

## 1. Prepare the VPS (once)

`deploy/vps/bootstrap.sh` has three idempotent steps. Run them as root the first time:

```bash
curl -fsSLO https://raw.githubusercontent.com/na06081997-star/CRM_Project/main/deploy/vps/bootstrap.sh
SSH_PUBKEY="ssh-ed25519 AAAA… you@pc" bash bootstrap.sh setup   # updates, deploy user, swap, ufw, fail2ban, auto-updates
# test `ssh deploy@<ip>` + `sudo -v` from a NEW terminal, then:
sudo bash bootstrap.sh lock-ssh                                  # key-only SSH, root login off
sudo bash bootstrap.sh dokploy                                   # Docker (+ log rotation) and Dokploy
```

Then, right away, open `http://<ip>:3000` and create the Dokploy admin account (the first visitor becomes admin).

Docker-published ports bypass `ufw`, so once the panel has a domain, close **3000** in the provider's network firewall (Hostinger hPanel → VPS → Firewall: allow only 22, 80, 443).

## 2. Dokploy settings

- **Settings → Web Server → Server domain**: `panel.kehilaplus.com`, Let's Encrypt on, your e-mail.
- **Settings → Git → GitHub**: optional (the repo is public, a plain Git URL is enough).
- Enable 2FA on the admin profile.

## 3. Deploy a client

1. **Projects → Create project** `crm-maale-ramot` → **Create service → Compose**.
   - Provider Git, repo `https://github.com/na06081997-star/CRM_Project.git`, branch `main` (`develop` / `demo/<client>` for the other environments), compose path `./docker-compose.yml`.
   - **Auto Deploy** off.
2. **Environment**: paste the client's production env (generate secrets with `openssl rand -hex 32`; keep the file outside git). Required:
   ```env
   APP_ENV=production
   DOMAIN=crm.example.co.il
   CORS_ORIGINS=https://crm.example.co.il
   AUTH_MODE=firebase   # + FIREBASE_* and BOOTSTRAP_ADMIN_EMAILS, see firebase-setup.md
   POSTGRES_PASSWORD / REDIS_PASSWORD / JWT_SECRET / GARAGE_* / S3_*   (random)
   ORG_NAME / ORG_SHORT_NAME / ORG_LOGO_URL / ORG_MARK_URL              (client branding)
   ```
   Do **not** set `COMPOSE_FILE`: Dokploy must run `docker-compose.yml` alone.
3. **Domains → Add**: host = `DOMAIN`, service **frontend**, port **80**, HTTPS on, certificate Let's Encrypt.
4. **Deploy**. The first boot runs the migrations and fills the client branding from `ORG_*`.
5. Add the domain to Firebase → Authentication → Settings → **Authorized domains** (production and development).

Development uses the production settings too (`APP_ENV=production`, `AUTH_MODE=firebase`, no demo data). The demo uses `APP_ENV=development`, `AUTH_MODE=dev` and `DEMO_ACCESS_CODE`.

`APP_ENV=production` refuses `AUTH_MODE=dev` and skips demo data (`SEED_DEMO=true` on an empty database forces it, but demo accounts cannot sign in with Google).

Signage: open `https://<domain>/signage/lobby` full screen on the lobby TV. No login needed; it updates live.

## Backups

- PostgreSQL: Dokploy → the compose service → **Backups** (S3 destination), or a nightly cron on the host:
  `docker exec <postgres-container> pg_dump -U crm crm | gzip > /var/backups/crm-$(date +%F).sql.gz`
- Uploaded files: volumes `garage-meta` and `garage-data`.
- Also enable the provider's weekly VPS snapshot.

## Without Dokploy (plain Docker)

```bash
git clone https://github.com/na06081997-star/CRM_Project.git && cd CRM_Project
cp .env.example .env && nano .env      # keeps COMPOSE_FILE=…standalone.yml (bundled Traefik, HTTP)
docker compose up -d --build
```

For HTTPS without Dokploy, put Caddy or Cloudflare in front.
