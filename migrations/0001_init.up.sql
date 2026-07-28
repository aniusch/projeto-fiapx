-- 0001_init.up.sql
-- Initial schema for the FIAP X video-processing system.
--
-- Two aggregates: users (authentication) and videos (processing jobs owned by a
-- user). This file doubles as the "script de criação do banco" deliverable.

-- Extensions ---------------------------------------------------------------

-- citext gives us case-insensitive text, used for emails so that
-- "User@x.com" and "user@x.com" collide on the UNIQUE constraint.
CREATE EXTENSION IF NOT EXISTS citext;

-- Types --------------------------------------------------------------------

-- The lifecycle of a video job. Using an ENUM keeps invalid states out of the
-- database entirely, rather than relying on the application to police a string.
CREATE TYPE video_status AS ENUM ('PENDING', 'PROCESSING', 'DONE', 'FAILED');

-- Tables -------------------------------------------------------------------

CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT      UNIQUE NOT NULL,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE videos (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    original_name TEXT         NOT NULL,
    status        video_status NOT NULL DEFAULT 'PENDING',
    source_key    TEXT         NOT NULL DEFAULT '', -- object-storage key of the uploaded video
    zip_key       TEXT         NOT NULL DEFAULT '', -- object-storage key of the result zip
    frame_count   INTEGER      NOT NULL DEFAULT 0,
    error_message TEXT         NOT NULL DEFAULT '', -- populated when status = FAILED
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- The status listing is always scoped to a user and shown newest-first, so we
-- index exactly that access pattern.
CREATE INDEX idx_videos_user_created ON videos (user_id, created_at DESC);

-- Keep updated_at honest without the application having to remember to set it.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER videos_set_updated_at
    BEFORE UPDATE ON videos
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
