import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../api/client";
import { ReviewPage } from "./ReviewPage";

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return {
    ...actual,
    createMetric: vi.fn(),
    getCheckpoint: vi.fn(),
    getDashboard: vi.fn(),
    getMetricDefs: vi.fn(),
    getMetrics: vi.fn(),
    getReviews: vi.fn(),
    saveCheckpoint: vi.fn(),
    upsertReview: vi.fn(),
  };
});

const metricDefs: api.MetricDef[] = [
  {
    name: "p95 latency (cached)",
    slug: "p95_latency",
    unit: "s",
    baseline: ">1s",
    target: "<0.8s",
    definition: "95th-percentile request latency under the standard mixed profile",
    how_to_measure: "Grafana after a 10-min k6 run",
    sort_order: 1,
  },
  {
    name: "Cache hit rate",
    slug: "cache_hit_rate",
    unit: "%",
    baseline: "<20%",
    target: ">60%",
    definition: "hits / (hits + misses)",
    how_to_measure: "Gateway cache metric over a 10-min replayed-traffic window",
    sort_order: 2,
  },
];

const checkpointHelpers = [
  "Good = the exact sentence you'd say out loud, with real X and Y.",
  "Name the belief that died.",
  "Status + date of last maintainer contact + your next move.",
  "Yes/no. If yes: which one gets archived this week.",
  "Gut check, one paragraph max.",
];

describe("ReviewPage", () => {
  let metricID = 0;

  beforeEach(() => {
    metricID = 0;
    window.history.replaceState(null, "", "/review");
    vi.mocked(api.getDashboard).mockResolvedValue(dashboard("W5"));
    vi.mocked(api.getReviews).mockResolvedValue([]);
    vi.mocked(api.getMetricDefs).mockResolvedValue(metricDefs);
    vi.mocked(api.getMetrics).mockResolvedValue([]);
    vi.mocked(api.getCheckpoint).mockResolvedValue({
      week: "W12",
      questions: [
        "interview-grade eval number?",
        "chaos falsified anything?",
        "PR merged or stale?",
        "fourth repo?",
        "still AI-infra path?",
      ],
      helpers: checkpointHelpers,
      answers: null,
      completed_at: null,
    });
    vi.mocked(api.upsertReview).mockImplementation(async (input) => reviewFromInput(input));
    vi.mocked(api.createMetric).mockImplementation(async (input) => ({
      id: ++metricID,
      name: input.name,
      value: input.value,
      unit: input.unit ?? null,
      note: input.note ?? null,
      recorded_at: "2026-09-24T20:10:00Z",
    }));
    vi.mocked(api.saveCheckpoint).mockImplementation(async (week, answers) => ({
      week,
      questions: [
        "interview-grade eval number?",
        "chaos falsified anything?",
        "PR merged or stale?",
        "fourth repo?",
        "still AI-infra path?",
      ],
      helpers: checkpointHelpers,
      answers,
      completed_at: "2026-11-15T20:10:00Z",
    }));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("upserts a second save for the same date without adding another reviewed-date marker", async () => {
    const user = userEvent.setup();

    renderReview();

    const dateInput = (await screen.findByLabelText("Review date")) as HTMLInputElement;
    const selectedDate = dateInput.value;
    const learned = screen.getByLabelText("Learned today");

    await user.type(learned, "first pass");
    await user.click(screen.getByRole("button", { name: "Save review" }));

    await waitFor(() => expect(api.upsertReview).toHaveBeenCalledTimes(1));

    await user.clear(learned);
    await user.type(learned, "second pass");
    await user.click(screen.getByRole("button", { name: "Save review" }));

    await waitFor(() => expect(api.upsertReview).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.upsertReview).mock.calls.map(([input]) => input.date)).toEqual([
      selectedDate,
      selectedDate,
    ]);
    expect(screen.getAllByRole("button", { name: `Existing review ${selectedDate}` })).toHaveLength(
      1,
    );
  });

  it("writes a backfilled review to the picked earlier date", async () => {
    const user = userEvent.setup();

    renderReview();

    const dateInput = (await screen.findByLabelText("Review date")) as HTMLInputElement;
    const earlierDate = previousDay(dateInput.value);

    fireEvent.change(dateInput, { target: { value: earlierDate } });
    await user.type(screen.getByLabelText("Blocker / issue"), "Sunday ritual happened Monday");
    await user.click(screen.getByRole("button", { name: "Save review" }));

    await waitFor(() => expect(api.upsertReview).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.upsertReview).mock.calls[0]?.[0]).toEqual(
      expect.objectContaining({ date: earlierDate }),
    );
  });

  it("submits seeded metric names exactly and accepts the other free-text fallback", async () => {
    const user = userEvent.setup();

    renderReview();

    await screen.findByText(/target <0.8s/);
    expect(screen.getByText(/95th-percentile request latency/)).toBeInTheDocument();
    expect(screen.getByText(/Grafana after a 10-min k6 run/)).toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText("Metric"), "p95 latency (cached)");
    await user.type(screen.getByLabelText("Value"), "0.72");
    await user.click(screen.getByRole("button", { name: "Save metric" }));

    await waitFor(() => expect(api.createMetric).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.createMetric).mock.calls[0]?.[0]).toEqual(
      expect.objectContaining({ name: "p95 latency (cached)", unit: "s" }),
    );

    expect(screen.getByLabelText("Unit")).toHaveAttribute("readonly");

    await user.selectOptions(screen.getByLabelText("Metric"), "__other__");
    await user.type(await screen.findByLabelText("Metric name"), "Novel score");
    expect(screen.getByLabelText("Unit")).not.toHaveAttribute("readonly");
    await user.type(screen.getByLabelText("Value"), "7");
    await user.click(screen.getByRole("button", { name: "Save metric" }));

    await waitFor(() => expect(api.createMetric).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.createMetric).mock.calls[1]?.[0]).toEqual(
      expect.objectContaining({ name: "Novel score" }),
    );
  });

  it("renders the checkpoint form on W12", async () => {
    vi.mocked(api.getDashboard).mockResolvedValue(dashboard("W12"));

    renderReview();

    expect(await screen.findByText("interview-grade eval number?")).toBeInTheDocument();
    expect(
      screen.getByText("Good = the exact sentence you'd say out loud, with real X and Y."),
    ).toBeInTheDocument();
    expect(screen.getByText("W12 Boss review")).toBeInTheDocument();
  });

  it("keeps the checkpoint form absent on W5", async () => {
    vi.mocked(api.getDashboard).mockResolvedValue(dashboard("W5"));

    renderReview();

    await screen.findByText("Daily closeout");

    expect(api.getCheckpoint).not.toHaveBeenCalled();
    expect(screen.queryByText(/checkpoint/i)).not.toBeInTheDocument();
  });

  it("renders daily helpers, placeholders, and the empty example", async () => {
    renderReview();

    expect(await screen.findByText(/Three bullets. Two minutes/)).toBeInTheDocument();
    expect(screen.getByText(/One thing you understand now/)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/pgxpool MaxConns/)).toBeInTheDocument();
    expect(await screen.findByText("Example")).toBeInTheDocument();
    expect(screen.getByText(/ValidateAgainstEvidence rejects/)).toBeInTheDocument();
  });

  it("hides the empty example once any review exists", async () => {
    vi.mocked(api.getReviews).mockResolvedValue([
      {
        date: "2026-09-23",
        learned: "one thing",
        issue: null,
        next: null,
        minutes: null,
        created_at: "2026-09-23T20:10:00Z",
        updated_at: "2026-09-23T20:10:00Z",
      },
    ]);

    renderReview();

    await screen.findByRole("button", { name: "Existing review 2026-09-23" });
    expect(screen.queryByText("Example")).not.toBeInTheDocument();
  });

  it("toggles field guidance with aria-expanded", async () => {
    const user = userEvent.setup();

    renderReview();

    const guide = await screen.findByRole("button", { name: "Daily review guidance" });
    expect(guide).toHaveAttribute("aria-expanded", "false");

    await user.click(guide);

    expect(guide).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText(/Good entries are specific enough/)).toBeInTheDocument();
  });
});

