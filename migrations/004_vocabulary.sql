-- 004_vocabulary.sql
--
-- Plan vocabulary is data, not schema. Projects, phases and skill tiers move into reference
-- tables; categories lose the plan-specific slug CHECK. The rebuild preserves every existing
-- domain row, then the seed loader refreshes labels/config from the active plan document.

CREATE TABLE projects (
    id         TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    label      TEXT    NOT NULL CHECK (length(trim(label)) > 0),
    sort_order INTEGER NOT NULL
);

CREATE TABLE phases (
    id         TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    label      TEXT    NOT NULL CHECK (length(trim(label)) > 0),
    sort_order INTEGER NOT NULL
);

CREATE TABLE skill_tiers (
    id         TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    label      TEXT    NOT NULL CHECK (length(trim(label)) > 0),
    sort_order INTEGER NOT NULL
);

CREATE TABLE plan_meta (
    id        TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    name      TEXT NOT NULL CHECK (length(trim(name)) > 0),
    loaded_at TEXT NOT NULL
);

CREATE TABLE plan_config (
    key   TEXT PRIMARY KEY CHECK (length(trim(key)) > 0),
    value TEXT NOT NULL
);

INSERT INTO projects (id, label, sort_order)
SELECT id, id, row_number() OVER (ORDER BY id) * 10
FROM (
    SELECT project AS id FROM goals
    UNION
    SELECT project AS id FROM tasks
)
WHERE id IS NOT NULL AND length(trim(id)) > 0;

INSERT INTO phases (id, label, sort_order)
SELECT id, id, row_number() OVER (ORDER BY id) * 10
FROM (
    SELECT phase AS id FROM weeks
    UNION
    SELECT phase AS id FROM goals WHERE phase IS NOT NULL
)
WHERE id IS NOT NULL AND length(trim(id)) > 0;

INSERT INTO skill_tiers (id, label, sort_order)
SELECT id, id, row_number() OVER (ORDER BY id) * 10
FROM (
    SELECT DISTINCT trim(target_tier) AS id FROM skills WHERE target_tier IS NOT NULL
)
WHERE id IS NOT NULL AND length(trim(id)) > 0;

INSERT INTO plan_config (key, value) VALUES ('active_goal_limit', '3');

INSERT INTO plan_meta (id, name, loaded_at)
SELECT 'master-plan-v5', 'Master Plan v5', strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE EXISTS (SELECT 1 FROM weeks)
   OR EXISTS (SELECT 1 FROM goals)
   OR EXISTS (SELECT 1 FROM tasks);

CREATE TEMP TABLE _m10_counts (
    name TEXT PRIMARY KEY,
    n    INTEGER NOT NULL
);

INSERT INTO _m10_counts (name, n) VALUES
    ('weeks', (SELECT count(*) FROM weeks)),
    ('categories', (SELECT count(*) FROM categories)),
    ('goals', (SELECT count(*) FROM goals)),
    ('tasks', (SELECT count(*) FROM tasks)),
    ('log_entries', (SELECT count(*) FROM log_entries)),
    ('checkpoints', (SELECT count(*) FROM checkpoints)),
    ('skills', (SELECT count(*) FROM skills)),
    ('task_skills', (SELECT count(*) FROM task_skills)),
    ('log_skills', (SELECT count(*) FROM log_skills));

CREATE TEMP TABLE _m10_assert (
    ok INTEGER NOT NULL CHECK (ok = 1)
);

CREATE TABLE weeks_new (
    code       TEXT PRIMARY KEY,
    phase      TEXT    NOT NULL REFERENCES phases (id) ON DELETE RESTRICT,
    start_date TEXT    NOT NULL,
    end_date   TEXT    NOT NULL,
    focus      TEXT,
    sort_order INTEGER NOT NULL,
    CHECK (start_date <= end_date)
);

