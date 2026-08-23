import { describe, expect, it } from "vitest";

import type { Goal, GoalMoveResponse } from "../api/client";
import {
  applyMove,
  applyServerMove,
  coerceGoal,
  columnDroppableID,
  columnGoals,
  planMove,
  preferCard,
  replaceGoal,
  resolveDropTarget,
  sortStep,
} from "./goalBoard";

function goal(overrides: Partial<Goal> & Pick<Goal, "id">): Goal {
  return {
    code: `G${overrides.id}`,
    title: `Goal ${overrides.id}`,
    done_means: null,
    project: "dash",
    phase: "P1",
    status: "active",
    target: null,
    sort_order: overrides.id * sortStep,
    log_count: 0,
    version: 1,
    created_at: "2026-08-24T09:00:00Z",
    completed_at: null,
    ...overrides,
  };
}

/** Three active cards at 100/200/300, plus one in backlog. */
function board(): Goal[] {
  return [
    goal({ id: 1, sort_order: 100 }),
    goal({ id: 2, sort_order: 200 }),
    goal({ id: 3, sort_order: 300 }),
    goal({ id: 9, status: "backlog", sort_order: 100 }),
  ];
}

describe("columnGoals", () => {
  it("returns one column in sort order", () => {
    expect(columnGoals(board(), "active").map((g) => g.id)).toEqual([1, 2, 3]);
    expect(columnGoals(board(), "backlog").map((g) => g.id)).toEqual([9]);
    expect(columnGoals(board(), "done")).toEqual([]);
  });

  it("breaks ties on id so the order is never arbitrary", () => {
    const tied = [goal({ id: 5, sort_order: 100 }), goal({ id: 2, sort_order: 100 })];
    expect(columnGoals(tied, "active").map((g) => g.id)).toEqual([2, 5]);
  });
});

describe("planMove", () => {
  // position counts the target column with the dragged card removed, which is exactly what
  // the server's `position` means.
  it("sends the requested position when moving within a column", () => {
    expect(planMove(board(), 3, "active", 0)).toEqual({ status: "active", position: 0 });
    expect(planMove(board(), 1, "active", 1)).toEqual({ status: "active", position: 1 });
  });

  it("sends the requested position when moving between columns", () => {
    expect(planMove(board(), 9, "active", 1)).toEqual({ status: "active", position: 1 });
  });

  it("clamps a position past the end of the column", () => {
    expect(planMove(board(), 9, "active", 99)).toEqual({ status: "active", position: 3 });
  });

  it("returns null when the card would not actually move", () => {
    expect(planMove(board(), 1, "active", 0)).toBeNull();
    expect(planMove(board(), 2, "active", 1)).toBeNull();
  });

  it("returns null for an unknown card", () => {
    expect(planMove(board(), 404, "active", 0)).toBeNull();
  });
});

describe("applyMove", () => {
  const now = "2026-09-24T17:40:00Z";

  it("renumbers the target column in sparse steps", () => {
    const next = applyMove(board(), 3, { status: "active", position: 0 }, now);

    expect(columnGoals(next, "active").map((g) => g.id)).toEqual([3, 1, 2]);
    expect(columnGoals(next, "active").map((g) => g.sort_order)).toEqual([100, 200, 300]);
  });

  it("renumbers the column the card left", () => {
    const next = applyMove(board(), 1, { status: "backlog", position: 0 }, now);

    expect(columnGoals(next, "active").map((g) => g.id)).toEqual([2, 3]);
    expect(columnGoals(next, "active").map((g) => g.sort_order)).toEqual([100, 200]);
    expect(columnGoals(next, "backlog").map((g) => g.id)).toEqual([1, 9]);
  });

  it("bumps the version so a second move sends a fresh If-Match", () => {
    const next = applyMove(board(), 1, { status: "done", position: 0 }, now);
    expect(next.find((g) => g.id === 1)?.version).toBe(2);
  });

  it("stamps completed_at on the way into Done and clears it on the way out", () => {
    const done = applyMove(board(), 1, { status: "done", position: 0 }, now);
    expect(done.find((g) => g.id === 1)?.completed_at).toBe(now);

    const back = applyMove(done, 1, { status: "active", position: 0 }, now);
    expect(back.find((g) => g.id === 1)?.completed_at).toBeNull();
  });

  it("leaves completed_at alone when reordering within Done", () => {
    const first = applyMove(board(), 1, { status: "done", position: 0 }, now);
    const second = applyMove(first, 2, { status: "done", position: 0 }, "2026-09-25T09:00:00Z");

    expect(second.find((g) => g.id === 1)?.completed_at).toBe(now);
  });

  it("leaves the board untouched for an unknown card", () => {
    const before = board();
    expect(applyMove(before, 404, { status: "done", position: 0 }, now)).toEqual(before);
  });
});

