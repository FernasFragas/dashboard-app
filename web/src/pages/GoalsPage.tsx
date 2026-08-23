import {
  DndContext,
  MeasuringStrategy,
  PointerSensor,
  pointerWithin,
  rectIntersection,
  useDroppable,
  useSensor,
  useSensors,
  type CollisionDetection,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  CheckCircle2,
  GripVertical,
  Loader2,
  ScrollText,
  Trash2,
  X,
} from "lucide-react";
import { useEffect, useState } from "react";

import {
  ApiError,
  deleteGoal,
  getGoalsBoard,
  moveGoal,
  type GameEnvelope,
  type Goal,
  type GoalsResponse,
} from "../api/client";
import { LevelUpMoment } from "../components/LevelUpMoment";
import { gameCopy, gameToast } from "../copy/game";
import { usePrefersReducedMotion } from "../lib/motion";
import {
  gameEventsQueryKey,
  gameProfileQueryKey,
  gameSkillsQueryKey,
  goalsQueryKey,
} from "../lib/queryKeys";
import {
  applyMove,
  applyServerMove,
  coerceGoal,
  columnDroppableID,
  columnGoals,
  columns,
  planMove,
  preferCard,
  replaceGoal,
  resolveDropTarget,
  type GoalStatus,
  type MovePlan,
} from "./goalBoard";

