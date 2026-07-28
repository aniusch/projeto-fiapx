-- 0001_init.down.sql
-- Reverses 0001_init.up.sql. Drop order respects dependencies:
-- trigger -> table -> function -> table -> type -> extension.

DROP TRIGGER IF EXISTS videos_set_updated_at ON videos;
DROP TABLE IF EXISTS videos;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS video_status;
-- citext is left installed; other schemas may rely on it.
