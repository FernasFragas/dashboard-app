import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../api/client";
import { TodayPage } from "./TodayPage";

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return {
    ...actual,
    createTask: vi.fn(),
    createLog: vi.fn(),
    getCategories: vi.fn(),
    getDashboard: vi.fn(),
    getGameProfile: vi.fn(),
    getGameSkills: vi.fn(),
    getGoalsBoard: vi.fn(),
    getProjects: vi.fn(),
    getSkills: vi.fn(),
    updateTaskStatus: vi.fn(),
    updateTaskSkills: vi.fn(),
  };
});

const baseTask: api.Task = {
  id: 41,
  week: "W5",
  title: "Fail-open on provider timeout",
  project: "gateway",
  goal_id: 3,
  steps: ["Add a 2s context deadline per provider call."],
  done_means: "a killed provider does not surface an error to the caller",
  status: "todo",
  done_at: null,
  skill_ids: [1],
  sort_order: 100,
  version: 1,
  created_at: "2026-09-24T17:40:00Z",
};

const baseDashboard: api.DashboardResponse = {
  today: "2026-09-24",
  week: {
    code: "W5",
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
  tasks: [baseTask],
  completion: {
    done: 0,
    total: 1,
  },
  counters: [{ category_id: "application", label: "Application", icon: "📮", count: 3 }],
  streak: {
    days: 12,
    counts_today: false,
  },
};

const categories: api.Category[] = [
  { id: "application", label: "Application", icon: "📮", sort_order: 1 },
  { id: "module", label: "Course module", icon: "📚", sort_order: 2 },
];

const projects: api.Project[] = [
  { id: "dash", label: "dash", sort_order: 10 },
  { id: "synapse", label: "synapse", sort_order: 20 },
  { id: "gateway", label: "gateway", sort_order: 30 },
];

const goals: api.Goal[] = [
  {
    id: 3,
    code: "G2",
    title: "Reliability proven",
    done_means: "RELIABILITY.md",
    project: "gateway",
    phase: "P1",
    status: "active",
    target: "W7",
    sort_order: 100,
    log_count: 1,
    version: 1,
    created_at: "2026-09-24T17:40:00Z",
    completed_at: null,
  },
];

describe("TodayPage", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", "/");
    vi.mocked(api.getDashboard).mockResolvedValue(clone(baseDashboard));
    vi.mocked(api.getCategories).mockResolvedValue(categories);
    vi.mocked(api.getProjects).mockResolvedValue({ projects });
    vi.mocked(api.getGameProfile).mockResolvedValue({
      total_xp: 40,
      level: 0,
      title: "Backend Engineer",
      current_threshold: 0,
      next_level: 1,
      next_threshold: 50,
      progress_xp: 40,
      progress_required: 50,
      streak: { days: 3, counts_today: true, shielded: false },
      badges: [],
    });
    vi.mocked(api.getGameSkills).mockResolvedValue({
      skills: [
        {
          skill_id: 1,
          code: "evals",
          name: "Evaluation systems",
          target_tier: "Expert",
          xp: 40,
          tier: "Novice",
          tier_min: 1,
          next_tier_xp: 60,
          next_tier: "Apprentice",
        },
      ],
    });
    vi.mocked(api.getSkills).mockResolvedValue({
      skills: [
        {
          id: 1,
          code: "evals",
          name: "Evaluation systems",
          description: "d",
          associate_when: "the task creates, runs, or gates on an evaluation.",
          target_tier: "Expert",
          sort_order: 10,
          task_count: 1,
          task_done: 0,
          evidence_count: 0,
          last_activity: null,
        },
        {
          id: 2,
          code: "chaos",
          name: "Load & chaos testing",
          description: "d",
          associate_when: "the task generates load, or breaks something on purpose.",
          target_tier: "Expert",
          sort_order: 20,
          task_count: 0,
          task_done: 0,
          evidence_count: 0,
          last_activity: null,
        },
      ],
      unlinked_tasks: [],
    });
    vi.mocked(api.getGoalsBoard).mockResolvedValue({
      goals: goals,
      active_count: 0,
      active_limit: 3,
    });
    vi.mocked(api.createLog).mockResolvedValue({
      id: 1,
      category_id: "application",
      category_label: "Application",
      icon: "📮",
      title: "Applied",
      note: null,
      url: null,
      goal_id: null,
      goal_code: null,
      skill_ids: [],
      occurred_at: "2026-09-24T17:40:00Z",
      created_at: "2026-09-24T17:40:00Z",
    });
    vi.mocked(api.updateTaskSkills).mockImplementation(async (task, skillIDs) => ({
      ...task,
      skill_ids: skillIDs,
      version: task.version + 1,
    }));
    vi.mocked(api.createTask).mockImplementation(async (input) => ({
      id: 99,
      week: input.week,
      title: input.title,
      project: input.project,
      goal_id: input.goal_id ?? null,
      steps: input.steps ?? [],
      done_means: input.done_means ?? null,
      status: "todo",
      done_at: null,
      skill_ids: input.skill_ids,
      sort_order: 200,
      version: 1,
      created_at: "2026-09-24T18:00:00Z",
    }));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders an optimistic checked state immediately and rolls back on a 500", async () => {
    const user = userEvent.setup();
    let rejectPatch: (error: unknown) => void = () => {};
    vi.mocked(api.updateTaskStatus).mockReturnValue(
      new Promise((_resolve, reject) => {
        rejectPatch = reject;
      }),
    );

    renderToday();

    const checkbox = await screen.findByRole("checkbox", {
      name: "Mark done: Fail-open on provider timeout",
    });

    await user.click(checkbox);
    expect(checkbox).toHaveAttribute("aria-checked", "true");

    rejectPatch(new api.ApiError("server exploded", 500));

    await screen.findByText("server exploded");
    expect(checkbox).toHaveAttribute("aria-checked", "false");
  });

  it("handles a 412 by rendering server truth and showing a refresh toast", async () => {
    const user = userEvent.setup();
    let rejectPatch: (error: unknown) => void = () => {};
    vi.mocked(api.updateTaskStatus).mockReturnValue(
      new Promise((_resolve, reject) => {
        rejectPatch = reject;
      }),
    );

    renderToday();

    const checkbox = await screen.findByRole("checkbox", {
      name: "Mark done: Fail-open on provider timeout",
    });

    await user.click(checkbox);
    expect(checkbox).toHaveAttribute("aria-checked", "true");

    rejectPatch(
      new api.ApiError("task was modified elsewhere", 412, {
        ...baseTask,
        version: 7,
        status: "todo",
        done_at: null,
      }),
    );

    await screen.findByText("Updated elsewhere · refreshed");
    expect(checkbox).toHaveAttribute("aria-checked", "false");
  });

  it("applies project filters from the URL and writes multi-select changes back", async () => {
    const user = userEvent.setup();
    window.history.replaceState(null, "", "/?project=synapse,gateway");

    renderToday();

    await screen.findByText("Fail-open on provider timeout");
    expect(vi.mocked(api.getDashboard)).toHaveBeenCalledWith(["synapse", "gateway"]);

    const synapse = screen.getByRole("button", { name: "synapse" });
    const gateway = screen.getByRole("button", { name: "gateway" });
    const dash = screen.getByRole("button", { name: "dash" });

    expect(synapse).toHaveAttribute("aria-pressed", "true");
    expect(gateway).toHaveAttribute("aria-pressed", "true");
    expect(dash).toHaveAttribute("aria-pressed", "false");

    await user.click(dash);

    expect(new URLSearchParams(window.location.search).get("project")).toBe("synapse,gateway,dash");
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "dash" })).toHaveAttribute("aria-pressed", "true"),
    );
  });

  it("keeps save disabled for a blank quick log and closes after a successful save", async () => {
    const user = userEvent.setup();

    renderToday();

    await user.click(await screen.findByRole("button", { name: /Application/ }));

    const save = screen.getByRole("button", { name: /Save/ });
    expect(save).toBeDisabled();

    await user.type(screen.getByLabelText("Title"), "Applied - Platform Engineer @ Acme");
    expect(save).toBeEnabled();

    await user.click(save);

    await waitFor(() => {
      expect(vi.mocked(api.createLog).mock.calls[0]?.[0]).toEqual(
        expect.objectContaining({
          category_id: "application",
          title: "Applied - Platform Engineer @ Acme",
        }),
      );
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("edits task skills from the task detail sheet", async () => {
    const user = userEvent.setup();

    renderToday();

    await user.click(await screen.findByRole("button", { name: /Fail-open on provider timeout/ }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("checkbox", { name: /Evaluation systems/ })).toBeChecked();
    expect(
      within(dialog).getByText("the task generates load, or breaks something on purpose."),
    ).toBeInTheDocument();

    await user.click(within(dialog).getByRole("checkbox", { name: /Load & chaos testing/ }));
    await user.click(within(dialog).getByRole("button", { name: "Save skills" }));

    await waitFor(() =>
      expect(vi.mocked(api.updateTaskSkills)).toHaveBeenCalledWith(
        expect.objectContaining({ id: 41 }),
        [1, 2],
      ),
    );
    expect(await screen.findByText("Skills saved")).toBeInTheDocument();
  });

  it("requires a skill when adding an ad-hoc task", async () => {
    const user = userEvent.setup();

    renderToday();

    await user.click(await screen.findByRole("button", { name: "Add task" }));

    const dialog = screen.getByRole("dialog");
    const save = within(dialog).getByRole("button", { name: "Add task" });
    expect(save).toBeDisabled();
    expect(
      within(dialog).getByText(
        "Every task builds at least one skill — pick what this work trains.",
      ),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("Pick at least one skill before saving.")).toBeInTheDocument();

    await user.type(within(dialog).getByLabelText("Title"), "Capture p99 from the mixed profile");
    expect(save).toBeDisabled();

    await user.click(within(dialog).getByRole("checkbox", { name: /Load & chaos testing/ }));
    expect(save).toBeEnabled();

    await user.click(save);

    await waitFor(() =>
      expect(vi.mocked(api.createTask)).toHaveBeenCalledWith(
        expect.objectContaining({
          week: "W5",
          title: "Capture p99 from the mixed profile",
          project: "dash",
          skill_ids: [2],
        }),
      ),
    );
    expect(await screen.findByText("Task added")).toBeInTheDocument();
  });

  it("attaches optional skill evidence to a quick log", async () => {
    const user = userEvent.setup();

    renderToday();

    await user.click(await screen.findByRole("button", { name: /Application/ }));

    const dialog = screen.getByRole("dialog");
    await user.type(within(dialog).getByLabelText("Title"), "Applied - Platform Engineer @ Acme");
    await user.click(within(dialog).getByRole("checkbox", { name: /Evaluation systems/ }));
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(vi.mocked(api.createLog).mock.calls[0]?.[0]).toEqual(
        expect.objectContaining({
          category_id: "application",
          title: "Applied - Platform Engineer @ Acme",
          skill_ids: [1],
        }),
      ),
    );
  });

  // The banner used to render nothing before the plan started, leaving a blank panel above a
  // task list with no hint of why the week was missing.
  it("explains the wait when the plan has not started yet", async () => {
    vi.mocked(api.getDashboard).mockResolvedValue({
      ...baseDashboard,
      today: "2026-08-22",
      week: {
        code: null,
        phase: null,
        focus: null,
        start_date: null,
        end_date: null,
        state: "not_started",
      },
      task_week: {
        code: "W1",
        start_date: "2026-08-24",
        end_date: "2026-08-30",
        focus: "Golden set",
      },
    });

    renderToday();

    expect(await screen.findByText("Not started yet")).toBeInTheDocument();
    expect(screen.getByText("W1 · Golden set")).toBeInTheDocument();

    // 22nd to 24th is two days, and the date is named so it does not need counting.
    expect(screen.getByText(/in 2 days/)).toBeInTheDocument();
    expect(screen.getByText(/Mon, Aug 24/)).toBeInTheDocument();

    // And it says why there is a task list at all.
    expect(screen.getByText(/first week's tasks/)).toBeInTheDocument();
  });

  // Regression: before the plan starts there is no active week, but the screen still shows the
  // first week's tasks. Gating "Add task" on the active week left it greyed out with no reason
  // given — the sheet could never open at all.
  it("allows adding a task before the plan has started", async () => {
    const user = userEvent.setup();

    vi.mocked(api.getDashboard).mockResolvedValue({
      ...baseDashboard,
      today: "2026-08-22",
      week: {
        code: null,
        phase: null,
        focus: null,
        start_date: null,
        end_date: null,
        state: "not_started",
      },
      task_week: {
        code: "W1",
        start_date: "2026-08-24",
        end_date: "2026-08-30",
        focus: "Golden set",
      },
    });

    renderToday();

    const addTask = await screen.findByRole("button", { name: /Add task/ });
    expect(addTask).toBeEnabled();

    await user.click(addTask);

    // The sheet opens against the week the task list actually came from.
    expect(await screen.findByText("W1")).toBeInTheDocument();
  });
});

function renderToday() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <TodayPage />
    </QueryClientProvider>,
  );
}

function clone<T>(value: T): T {
  return structuredClone(value);
}
