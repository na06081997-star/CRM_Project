-- Password + e-mail code authentication (replaces Google / Firebase sign-in).
-- Existing ids, e-mails, roles and business data are kept as they are.

ALTER TABLE users
    ADD COLUMN username            text,
    ADD COLUMN password_hash       text,
    ADD COLUMN password_changed_at timestamptz,
    ADD COLUMN activated_at        timestamptz,
    ADD COLUMN first_login_at      timestamptz,
    ADD COLUMN last_login_at       timestamptz,
    -- false for accounts created before this migration: their past logins were never recorded
    ADD COLUMN login_history_known boolean NOT NULL DEFAULT true,
    -- bumped to revoke every session of the account (password reset, recovery)
    ADD COLUMN token_version       integer NOT NULL DEFAULT 0;

UPDATE users SET login_history_known = false;
-- Accounts already in use count as activated; they choose a password through
-- an invitation or "forgot password" and then always log in with an e-mail code.
UPDATE users SET activated_at = created_at WHERE is_active;

-- Usernames from the e-mail's local part, made unique with a number.
DO $$
DECLARE
    r    record;
    base text;
    cand text;
    n    int;
BEGIN
    FOR r IN SELECT id, email FROM users ORDER BY created_at, id LOOP
        base := lower(regexp_replace(split_part(coalesce(r.email, ''), '@', 1), '[^a-zA-Z0-9._-]', '', 'g'));
        base := regexp_replace(base, '^[._-]+', '');
        IF length(base) < 3 THEN
            base := 'user' || base;
        END IF;
        base := left(base, 28);
        cand := base;
        n := 1;
        WHILE EXISTS (SELECT 1 FROM users WHERE lower(username) = cand) LOOP
            n := n + 1;
            cand := base || n;
        END LOOP;
        UPDATE users SET username = cand WHERE id = r.id;
    END LOOP;
END $$;

ALTER TABLE users ALTER COLUMN username SET NOT NULL;
CREATE UNIQUE INDEX users_username_lower_key ON users (lower(username));
-- Not unique: existing e-mails differing only by case are reported by
-- `server auth-report` and must be fixed by the operator.
CREATE INDEX users_email_lower_idx ON users (lower(email));

-- Single-use links (invitation, password reset). Only the SHA-256 of the
-- token is stored; issuing a new one revokes the previous ones.
CREATE TABLE auth_tokens (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose        text NOT NULL CHECK (purpose IN ('invite', 'reset')),
    token_hash     bytea NOT NULL UNIQUE,
    expires_at     timestamptz NOT NULL,
    consumed_at    timestamptz,
    revoked_at     timestamptz,
    sent_at        timestamptz,
    delivery_error text,
    created_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_tokens_user_idx ON auth_tokens (user_id, purpose, created_at DESC);

-- A login (or activation) waiting for its e-mail code. The browser holds an
-- opaque handle (only its hash is stored); the code is kept as an HMAC whose
-- key lives outside the database.
CREATE TABLE auth_challenges (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    flow            text NOT NULL CHECK (flow IN ('activation', 'login')),
    handle_hash     bytea NOT NULL UNIQUE,
    code_hmac       bytea,
    code_expires_at timestamptz,
    generation      integer NOT NULL DEFAULT 0,
    attempts        integer NOT NULL DEFAULT 0,
    sends           integer NOT NULL DEFAULT 0,
    last_sent_at    timestamptz,
    expires_at      timestamptz NOT NULL,
    consumed_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_challenges_user_idx ON auth_challenges (user_id, created_at DESC);

-- Sliding-window rate limits (failed logins, wrong codes, e-mails sent),
-- keyed by account or IP. Old rows are purged by the scheduler.
CREATE TABLE auth_events (
    id         bigserial PRIMARY KEY,
    kind       text NOT NULL,
    key        text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_events_lookup_idx ON auth_events (kind, key, created_at);

-- One-time facts about the instance (initial super administrator…).
CREATE TABLE system_state (
    key        text PRIMARY KEY,
    value      text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