export function GoalsPage() {
  const queryClient = useQueryClient();
  const [toast, setToast] = useState<string | null>(null);
  const [selectedGoalID, setSelectedGoalID] = useState<number | null>(null);
  const [draggingID, setDraggingID] = useState<number | null>(null);
  const [confetti, setConfetti] = useState(false);
  const [levelUp, setLevelUp] = useState<NonNullable<GameEnvelope["level_up"]> | null>(null);
  const isDesktop = useIsDesktop();
  const reducedMotion = usePrefersReducedMotion();

  const boardQuery = useQuery({
    queryKey: goalsQueryKey,
    queryFn: getGoalsBoard,
  });

  const moveMutation = useMutation({
    mutationFn: ({ goal, plan }: { goal: Goal; plan: MovePlan }) =>
      moveGoal(goal, plan.status, plan.position),
    onMutate: async ({ goal, plan }) => {
      await queryClient.cancelQueries({ queryKey: goalsQueryKey });
      const previous = queryClient.getQueryData<GoalsResponse>(goalsQueryKey);

      queryClient.setQueryData<GoalsResponse>(goalsQueryKey, (current) =>
        current
          ? { ...current, goals: applyMove(current.goals, goal.id, plan, new Date().toISOString()) }
          : current,
      );

      return { previous };
    },
    onError: (error, _variables, context) => {
      // Undo the optimistic move first, whatever went wrong. The move did not happen, so none
      // of the renumbering it implied happened either — leaving those neighbours shuffled
      // would show an order the server never had.
      if (context?.previous) {
        queryClient.setQueryData(goalsQueryKey, context.previous);
      }

      // The one conflict this can hit is the phone and the laptop open at once. There is
      // nothing to merge on a card position, so server truth wins and the toast explains why
      // the card jumped back.
      if (error instanceof ApiError && error.status === 412) {
        const current = coerceGoal(error.current);
        if (current) {
          queryClient.setQueryData<GoalsResponse>(goalsQueryKey, (board) =>
            board ? { ...board, goals: replaceGoal(board.goals, current) } : board,
          );
        }
        setToast("Updated elsewhere · refreshed");
        // The 412 body describes the moved card only; the rest of the board is whatever the
        // other device left behind, so this is the one path that genuinely needs a read.
        void queryClient.invalidateQueries({ queryKey: goalsQueryKey });
        return;
      }

      setToast(errorMessage(error, "Move failed"));
    },
    // `reordered` carries every card the server renumbered, in both columns, so the response is
    // the whole truth about the move — no follow-up read (docs/API.md, PATCH /api/goals/{id}).
    onSuccess: (response) => {
      queryClient.setQueryData<GoalsResponse>(goalsQueryKey, (board) =>
        board ? { ...board, goals: applyServerMove(board.goals, response) } : board,
      );
      if (response.game) {
        const message = gameToast(response.game);
        if (message) {
          setToast(message);
        }
        if (response.game.xp_awarded > 0 && response.goal.status === "done" && !reducedMotion) {
          setConfetti(true);
        }
        if (response.game.level_up) {
          setLevelUp(response.game.level_up);
        }
        void queryClient.invalidateQueries({ queryKey: gameProfileQueryKey });
        void queryClient.invalidateQueries({ queryKey: gameSkillsQueryKey });
        void queryClient.invalidateQueries({ queryKey: gameEventsQueryKey });
      }
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteGoal(id),
    onSuccess: () => {
      setSelectedGoalID(null);
      setToast("Goal deleted");
      void queryClient.invalidateQueries({ queryKey: goalsQueryKey });
    },
    onError: (error) => setToast(errorMessage(error, "Delete failed")),
  });

  const goals = boardQuery.data?.goals ?? [];
  const activeLimit = boardQuery.data?.active_limit ?? 0;
  const selectedGoal = goals.find((goal) => goal.id === selectedGoalID) ?? null;

  // Derived rather than read from the response: an optimistic move must move the badge with it.
  const activeCount = columnGoals(goals, "active").length;

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 8 } }));

  useEffect(() => {
    if (!confetti) {
      return;
    }
    const id = window.setTimeout(() => setConfetti(false), 1_600);
    return () => window.clearTimeout(id);
  }, [confetti]);

  const submitMove = (goalID: number, status: GoalStatus, index: number) => {
    const plan = planMove(goals, goalID, status, index);
    if (!plan) {
      return;
    }

    const goal = goals.find((candidate) => candidate.id === goalID);
    if (goal) {
      moveMutation.mutate({ goal, plan });
    }
  };

  const onDragEnd = (event: DragEndEvent) => {
    setDraggingID(null);

    const goalID = Number(event.active.id);
    const over = event.over;
    if (!over) {
      return;
    }

    const target = resolveDropTarget(goals, goalID, String(over.id));
    if (target) {
      submitMove(goalID, target.status, target.index);
    }
  };

  return (
    <section className="min-h-screen px-4 py-5 md:px-10 md:py-8">
      <div className="mx-auto flex max-w-6xl flex-col gap-5">
        <header className="flex flex-col gap-2">
          <p className="text-sm font-medium uppercase text-cyan-300">{gameCopy.quests}</p>
          <h1 className="text-3xl font-semibold text-white md:text-4xl">Quest board</h1>
        </header>

        {boardQuery.isLoading ? <LoadingState /> : null}
        {boardQuery.isError ? <ErrorState message={errorMessage(boardQuery.error, "")} /> : null}

        {boardQuery.data ? (
          <DndContext
            sensors={isDesktop ? sensors : undefined}
            collisionDetection={boardCollisionDetection}
            // Columns change height as cards move between them, so droppable rects measured
            // once at drag start go stale mid-drag and the wrong column wins.
            measuring={{ droppable: { strategy: MeasuringStrategy.Always } }}
            onDragStart={(event: DragStartEvent) => setDraggingID(Number(event.active.id))}
            onDragCancel={() => setDraggingID(null)}
            onDragEnd={onDragEnd}
          >
            <div className="grid gap-4 md:grid-cols-3">
              {columns.map((column) => (
                <BoardColumn
                  key={column.status}
                  status={column.status}
                  label={column.label}
                  goals={columnGoals(goals, column.status)}
                  draggingID={draggingID}
                  overLimit={column.status === "active" && activeCount > activeLimit}
                  activeCount={activeCount}
                  onSelect={(goal) => setSelectedGoalID(goal.id)}
                />
              ))}
            </div>
          </DndContext>
        ) : null}
      </div>

      {selectedGoal ? (
        <GoalDetailSheet
          goal={selectedGoal}
          goals={goals}
          moving={moveMutation.isPending}
          deleting={deleteMutation.isPending}
          activeLimit={activeLimit}
          onClose={() => setSelectedGoalID(null)}
          onMove={(status) => {
            // Moving from the sheet appends to the end of the target column.
            const target = columnGoals(goals, status).filter(
              (candidate) => candidate.id !== selectedGoal.id,
            );
            submitMove(selectedGoal.id, status, target.length);
            setSelectedGoalID(null);
          }}
          onDelete={() => deleteMutation.mutate(selectedGoal.id)}
        />
      ) : null}

      {toast ? <Toast message={toast} onClose={() => setToast(null)} /> : null}
      {confetti && !reducedMotion ? <ConfettiBurst /> : null}
      {levelUp ? <LevelUpMoment levelUp={levelUp} onClose={() => setLevelUp(null)} /> : null}
    </section>
  );
}

