# Contract signatures with OpenSign

Decision of 25/11/2026 (owner): digital contracts are signed through a
**self-hosted OpenSign**, and the signed PDFs are archived in Garage with the
contract. The owner accepted the risk of using OpenSign's internal API (the
free edition has no public API). The built-in signature stays as a fallback.

## How it works

```
manager ── "send contract" ─► backend ── .docx (Hebrew RTL) ─► OpenSign /docxtopdf (LibreOffice)
                                 │                               └─ PDF + signature box on the last page
                                 └─ e-mail with the CRM link /sign/<token>
signer ── /sign/<token> (reads the text) ── "מעבר לחתימה" ─► OpenSign signing page
OpenSign ── done ─► back to /sign/<token>?done=1
backend (every minute + on that page) ── document completed? ─► download signed PDF + certificate
                                                              ─► Garage (contract documents), account opens
```

- The CRM link, the e-mail, "copy link" and "שליחה חוזרת" are unchanged.
  Re-sending renews the link and the OpenSign expiry (14 days).
- The PDF has the contract text and a last page **אישור וחתימה** (parties,
  signer, e-mail, date, consent sentence) with the signature box.
- On completion two PDFs are attached to the contract: `<title> - חתום.pdf`
  (sealed by the instance's signing certificate) and
  `אישור חתימה דיגיטלית.pdf` (OpenSign's audit certificate: IP, dates).
- The signer declines in OpenSign → the contract is cancelled and the
  managers get an e-mail.
- OpenSign not configured, or unreachable when sending → the contract uses the
  built-in signature pad (`provider: builtin`), so it can always be signed.
- Scanned contracts (`method: scan`) are not affected.

Code: `backend/internal/opensign` (client), `backend/internal/docx` (Word
writer), `backend/internal/service/opensign.go` (flow), migration
`0007_opensign.sql`. Tests run against an in-memory OpenSign
(`opensign/opensigntest`).

## Deployment (per environment)

OpenSign runs inside each environment's compose stack (profile `opensign`),
so every client keeps its documents in its own stack:

| service | role |
| --- | --- |
| `opensign-init` | one-shot: creates the master key and a self-signed signing certificate in the `opensign-secrets` volume (kept across deploys) |
| `opensign-mongo` | OpenSign's database (`opensign-mongo` volume) |
| `opensign-server` | OpenSign API + LibreOffice (`opensign-files` volume) |
| `opensign-client` | OpenSign web app |
| `opensign` | Caddy entry point: `/api/*` → server, rest → web app |

The backend reads the master key from the same volume
(`/run/opensign/master_key`). It creates its own OpenSign account on first use
(e-mail = `OPENSIGN_ACCOUNT_EMAIL`, default the address of `MAIL_FROM`;
password derived from the master key). **No secret is typed or copied.**
A fresh OpenSign shows its "add admin" page to anyone, so the backend claims
the account at start-up and then every minute until it succeeds (log
`opensign: not ready` while OpenSign is still starting).

To enable it on an environment:

1. DNS (Hostinger): `A` record `sign.<env>.<client>` → the VPS IP
   (e.g. `sign.demo.maaleramot`).
2. Dokploy → the environment's compose → **Environment**, add:
   ```env
   COMPOSE_PROFILES=opensign
   OPENSIGN_PUBLIC_URL=https://sign.demo.maaleramot.kehilaplus.com
   ```
3. Dokploy → **Domains → Add**: host = that name, service **opensign**,
   port **80**, HTTPS, Let's Encrypt.
4. **Deploy**. The first start pulls the OpenSign images (~2.2 GB, shared by
   all environments) and creates the secrets. The backend log line
   `server listening … opensign=true` confirms the link.

Resources: about 0.5 GB of RAM per environment at rest, capped by
`mem_limit` (MongoDB 512 MB with a 256 MB cache, server 1 GB while LibreOffice
converts, web app 256 MB, Caddy 64 MB), so OpenSign cannot starve the other
stacks of the VPS (3.8 GB in November 2026).

Backups: add the `opensign-mongo` and `opensign-files` volumes to the VPS
backups. The signed PDFs that matter are also in Garage with the contract.

## Limits and risks

- OpenSign's signing pages are in English/French (no Hebrew translation in
  OpenSign); the contract text itself is Hebrew and is read on the CRM page.
- The images are `opensign/*:main` (OpenSign publishes no version tags). An
  OpenSign update can change the internal API: after redeploying, send a test
  contract on the demo before production. If a send fails, the CRM falls back
  to the built-in signature and logs `opensign: send failed`.
- The signing certificate is self-signed per instance: it proves the PDF was
  not modified after signing, not a legal identity (same level as the
  built-in signature, plus OpenSign's audit certificate).

## Rolling back

Remove `COMPOSE_PROFILES` and `OPENSIGN_PUBLIC_URL` from the environment and
redeploy: new contracts use the built-in signature again. Contracts already
signed keep their archived PDFs. Contracts still pending in OpenSign stay
pending: cancel them and send them again.
