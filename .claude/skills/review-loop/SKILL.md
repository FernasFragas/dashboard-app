---
name: review-loop
description: The required loop before reporting a code change in this repository as done — classify risk, run the checks for that risk, self-review against AGENTS.md and the brief, get an independent review from the reviewer subagent, fix findings, rerun. Use at the end of every implementation task.
---

# Review loop

Follows `docs/agents/PLAYBOOK.md` §4–§6.

1. **Classify the risk** of the change (low, medium or high) with the table in the playbook. When
   unsure, take the higher level.
2. **Run the required checks** for that level. Fix failures before going on. Never describe a
   failing or skipped check as passing.
3. **Self-review.** Read `git diff HEAD` and `git status --short` in full and confirm:
   - each "done means" item from the brief is proven, with evidence;
   - AGENTS.md rules hold, including those enforced only by review, plus the matching
     `.claude/rules/*.md`;
   - new behaviour has tests that fail without the change;
   - docs changed with the code: `docs/API.md`, `docs/DATABASE.md`, `docs/PROGRESS.md`,
     `docs/TECH_DEBT.md`;
   - no debug output, stray files, or secrets in logs.
4. **Independent review.** Launch the `reviewer` subagent with the brief (or goal) and the scope.
   If the session's instructions say not to spawn agents without the user asking, ask the user once
   whether to run it, or suggest they run `/code-review`. If neither happens, do a second self-review
   pass file by file and say in the handoff that no independent review ran.
5. **Fix** every finding, or record why not: disagree with a reason, out of scope, or deferred as a
   `docs/TECH_DEBT.md` entry.
6. **Rerun** the checks from step 2. If the fixes were more than trivial, repeat steps 3–4. Stop after
   three rounds and hand the open findings to the user.
7. **Hand off** with: what changed, the checks run and their results, the review verdict and how each
   finding was resolved, and anything not verified.
