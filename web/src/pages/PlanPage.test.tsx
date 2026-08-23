import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../api/client";
import { PlanPage } from "./PlanPage";

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return {
    ...actual,
    applyPlan: vi.fn(),
    createPair: vi.fn(),
    getPlan: vi.fn(),
    getPairSVG: vi.fn(),
    previewPlan: vi.fn(),
  };
});

const counts: api.PlanCounts = {
  weeks: 1,
  goals: 1,
  tasks: 1,
  skills: 0,
  projects: 1,
};

beforeEach(() => {
  vi.clearAllMocks();
  window.history.pushState(null, "", "/");
  vi.mocked(api.getPlan).mockResolvedValue({
    plan: {
      id: "master-plan-v5",
      name: "Master Plan v5",
      loaded_at: "2026-09-24T17:40:00Z",
    },
    source: null,
    counts,
  });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("PlanPage", () => {
  it("renders preview errors before any diff or apply controls", async () => {
    const user = userEvent.setup();
    vi.mocked(api.previewPlan).mockResolvedValue({
      errors: [{ line: 10, message: "line 10: unrecognised line", excerpt: "bad line" }],
    });

    renderPlanPage();

    await user.type(await screen.findByLabelText("Markdown source"), "# Broken");
    await user.click(screen.getByRole("button", { name: "Preview" }));

    expect(await screen.findByText("Fix these lines first")).toBeInTheDocument();
    expect(screen.getByText("Line 10")).toBeInTheDocument();
    expect(screen.getByText("bad line")).toBeInTheDocument();
    expect(screen.queryByText("Ready to apply")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Type new plan name")).not.toBeInTheDocument();
  });

  it("keeps replace apply disabled until the new plan name is typed", async () => {
    const user = userEvent.setup();
    vi.mocked(api.previewPlan).mockResolvedValue(cleanReplacePreview());
    vi.mocked(api.applyPlan).mockResolvedValue({
      plan: { id: "uploaded-plan", name: "Uploaded Plan", loaded_at: "2026-09-24T18:00:00Z" },
      mode: "replace",
      counts,
      seed: {},
      source: {
        id: 1,
        plan_id: "uploaded-plan",
        sha256: "abc123",
        source: "# Uploaded",
        loaded_at: "2026-09-24T18:00:00Z",
      },
      backup_path: "/tmp/backups/pre-plan-uploaded-plan.db",
    });

    renderPlanPage();

    await user.type(await screen.findByLabelText("Markdown source"), "# Uploaded");
    await user.click(screen.getByRole("button", { name: "Preview" }));

    const apply = await screen.findByRole("button", {
      name: "Replace Master Plan v5 with Uploaded Plan",
    });
    expect(apply).toBeDisabled();

    await user.type(screen.getByLabelText("Type new plan name"), "Uploaded Plan");
    expect(apply).toBeEnabled();
    await user.click(apply);

    await waitFor(() =>
      expect(api.applyPlan).toHaveBeenCalledWith({
        source: "# Uploaded",
        source_sha256: "abc123",
        confirm_plan_name: "Uploaded Plan",
      }),
    );
    expect(await screen.findByText("Plan applied")).toBeInTheDocument();
  });

  it("mints a phone pairing QR for the current route and warns on localhost", async () => {
    const user = userEvent.setup();
    const expiresAt = new Date(Date.now() + 90_000).toISOString();
    vi.mocked(api.createPair).mockResolvedValue({
      url: "http://dash.test:8484/pair/abc",
      svg_url: "/api/pair/abc.svg",
      expires_at: expiresAt,
      expires_in_seconds: 90,
    });
    vi.mocked(api.getPairSVG).mockResolvedValue('<svg><path d="M0 0h1v1h-1z"/></svg>');
    window.history.pushState(null, "", "/goals?project=synapse");

    renderPlanPage();

    expect(
      await screen.findByText(
        "You're on localhost - open the dashboard on its tailnet address before scanning.",
      ),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Pair phone" }));

    await waitFor(() => expect(api.createPair).toHaveBeenCalledWith("/goals?project=synapse"));
    expect(api.getPairSVG).toHaveBeenCalledWith("/api/pair/abc.svg");
    expect(await screen.findByAltText("Pair phone QR")).toBeInTheDocument();
    expect(screen.getByText("http://dash.test:8484/pair/abc")).toBeInTheDocument();
    expect(screen.getByText(/^Expires in 1:/)).toBeInTheDocument();
  });

  it("marks an expired pairing QR as dead", async () => {
    const user = userEvent.setup();
    vi.mocked(api.createPair).mockResolvedValue({
      url: "http://dash.test:8484/pair/expired",
      svg_url: "/api/pair/expired.svg",
      expires_at: new Date(Date.now() - 1_000).toISOString(),
      expires_in_seconds: 90,
    });
    vi.mocked(api.getPairSVG).mockResolvedValue('<svg><path d="M0 0h1v1h-1z"/></svg>');

    renderPlanPage();
    await user.click(await screen.findByRole("button", { name: "Pair phone" }));

    expect(await screen.findByText("Expired")).toBeInTheDocument();
    expect(screen.getByAltText("Pair phone QR").parentElement).toHaveClass("opacity-35");
    expect(screen.getByRole("button", { name: "Show a new code" })).toBeInTheDocument();
  });
});

function renderPlanPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });

  return render(
    <QueryClientProvider client={client}>
      <PlanPage />
    </QueryClientProvider>,
  );
}

function cleanReplacePreview(): api.PlanPreviewResponse {
  return {
    source_sha256: "abc123",
    plan: { id: "uploaded-plan", name: "Uploaded Plan" },
    current_plan: { id: "master-plan-v5", name: "Master Plan v5" },
    mode: "replace",
    parsed: counts,
    changes: {
      weeks: { added: 1, removed: 19 },
      tasks: { added: 1, orphaned: 66 },
      goals: { removed: 18 },
    },
    at_risk: {
      task_completions: 12,
      goal_positions: 4,
      checkpoint_answers: 1,
      log_goal_links: 2,
      log_skill_links: 3,
      retired_categories: 1,
    },
    errors: [],
  };
}
