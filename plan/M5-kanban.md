# M5 · Goals Kanban

**Depends on:** M3, M4 (shell + mutation conventions).
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Rationale:** `dashboard-plan-v2.md` §1 "Screens", §2

Three columns — **Backlog · Active · Done** — from `GET /api/goals` grouped by `status`.

## Seeded state

All 18 goals seed as `backlog`. **Status changes only by your hand** — nothing derives Active
from the target date. The Active column starts empty on purpose: choosing what's active is the
accountability mechanism the board exists to force, and automating it removes the decision.

## Card

Face (phone-readable at a glance):

```
G7 · OSS presence                    [oss]
target W9 / P2                    ◷ 4 logs
```

- `code · title` + project tag
- **target** — the deadline from the seed; without it Active tells you what you're doing but
  not what's overdue
- **linked-log count chip** — entries whose `goal_id` points here. v2 calls this "proof of
  motion": the one number separating a goal you're working from one that's been Active for six
  weeks.

**Tap the card** → detail sheet with `done_means` in full, plus Move-to and Delete. It's a
whole sentence per goal and would swamp a phone column on the face, so it lives one tap in.

## Moving cards

- **Desktop:** dnd-kit drag between and within columns.
- **Phone:** tap card → "Move to…" in the detail sheet. This is the *primary* mobile
  interaction, not a fallback — drag on mobile web is unreliable and not worth fighting (v2 §5).
- Both paths issue the same `PATCH /api/goals/{id}` with `{status, position}` (0-based index in
  the target column). The server renumbers that column in sparse steps of 100 in one
  transaction and returns the affected cards; the client reconciles from the response.
- Optimistic, per the M4 conventions: card jumps immediately, rolls back on error, and a 412
  refetches with the "Updated elsewhere · refreshed" toast.
- Moving into **Done** stamps `completed_at`; moving back out clears it.

## WIP limit — warning only

Active > 3 renders a warning badge on the column header showing the count. **The move always
succeeds** — no confirm, no block. A hard limit on your own goal board becomes something you
fight during a real week, and then route around by lying about statuses. The number being
visible is what changes behaviour.

## Tests

1. Drop into a column at index 1 → the PATCH body carries `position: 1`, and the rendered order
   matches the server's renumbered response.
2. 412 on a move → board reflects server truth, toast shown.
3. Active count crossing 3 → badge appears; the move is not blocked.
4. Move to Done → card shows a completion stamp; move back to Active → stamp gone.

## Done when

1. Drag a card on desktop, hard-refresh — it stayed, in the same position within its column.
2. The same move works via tap → "Move to…" on the phone.
3. Four goals in Active shows the warning badge and still lets you add a fifth.
