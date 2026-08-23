# M5 · Follow-ups

**Depends on:** M5 as shipped. **Blocks:** nothing — but F1 is a correctness bug and should not
reach the phone.
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Rationale:** [M5-kanban.md](M5-kanban.md)

Loose ends left by the M5 build: one verified bug, one design wart it exposed, one thing never
actually exercised, and documentation that has drifted from the code. Ordered by what matters.

> **Repo state:** M4, M5 and M6 are all sitting uncommitted together in `web/src/pages/`.
> Splitting them is a separate job; the script for it is in the session scratchpad.

---

## F1 · Stop the version churn on renumber *(bug)* — **DONE**

### The problem

A Kanban move bumps `version` on **every goal in both the source and the target column**, not
just the card that moved. Measured against a freshly seeded database:

```
seeded:        G0 v1   G1 v1   G2 v1
move G1 →      G0 v2   G1 v3   G2 v2      one card moved; three versions changed
move G2 →      G0 v3   G1 v4   G2 v4      only G2 moved; G1 was merely renumbered
```

Two distinct faults, both in `Store.UpdateGoal` (`internal/store/goals.go`):

1. **The moved card double-bumps.** The main `UPDATE` sets `version = version + 1`, and then
   `renumberColumn` updates the same row again with another `version = version + 1`.
2. **Untouched cards bump.** `renumberColumn` rewrites `sort_order` for every card in the
   column and bumps each one's version — and it runs twice, once for the target column and
   once for the column the card left.

With 18 seeded goals in Backlog, moving one card invalidates all 18 ETags.

### Why it matters

`version` exists to guard *user edits* — title, status, project. `sort_order` renumbering is a
server-side consequence of someone else's move, not a competing edit to this card. Treating it
as one produces two real failures:

- **Spurious 412 on a second quick move.** The frontend's optimistic `applyMove` bumps version
  by exactly 1 (`goalBoard.ts`). The server bumps the moved card by 2. Drag twice before the
  first response lands and the second `If-Match` is stale, so a legitimate move is rejected as
  a conflict.
- **Spurious 412 on an unrelated card.** Any client holding a goal in either column — an open
  detail sheet, the other device — gets a conflict on its next edit because a neighbour moved.
  This is the exact "phone and laptop both open" case ETags were added for, now firing when
  nothing actually conflicts.

### Scope

- `renumberColumn` updates `sort_order` only. Drop `version = version + 1` from its `UPDATE`.
- The moved card's single bump stays where it belongs: the main `UPDATE` in `UpdateGoal`.
- No frontend change: `applyMove`'s `+1` becomes correct rather than coincidental.

### Done when

1. A new store test asserts a move bumps the moved goal's version by **exactly one**.
2. A new store test asserts a move leaves **every other goal's version unchanged**, in both the
   source and the target column.
3. `TestUpdateGoalRenumbersColumn` still passes — it asserts ordering, which does not change.
4. Two moves in a row against a real server, replaying each response's version, both succeed.

---

## F2 · Return every renumbered card, and delete the compensating refetch — **DONE**

### The problem

`UpdateGoal` renumbers both columns but returns only the target column in `reordered`. The
frontend compensates by firing `invalidateQueries` after every successful move, so each move
costs a PATCH **and** a follow-up GET of the whole board.

That refetch also hides mistakes: it silently corrects any reconciliation bug, so the
`applyServerMove` path is never really under test.

### Scope

- `docs/API.md` first — it is the contract, and the schema/contract docs are upstream of the
  code (v2 §5). State that `reordered` carries every card the server renumbered, from both
  columns, and that a card appears at most once.
- `Store.UpdateGoal`: collect the source column's renumbered cards and append them to
  `reordered` instead of discarding them.
- `GoalsPage.tsx`: drop `void queryClient.invalidateQueries(...)` from the move mutation's
  `onSuccess`. The 412 path keeps its invalidation — there, a refetch is the point.

### Done when

1. A store test moving a card between two populated columns gets both columns back in
   `reordered`, with no duplicate ids.
2. A frontend test asserts exactly **one** board GET (the initial load) after a successful
   move — no refetch.
3. The `serveBoard` helper in `GoalsPage.test.tsx` is still needed only by the 412 test; if a
   success-path test still depends on it, the reconciliation is not actually working.

---

## F3 · Actually exercise a drag — **BUG FOUND AND FIXED, AWAITING RE-TEST**

### The problem

No drag gesture has ever been performed. `resolveDropTarget → planMove → applyMove` are
unit-tested, and the phone tap-move path is tested through the UI, but the dnd-kit wiring
itself — sensor activation, `SortableContext` ids matching `useSortable` ids, collision
detection, the droppable ids `column:*` — is unverified. Every one of those could be wrong with
all 29 tests still green.

### Scope

**Required — a manual pass, written into the README's checklist:**