function BoardColumn({
  status,
  label,
  goals,
  draggingID,
  overLimit,
  activeCount,
  onSelect,
}: {
  status: GoalStatus;
  label: string;
  goals: Goal[];
  draggingID: number | null;
  overLimit: boolean;
  activeCount: number;
  onSelect: (goal: Goal) => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: columnDroppableID(status) });

  return (
    <section
      aria-label={label}
      className={[
        "flex min-h-32 flex-col rounded-md border bg-white/[0.03] transition",
        isOver ? "border-cyan-300" : "border-white/10",
      ].join(" ")}
    >
      <div className="flex items-center justify-between gap-2 border-b border-white/10 px-4 py-3">
        <h2 className="text-lg font-semibold text-white">{label}</h2>

        {overLimit ? (
          <span
            role="status"
            className="inline-flex items-center gap-1 rounded-sm bg-amber-300/15 px-2 py-1 text-xs font-semibold text-amber-200"
          >
            <AlertTriangle className="size-3.5" aria-hidden="true" />
            {activeCount} active
          </span>
        ) : (
          <span className="text-sm font-medium text-zinc-400">{goals.length}</span>
        )}
      </div>

      <SortableContext items={goals.map((goal) => goal.id)} strategy={verticalListSortingStrategy}>
        {/*
          The droppable is the card list, not the whole section. A section-sized rect covers the
          header and all the empty space below the cards, which lets the source column keep
          winning the collision while the pointer is already over another column — the drop then
          resolves to a same-column move that changes nothing, and the card silently springs back.
          min-h keeps an empty column a real target.
        */}
        <div ref={setNodeRef} className="flex min-h-24 flex-1 flex-col gap-2 p-3">
          {goals.length === 0 ? (
            <p className="px-1 py-3 text-sm text-zinc-500">Nothing here.</p>
          ) : (
            goals.map((goal) => (
              <GoalCard
                key={goal.id}
                goal={goal}
                dragging={draggingID === goal.id}
                onSelect={() => onSelect(goal)}
              />
            ))
          )}
        </div>
      </SortableContext>
    </section>
  );
}

function GoalCard({
  goal,
  dragging,
  onSelect,
}: {
  goal: Goal;
  dragging: boolean;
  onSelect: () => void;
}) {
  const { listeners, setActivatorNodeRef, setNodeRef, transform, transition } = useSortable({
    id: goal.id,
  });

  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={[
        "grid grid-cols-[minmax(0,1fr)_2rem] gap-2 rounded-md border border-white/10 bg-zinc-950 p-3 transition",
        dragging ? "opacity-50" : "",
      ].join(" ")}
    >
      <button
        type="button"
        aria-label={goalCardLabel(goal)}
        className="min-w-0 text-left outline-none focus-visible:ring-2 focus-visible:ring-cyan-300"
        onClick={onSelect}
      >
        <span className="block text-base font-medium text-white">
          {goal.code ? <span>{goal.code} · </span> : null}
          <span>{goal.title}</span>
        </span>

        <span className="mt-2 flex flex-wrap items-center gap-2">
          <span className="inline-flex rounded-sm bg-amber-300/15 px-2 py-1 text-xs font-medium text-amber-200">
            {goal.project}
          </span>

          {goal.target ? (
            <span className="text-xs font-medium text-zinc-400">target {goal.target}</span>
          ) : null}

          <span
            className="ml-auto inline-flex items-center gap-1 text-xs font-medium text-zinc-400"
            title={`${goal.log_count} linked log entries`}
          >
            <ScrollText className="size-3.5" aria-hidden="true" />
            {goal.log_count}
          </span>
        </span>

        {goal.completed_at ? (
          <span className="mt-2 flex items-center gap-1 text-xs font-medium text-emerald-300">
            <CheckCircle2 className="size-3.5" aria-hidden="true" />
            completed {goal.completed_at.slice(0, 10)}
          </span>
        ) : null}
      </button>

      <span
        ref={setActivatorNodeRef}
        className="grid size-8 cursor-grab place-items-center rounded-md text-zinc-500 hover:text-zinc-200"
        aria-hidden="true"
        {...listeners}
      >
        <GripVertical className="size-4" aria-hidden="true" />
      </span>
    </div>
  );
}

