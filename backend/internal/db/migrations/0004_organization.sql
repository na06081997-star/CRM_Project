-- Client branding: one row per deployment (each matnas runs its own instance).
CREATE TABLE organization (
    id          boolean PRIMARY KEY DEFAULT true CHECK (id),
    name        text NOT NULL DEFAULT '',
    short_name  text NOT NULL DEFAULT '',
    subtitle    text NOT NULL DEFAULT '',
    logo_url    text NOT NULL DEFAULT '',
    mark_url    text NOT NULL DEFAULT '',
    brand_color text NOT NULL DEFAULT '',
    phone       text NOT NULL DEFAULT '',
    email       text NOT NULL DEFAULT '',
    website     text NOT NULL DEFAULT '',
    address     text NOT NULL DEFAULT '',
    updated_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO organization DEFAULT VALUES;
