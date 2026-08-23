import {
  type InfiniteData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { ExternalLink, Loader2, RotateCcw, Trash2 } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import {
  deleteLog,
  getCategories,
  getLogs,
  getLogSummary,
  type Category,
  type CategoryCount,
  type LogFeedResponse,
  type LogEntry,
} from "../api/client";
import { formatLisbonDayHeading, formatLisbonTime, formatWeekOf, lisbonDateKey } from "../lib/date";

const logQueryRoot = ["logs"] as const;
const summaryQueryKey = ["log-summary", "week"] as const;
const deleteUndoMS = 3_500;

export function LogPage() {
  const queryClient = useQueryClient();
  const [selectedCategories, setSelectedCategories] = useCategoryFilter();
  const [pendingDeleteIDs, setPendingDeleteIDs] = useState<Set<number>>(new Set());
  const [undoNotice, setUndoNotice] = useState<{ id: number; title: string } | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const timersRef = useRef<Map<number, number>>(new Map());
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  const categoriesQuery = useQuery({
    queryKey: ["categories"],
    queryFn: getCategories,
  });

  const summaryQuery = useQuery({
    queryKey: summaryQueryKey,
    queryFn: () => getLogSummary("week"),
  });

  const logsQuery = useInfiniteQuery({
    queryKey: [...logQueryRoot, selectedCategories],
    queryFn: ({ pageParam }) =>
      getLogs({
        categories: selectedCategories,
        cursor: pageParam,
        limit: 40,
      }),
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.next_cursor,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteLog(id),
    onSuccess: (_unused, id) => {
      queryClient.setQueriesData<InfiniteData<LogFeedResponse>>(
        { queryKey: logQueryRoot },
        (current) => (current ? removeEntryFromPages(current, id) : current),
      );
      setPendingDeleteIDs((current) => removeSetValue(current, id));
      void queryClient.invalidateQueries({ queryKey: logQueryRoot });
      void queryClient.invalidateQueries({ queryKey: summaryQueryKey });
      void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
    },
    onError: (error, id) => {
      setPendingDeleteIDs((current) => removeSetValue(current, id));
      setToast(error instanceof Error ? error.message : "Delete failed");
    },
  });

  const fetchNextPage = logsQuery.fetchNextPage;
  const hasNextPage = logsQuery.hasNextPage;
  const isFetchingNextPage = logsQuery.isFetchingNextPage;

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || !hasNextPage || isFetchingNextPage || !("IntersectionObserver" in window)) {
      return;
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          void fetchNextPage();
        }
      },
      { rootMargin: "360px" },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [fetchNextPage, hasNextPage, isFetchingNextPage]);

  useEffect(() => {
    const timers = timersRef.current;
    return () => {
      timers.forEach((timer) => window.clearTimeout(timer));
      timers.clear();
    };
  }, []);

  const entries = useMemo(
    () => logsQuery.data?.pages.flatMap((page) => page.entries) ?? [],
    [logsQuery.data],
  );
  const visibleEntries = useMemo(
    () => entries.filter((entry) => !pendingDeleteIDs.has(entry.id)),
    [entries, pendingDeleteIDs],
  );
  const groups = useMemo(() => groupEntriesByLisbonDay(visibleEntries), [visibleEntries]);

  const categories = categoriesQuery.data ?? [];
  const summary = summaryQuery.data;

  const requestDelete = (entry: LogEntry) => {
    if (timersRef.current.has(entry.id)) {
      return;
    }

    setPendingDeleteIDs((current) => addSetValue(current, entry.id));
    setUndoNotice({ id: entry.id, title: entry.title });

    const timer = window.setTimeout(() => {
      timersRef.current.delete(entry.id);
      setUndoNotice((current) => (current?.id === entry.id ? null : current));
      deleteMutation.mutate(entry.id);
    }, deleteUndoMS);

    timersRef.current.set(entry.id, timer);
  };

  const undoDelete = (id: number) => {
    const timer = timersRef.current.get(id);
    if (!timer) {
      return;
    }

    window.clearTimeout(timer);
    timersRef.current.delete(id);
    setPendingDeleteIDs((current) => removeSetValue(current, id));
    setUndoNotice((current) => (current?.id === id ? null : current));
  };

  return (
    <section className="min-h-screen px-4 py-5 md:px-10 md:py-8">
      <div className="mx-auto flex max-w-5xl flex-col gap-5">
        <header className="flex flex-col gap-2">
          <p className="text-sm font-medium uppercase text-cyan-300">Log</p>
          <div className="flex flex-wrap items-end justify-between gap-3">
            <h1 className="text-3xl font-semibold text-white md:text-4xl">What happened</h1>
            <p className="text-sm text-zinc-400">Newest first</p>
          </div>
        </header>

        <WeeklyRecap
          loading={summaryQuery.isLoading}
          from={summary?.from ?? null}
          counts={summary?.counts ?? []}
        />

        <CategoryChips
          categories={categories}
          selected={selectedCategories}
          onChange={setSelectedCategories}
        />

        {logsQuery.isLoading ? <LoadingState /> : null}
        {logsQuery.isError ? <ErrorState message={errorMessage(logsQuery.error)} /> : null}

        {!logsQuery.isLoading && !logsQuery.isError ? (
          <LogFeed groups={groups} onDelete={requestDelete} />
        ) : null}

        <div ref={sentinelRef} className="h-2" aria-hidden="true" />

        {logsQuery.hasNextPage ? (
          <button
            type="button"
            className="mx-auto flex min-h-11 items-center justify-center gap-2 rounded-md border border-white/10 px-4 text-sm font-medium text-zinc-200 disabled:cursor-wait disabled:text-zinc-500"
            disabled={logsQuery.isFetchingNextPage}
            onClick={() => void logsQuery.fetchNextPage()}
          >
            {logsQuery.isFetchingNextPage ? (
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            ) : null}
            Load more
          </button>
        ) : null}
      </div>

      {undoNotice ? (
        <UndoToast title={undoNotice.title} onUndo={() => undoDelete(undoNotice.id)} />
      ) : null}

      {toast ? <Toast message={toast} onClose={() => setToast(null)} /> : null}
    </section>
  );
}

