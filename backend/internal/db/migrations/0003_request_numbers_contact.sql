-- 0003: human-friendly request numbers (#1042) and per-request contact details.
ALTER TABLE booking_requests
    ADD COLUMN number        bigint GENERATED ALWAYS AS IDENTITY (START WITH 1001),
    ADD COLUMN contact_name  text NOT NULL DEFAULT '',
    ADD COLUMN contact_phone text NOT NULL DEFAULT '';

CREATE UNIQUE INDEX booking_requests_number_idx ON booking_requests (number);
