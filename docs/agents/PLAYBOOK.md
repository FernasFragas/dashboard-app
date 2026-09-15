# Agent playbook

How work moves from an idea to a merged change in this repository, whether an agent, a human, or
both do it. The operating rules are in [AGENTS.md](../../AGENTS.md); this document is the process.

## 1. Lifecycle

```
brief → plan → implement → verify → self-review → independent review → human review → merge → record
```

Nothing skips a step. Small changes move through the steps quickly; they do not skip them.

## 2. Write a task brief

Before code, fill in [TASK_BRIEF.md](TASK_BRIEF.md): goal, **done means** (checkable, each with the
command or manual step that proves it), requirements, constraints, relevant files, tools, risk and
dependencies. A brief can live in the prompt, the pull request description, or a `plan/` file for
milestone-sized work.

If you cannot write "done means" as checks, the task is not ready. Ask, or split it.

## 3. Split and coordinate

- **Size.** One pull request is one reviewable concern: roughly 400 changed lines or fewer,
  excluding generated files (`internal/seed/seed.json`, lockfiles). Split larger goals into tasks
  with their own briefs.
- **Dependencies.** Every brief records `Depends on`, `Blocks` and `Parallel-safe with`. A task
  starts only when its dependencies are merged, or explicitly stacked on their branch.
- **Parallel work.** Each agent works in its own git worktree and runs its own
  `make dev-instance`, so neither files nor ports nor databases collide.
- **Serialized hotspots.** These files conflict by nature. Two concurrent tasks must not both
  change them; sequence those tasks instead:
  - `migrations/` (numbering)
  - `internal/seed/seed.json` and `master-plan-v5.md`
  - the endpoint table in `docs/API.md` and `NewMux` in `internal/api/server.go`
  - `web/src/api/client.ts`

## 4. Risk levels and required checks

| Risk | Typical changes | Required before review |
|---|---|---|
| **Low** | Docs, copy strings, tests only | `make check-docs`; plus `cd web && pnpm lint` for copy changes |
| **Medium** | Handler, store method, page or component, refactor within a package | `make check-ci`, and the affected flow exercised in `make dev-instance` |
| **High** | Migration, seed or plan, pairing, token or auth, backup, build, CI, release | Everything for medium, a boot against a copy of real-shaped data, and human review of the full diff. Release steps only with explicit approval. |

When in doubt, take the higher level.

## 5. Verification evidence

"It works" is not evidence. The handoff lists what ran and what came back:

- commands and their result (`make check-ci` → pass; `go test ./internal/api -run TestX` → pass);
- for API changes, the `curl` call and the relevant part of the response;
- for UI changes, what was checked in the browser, at which widths (about 390 px and ≥768 px), and
  whether the console was clean;
- anything that could not be verified, stated plainly.

## 6. Review loop

1. **Self-review.** Read the whole diff as a hostile reviewer:
   - every "done means" item is checked with evidence;
   - AGENTS.md rules hold, including the ones marked "Review";
   - new behaviour has tests; docs changed with the code (`docs/API.md`, `docs/DATABASE.md`,
     `docs/PROGRESS.md`, `docs/TECH_DEBT.md`);
   - no debug output, stray files, commented-out code, or secrets in logs.
2. **Independent review.** A second agent with no stake in the change reviews the diff against
   the brief. In Claude Code, that is the `reviewer` subagent (`.claude/agents/reviewer.md`); any
   other tool can use that file as the reviewer prompt. Findings use the format in §8.
3. **Fix** each finding, or record why not: wrong, out of scope, or deferred to `docs/TECH_DEBT.md`.
4. **Rerun** the checks for the risk level. If the fixes were more than trivial, review again.
5. **Stop after three rounds** and hand the open findings to a human instead of looping.

## 7. Human validation

A human reviews every pull request before merge, using the checklist in
`.github/pull_request_template.md`:

- **Correctness:** behaviour matches "done means"; edge cases such as Lisbon day boundaries,
  `412` conflicts and an empty plan are handled.
- **Security:** nothing binds `0.0.0.0`; no token or pairing code is logged; input is validated.
- **Performance:** no per-row queries in list handlers; no unbounded responses.
- **Maintainability:** layering holds; no duplicate of an existing helper; tests are readable.
- **Requirements:** nothing out of scope slipped in; migrations are new files only.

Read the test output and the diff, not just the green check.

## 8. Feedback loop

Feedback to an agent is specific enough to act on without a conversation:

```
[blocker|major|minor] <one-line summary>
Where:    path:line
Problem:  what happens, with inputs and the wrong result
Expected: what should happen, and the rule or brief item it comes from
Evidence: command and output, or a code excerpt
```

The agent reproduces the problem first (ideally as a failing test), fixes it, reruns the checks,
and answers each item with what changed and the evidence. The reviewer verifies the revision rather
than trusting the answer, then approves and merges.

When the same feedback appears twice, turn it into a check (lint rule, contract test, doccheck
rule or hook) and add a row to "Feedback already turned into checks" in `docs/TECH_DEBT.md`.

## 9. Autonomy levels

| Level | The agent may | Gate |
|---|---|---|
| L0 Explore | Read, run read-only commands, answer questions | None |
| L1 Change | Edit, run checks, run isolated instances | Human reviews the diff before commit |
| L2 Propose | Commit on a branch and open a draft pull request with evidence | Human review and CI |
| L3 Deliver | Merge pull requests of a listed low-risk class after green CI and a clean independent review | Only for classes listed below, after a successful pilot |

**Current setting:** L1 by default; L2 when the user asks for it. No L3 classes yet.

Expand one step at a time, per class of task, after the previous step ran cleanly several times.
The order to automate: reproduce a bug as a failing test → fix → verify → open the pull request →
merge low-risk classes.

**Never autonomous, at any level:** anything touching `~/dashboard-data`, running migrations against
the live database, releases and `launchctl`, a `/plan` replace, force-pushes, and changes to CI
protections.

## 10. Maintenance schedule

| Cadence | What | How |
|---|---|---|
| Every pull request | Doc links and paths, API and schema contracts, layering, migration immutability | Automatic in `make check-ci` |
| Weekly | Full CI on an unchanged tree (toolchain drift) | Scheduled workflow |
| After each milestone, or monthly | Doc sweep: compare `docs/ARCHITECTURE.md`, `docs/CODEMAP.md`, `docs/API.md` and `docs/DATABASE.md` with the code; remove duplication; update `docs/PROGRESS.md` and `docs/TECH_DEBT.md`; promote one recurring review comment into a check | Run the `reviewer` agent with "review docs against code" as the brief |
| After each plan change | Runbook order: validate, seed-gen, reseed mode, tests | `docs/PLAN_CHANGE_RUNBOOK.md` |

## 11. Pilot

This repository has one maintainer, so "train teammates" reduces to proving the workflow on real
work before relying on it:

1. Pick one medium-risk task and run it through the whole lifecycle using the recipe in
   [RECIPES.md](RECIPES.md) and the review loop.
2. Record in `docs/TECH_DEBT.md` anything the loop missed and what would have caught it.
3. Only after a clean pilot, consider L2 as the default.

If collaborators join: pair on one brief-to-merge cycle, with the newcomer writing the brief and
judging the agent's evidence, before they supervise agents alone.