CREATE TABLE categories_new (
    id         TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    label      TEXT    NOT NULL,
    icon       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE metric_defs_new (
    name           TEXT PRIMARY KEY,
    unit           TEXT,
    baseline       TEXT,
    target         TEXT,
    sort_order     INTEGER NOT NULL,
    slug           TEXT,
    definition     TEXT,
    how_to_measure TEXT
);

CREATE TABLE goals_new (
    id           INTEGER PRIMARY KEY,
    code         TEXT UNIQUE,
    title        TEXT    NOT NULL CHECK (length(trim(title)) > 0),
    done_means   TEXT,
    project      TEXT    NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    phase        TEXT    REFERENCES phases (id) ON DELETE RESTRICT,
    status       TEXT    NOT NULL DEFAULT 'backlog'
                         CHECK (status IN ('backlog', 'active', 'done')),
    target       TEXT,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    version      INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT    NOT NULL,
    completed_at TEXT,
    CHECK (status = 'done' OR completed_at IS NULL)
);

CREATE TABLE tasks_new (
    id         INTEGER PRIMARY KEY,
    week       TEXT    NOT NULL REFERENCES weeks_new (code) ON DELETE RESTRICT,
    title      TEXT    NOT NULL CHECK (length(trim(title)) > 0),
    project    TEXT    NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    goal_id    INTEGER REFERENCES goals_new (id) ON DELETE SET NULL,
    steps      TEXT    NOT NULL DEFAULT '[]',
    done_means TEXT,
    status     TEXT    NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'done')),
    done_at    TEXT,
    seed_key   TEXT UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    version    INTEGER NOT NULL DEFAULT 1,
    created_at TEXT    NOT NULL,
    CHECK (status = 'done' OR done_at IS NULL)
);

CREATE TABLE log_entries_new (
    id          INTEGER PRIMARY KEY,
    category_id TEXT    NOT NULL REFERENCES categories_new (id) ON DELETE RESTRICT,
    title       TEXT    NOT NULL CHECK (length(trim(title)) > 0),
    note        TEXT,
    url         TEXT,
    goal_id     INTEGER REFERENCES goals_new (id) ON DELETE SET NULL,
    occurred_at TEXT    NOT NULL,
    created_at  TEXT    NOT NULL
);

CREATE TABLE checkpoints_new (
    id           INTEGER PRIMARY KEY,
    week         TEXT NOT NULL UNIQUE REFERENCES weeks_new (code) ON DELETE RESTRICT,
    questions    TEXT NOT NULL,
    answers      TEXT,
    completed_at TEXT,
    helpers      TEXT
);

CREATE TABLE skills_new (
    id             INTEGER PRIMARY KEY,
    code           TEXT    NOT NULL UNIQUE,
    name           TEXT    NOT NULL CHECK (length(trim(name)) > 0),
    description    TEXT    NOT NULL CHECK (length(trim(description)) > 0),
    associate_when TEXT    NOT NULL CHECK (length(trim(associate_when)) > 0),
    target_tier    TEXT    REFERENCES skill_tiers (id) ON DELETE RESTRICT,
    sort_order     INTEGER NOT NULL,
    created_at     TEXT    NOT NULL
);

CREATE TABLE task_skills_new (
    task_id  INTEGER NOT NULL REFERENCES tasks_new (id) ON DELETE CASCADE,
    skill_id INTEGER NOT NULL REFERENCES skills_new (id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, skill_id)
);

CREATE TABLE log_skills_new (
    log_id   INTEGER NOT NULL REFERENCES log_entries_new (id) ON DELETE CASCADE,
    skill_id INTEGER NOT NULL REFERENCES skills_new (id) ON DELETE CASCADE,
    PRIMARY KEY (log_id, skill_id)
);

INSERT INTO weeks_new SELECT code, phase, start_date, end_date, focus, sort_order FROM weeks;
INSERT INTO categories_new SELECT id, label, icon, sort_order FROM categories;
INSERT INTO metric_defs_new
SELECT name, unit, baseline, target, sort_order, slug, definition, how_to_measure FROM metric_defs;
INSERT INTO goals_new
SELECT id, code, title, done_means, project, phase, status, target, sort_order, version,
       created_at, completed_at
FROM goals;
INSERT INTO tasks_new
SELECT id, week, title, project, goal_id, steps, done_means, status, done_at, seed_key,
       sort_order, version, created_at
