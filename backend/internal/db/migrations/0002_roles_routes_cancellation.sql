-- 0002: super admin role, request routes (rental / activity via coordinator),
-- hall setup counts and cancellation requests.
ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'super_admin' BEFORE 'admin';

ALTER TABLE booking_requests
    ADD COLUMN route        text NOT NULL DEFAULT 'rental' CHECK (route IN ('rental', 'activity')),
    ADD COLUMN activity_id  uuid REFERENCES activities (id) ON DELETE SET NULL,
    ADD COLUMN tables_count int NOT NULL DEFAULT 0 CHECK (tables_count >= 0),
    ADD COLUMN chairs_count int NOT NULL DEFAULT 0 CHECK (chairs_count >= 0);

ALTER TABLE bookings
    ADD COLUMN tables_count          int NOT NULL DEFAULT 0 CHECK (tables_count >= 0),
    ADD COLUMN chairs_count          int NOT NULL DEFAULT 0 CHECK (chairs_count >= 0),
    ADD COLUMN cancel_requested_at   timestamptz,
    ADD COLUMN cancel_requested_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN cancel_request_reason text NOT NULL DEFAULT '';

CREATE INDEX bookings_cancel_requested_idx ON bookings (cancel_requested_at) WHERE cancel_requested_at IS NOT NULL;
