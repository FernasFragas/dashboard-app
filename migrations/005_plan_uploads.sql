-- 005_plan_uploads.sql
--
-- Browser-loaded plans need two bits of durable state:
-- - categories can be retired instead of deleted when old log entries still reference them;
-- - uploaded source text is retained so exports can answer exactly what was loaded.

ALTER TABLE categories ADD COLUMN retired_at TEXT;

CREATE TABLE plan_sources (
    id        INTEGER PRIMARY KEY,
    plan_id   TEXT NOT NULL CHECK (length(trim(plan_id)) > 0),
    sha256    TEXT NOT NULL CHECK (length(sha256) = 64),
    source    TEXT NOT NULL,
    loaded_at TEXT NOT NULL,
    UNIQUE (plan_id, sha256)
);

CREATE INDEX idx_plan_sources_plan_loaded ON plan_sources (plan_id, loaded_at DESC, id DESC);