function WeeklyRecap({
  loading,
  from,
  counts,
}: {
  loading: boolean;
  from: string | null;
  counts: CategoryCount[];
}) {
  const text = recapText(counts);

  return (
    <section className="rounded-md border border-cyan-300/40 bg-cyan-300/10 p-4">
      <p className="text-xs font-semibold uppercase text-cyan-200">Weekly recap</p>
      <p className="mt-2 text-lg font-semibold text-white">
        {loading ? "Loading recap" : `${formatWeekOf(from)} - ${text}`}
      </p>
    </section>
  );
}

function CategoryChips({
  categories,
  selected,
  onChange,
}: {
  categories: Category[];
  selected: string[];
  onChange: (categories: string[]) => void;
}) {
  return (
    <section className="flex flex-wrap gap-2" aria-label="Category filters">
      {categories.map((category) => {
        const active = selected.includes(category.id);

        return (
          <button
            key={category.id}
            type="button"
            className={[
              "flex min-h-11 items-center gap-2 rounded-md border px-3 text-sm font-medium transition",
              active
                ? "border-emerald-300 bg-emerald-300 text-zinc-950"
                : "border-white/10 bg-white/[0.03] text-zinc-300",
            ].join(" ")}
            aria-pressed={active}
            onClick={() => onChange(toggleString(selected, category.id))}
          >
            <span aria-hidden="true">{category.icon}</span>
            {category.label}
          </button>
        );
      })}
    </section>
  );
}