| # | Step | Result       |
|---|---|--------------|
| 1 | `make dev`, open `/goals` on the desktop at ≥768px | ✅           |
| 2 | Drag a Backlog card onto an Active card → it lands *in that card's place* | ✅           |
| 3 | Drag a card onto an empty column → it lands there | ✅ |
| 4 | Drag a card within its own column → order changes | ✅           |
| 5 | Hard-refresh after each → the position stayed | ✅           |
| 6 | Below 768px → dragging does nothing, page still scrolls | ✅           |
| 7 | Tap a card → sheet opens with `done_means`, Move-to, Delete | ✅ |

### What step 2 turned up

The manual pass earned its place: every unit test was green and cross-column drag was broken.
Within-column reordering worked, which is what narrowed it down.

**Mechanism.** The column droppable was registered on the whole `<section>`, so its rectangle
covered the header and all the empty space below the cards. Under distance-based collision
detection (`closestCorners`), that oversized rectangle could stay the nearest target even with
the pointer well inside another column. The drop then resolved to the *source* column, at the
position the card already held — `planMove` correctly returned `null`, no request was sent, and
the card sprang back with no error. A silent no-op, which is why it looked like nothing at all.

**Fix**, three changes in `GoalsPage.tsx`:

1. **Pointer-first collision detection.** `pointerWithin`, falling back to `rectIntersection`
   for the gaps between columns. The target becomes whatever is literally under the cursor
   rather than whatever is nearest by rectangle.
2. **A card beats its column.** Both contain the pointer, so `preferCard` (in `goalBoard.ts`,
   unit-tested) keeps the more specific target.
3. **The droppable moved to the card list**, not the section, with a `min-h` so an empty column
   is still a target — and `MeasuringStrategy.Always`, because columns change height as cards
   move and rects measured once at drag start go stale mid-drag.

`columnDroppableID()` is now the single source of the `column:` prefix that `resolveDropTarget`
parses, with a test pinning the two together.

### Still to do

**Re-run the manual pass.** The fix is reasoned from the symptom and covered by unit tests at
the seams, but no drag has been performed against it — the same gap that let this through the
first time. Steps 2, 3 and 7 are the ones to watch: 3 exercises the new `min-h` empty-column
target, 7 confirms the drag handle did not swallow the tap.

**Then decide on Playwright.** F3 said a wiring failure would be the evidence for automating
this. That evidence now exists: a silent no-op that 48 unit tests could not see. The counter-
argument is unchanged — a browser dependency and CI minutes for a single-user app — so this is
a judgement call, not an obligation.

### Done when

All seven manual steps pass against the fixed build, with step 2 confirmed specifically.

---

## F4 · Fold the M5 decisions back into the docs — **DONE**

### The problem

Six decisions were made while implementing M5 and live only in the code and the session
transcript. v2 §5 lists docs drift as a named risk, and `docs/` is supposed to be sufficient to
rebuild from.

### Scope

Into [`plan/M5-kanban.md`](M5-kanban.md), as decisions taken during the build:

| Decision | Why it is not obvious from the code |
|---|---|
| Move logic lives in `goalBoard.ts`, not the component | It is there to be testable, because jsdom cannot simulate a real drag |
| The card is a single `<button>` | Wrapping a button in dnd-kit's draggable nests a control inside `role="button"` |
| Drag sensors are disabled below 768px | Deliberate, per v2 §5 — not an oversight |
| The WIP badge counts the goals array, not `active_count` | So an optimistic move moves the badge with it |
| The sheet's Move-to appends to the end of the column | The sheet has no notion of position |
| Delete is a two-tap confirm | The plan said "Delete" without saying how destructive it should feel |

Into `docs/API.md`, under `PATCH /api/goals/{id}`:

- `position` is the 0-based index **in the target column with the moved card removed** — the
  same thing the client computes. This is currently implied by an example, not stated.
- The version rule after F1: a move bumps only the moved card.
- The `reordered` contents after F2.

### Done when

`plan/M5-kanban.md` and `docs/API.md` describe the code as it actually is, and F1's and F2's
contract changes are written down before the code changes land.

---

## F5 · Collapse `getGoals` and `getGoalsBoard` — **DONE**

### The problem

`web/src/api/client.ts` has two functions hitting `GET /api/goals`: `getGoals` returns just the
array (used by TodayPage's goal picker), `getGoalsBoard` returns the full envelope (used by the
Kanban). Two names for one endpoint is how a client layer starts to rot.

### Scope

Keep `getGoalsBoard` as the single fetcher; have TodayPage read `.goals` from it. Both pages
then share one query key and one cache entry, so logging against a goal on Today invalidates
the board's log counts for free.

### Done when

One function calls `/api/goals`, both pages use it, and the full frontend gate stays green.

---

## Status

F1, F2, F4 and F5 are done. **F3 found a real bug** — cross-column drag was a silent no-op —
which is now fixed, but the fix itself has not been dragged. F3 stays open until the manual
pass is re-run.

One thing F2 turned up that was not in the original plan: the 412 path had lost its rollback as
well as its refetch, so a failed move left the neighbours shuffled by an optimistic renumber
that never happened on the server. `onError` now rolls back first, whatever the failure, and a
test pins the bystanders in place.