function renderReview() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <ReviewPage />
    </QueryClientProvider>,
  );
}

function dashboard(weekCode: string): api.DashboardResponse {
  return {
    today: "2026-09-24",
    week: {
      code: weekCode,
      phase: "P1",
      focus: "Chaos: fail-open / fail-static",
      start_date: "2026-09-21",
      end_date: "2026-09-27",
      state: "active",
    },
    task_week: { code: "W5", start_date: "2026-09-21", end_date: "2026-09-27", focus: "Chaos" },
    rhythm: {
      label: "Thu",
      slot: "SAA prep + 30 min community",
    },
    tasks: [],
    completion: {
      done: 3,
      total: 5,
    },
    counters: [],
    streak: {
      days: 12,
      counts_today: true,
    },
  };
}

function reviewFromInput(input: api.UpsertReviewInput): api.DailyReview {
  return {
    date: input.date ?? "2026-09-24",
    learned: input.learned ?? null,
    issue: input.issue ?? null,
    next: input.next ?? null,
    minutes: input.minutes ?? null,
    created_at: "2026-09-24T20:10:00Z",
    updated_at: "2026-09-24T20:10:00Z",
  };
}

function previousDay(dateKey: string): string {
  const date = new Date(`${dateKey}T12:00:00Z`);
  date.setUTCDate(date.getUTCDate() - 1);
  return date.toISOString().slice(0, 10);
}