function GoalDetailSheet({
  goal,
  goals,
  moving,
  deleting,
  activeLimit,
  onClose,
  onMove,
  onDelete,
}: {
  goal: Goal;
  goals: Goal[];
  moving: boolean;
  deleting: boolean;
  activeLimit: number;
  onClose: () => void;
  onMove: (status: GoalStatus) => void;
  onDelete: () => void;
}) {
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const activeCount = columnGoals(goals, "active").length;

  return (
    <div className="fixed inset-0 z-40 bg-black/55" role="dialog" aria-modal="true">
      <div className="absolute inset-x-0 bottom-0 max-h-[85vh] overflow-y-auto rounded-t-md border-t border-white/10 bg-zinc-950 p-4 shadow-2xl md:left-auto md:right-6 md:top-6 md:w-[28rem] md:rounded-md md:border">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-xs font-medium uppercase text-cyan-300">
              {goal.code ?? "Goal"} · {goal.project}
            </p>
            <h2 className="text-xl font-semibold text-white">{goal.title}</h2>
          </div>
          <button
            type="button"
            className="grid size-11 shrink-0 place-items-center rounded-md border border-white/10 text-zinc-300"
            aria-label="Close"
            onClick={onClose}
          >
            <X className="size-5" aria-hidden="true" />
          </button>
        </div>

        {goal.done_means ? (
          <div className="mt-4">
            <p className="text-xs font-semibold uppercase text-zinc-400">Done means</p>
            <p className="mt-1 border-l border-cyan-300/50 pl-3 text-sm text-zinc-200">
              {goal.done_means}
            </p>
          </div>
        ) : null}

        <dl className="mt-4 grid grid-cols-2 gap-3 text-sm">
          <div>
            <dt className="text-xs font-semibold uppercase text-zinc-400">Target</dt>
            <dd className="mt-1 text-zinc-200">{goal.target ?? "—"}</dd>
          </div>
          <div>
            <dt className="text-xs font-semibold uppercase text-zinc-400">Linked logs</dt>
            <dd className="mt-1 text-zinc-200">{goal.log_count}</dd>
          </div>
        </dl>

        <div className="mt-5">
          <p className="text-xs font-semibold uppercase text-zinc-400">Move to</p>
          <div className="mt-2 grid grid-cols-3 gap-2">
            {columns.map((column) => {
              const current = column.status === goal.status;

              return (
                <button
                  key={column.status}
                  type="button"
                  disabled={current || moving}
                  aria-label={`Move to ${column.label}`}
                  className="min-h-11 rounded-md border border-white/10 bg-white/[0.03] px-2 text-sm font-medium text-zinc-200 disabled:cursor-not-allowed disabled:text-zinc-600"
                  onClick={() => onMove(column.status)}
                >
                  {column.label}
                </button>
              );
            })}
          </div>

          {goal.status !== "active" && activeCount >= activeLimit ? (
            <p className="mt-2 text-xs text-amber-200">
              {activeCount} goals are already active. Moving another one is allowed.
            </p>
          ) : null}
        </div>

        <div className="mt-6 border-t border-white/10 pt-4">
          {confirmingDelete ? (
            <div className="flex items-center gap-2">
              <button
                type="button"
                disabled={deleting}
                className="flex min-h-11 flex-1 items-center justify-center gap-2 rounded-md bg-red-400 px-3 font-semibold text-zinc-950 disabled:bg-zinc-700"
                onClick={onDelete}
              >
                {deleting ? (
                  <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                ) : (
                  <Trash2 className="size-4" aria-hidden="true" />
                )}
                Delete goal
              </button>
              <button
                type="button"
                className="min-h-11 rounded-md border border-white/10 px-3 text-sm font-medium text-zinc-300"
                onClick={() => setConfirmingDelete(false)}
              >
                Cancel
              </button>
            </div>
          ) : (
            <button
              type="button"
              className="flex min-h-11 items-center gap-2 text-sm font-medium text-red-300"
              onClick={() => setConfirmingDelete(true)}
            >
              <Trash2 className="size-4" aria-hidden="true" />
              Delete
            </button>
          )}
          <p className="mt-2 text-xs text-zinc-500">
            Deleting a goal keeps its tasks and log entries; they simply stop pointing at it.
          </p>
        </div>
      </div>
    </div>
  );
}

