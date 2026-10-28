# Google / Firebase login — setup (≈5 minutes)

Until this is done the app runs with `AUTH_MODE=dev` (demo login screen). Nothing in the code changes — only `.env`.

1. Go to <https://console.firebase.google.com> → **Add project** (e.g. `kehila-crm`). Google Analytics is not needed.
2. **Build → Authentication → Get started → Sign-in method → Google → Enable**. Pick a support e-mail. Save.
3. **Authentication → Settings → Authorized domains**: add your production domain (e.g. `crm.example.co.il`). `localhost` is already there.
4. **Project settings (⚙) → General → Your apps → Web (`</>`)** → register an app (no hosting). Copy the config values.
5. Fill `.env`:

   ```env
   AUTH_MODE=firebase
   FIREBASE_PROJECT_ID=kehila-crm
   FIREBASE_API_KEY=AIza...
   FIREBASE_AUTH_DOMAIN=kehila-crm.firebaseapp.com
   FIREBASE_APP_ID=1:1234567890:web:abc123
   BOOTSTRAP_ADMIN_EMAILS=you@gmail.com
   ```

6. Restart the backend (`docker compose up -d backend`). The login page now shows **התחברות עם Google**.

### How it works

- The browser signs in with Google through Firebase and sends the Firebase **ID token** as `Authorization: Bearer …`.
- The backend verifies it **without the Admin SDK and without a service account**: RS256 signature against Google's published keys, `iss`/`aud` = your project id, expiry, verified e-mail.
- First login creates the user. E-mails in `BOOTSTRAP_ADMIN_EMAILS` become admins; everyone else starts as a portal user. Admins then promote people in **משתמשים והרשאות**.
- A user pre-created by an admin (same e-mail) is linked automatically on first Google login, keeping the assigned role.

The API key in the web config is public by design (it identifies the project); it is not a secret.
