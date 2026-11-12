-- 0005: operational tickets reported by coordinators (with photos), weekly
-- recurring requests, the booking requirements checklist and coordinator
-- contracts signed online.
ALTER TYPE task_kind ADD VALUE IF NOT EXISTS 'supplies';

-- A ticket is a task opened by someone outside the operations team (a רכז
-- reporting a broken item, missing toner, a cleaning need…).
ALTER TABLE tasks ADD COLUMN reported_by uuid REFERENCES users (id) ON DELETE SET NULL;
CREATE INDEX tasks_reported_by_idx ON tasks (reported_by, created_at DESC) WHERE reported_by IS NOT NULL;
CREATE INDEX tasks_booking_idx ON tasks (booking_id) WHERE booking_id IS NOT NULL;

-- Weekly repetition of a request (a חוג), until this date inclusive.
ALTER TABLE booking_requests ADD COLUMN repeat_until date;

-- Checklist items (chairs and tables are counts on the booking itself).
INSERT INTO equipment_items (code, label, flat_price) VALUES
    ('sound', 'מערכת הגברה', 0),
    ('lighting', 'תאורה', 0),
    ('catering', 'כיבוד', 0),
    ('partitions', 'מחיצות', 0)
ON CONFLICT (code) DO NOTHING;

CREATE TABLE contracts (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       text NOT NULL,
    body        text NOT NULL,
    -- digital: signed on the public link; scan: a signed paper copy is uploaded
    method      text NOT NULL DEFAULT 'digital' CHECK (method IN ('digital', 'scan')),
    status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'signed', 'cancelled')),
    token       text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    sent_at     timestamptz,
    signed_name text NOT NULL DEFAULT '',
    signed_at   timestamptz,
    signed_ip   text NOT NULL DEFAULT '',
    created_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX contracts_user_idx ON contracts (user_id, created_at DESC);