function Toast({ message, onClose }: { message: string; onClose: () => void }) {
  useEffect(() => {
    const id = window.setTimeout(onClose, 4_000);
    return () => window.clearTimeout(id);
  }, [onClose]);

  return (
    <div
      role="status"
      className="fixed right-4 top-4 z-50 rounded-md border border-white/10 bg-zinc-900 px-4 py-3 text-sm font-medium text-white shadow-xl shadow-black/30"
    >
      {message}
    </div>
  );
}

function ConfettiBurst() {
  const colors = ["bg-cyan-300", "bg-emerald-300", "bg-amber-300", "bg-fuchsia-300"];

  return (
    <div className="pointer-events-none fixed inset-0 z-40 overflow-hidden" aria-hidden="true">
      {Array.from({ length: 24 }, (_, index) => (
        <span
          key={index}
          className={[
            "absolute left-1/2 top-1/3 size-2 animate-bounce rounded-sm",
            colors[index % colors.length],
          ].join(" ")}
          style={{
            transform: `translate(${(index % 8) * 22 - 78}px, ${Math.floor(index / 8) * 18}px) rotate(${index * 18}deg)`,
            animationDelay: `${(index % 6) * 45}ms`,
            animationDuration: "900ms",
          }}
        />
      ))}
    </div>
  );
}

function LoadingState() {
  return (
    <div className="grid min-h-52 place-items-center rounded-md border border-white/10 bg-white/[0.03] text-zinc-300">
      <Loader2 className="mr-2 inline size-4 animate-spin" aria-hidden="true" />
      Loading
    </div>
  );
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="rounded-md border border-red-300/30 bg-red-300/10 p-4 text-red-100">
      {message}
    </div>
  );
}

/**
 * Resolve what the pointer is over, preferring a card to the column containing it.
 *
 * Distance-based detection (closestCorners and friends) compares rectangles, and a column's
 * rectangle is far larger than a card's — so while dragging between columns the source column
 * could stay the nearest target even with the pointer well inside another one. The drop then
 * resolved to a same-column move to the position the card already held, `planMove` correctly
 * returned null, and the card sprang back with nothing to show for it.
 *
 * Going pointer-first removes the ambiguity: the target is whatever is literally under the
 * cursor. A card and its column both qualify, so the card — the more specific target — wins.
 * `rectIntersection` is the fallback for a pointer outside every droppable, which happens when
 * dragging over the gap between columns.
 */
const boardCollisionDetection: CollisionDetection = (args) => {
  const collisions = pointerWithin(args);
  return preferCard(collisions.length > 0 ? collisions : rectIntersection(args));
};

/**
 * Drag is a desktop nicety. On a phone the pointer sensor fights the scroll gesture, so it is
 * left off entirely and "Move to…" in the detail sheet is the interaction (v2 section 5).
 */
function useIsDesktop(): boolean {
  const [isDesktop, setIsDesktop] = useState(() => matches());

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) {
      return;
    }

    const query = window.matchMedia("(min-width: 768px)");
    const onChange = () => setIsDesktop(query.matches);

    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, []);

  return isDesktop;
}

function matches(): boolean {
  if (typeof window === "undefined" || !window.matchMedia) {
    return false;
  }
  return window.matchMedia("(min-width: 768px)").matches;
}

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error) {
    return error.message;
  }
  return fallback || "Request failed";
}

function goalCardLabel(goal: Goal): string {
  return `${goal.code ? `${goal.code} · ` : ""}${goal.title}`;
}
