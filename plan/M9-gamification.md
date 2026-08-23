# M9 · Gamification — the plan as a game you're visibly winning

**Goal:** interacting with the dashboard should feel like progression in a game: actions earn XP, XP raises a level whose *titles are the actual career ladder*, skills tier up from Novice toward the master plan's targets, and real milestones unlock achievements. Evolving in the game = evolving in reality, because every point maps to a real event.
**Depends on:** M7 shipped, M8 merged (XP flows through skill associations).
**Sequencing:** post-v1. Not started before M7 ships and M8 is merged.

## Design principles (these are constraints, not vibes)

1. **XP mirrors real value.** Big numbers only for real outcomes (exam passed, post published, goal done). Never award opening the app, and never award streaks with XP — streaks are their own display.
2. **Rest is sacred.** Sundays never break a streak (Europe/Lisbon day boundary, per global decisions). Guardrail #5 outranks game feel.
3. **Celebrate once.** Each celebration fires exactly once per event. Repetition turns juice into noise.
4. **No guilt mechanics.** No decay, no "you're falling behind" copy, no red states for inactivity. The game rewards; it never punishes.
5. **Honesty is rewarded.** Publishing a worse-than-hoped number earns a badge, not a penalty — same ethos as the master plan.
6. Respect `prefers-reduced-motion`; sound off, no sound in v1.

## Mechanics

### XP rules (seeded `xp_rules` table — data, tunable without code changes)

| source | XP |
|---|---|
| task done (seeded) | 10 |
| task done (ad-hoc, self-created) | 5 |
| goal moved to Done | 150 |
| daily review saved | 5 |
| review including a metric | +10 |
| log: application 📮 | 15 |
| log: course module 📚 | 10 |
| log: portfolio 🔧 | 5 |
| log: Medium post ✍️ | 40 |
| log: OSS event 🔀 | 25 |
| log: benchmark/number 📊 | 30 |
| log: networking 🤝 | 10 |
| log: exam/cert 🎓 | 100 |

