import type { Goal, GoalMoveResponse } from "../api/client";

/**
 * Pure board logic, kept out of the component so it can be tested directly.
 *
 * Both interactions — desktop drag and the phone "Move to…" sheet — funnel through planMove
 * and applyMove, so there is exactly one definition of what a move means.
 */

export type GoalStatus = Goal["status"];

export const columns: { status: GoalStatus; label: string }[] = [
  { status: "backlog", label: "Backlog" },
  { status: "active", label: "Active" },
  { status: "done", label: "Done" },
];

/** Matches the server's sparse renumbering so optimistic values look like real ones. */
export const sortStep = 100;

export interface MovePlan {
  status: GoalStatus;
  /** 0-based index within the target column, counted with the moved card removed. */
  position: number;
}

/** One column's cards, in board order. */
export function columnGoals(goals: Goal[], status: GoalStatus): Goal[] {
  return goals
    .filter((goal) => goal.status === status)
    .sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
}

/**
 * Work out the request a move should send, or null when it would change nothing.
 *
 * `targetIndex` counts positions in the target column with the dragged card already removed,
 * which is the same thing the server's `position` means.
 */
export function planMove(
  goals: Goal[],
  goalID: number,
  targetStatus: GoalStatus,
  targetIndex: number,
): MovePlan | null {
  const goal = goals.find((candidate) => candidate.id === goalID);
  if (!goal) {
    return null;
  }

  const without = columnGoals(goals, targetStatus).filter((candidate) => candidate.id !== goalID);
  const position = clamp(targetIndex, 0, without.length);

  if (goal.status === targetStatus) {
    const current = columnGoals(goals, targetStatus).map((candidate) => candidate.id);
    const next = without.map((candidate) => candidate.id);
    next.splice(position, 0, goalID);

    if (current.length === next.length && current.every((id, index) => id === next[index])) {
      return null;
    }
  }

  return { status: targetStatus, position };
}

/**
 * Apply a move locally for the optimistic update, mirroring what the server will do: renumber
 * the target column in sparse steps, renumber the column the card left, and stamp or clear
 * completed_at on the way into or out of Done.
 */
export function applyMove(goals: Goal[], goalID: number, plan: MovePlan, now: string): Goal[] {
  const goal = goals.find((candidate) => candidate.id === goalID);
  if (!goal) {
    return goals;
  }

  const from = goal.status;

  const moved: Goal = {
    ...goal,
    status: plan.status,
    version: goal.version + 1,
    completed_at: nextCompletedAt(goal.completed_at, from, plan.status, now),
  };

  const targetIDs = columnGoals(goals, plan.status)
    .filter((candidate) => candidate.id !== goalID)
    .map((candidate) => candidate.id);
  targetIDs.splice(plan.position, 0, goalID);

  const orders = new Map<number, number>();
  targetIDs.forEach((id, index) => orders.set(id, (index + 1) * sortStep));

  if (from !== plan.status) {
    columnGoals(goals, from)
      .filter((candidate) => candidate.id !== goalID)
      .forEach((candidate, index) => orders.set(candidate.id, (index + 1) * sortStep));
  }

  return goals.map((candidate) => {
    const base = candidate.id === goalID ? moved : candidate;
    const order = orders.get(base.id);
    return order === undefined ? base : { ...base, sort_order: order };
  });
}

/**
 * Turn a dnd-kit drop target into a column and an index.
 *
 * The target is either a column droppable — an empty column, or the space below the last card —
 * or another card, in which case the dragged card takes that card's place.
 */
export function resolveDropTarget(
  goals: Goal[],
  goalID: number,
  overID: string,
): { status: GoalStatus; index: number } | null {
  if (overID.startsWith("column:")) {
    const status = overID.slice("column:".length) as GoalStatus;
    const target = columnGoals(goals, status).filter((goal) => goal.id !== goalID);
    return { status, index: target.length };
  }

  const overGoal = goals.find((goal) => goal.id === Number(overID));
  if (!overGoal) {
    return null;
  }

  const target = columnGoals(goals, overGoal.status).filter((goal) => goal.id !== goalID);
  const index = target.findIndex((goal) => goal.id === overGoal.id);

  return { status: overGoal.status, index: index < 0 ? target.length : index };
}

/** The droppable id of a column, as registered by the board. */
export function columnDroppableID(status: GoalStatus): string {
  return `column:${status}`;
}

/**
 * Given everything the pointer is over, keep the most specific target.
 *
 * Cards are nested inside columns, so a pointer over a card is over both. The card is what the
 * user means. Leaving the column in play is what let a drag between columns resolve to the
 * source column and quietly do nothing.
 */
export function preferCard<T extends { id: string | number }>(collisions: T[]): T[] {
  const card = collisions.find((collision) => !String(collision.id).startsWith("column:"));
  return card ? [card] : collisions;
}

/** Replace the moved card and every card the server renumbered alongside it. */
export function applyServerMove(goals: Goal[], response: GoalMoveResponse): Goal[] {
  const updates = new Map<number, Goal>();
  updates.set(response.goal.id, response.goal);
  for (const goal of response.reordered) {
    updates.set(goal.id, goal);
  }

  return goals.map((goal) => updates.get(goal.id) ?? goal);
}

/** Replace a single card with the server's version of it, used on a 412. */
export function replaceGoal(goals: Goal[], next: Goal): Goal[] {
  return goals.map((goal) => (goal.id === next.id ? next : goal));
}

/**
 * Read a goal out of a 412 body. The server sends the current resource so the client can
 * reconcile without a second round trip.
 */
export function coerceGoal(value: unknown): Goal | null {
  if (value && typeof value === "object" && "id" in value && "status" in value) {
    return value as Goal;
  }
  return null;
}

function nextCompletedAt(
  current: string | null,
  from: GoalStatus,
  to: GoalStatus,
  now: string,
): string | null {
  if (to === "done" && from !== "done") {
    return now;
  }
  if (to !== "done" && from === "done") {
    return null;
  }
  return current;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}