function LogFeed({
  groups,
  onDelete,
}: {
  groups: Array<{ dateKey: string; entries: LogEntry[] }>;
  onDelete: (entry: LogEntry) => void;
}) {
  if (groups.length === 0) {
    return (
      <section className="rounded-md border border-white/10 bg-white/[0.03] p-5">
        <p className="text-sm text-zinc-300">No log entries match this filter.</p>
      </section>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      {groups.map((group) => (
        <section key={group.dateKey} className="rounded-md border border-white/10 bg-white/[0.03]">
          <div className="flex items-baseline justify-between gap-3 border-b border-white/10 px-4 py-3">
            <h2 className="text-lg font-semibold text-white">
              {formatLisbonDayHeading(group.dateKey)}
            </h2>
            <span className="text-xs font-medium text-zinc-500">{group.dateKey}</span>
          </div>

          <div className="divide-y divide-white/10">
            {group.entries.map((entry) => (
              <LogEntryRow key={entry.id} entry={entry} onDelete={() => onDelete(entry)} />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

function LogEntryRow({ entry, onDelete }: { entry: LogEntry; onDelete: () => void }) {
  const noteIsLink = entry.note?.startsWith("http") ?? false;

  return (
    <article className="grid grid-cols-[1fr_44px] gap-3 p-4">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-lg" aria-hidden="true">
            {entry.icon}
          </span>
          <h3 className="min-w-0 text-base font-semibold text-white">{entry.title}</h3>
          {entry.goal_code ? (
            <span className="rounded-sm bg-amber-300/15 px-2 py-1 text-xs font-medium text-amber-200">
              {entry.goal_code}
            </span>
          ) : null}
        </div>

        <p className="mt-1 text-xs font-medium text-zinc-500">
          {entry.category_label} · {formatLisbonTime(entry.occurred_at)}
        </p>

        {entry.note ? (
          noteIsLink ? (
            <a
              className="mt-2 inline-flex max-w-full items-center gap-1 break-all text-sm font-medium text-cyan-200 underline-offset-4 hover:underline"
              href={entry.note}
              target="_blank"
              rel="noreferrer"
            >
              {entry.note}
              <ExternalLink className="size-3.5 shrink-0" aria-hidden="true" />
            </a>
          ) : (
            <p className="mt-2 text-sm leading-6 text-zinc-300">{entry.note}</p>
          )
        ) : null}

        {entry.url ? (
          <a
            className="mt-2 inline-flex max-w-full items-center gap-1 break-all text-sm font-medium text-cyan-200 underline-offset-4 hover:underline"
            href={entry.url}
            target="_blank"
            rel="noreferrer"
          >
            {entry.url}
            <ExternalLink className="size-3.5 shrink-0" aria-hidden="true" />
          </a>
        ) : null}
      </div>

      <button
        type="button"
        className="grid size-11 place-items-center rounded-md border border-white/10 text-zinc-400 transition hover:border-red-300/50 hover:text-red-200"
        aria-label={`Delete ${entry.title}`}
        onClick={onDelete}
      >
        <Trash2 className="size-5" aria-hidden="true" />
      </button>
    </article>
  );
}

function UndoToast({ title, onUndo }: { title: string; onUndo: () => void }) {
  return (
    <div
      role="status"
      className="fixed inset-x-4 bottom-24 z-50 mx-auto flex max-w-md items-center justify-between gap-3 rounded-md border border-white/10 bg-zinc-900 px-4 py-3 text-sm text-white shadow-xl shadow-black/30 md:bottom-5"
    >
      <span className="min-w-0 truncate">Deleting {title}</span>
      <button
        type="button"
        className="inline-flex min-h-9 shrink-0 items-center gap-2 rounded-md bg-cyan-300 px-3 font-semibold text-zinc-950"
        onClick={onUndo}
      >
        <RotateCcw className="size-4" aria-hidden="true" />
        Undo
      </button>
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

function useCategoryFilter(): [string[], (categories: string[]) => void] {
  const [selected, setSelected] = useState<string[]>(() => parseCategoriesFromURL());

  useEffect(() => {
    const onPopState = () => setSelected(parseCategoriesFromURL());
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  const update = (next: string[]) => {
    setSelected(next);
    const params = new URLSearchParams(window.location.search);
    if (next.length > 0) {
      params.set("category", next.join(","));
    } else {
      params.delete("category");
    }
    const query = params.toString();
    window.history.replaceState(null, "", `${window.location.pathname}${query ? `?${query}` : ""}`);
  };

  return [selected, update];
}

function parseCategoriesFromURL(): string[] {
  const raw = new URLSearchParams(window.location.search).get("category");
  if (!raw) {
    return [];
  }
  return raw
    .split(",")
    .map((value) => value.trim())
    .filter(Boolean);
}

function groupEntriesByLisbonDay(
  entries: LogEntry[],
): Array<{ dateKey: string; entries: LogEntry[] }> {
  const groups = new Map<string, LogEntry[]>();
  for (const entry of entries) {
    const dateKey = lisbonDateKey(entry.occurred_at);
    groups.set(dateKey, [...(groups.get(dateKey) ?? []), entry]);
  }

  return Array.from(groups, ([dateKey, groupEntries]) => ({ dateKey, entries: groupEntries }));
}

function recapText(counts: CategoryCount[]): string {
  const activeCounts = counts.filter((count) => count.count > 0);
  if (activeCounts.length === 0) {
    return "No logs this week";
  }

  return activeCounts
    .map((count) => `${count.count} ${pluralize(recapCategoryLabel(count), count.count)}`)
    .join(" · ");
}

function recapCategoryLabel(count: CategoryCount): string {
  if (count.category_id === "module") {
    return "module";
  }
  if (count.category_id === "number") {
    return "benchmark";
  }
  if (count.category_id === "oss") {
    return "OSS comment";
  }
  return count.label.toLowerCase();
}

function pluralize(label: string, count: number): string {
  if (count === 1) {
    return label;
  }
  return label.endsWith("s") ? label : `${label}s`;
}

function toggleString(selected: string[], value: string): string[] {
  if (selected.includes(value)) {
    return selected.filter((existing) => existing !== value);
  }
  return [...selected, value];
}

function addSetValue<T>(current: Set<T>, value: T): Set<T> {
  const next = new Set(current);
  next.add(value);
  return next;
}

function removeSetValue<T>(current: Set<T>, value: T): Set<T> {
  const next = new Set(current);
  next.delete(value);
  return next;
}

function removeEntryFromPages(
  current: InfiniteData<LogFeedResponse>,
  id: number,
): InfiniteData<LogFeedResponse> {
  return {
    ...current,
    pages: current.pages.map((page) => ({
      ...page,
      entries: page.entries.filter((entry) => entry.id !== id),
    })),
  };
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Request failed";
}
