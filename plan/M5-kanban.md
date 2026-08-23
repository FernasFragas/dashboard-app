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
- Both paths issue the same `PATCH /api/goals/{id}` with `{status, position}` — a 0-based index
  in the target column, counted with the moved card removed. The server renumbers the affected
  columns in sparse steps of 100 in one transaction and returns **every** card whose
  `sort_order` changed, in both columns; the client applies that payload and needs no
  follow-up read. Only the moved card's `version` is bumped (F1).
- Optimistic, per the M4 conventions: the card jumps immediately, and any failure rolls the
  whole optimistic move back — including the neighbours it shuffled to make room. A 412 then
  applies the server's version of the moved card, shows the "Updated elsewhere · refreshed"
  toast, and refetches, because the 412 body describes that one card and nothing else.
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

---

## Decisions taken during the build

Recorded here because none of them is obvious from reading the code, and this plan is what a
future reader checks the code against.

**Move logic lives in `goalBoard.ts`, not the component.** `resolveDropTarget → planMove →
applyMove` are pure functions in a separate module so they can be tested directly. jsdom cannot
simulate a real pointer drag reliably, so testing them through the component would have meant
testing nothing. The consequence is stated plainly in F3: the computation behind a drag is
covered, the gesture itself is not.

**The card is a single `<button>`.** The first version wrapped a button in dnd-kit's draggable,
which puts `role="button"` on the wrapper — an interactive control nested inside another, and
every card matching twice in the tests. One element that is both draggable and tappable fixes
both. The pointer sensor's distance threshold is what keeps a tap a tap.

**Drag sensors are disabled below 768px.** Deliberate, per v2 §5: on touch the pointer sensor
fights the scroll gesture. Mobile gets tap-to-move only, which is the primary interaction there
anyway — this is not an oversight to "fix" later.

**The WIP badge counts the goals array, not the response's `active_count`.** An optimistic move
updates the column immediately; reading the count from the last server response would leave the
badge disagreeing with the cards underneath it until the next fetch.

**"Move to…" in the sheet appends to the end of the target column.** The sheet shows one card
and has no notion of position, so it sends `position = length of the target column`. Only drag
can place a card between two others.

**Delete is a two-tap confirm.** The plan listed Delete in the sheet without saying how
destructive it should feel. One tap arms it, the second commits, and the sheet says that tasks
and log entries survive — they simply stop pointing at the goal.

## Follow-ups

Tracked in [M5-followups.md](M5-followups.md). F1 (version churn on renumber) and F2 (return
every renumbered card, drop the compensating refetch) are done; F3 (perform an actual drag) is
still open, so item 1 of "Done when" above has **not** been verified.
