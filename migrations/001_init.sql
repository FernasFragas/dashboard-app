-- 001_init.sql
--
-- Transcribed 1:1 from docs/DATABASE.md sections 4 and 5. If this file and that document
-- disagree, the document is right and this file is a bug. Schema changes start in the doc.
--
-- schema_migrations is created by the migration runner before any migration is applied,
-- so it is deliberately absent here (see docs/DATABASE.md section 7).

-- Section 4.2 - seeded plan windows, read-only at runtime.
CREATE TABLE weeks (
    code       TEXT PRIMARY KEY,
    phase      TEXT    NOT NULL CHECK (phase IN ('P1', 'P2', 'P3')),
    start_date TEXT    NOT NULL,
    end_date   TEXT    NOT NULL,
    focus      TEXT,
    sort_order INTEGER NOT NULL,
    CHECK (start_date <= end_date)
);

-- Section 4.3 - the operating-system table. weekdays is a CSV of ISO weekday numbers
-- (1=Mon .. 7=Sun) because the plan groups days ("Tue-Wed" -> "2,3").
-- label is UNIQUE because the seed loader matches on it (section 8).
CREATE TABLE rhythm (
    id         INTEGER PRIMARY KEY,
    label      TEXT    NOT NULL UNIQUE,
    weekdays   TEXT    NOT NULL,
    slot       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL
);

-- Section 4.4 - the eight quick-log buttons. The CHECK on a seeded slug turns a typo in the
-- seed into a boot failure instead of an orphan category.
CREATE TABLE categories (
    id         TEXT PRIMARY KEY CHECK (id IN (
                   'application', 'module', 'portfolio', 'post',
                   'oss', 'number', 'network', 'exam'
               )),
    label      TEXT    NOT NULL,
    icon       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL
);

-- Section 4.5 - metric-name catalogue. baseline and target are TEXT because the plan states
-- several of them as prose; they are displayed, never computed with.
CREATE TABLE metric_defs (
    name       TEXT PRIMARY KEY,
    unit       TEXT,
    baseline   TEXT,
    target     TEXT,
    sort_order INTEGER NOT NULL
);

-- Section 4.6
CREATE TABLE goals (
    id           INTEGER PRIMARY KEY,
    code         TEXT UNIQUE,
    title        TEXT    NOT NULL CHECK (length(trim(title)) > 0),
    done_means   TEXT,
    project      TEXT    NOT NULL CHECK (project IN (
                     'synapse', 'gateway', 'dash', 'oss',
                     'learn', 'write', 'career', 'all'
                 )),
    phase        TEXT    CHECK (phase IS NULL OR phase IN ('P1', 'P2', 'P3')),
    status       TEXT    NOT NULL DEFAULT 'backlog'
                     CHECK (status IN ('backlog', 'active', 'done')),
    target       TEXT,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    version      INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT    NOT NULL,
    completed_at TEXT,
    CHECK (status = 'done' OR completed_at IS NULL)
);

-- Section 4.7 - steps is a JSON array: always read and written whole, never queried into.
CREATE TABLE tasks (
    id         INTEGER PRIMARY KEY,
    week       TEXT    NOT NULL REFERENCES weeks (code) ON DELETE RESTRICT,
    title      TEXT    NOT NULL CHECK (length(trim(title)) > 0),
    project    TEXT    NOT NULL CHECK (project IN (
                   'synapse', 'gateway', 'dash', 'oss',
                   'learn', 'write', 'career', 'all'
               )),
    goal_id    INTEGER REFERENCES goals (id) ON DELETE SET NULL,
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

-- Section 4.8 - append and delete only, never updated. RESTRICT on the category is
-- intentional: categories are seeded and must never be deleted out from under history.
CREATE TABLE log_entries (
    id          INTEGER PRIMARY KEY,
    category_id TEXT    NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    title       TEXT    NOT NULL CHECK (length(trim(title)) > 0),
    note        TEXT,
    url         TEXT,
    goal_id     INTEGER REFERENCES goals (id) ON DELETE SET NULL,
    occurred_at TEXT    NOT NULL,
    created_at  TEXT    NOT NULL
);

-- Section 4.9 - an entirely empty review must not count toward the streak, so it must
-- not exist. "next" is quoted throughout for safety.
CREATE TABLE daily_reviews (
    date       TEXT PRIMARY KEY,
    learned    TEXT,
    issue      TEXT,
    "next"     TEXT,
    minutes    INTEGER CHECK (minutes IS NULL OR minutes >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (coalesce(learned, '') || coalesce(issue, '') || coalesce("next", '') <> '')
);

-- Section 4.10 - name is deliberately NOT a foreign key to metric_defs: the Review screen's
-- "other..." field must accept a new metric without a migration.
CREATE TABLE metrics (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    value       REAL NOT NULL,
    unit        TEXT,
    note        TEXT,
    recorded_at TEXT NOT NULL
);

-- Section 4.11 - answers is index-aligned with questions.
CREATE TABLE checkpoints (
    id           INTEGER PRIMARY KEY,
    week         TEXT NOT NULL UNIQUE REFERENCES weeks (code) ON DELETE RESTRICT,
    questions    TEXT NOT NULL,
    answers      TEXT,
    completed_at TEXT
);

-- Section 5 - every index exists for one named query.

-- GET /api/logs - the reverse-chronological feed and its cursor pagination.
CREATE INDEX idx_log_entries_occurred_at ON log_entries (occurred_at DESC);

-- GET /api/logs?category=... and GET /api/logs/summary - per-category counters.
CREATE INDEX idx_log_entries_category_occurred ON log_entries (category_id, occurred_at DESC);

-- The linked-log count chip on every Kanban card.
CREATE INDEX idx_log_entries_goal_id ON log_entries (goal_id);

-- GET /api/tasks?week=W5 and the dashboard bundle's week checklist.
CREATE INDEX idx_tasks_week ON tasks (week, sort_order);

-- GET /api/goals grouped into the three Kanban columns, in column order.
CREATE INDEX idx_goals_status_sort ON goals (status, sort_order);

-- The Review screen's "last few readings" for the selected metric.
CREATE INDEX idx_metrics_name_recorded ON metrics (name, recorded_at DESC);
