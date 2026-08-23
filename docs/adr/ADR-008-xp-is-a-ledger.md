# ADR-008: XP is a ledger

## Status

Accepted.

## Context

Gamification should reflect real progress without inventing state that can drift. The app needs
player XP, levels, skill XP, tiers, streaks, and achievements, but most of those are summaries
of actions already taken.

## Decision

Store XP as append-only rows in `xp_events`, keyed by `(source_type, source_id)`. Mirror each
event to linked skills through `xp_event_skills`. Derive player XP, level, title, skill XP,
skill tier, and streak from those tables at read time.

XP values live in `xp_rules` so tuning does not require recompilation. Existing `xp_events`
keep their historical `amount`.

Achievement conditions live in Go, keyed by `achievements.code`. The only stored derived fact
is `achievement_unlocks`, because the unlock timestamp is history.

## Consequences

- Repeating the same action cannot award duplicate XP.
- Unchecking a task deletes that task XP event; other XP sources stay append-only.
- Skill XP cannot disagree with the ledger.
- Sunday streak shielding is computed from XP event days in `Europe/Lisbon`.
- Changing future XP values is a data change; past awards remain auditable.