describe("applyServerMove", () => {
  it("takes the server's ordering for every card it renumbered", () => {
    const response: GoalMoveResponse = {
      goal: goal({ id: 3, sort_order: 100, version: 2 }),
      reordered: [
        goal({ id: 3, sort_order: 100, version: 2 }),
        goal({ id: 1, sort_order: 200, version: 2 }),
        goal({ id: 2, sort_order: 300, version: 2 }),
      ],
    };

    const next = applyServerMove(board(), response);

    expect(columnGoals(next, "active").map((g) => g.id)).toEqual([3, 1, 2]);
    expect(next.find((g) => g.id === 9)?.sort_order).toBe(100);
  });
});

describe("resolveDropTarget", () => {
  it("drops onto a card, taking its place", () => {
    expect(resolveDropTarget(board(), 3, "1")).toEqual({ status: "active", index: 0 });
    expect(resolveDropTarget(board(), 1, "3")).toEqual({ status: "active", index: 1 });
  });

  it("drops onto a column, appending to the end", () => {
    expect(resolveDropTarget(board(), 9, "column:active")).toEqual({
      status: "active",
      index: 3,
    });
    expect(resolveDropTarget(board(), 1, "column:done")).toEqual({ status: "done", index: 0 });
  });

  it("returns null for an unrecognised target", () => {
    expect(resolveDropTarget(board(), 1, "404")).toBeNull();
  });
});

// Cards sit inside columns, so a pointer over a card is over both droppables. Letting the
// column win is what made a drag between columns resolve to a same-column move and silently
// do nothing.
describe("preferCard", () => {
  it("keeps the card when a card and a column both match", () => {
    expect(preferCard([{ id: "column:active" }, { id: 7 }])).toEqual([{ id: 7 }]);
  });

  it("keeps the card regardless of the order they arrive in", () => {
    expect(preferCard([{ id: 7 }, { id: "column:active" }])).toEqual([{ id: 7 }]);
  });

  it("falls back to the column when that is all there is", () => {
    expect(preferCard([{ id: "column:done" }])).toEqual([{ id: "column:done" }]);
  });

  it("passes an empty list through", () => {
    expect(preferCard([])).toEqual([]);
  });

  // The prefix is the contract between the droppable id and resolveDropTarget; if one side
  // changes it, this pair of tests is what notices.
  it("agrees with the id resolveDropTarget parses", () => {
    const id = columnDroppableID("active");

    expect(id).toBe("column:active");
    expect(preferCard([{ id }])).toEqual([{ id }]);
    expect(resolveDropTarget(board(), 9, id)?.status).toBe("active");
  });
});

describe("replaceGoal and coerceGoal", () => {
  it("replaces a single card", () => {
    const next = replaceGoal(board(), goal({ id: 2, title: "server truth", version: 7 }));
    expect(next.find((g) => g.id === 2)?.title).toBe("server truth");
  });

  it("reads a goal out of a 412 body and rejects anything else", () => {
    expect(coerceGoal(goal({ id: 1 }))?.id).toBe(1);
    expect(coerceGoal({ nope: true })).toBeNull();
    expect(coerceGoal(null)).toBeNull();
    expect(coerceGoal("nope")).toBeNull();
  });
});