Player XP = sum of events, counted once. Skill XP = the same award mirrored in full to **each** linked skill (skills are facets, not a currency — there's no economy to break). Unchecking a task deletes its event (unique on `source_type, source_id`); everything else is append-only.

### Player levels — titles are the career ladder

Cumulative XP for level *n* = `25 · n · (n+1)` → L1 50 · L3 300 · L5 750 · L7 1400 · L10 2750 · L13 4550.

| Levels | Title |
|---|---|
| 1–3 | Backend Engineer |
| 4–6 | Systems Engineer |
| 7–9 | Platform Engineer |
| 10–12 | Distributed Systems Engineer |
| 13–15 | AI Infrastructure Engineer |
| 16+ | Staff / Founder track |

Pacing intent: at the plan's normal rate (~250–450 XP/week), **"AI Infrastructure Engineer" lands around the Nov 9 checkpoint** — the game's arc is the plan's arc. Verify against the seed and tune thresholds in `xp_rules`-adjacent config if needed.

### Skill tiers (per-skill XP)

Untrained 0 · Novice 1 · Apprentice 60 · Practitioner 150 · Adept 300 · Expert 500.
Mapping to master-plan targets: M → Practitioner, H/certified → Expert. Skill cards read: `Postgres internals — Apprentice → target Expert`.

### Streak

Days with ≥1 XP event. **Sunday shield:** Sundays count as protected — a zero-XP Sunday freezes the streak instead of breaking it, shown with a shield icon on the flame.

### Achievements (seeded `achievements`; conditions live in Go, keyed by `code` — ADR-008)

| code | name | unlocks when |
|---|---|---|
| first_task | First Step | first task completed |
| first_number | Shipped a Number | first 📊 log |
| gatekeeper | Gatekeeper | W3 CI-gate task done (`seed_key`) |
| chaos_suite | Chaos Monkey | all five W5–W7 chaos tasks done |
| honest_number | Honest Number | W10 "honest number" task done — publishing it even if worse |
| in_the_arena | In the Arena | W4 PR-open task done |
| shepherd | Shepherd | goal G7 (2 PRs merged) done |
| cold_caller | Cold Caller | 10 📮 logs |
| wordsmith | Wordsmith | 3 ✍️ logs |
| iron_week | Iron Week | a plan week at 100% task completion |
| well_rested | Well Rested | 4 consecutive Sundays with zero XP events |
| boss_w12 | Checkpoint Cleared | W12 checkpoint form submitted |
| streak_7 / streak_30 | On a Roll / Unstoppable | 7- / 30-day streak |

## Data model (change `../docs/DATABASE.md` first)

```
xp_events            id PK · source_type (task|log|goal|review|achievement)
                     · source_id INT · amount INT · created_at
                     · UNIQUE(source_type, source_id)
xp_event_skills      event_id FK · skill_id FK · PK(event_id, skill_id)
xp_rules             source TEXT PK · amount INT            -- seeded
achievements         id PK · code UNIQUE · name · description · sort_order   -- seeded
achievement_unlocks  achievement_id PK/FK · unlocked_at
```

**ADR-008 · XP is a ledger; everything else is derived.** Player XP, level, skill XP, tiers, streak — all computed from `xp_events` (recomputed on boot, cached in memory). Unlock conditions are Go functions keyed by achievement code, evaluated in the API layer after each successful write (store stays pure, per ADR-004). One stored exception: `achievement_unlocks`, because unlock moments are historical facts.

## API (update `../docs/API.md`)

| Method + path | Purpose |
|---|---|
| GET /api/game/profile | level, title, total XP, next threshold, streak (+shield state), badges |
| GET /api/game/skills | per-skill XP + tier + target tier (feeds the M8 tab's bars) |
| GET /api/game/events?limit= | recent XP feed ("Tue · +30 📊 benchmark → chaos, postgres") |

Writes need no new endpoints — awarding + unlock evaluation hook into existing task/log/goal/review handlers; responses gain an optional `game` envelope (`xp_awarded`, `level_up`, `unlocks[]`) so the UI can react in the same round trip.

## UI moments

1. **Level ring** in the Today header: current level + progress to next; tap → profile sheet (title, XP, streak+shield, badge grid, per-skill tier bars).
2. **XP toast** on any awarding action: `+30 · benchmark → chaos, postgres`. One toast, 2s, no stacking.
3. **Level-up:** full-screen moment naming the new title — `Level 7 — Platform Engineer`. Once, dismiss on tap.
4. **Quest copy layer:** presentation-only renames in `web/src/copy/game.ts` — goals render as *quests*, weeks as *chapters* (`Chapter W5 · Chaos`), checkpoint as *boss review*, Sunday recap as *quest report* (XP earned, skills advanced, badges). Schema names never change.
5. **Kanban:** moving a goal to Done fires confetti — once per goal, ever.
6. **Skills tab (M8) upgraded:** tier name + XP bar per skill; `Adept 340/500 → Expert` next to the master-plan target.
7. All effects gated by `prefers-reduced-motion`.

## Anti-patterns deliberately avoided

Daily-login rewards · XP for opening/viewing · streak-loss punishment copy · leaderboards (single player) · purchasable anything · variable-ratio rewards. The plan is already meaningful; the game's job is *feedback*, not manufactured compulsion — over-rewarding trivial actions is how gamification kills intrinsic motivation.

## Acceptance

1. Completing a seeded task awards 10 XP once; unchecking removes exactly that event; re-checking re-awards once.
2. Skill XP mirrors correctly: a task linked to two skills advances both by the full amount; player XP counts it once.
3. Level-up fires the moment and the new title matches the table; profile shows correct next-threshold math.
4. A zero-activity Sunday shows the shield and the streak survives to Monday; a zero-activity Tuesday breaks it.
5. `gatekeeper`, `iron_week`, and `well_rested` unlock under simulated histories in tests; each unlock is recorded once.
6. Every response that awards XP carries the `game` envelope; UI reacts without a second fetch.
7. `prefers-reduced-motion` disables confetti/level-up animation but not the information.
8. `DATABASE.md`, `API.md`, ADR-008 updated with the migration; XP values changeable via `xp_rules` without recompiling.
