import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../api/client";
import { SkillsPage } from "./SkillsPage";

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return { ...actual, getGameSkills: vi.fn(), getSkills: vi.fn(), getSkill: vi.fn() };
});

function skill(overrides: Partial<api.Skill> & Pick<api.Skill, "id">): api.Skill {
  return {
    code: `skill-${overrides.id}`,
    name: `Skill ${overrides.id}`,
    description: "What it represents.",
    associate_when: "the task does the thing.",
    target_tier: null,
    sort_order: overrides.id * 10,
    task_count: 0,
    task_done: 0,
    evidence_count: 0,
    last_activity: null,
    ...overrides,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });

  return render(
    <QueryClientProvider client={client}>
      <SkillsPage />
    </QueryClientProvider>,
  );
}

const mockedGetSkills = vi.mocked(api.getSkills);
const mockedGetSkill = vi.mocked(api.getSkill);
const mockedGetGameSkills = vi.mocked(api.getGameSkills);

beforeEach(() => {
  vi.clearAllMocks();
  mockedGetGameSkills.mockResolvedValue({ skills: [] });
});

describe("SkillsPage", () => {
  it("shows derived done/total per skill", async () => {
    mockedGetSkills.mockResolvedValue({
      skills: [
        skill({
          id: 1,
          name: "Evaluation systems",
          task_count: 4,
          task_done: 3,
          target_tier: "Expert",
        }),
      ],
      unlinked_tasks: [],
    });
    mockedGetGameSkills.mockResolvedValue({
      skills: [
        {
          skill_id: 1,
          code: "skill-1",
          name: "Evaluation systems",
          target_tier: "Expert",
          xp: 340,
          tier: "Adept",
          tier_min: 300,
          next_tier_xp: 500,
          next_tier: "Expert",
        },
      ],
    });

    renderPage();

    expect(await screen.findByText("Evaluation systems")).toBeInTheDocument();
    expect(screen.getByText("3 of 4 tasks done")).toBeInTheDocument();
    expect(screen.getByText("Adept 340/500")).toBeInTheDocument();
    expect(screen.getByText("Expert")).toBeInTheDocument();

    // The bar reports the same numbers it renders, so assistive tech agrees with the label.
    const bar = screen.getByRole("progressbar", { name: "Evaluation systems task completion" });
    expect(bar).toHaveAttribute("aria-valuenow", "3");
    expect(bar).toHaveAttribute("aria-valuemax", "4");
  });

  it("says so when a skill has nothing linked, rather than showing a blank card", async () => {
    mockedGetSkills.mockResolvedValue({ skills: [skill({ id: 2 })], unlinked_tasks: [] });

    renderPage();

    expect(await screen.findByText("No tasks linked yet")).toBeInTheDocument();
    expect(screen.getByText("Not started")).toBeInTheDocument();
  });

  // Tasks predating M8 have no skill. The tab surfaces them instead of implying the invariant
  // already holds everywhere.
  it("flags tasks that still need a skill", async () => {
    mockedGetSkills.mockResolvedValue({ skills: [skill({ id: 1 })], unlinked_tasks: [7, 9] });

    renderPage();

    expect(await screen.findByRole("status")).toHaveTextContent("2 tasks needs skill");
  });

  it("opens the detail sheet with the associate-when line, tasks by week and evidence", async () => {
    const user = userEvent.setup();

    mockedGetSkills.mockResolvedValue({
      skills: [skill({ id: 5, name: "LLM serving", task_count: 2, task_done: 1 })],
      unlinked_tasks: [],
    });

    mockedGetSkill.mockResolvedValue({
      ...skill({ id: 5, name: "LLM serving", task_count: 2, task_done: 1 }),
      description: "vLLM, batching, token streaming.",
      associate_when: "the task runs, routes to, or measures a model server.",
      tasks: [
        {
          id: 1,
          week: "W8",
          title: "batching + token streaming",
          project: "gateway",
          goal_id: null,
          steps: [],
          done_means: null,
          status: "done",
          done_at: "2026-10-15T10:00:00Z",
          skill_ids: [5],
          sort_order: 100,
          version: 2,
          created_at: "2026-08-24T09:00:00Z",
        },
      ],
      evidence: [
        {
          id: 3,
          category_id: "number",
          category_label: "Benchmark/number",
          icon: "📊",
          title: "bake-off: 42 tok/s",
          note: null,
          url: null,
          goal_id: null,
          goal_code: null,
          occurred_at: "2026-10-16T09:00:00Z",
          created_at: "2026-10-16T09:00:00Z",
        },
      ],
    });

    renderPage();

    await user.click(await screen.findByRole("button", { name: /LLM serving/ }));

    // The affordance for correct tagging leads the sheet.
    expect(await screen.findByText("Tag a task with this when…")).toBeInTheDocument();
    expect(
      screen.getByText("the task runs, routes to, or measures a model server."),
    ).toBeInTheDocument();

    expect(screen.getByText("W8")).toBeInTheDocument();
    expect(screen.getByText("batching + token streaming")).toBeInTheDocument();
    expect(screen.getByText(/bake-off: 42 tok\/s/)).toBeInTheDocument();

    await waitFor(() => expect(mockedGetSkill).toHaveBeenCalledWith(5));
  });

  it("reports an empty catalogue rather than rendering nothing", async () => {
    mockedGetSkills.mockResolvedValue({ skills: [], unlinked_tasks: [] });

    renderPage();

    expect(await screen.findByText(/No skills yet/)).toBeInTheDocument();
  });
});
