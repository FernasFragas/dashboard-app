import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../api/client";
import { LogPage } from "./LogPage";

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return {
    ...actual,
    deleteLog: vi.fn(),
    getCategories: vi.fn(),
    getLogs: vi.fn(),
    getLogSummary: vi.fn(),
  };
});

const categories: api.Category[] = [
  { id: "application", label: "Application", icon: "📮", sort_order: 1 },
  { id: "module", label: "Course module", icon: "📚", sort_order: 2 },
  { id: "number", label: "Benchmark/number", icon: "📊", sort_order: 3 },
];

describe("LogPage", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", "/log");
    vi.mocked(api.getCategories).mockResolvedValue(categories);
    vi.mocked(api.getLogSummary).mockResolvedValue({
      range: "week",
      from: "2026-09-21",
      to: "2026-09-27",
      counts: [
        { category_id: "application", label: "Application", icon: "📮", count: 2 },
        { category_id: "module", label: "Course module", icon: "📚", count: 4 },
        { category_id: "number", label: "Benchmark/number", icon: "📊", count: 1 },
      ],
      total: 7,
    });
    vi.mocked(api.getLogs).mockResolvedValue({ entries: [], next_cursor: null });
    vi.mocked(api.deleteLog).mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("groups 23:30Z and 00:30Z entries by Lisbon day", async () => {
    vi.mocked(api.getLogs).mockResolvedValue({
      entries: [
        logEntry({
          id: 1,
          title: "Late Lisbon entry",
          occurred_at: "2026-09-24T23:30:00Z",
        }),
        logEntry({
          id: 2,
          title: "Early Lisbon entry",
          occurred_at: "2026-09-24T00:30:00Z",
        }),
      ],
      next_cursor: null,
    });

    renderLog();

    const sep25 = await screen.findByText("2026-09-25");
    const sep24 = await screen.findByText("2026-09-24");

    expect(within(sep25.closest("section")!).getByText("Late Lisbon entry")).toBeInTheDocument();
    expect(within(sep24.closest("section")!).getByText("Early Lisbon entry")).toBeInTheDocument();
  });

  it("does not delete when undone and deletes exactly once after the undo window lapses", async () => {
    vi.mocked(api.getLogs).mockResolvedValue({
      entries: [logEntry({ id: 9, title: "Keep or delete" })],
      next_cursor: null,
    });

    renderLog();

    await screen.findByText("Keep or delete");

    vi.useFakeTimers();
    fireEvent.click(screen.getByRole("button", { name: "Delete Keep or delete" }));
    expect(screen.queryByText("Keep or delete")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    await act(async () => {
      vi.advanceTimersByTime(3_500);
      await Promise.resolve();
    });

    expect(api.deleteLog).not.toHaveBeenCalled();
    expect(screen.getByText("Keep or delete")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Delete Keep or delete" }));
    await act(async () => {
      vi.advanceTimersByTime(3_500);
      await Promise.resolve();
    });

    expect(api.deleteLog).toHaveBeenCalledTimes(1);
    expect(vi.mocked(api.deleteLog).mock.calls[0]?.[0]).toBe(9);
  });
});

function renderLog() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <LogPage />
    </QueryClientProvider>,
  );
}

function logEntry(overrides: Partial<api.LogEntry>): api.LogEntry {
  return {
    id: 1,
    category_id: "application",
    category_label: "Application",
    icon: "📮",
    title: "Logged item",
    note: null,
    url: null,
    goal_id: 3,
    goal_code: "G2",
    occurred_at: "2026-09-24T17:40:00Z",
    created_at: "2026-09-24T17:40:02Z",
    ...overrides,
  };
}
