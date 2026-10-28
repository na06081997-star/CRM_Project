-- 0001_init: core schema for the CRM
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TYPE user_role      AS ENUM ('admin', 'coordinator', 'staff', 'user');
CREATE TYPE staff_kind     AS ENUM ('cleaner', 'technician', 'maintenance', 'security');
CREATE TYPE room_kind      AS ENUM ('event_hall', 'meeting_room', 'sports_hall', 'music_room', 'studio', 'classroom', 'other');
CREATE TYPE booking_type   AS ENUM ('activity', 'rental', 'internal', 'maintenance', 'closed');
CREATE TYPE booking_status AS ENUM ('pending', 'confirmed', 'cancelled', 'rejected');
CREATE TYPE request_status AS ENUM ('pending', 'approved', 'rejected', 'cancelled');
CREATE TYPE customer_kind  AS ENUM ('private', 'organization', 'internal');
CREATE TYPE payment_method AS ENUM ('bank_transfer', 'check', 'cash', 'credit_card');
CREATE TYPE task_kind      AS ENUM ('cleaning', 'setup', 'repair', 'inspection', 'other');
CREATE TYPE task_status    AS ENUM ('open', 'in_progress', 'done', 'cancelled');

-- ── People ───────────────────────────────────────────────────
CREATE TABLE users (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    firebase_uid     text UNIQUE,
    email            text NOT NULL UNIQUE,
    full_name        text NOT NULL,
    phone            text,
    role             user_role NOT NULL DEFAULT 'user',
    staff_kind       staff_kind,
    telegram_chat_id text,
    avatar_color     text NOT NULL DEFAULT '#0f766e',
    is_active        boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX users_role_idx ON users (role) WHERE is_active;

CREATE TABLE customers (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name         text NOT NULL,
    kind         customer_kind NOT NULL DEFAULT 'private',
    contact_name text,
    phone        text,
    email        text,
    tax_id       text,
    notes        text NOT NULL DEFAULT '',
    user_id      uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX customers_name_idx ON customers (lower(name));
CREATE UNIQUE INDEX customers_user_idx ON customers (user_id) WHERE user_id IS NOT NULL;

-- ── Facilities ───────────────────────────────────────────────
CREATE TABLE buildings (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    address    text NOT NULL DEFAULT '',
    opens_at   time NOT NULL DEFAULT '07:00',
    closes_at  time NOT NULL DEFAULT '23:00',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rooms (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    building_id    uuid NOT NULL REFERENCES buildings (id) ON DELETE RESTRICT,
    name           text NOT NULL,
    kind           room_kind NOT NULL DEFAULT 'other',
    capacity       int NOT NULL DEFAULT 0 CHECK (capacity >= 0),
    color          text NOT NULL DEFAULT '#0f766e',
    description    text NOT NULL DEFAULT '',
    rules          text NOT NULL DEFAULT '',
    equipment      text[] NOT NULL DEFAULT '{}',
    hourly_rate    numeric(10, 2) NOT NULL DEFAULT 0 CHECK (hourly_rate >= 0),
    min_hours      numeric(4, 2) NOT NULL DEFAULT 1 CHECK (min_hours >= 0),
    buffer_minutes int NOT NULL DEFAULT 15 CHECK (buffer_minutes BETWEEN 0 AND 240),
    is_bookable    boolean NOT NULL DEFAULT true,
    is_active      boolean NOT NULL DEFAULT true,
    image_key      text,
    sort_order     int NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (building_id, name)
);
CREATE INDEX rooms_building_idx ON rooms (building_id, sort_order);

-- Extra charge per optional equipment item for a room (sound, lighting, catering…)
CREATE TABLE equipment_items (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code       text NOT NULL UNIQUE,
    label      text NOT NULL,
    flat_price numeric(10, 2) NOT NULL DEFAULT 0 CHECK (flat_price >= 0)
);

-- ── Activities (recurring programs run by coordinators: חוגים) ─
CREATE TABLE activities (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text NOT NULL,
    description     text NOT NULL DEFAULT '',
    coordinator_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    default_room_id uuid REFERENCES rooms (id) ON DELETE SET NULL,
    color           text NOT NULL DEFAULT '#0f766e',
    audience        text NOT NULL DEFAULT '',
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- ── Bookings (calendar entries, incl. maintenance / closures) ─
CREATE TABLE bookings (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id          uuid NOT NULL REFERENCES rooms (id) ON DELETE RESTRICT,
    title            text NOT NULL,
    type             booking_type NOT NULL,
    status           booking_status NOT NULL DEFAULT 'confirmed',
    starts_at        timestamptz NOT NULL,
    ends_at          timestamptz NOT NULL,
    buffer_minutes   int NOT NULL DEFAULT 0 CHECK (buffer_minutes BETWEEN 0 AND 240),
    blocked_range    tstzrange NOT NULL,
    activity_id      uuid REFERENCES activities (id) ON DELETE SET NULL,
    coordinator_id   uuid REFERENCES users (id) ON DELETE SET NULL,
    customer_id      uuid REFERENCES customers (id) ON DELETE SET NULL,
    request_id       uuid,
    series_id        uuid,
    attendees        int NOT NULL DEFAULT 0 CHECK (attendees >= 0),
    equipment        text[] NOT NULL DEFAULT '{}',
    price            numeric(10, 2) NOT NULL DEFAULT 0 CHECK (price >= 0),
    notes            text NOT NULL DEFAULT '',
    internal_notes   text NOT NULL DEFAULT '',
    created_by       uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at),
    -- Hard guarantee: no two active bookings of the same room overlap
    -- (including the room's turnover buffer after each booking).
    CONSTRAINT bookings_no_overlap EXCLUDE USING gist (
        room_id WITH =,
        blocked_range WITH &&
    ) WHERE (status IN ('pending', 'confirmed'))
);
CREATE INDEX bookings_starts_idx      ON bookings (starts_at);
CREATE INDEX bookings_room_range_idx  ON bookings USING gist (room_id, blocked_range);
CREATE INDEX bookings_customer_idx    ON bookings (customer_id);
CREATE INDEX bookings_coordinator_idx ON bookings (coordinator_id);
CREATE INDEX bookings_series_idx      ON bookings (series_id) WHERE series_id IS NOT NULL;

CREATE FUNCTION bookings_set_range() RETURNS trigger AS $$
BEGIN
    NEW.blocked_range := tstzrange(
        NEW.starts_at,
        NEW.ends_at + make_interval(mins => NEW.buffer_minutes),
        '[)'
    );
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER bookings_set_range_trg
    BEFORE INSERT OR UPDATE ON bookings
    FOR EACH ROW EXECUTE FUNCTION bookings_set_range();

-- ── Portal booking requests (בקשות) ───────────────────────────
CREATE TABLE booking_requests (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    customer_id      uuid REFERENCES customers (id) ON DELETE SET NULL,
    room_id          uuid NOT NULL REFERENCES rooms (id) ON DELETE RESTRICT,
    title            text NOT NULL,
    event_kind       text NOT NULL DEFAULT '',
    starts_at        timestamptz NOT NULL,
    ends_at          timestamptz NOT NULL,
    attendees        int NOT NULL DEFAULT 0 CHECK (attendees >= 0),
    equipment        text[] NOT NULL DEFAULT '{}',
    notes            text NOT NULL DEFAULT '',
    status           request_status NOT NULL DEFAULT 'pending',
    quoted_price     numeric(10, 2) NOT NULL DEFAULT 0,
    final_price      numeric(10, 2),
    internal_notes   text NOT NULL DEFAULT '',
    rejection_reason text NOT NULL DEFAULT '',
    reviewed_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    reviewed_at      timestamptz,
    booking_id       uuid REFERENCES bookings (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);
CREATE INDEX booking_requests_status_idx ON booking_requests (status, created_at DESC);
CREATE INDEX booking_requests_user_idx   ON booking_requests (user_id, created_at DESC);

ALTER TABLE bookings
    ADD CONSTRAINT bookings_request_fk FOREIGN KEY (request_id) REFERENCES booking_requests (id) ON DELETE SET NULL;

-- ── Payments (תשלומים) ────────────────────────────────────────
CREATE TABLE payments (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id  uuid NOT NULL REFERENCES bookings (id) ON DELETE RESTRICT,
    amount      numeric(10, 2) NOT NULL CHECK (amount > 0),
    method      payment_method NOT NULL,
    reference   text NOT NULL DEFAULT '',
    paid_at     timestamptz NOT NULL DEFAULT now(),
    notes       text NOT NULL DEFAULT '',
    recorded_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payments_booking_idx ON payments (booking_id);
CREATE INDEX payments_paid_at_idx ON payments (paid_at DESC);

-- ── Operational staff tasks ──────────────────────────────────
CREATE TABLE task_schedules (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title            text NOT NULL,
    kind             task_kind NOT NULL DEFAULT 'cleaning',
    room_id          uuid REFERENCES rooms (id) ON DELETE CASCADE,
    assignee_id      uuid REFERENCES users (id) ON DELETE SET NULL,
    weekdays         int[] NOT NULL DEFAULT '{0,1,2,3,4}', -- 0 = Sunday
    time_of_day      time NOT NULL,
    duration_minutes int NOT NULL DEFAULT 60 CHECK (duration_minutes > 0),
    is_active        boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title            text NOT NULL,
    description      text NOT NULL DEFAULT '',
    kind             task_kind NOT NULL DEFAULT 'cleaning',
    status           task_status NOT NULL DEFAULT 'open',
    room_id          uuid REFERENCES rooms (id) ON DELETE SET NULL,
    booking_id       uuid REFERENCES bookings (id) ON DELETE CASCADE,
    schedule_id      uuid REFERENCES task_schedules (id) ON DELETE SET NULL,
    assignee_id      uuid REFERENCES users (id) ON DELETE SET NULL,
    due_at           timestamptz NOT NULL,
    completed_at     timestamptz,
    completion_note  text NOT NULL DEFAULT '',
    reminder_sent_at timestamptz,
    created_by       uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tasks_assignee_idx ON tasks (assignee_id, due_at);
CREATE INDEX tasks_reminder_idx ON tasks (due_at) WHERE status = 'open' AND reminder_sent_at IS NULL;
CREATE UNIQUE INDEX tasks_schedule_due_idx ON tasks (schedule_id, due_at) WHERE schedule_id IS NOT NULL;

-- ── Digital signage (שילוט דיגיטלי) ──────────────────────────
CREATE TABLE signage_screens (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug           text NOT NULL UNIQUE,
    name           text NOT NULL,
    building_id    uuid REFERENCES buildings (id) ON DELETE SET NULL,
    theme          text NOT NULL DEFAULT 'dark',
    welcome_text   text NOT NULL DEFAULT '',
    rotate_seconds int NOT NULL DEFAULT 12 CHECK (rotate_seconds BETWEEN 3 AND 300),
    room_ids       uuid[] NOT NULL DEFAULT '{}', -- empty = all rooms of the building
    is_active      boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE announcements (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title      text NOT NULL,
    body       text NOT NULL DEFAULT '',
    image_key  text,
    starts_at  timestamptz NOT NULL DEFAULT now(),
    ends_at    timestamptz,
    priority   int NOT NULL DEFAULT 0,
    screen_id  uuid REFERENCES signage_screens (id) ON DELETE CASCADE, -- NULL = every screen
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX announcements_window_idx ON announcements (starts_at, ends_at);

-- ── Files, notifications, audit ──────────────────────────────
CREATE TABLE attachments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity       text NOT NULL,
    entity_id    uuid NOT NULL,
    object_key   text NOT NULL UNIQUE,
    filename     text NOT NULL,
    content_type text NOT NULL,
    size_bytes   bigint NOT NULL,
    uploaded_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX attachments_entity_idx ON attachments (entity, entity_id);

CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid REFERENCES users (id) ON DELETE CASCADE,
    channel    text NOT NULL,
    recipient  text NOT NULL,
    subject    text NOT NULL DEFAULT '',
    body       text NOT NULL,
    status     text NOT NULL DEFAULT 'queued',
    error      text NOT NULL DEFAULT '',
    read_at    timestamptz,
    sent_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC);

CREATE TABLE audit_log (
    id         bigserial PRIMARY KEY,
    actor_id   uuid REFERENCES users (id) ON DELETE SET NULL,
    entity     text NOT NULL,
    entity_id  uuid,
    action     text NOT NULL,
    data       jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_entity_idx ON audit_log (entity, entity_id, created_at DESC);
