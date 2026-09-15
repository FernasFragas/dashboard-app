@../AGENTS.md

# Claude Code specifics

The import above is the tool-agnostic manual. If it did not load, read `AGENTS.md` before doing
anything else. This part covers what only Claude Code uses.

## Loaded automatically

- `.claude/rules/*.md`: conventions scoped by path (Go backend, frontend, migrations and schema,
  plan and seed). They load when you work on matching files.
- Hooks in `.claude/settings.json`:
  - after every Edit or Write, `gofmt` (Go) or Prettier (`web/`) formats the file;
  - an Edit or Write to a committed migration is blocked, and the message names the next free
    migration number;
  - Bash commands containing `-plan-reset`, `launchctl` or `~/dashboard-data` ask the user first.

## Use these

| When | Use |
|---|---|
| Starting non-trivial work | Write the brief from `docs/agents/TASK_BRIEF.md` in your reply before coding |
| A change needs to be seen running, not just tested | skill `run-dashboard` |
| Before reporting any code change as done | skill `review-loop` |
| An independent review of a diff | subagent `reviewer` (read-only: reports findings, never edits) |
| Checking the UI in a browser | the `claude-in-chrome` skill, pointed at the URL from `run-dashboard` |
