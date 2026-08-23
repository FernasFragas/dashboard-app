-- 006_gamification.sql
--
-- XP is a ledger: write events once, derive totals, levels, skill progress and streaks from
-- those rows. Achievements are the only stored derived fact because unlock time is history.

CREATE TABLE xp_rules (
    source TEXT    PRIMARY KEY CHECK (length(trim(source)) > 0),
    amount INTEGER NOT NULL CHECK (amount >= 0)
);

CREATE TABLE xp_events (
    id          INTEGER PRIMARY KEY,
    source_type TEXT    NOT NULL CHECK (source_type IN (
                    'task', 'log', 'goal', 'review', 'metric', 'achievement'
                )),
    source_id   INTEGER NOT NULL,
    amount      INTEGER NOT NULL CHECK (amount > 0),
    created_at  TEXT    NOT NULL,
    UNIQUE (source_type, source_id)
);

CREATE TABLE xp_event_skills (
    event_id INTEGER NOT NULL REFERENCES xp_events (id) ON DELETE CASCADE,
    skill_id INTEGER NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    PRIMARY KEY (event_id, skill_id)
);

CREATE TABLE achievements (
    id          INTEGER PRIMARY KEY,
    code        TEXT    NOT NULL UNIQUE CHECK (length(trim(code)) > 0),
    name        TEXT    NOT NULL CHECK (length(trim(name)) > 0),
    description TEXT    NOT NULL CHECK (length(trim(description)) > 0),
    sort_order  INTEGER NOT NULL
);

CREATE TABLE achievement_unlocks (
    achievement_id INTEGER PRIMARY KEY REFERENCES achievements (id) ON DELETE CASCADE,
    unlocked_at    TEXT    NOT NULL
);

INSERT INTO xp_rules (source, amount) VALUES
    ('task_seeded', 10),
    ('task_ad_hoc', 5),
    ('goal_done', 150),
    ('daily_review', 5),
    ('metric', 10),
    ('log:application', 15),
    ('log:module', 10),
    ('log:portfolio', 5),
    ('log:post', 40),
    ('log:oss', 25),
    ('log:number', 30),
    ('log:network', 10),
    ('log:exam', 100);

INSERT INTO achievements (code, name, description, sort_order) VALUES
    ('first_task', 'First Step', 'Complete your first task.', 10),
    ('first_number', 'Shipped a Number', 'Log your first benchmark or number.', 20),
    ('gatekeeper', 'Gatekeeper', 'Complete the W3 regression-gate-in-ci task.', 30),
    ('chaos_suite', 'Chaos Monkey', 'Complete the five W5-W7 chaos tasks.', 40),
    ('honest_number', 'Honest Number', 'Publish the W10 honest number.', 50),
    ('in_the_arena', 'In the Arena', 'Open the W4 pull request.', 60),
    ('shepherd', 'Shepherd', 'Move goal G7 to Done.', 70),
    ('cold_caller', 'Cold Caller', 'Log ten applications.', 80),
    ('wordsmith', 'Wordsmith', 'Log three posts.', 90),
    ('iron_week', 'Iron Week', 'Complete every seeded task in a plan week.', 100),
    ('well_rested', 'Well Rested', 'Protect four consecutive Sundays with no XP events.', 110),
    ('boss_w12', 'Checkpoint Cleared', 'Submit the W12 checkpoint review.', 120),
    ('streak_7', 'On a Roll', 'Build a seven-day XP streak.', 130),
    ('streak_30', 'Unstoppable', 'Build a thirty-day XP streak.', 140);

CREATE INDEX idx_xp_events_created_at ON xp_events (created_at DESC, id DESC);
CREATE INDEX idx_xp_event_skills_skill ON xp_event_skills (skill_id);

PRAGMA foreign_key_check;