FROM tasks;
INSERT INTO log_entries_new
SELECT id, category_id, title, note, url, goal_id, occurred_at, created_at FROM log_entries;
INSERT INTO checkpoints_new
SELECT id, week, questions, answers, completed_at, helpers FROM checkpoints;
INSERT INTO skills_new
SELECT id, code, name, description, associate_when, target_tier, sort_order, created_at FROM skills;
INSERT INTO task_skills_new SELECT task_id, skill_id FROM task_skills;
INSERT INTO log_skills_new SELECT log_id, skill_id FROM log_skills;

INSERT INTO _m10_assert (ok)
SELECT CASE WHEN (SELECT count(*) FROM weeks_new) = (SELECT n FROM _m10_counts WHERE name = 'weeks') THEN 1 ELSE 0 END;
INSERT INTO _m10_assert (ok)
SELECT CASE WHEN (SELECT count(*) FROM categories_new) = (SELECT n FROM _m10_counts WHERE name = 'categories') THEN 1 ELSE 0 END;
INSERT INTO _m10_assert (ok)
SELECT CASE WHEN (SELECT count(*) FROM goals_new) = (SELECT n FROM _m10_counts WHERE name = 'goals') THEN 1 ELSE 0 END;
INSERT INTO _m10_assert (ok)
SELECT CASE WHEN (SELECT count(*) FROM tasks_new) = (SELECT n FROM _m10_counts WHERE name = 'tasks') THEN 1 ELSE 0 END;
INSERT INTO _m10_assert (ok)
SELECT CASE WHEN (SELECT count(*) FROM log_entries_new) = (SELECT n FROM _m10_counts WHERE name = 'log_entries') THEN 1 ELSE 0 END;
INSERT INTO _m10_assert (ok)
SELECT CASE WHEN (SELECT count(*) FROM checkpoints_new) = (SELECT n FROM _m10_counts WHERE name = 'checkpoints') THEN 1 ELSE 0 END;
INSERT INTO _m10_assert (ok)
SELECT CASE WHEN (SELECT count(*) FROM skills_new) = (SELECT n FROM _m10_counts WHERE name = 'skills') THEN 1 ELSE 0 END;
INSERT INTO _m10_assert (ok)
SELECT CASE WHEN (SELECT count(*) FROM task_skills_new) = (SELECT n FROM _m10_counts WHERE name = 'task_skills') THEN 1 ELSE 0 END;
INSERT INTO _m10_assert (ok)
SELECT CASE WHEN (SELECT count(*) FROM log_skills_new) = (SELECT n FROM _m10_counts WHERE name = 'log_skills') THEN 1 ELSE 0 END;

DROP TABLE log_skills;
DROP TABLE task_skills;
DROP TABLE log_entries;
DROP TABLE checkpoints;
DROP TABLE tasks;
DROP TABLE goals;
DROP TABLE skills;
DROP TABLE metric_defs;
DROP TABLE weeks;
DROP TABLE categories;

ALTER TABLE weeks_new RENAME TO weeks;
ALTER TABLE categories_new RENAME TO categories;
ALTER TABLE metric_defs_new RENAME TO metric_defs;
ALTER TABLE goals_new RENAME TO goals;
ALTER TABLE tasks_new RENAME TO tasks;
ALTER TABLE log_entries_new RENAME TO log_entries;
ALTER TABLE checkpoints_new RENAME TO checkpoints;
ALTER TABLE skills_new RENAME TO skills;
ALTER TABLE task_skills_new RENAME TO task_skills;
ALTER TABLE log_skills_new RENAME TO log_skills;

CREATE UNIQUE INDEX idx_metric_defs_slug ON metric_defs (slug) WHERE slug IS NOT NULL;
CREATE INDEX idx_log_entries_occurred_at ON log_entries (occurred_at DESC);
CREATE INDEX idx_log_entries_category_occurred ON log_entries (category_id, occurred_at DESC);
CREATE INDEX idx_log_entries_goal_id ON log_entries (goal_id);
CREATE INDEX idx_tasks_week ON tasks (week, sort_order);
CREATE INDEX idx_goals_status_sort ON goals (status, sort_order);
CREATE INDEX idx_task_skills_skill ON task_skills (skill_id);
CREATE INDEX idx_log_skills_skill ON log_skills (skill_id);

PRAGMA foreign_key_check;
