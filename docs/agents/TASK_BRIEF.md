# Task brief template

Copy the block below into the prompt, the pull request description, or a `plan/` file. Delete
sections that truly do not apply; never delete "Done means".

```markdown
# <Short imperative title>

- **Owner:** <agent session or person>
- **Risk:** low | medium | high (docs/agents/PLAYBOOK.md §4)
- **Depends on:** <tasks or pull requests, or "none">
- **Blocks:** <tasks waiting on this, or "none">
- **Parallel-safe with:** <tasks that touch none of the same hotspots>

## Goal
One or two sentences describing the outcome for the user.

## Done means
Each item is checkable and says how to check it.
- [ ] <observable behaviour> — `<command>` or <manual step>
- [ ] Checks for the risk level pass — `make check-ci`
- [ ] Docs updated: <API.md / DATABASE.md / PROGRESS.md / none>

## Requirements
- <what must be true>

## Constraints
- <AGENTS.md rules that apply, e.g. "schema change starts in docs/DATABASE.md">
- Out of scope: <what not to touch>

## Relevant files
- `<path>` — <why>

## Tools and environment
- <isolated instance, browser check, a plan file to load, what is not available>

## Open questions
- <anything that needs a human decision before or during the work>
```

## Example

```markdown
# Return linked log count on GET /api/goals/{id}

- **Owner:** Claude Code session
- **Risk:** medium
- **Depends on:** none
- **Blocks:** goal detail sheet shows "4 logs"
- **Parallel-safe with:** tasks that do not touch docs/API.md §2 or web/src/api/client.ts

## Goal
The goal detail sheet can show how many log entries cite a goal without a second request.

## Done means
- [ ] `GET /api/goals/{id}` includes `log_count` — `go test ./internal/api -run TestGetGoal`
- [ ] The count matches `GET /api/goals` for the same goal — API test compares both
- [ ] `docs/API.md` §3 shows the field — `make check-docs`
- [ ] `make check-ci` passes

## Constraints
- Derived at read time, never stored (ADR-007 style).
- No change to the Kanban list query's performance.

## Relevant files
- `internal/store/goals.go` — list query already computes log_count
- `internal/api/handlers.go` — getGoal
- `web/src/api/client.ts` — Goal type
```
