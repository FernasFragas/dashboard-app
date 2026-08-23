-- 003_skills.sql
--
-- Transcribed from docs/DATABASE.md sections 4.12-4.14 and 5. Skill progress is derived from
-- these associations at read time and never stored -- see docs/adr/ADR-007.

-- Section 4.12 - the named capabilities the work builds. Seeded reference data.
CREATE TABLE skills (
    id             INTEGER PRIMARY KEY,
    code           TEXT    NOT NULL UNIQUE,
    name           TEXT    NOT NULL CHECK (length(trim(name)) > 0),
    description    TEXT    NOT NULL CHECK (length(trim(description)) > 0),
    -- The tagging rule, rendered beside every option in the picker: the description does the
    -- deciding, so choosing the right skill needs no outside knowledge.
    associate_when TEXT    NOT NULL CHECK (length(trim(associate_when)) > 0),
    target_tier    TEXT    CHECK (target_tier IS NULL OR target_tier IN ('Practitioner', 'Expert')),
    sort_order     INTEGER NOT NULL,
    created_at     TEXT    NOT NULL
);

-- Section 4.13 - which skills a task builds. Every task must have at least one; SQLite cannot
-- enforce a cross-table minimum, so the API, a seed test and a UI badge hold that invariant.
--
-- CASCADE on both sides rather than the SET NULL used for history: a link is not a record of
-- something that happened, it is a statement about a row. If the row goes, the link goes.
CREATE TABLE task_skills (
    task_id  INTEGER NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    skill_id INTEGER NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, skill_id)
);

-- Section 4.14 - optional evidence: a log entry can name the skills it demonstrates. Optional
-- by design, because the quick-log path is the app's most-used action and must stay cheap.
CREATE TABLE log_skills (
    log_id   INTEGER NOT NULL REFERENCES log_entries (id) ON DELETE CASCADE,
    skill_id INTEGER NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    PRIMARY KEY (log_id, skill_id)
);

-- Section 5 - the primary keys already cover lookups from a task or a log. These serve the
-- reverse direction, which is the one the Skills tab reads.

-- Per-skill derived stats: which tasks build this skill, and how many are done.
CREATE INDEX idx_task_skills_skill ON task_skills (skill_id);

-- A skill's evidence list and its last-activity date.
CREATE INDEX idx_log_skills_skill ON log_skills (skill_id);
