/**
 * Query keys shared across pages.
 *
 * A key that two pages use has to live somewhere both can import, or they drift apart and stop
 * sharing a cache entry — which is how Today ended up reading the goal list under one key while
 * the Kanban read the same endpoint under another.
 *
 * Keys used by a single page can stay in that page.
 */

/** GET /api/goals — the goal list and the Kanban board are the same request. */
export const goalsQueryKey = ["goals"] as const;

/** GET /api/projects — plan vocabulary for filters and task creation. */
export const projectsQueryKey = ["projects"] as const;

/** GET /api/skills — the profile grid, shared by the Skills tab and the task skill picker. */
export const skillsQueryKey = ["skills"] as const;

/** GET /api/game/profile — player level, streak and unlocked badges. */
export const gameProfileQueryKey = ["game", "profile"] as const;

/** GET /api/game/skills — per-skill XP and tiers. */
export const gameSkillsQueryKey = ["game", "skills"] as const;

/** GET /api/game/events — recent XP ledger entries. */
export const gameEventsQueryKey = ["game", "events"] as const;
