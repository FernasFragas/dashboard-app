import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../api/client";
import { GoalsPage } from "./GoalsPage";

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return {
    ...actual,
    getGoalsBoard: vi.fn(),
    moveGoal: vi.fn(),
    deleteGoal: vi.fn(),
  };
});

function goal(overrides: Partial<api.Goal> & Pick<api.Goal, "id">): api.Goal {
  return {
    code: `G${overrides.id}`,
    title: `Goal ${overrides.id}`,
    done_means: null,
    project: "dash",
    phase: "P1",
    status: "active",
    target: null,
    sort_order: overrides.id * 100,
    log_count: 0,
    version: 1,
    created_at: "2026-08-24T09:00:00Z",
    completed_at: null,
    ...overrides,
  };
}

function board(goals: api.Goal[], activeLimit = 3): api.GoalsResponse {
  return {
    goals,
    active_count: goals.filter((g) => g.status === "active").length,
    active_limit: activeLimit,
  };
}

function serveBoard(initial: api.Goal[], activeLimit = 3) {
  let state = initial;
  mockedGetBoard.mockImplementation(async () => board(state, activeLimit));

  return {
    settlesOn(next: api.Goal[]) {
      state = next;
    },
  };
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });

  return render(
    <QueryClientProvider client={client}>
      <GoalsPage />
    </QueryClientProvider>,
  );
}

/**
 * The cards inside one column, in rendered order. A column contains nothing but card buttons,
 * so the role query is the order the board actually renders.
 */
function cardTitles(columnLabel: string): string[] {
  const column = screen.getByRole("region", { name: columnLabel });
  return within(column)
    .queryAllByRole("button")
    .map((button) => button.textContent ?? "");
}

const mockedGetBoard = vi.mocked(api.getGoalsBoard);
const mockedMoveGoal = vi.mocked(api.moveGoal);
const mockedDeleteGoal = vi.mocked(api.deleteGoal);

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("GoalsPage", () => {
  it("groups seeded goals into the three columns", async () => {
    mockedGetBoard.mockResolvedValue(
      board([
        goal({ id: 1, status: "backlog", title: "Dashboard live" }),
        goal({ id: 2, status: "active", title: "Eval harness" }),
        goal({ id: 3, status: "done", title: "OSS presence" }),
      ]),
    );

    renderPage();

    expect(await screen.findByText(/Dashboard live/)).toBeInTheDocument();
    expect(cardTitles("Backlog")).toHaveLength(1);
    expect(cardTitles("Active")).toHaveLength(1);
    expect(cardTitles("Done")).toHaveLength(1);
  });

  it("shows the card face: code, project, target and the linked-log count", async () => {
    mockedGetBoard.mockResolvedValue(
      board([
        goal({
          id: 7,
          status: "active",
          code: "G7",
          title: "OSS presence",
          project: "oss",
          target: "W9 / P2",
          log_count: 4,
        }),
      ]),
    );

    renderPage();

    const card = await screen.findByRole("button", { name: /G7 · OSS presence/ });
    expect(card).toHaveTextContent("oss");
    expect(card).toHaveTextContent("target W9 / P2");
    expect(card).toHaveTextContent("4");
  });

  // Rule: Active > 3 renders a warning badge. The move still succeeds — a hard limit on your
  // own goal board becomes something you fight, and then route around by lying about statuses.
  it("warns when Active exceeds the limit without blocking the next move", async () => {
    const user = userEvent.setup();
    const active = [1, 2, 3].map((id) => goal({ id, status: "active" }));
    const waiting = goal({ id: 4, status: "backlog" });

    const moved = goal({ id: 4, status: "active", sort_order: 400, version: 2 });
    const server = serveBoard([...active, waiting]);
    mockedMoveGoal.mockImplementation(async () => {
      server.settlesOn([...active, moved]);
      return { goal: moved, reordered: [...active, moved] };
    });

    renderPage();

    // Three active is at the limit, not over it.
    expect(await screen.findByRole("button", { name: /G4 · Goal 4/ })).toBeInTheDocument();
    expect(screen.queryByText(/active$/)).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /G4 · Goal 4/ }));
    await user.click(screen.getByRole("button", { name: "Move to Active" }));

    // The move was not blocked...
    await waitFor(() => expect(mockedMoveGoal).toHaveBeenCalledTimes(1));

    // ...and the badge now reports four.
    expect(await screen.findByText("4 active")).toBeInTheDocument();
  });

  it("moves a card from the detail sheet and takes the order from the server response", async () => {
    const user = userEvent.setup();
    const goals = [
      goal({ id: 1, status: "active", sort_order: 100 }),
      goal({ id: 2, status: "active", sort_order: 200 }),
      goal({ id: 3, status: "backlog", sort_order: 100 }),
    ];

    const reordered = [
      goal({ id: 1, status: "active", sort_order: 100, version: 2 }),
      goal({ id: 2, status: "active", sort_order: 200, version: 2 }),
      goal({ id: 3, status: "active", sort_order: 300, version: 2 }),
    ];

    const server = serveBoard(goals);
    mockedMoveGoal.mockImplementation(async () => {
      server.settlesOn(reordered);
      return { goal: reordered[2], reordered };
    });

    renderPage();

    await user.click(await screen.findByRole("button", { name: /G3 · Goal 3/ }));
    await user.click(screen.getByRole("button", { name: "Move to Active" }));

    await waitFor(() => expect(mockedMoveGoal).toHaveBeenCalledTimes(1));

    // Appending from the sheet means position = the length of the target column.
    const [movedGoal, status, position] = mockedMoveGoal.mock.calls[0];
    expect(movedGoal.id).toBe(3);
    expect(status).toBe("active");
    expect(position).toBe(2);

    await waitFor(() => expect(cardTitles("Active")).toHaveLength(3));
    expect(cardTitles("Active")[2]).toContain("Goal 3");
    expect(cardTitles("Backlog")).toHaveLength(0);

    // The response carries every renumbered card in both columns, so a successful move needs
    // no follow-up read: one GET, the initial load.
    expect(mockedGetBoard).toHaveBeenCalledTimes(1);
  });

  // A move that fails changes nothing — including the neighbours the optimistic update
  // shuffled to make room. Leaving those in place would show an order the server never had.
  it("undoes the whole optimistic move when it conflicts, not just the moved card", async () => {
    const user = userEvent.setup();

    const backlog = [
      goal({ id: 1, status: "backlog", sort_order: 100 }),
      goal({ id: 2, status: "backlog", sort_order: 200 }),
    ];
    const active = [goal({ id: 3, status: "active", sort_order: 100 })];

    // The other device already moved G1 to Done.
    const elsewhere = goal({ id: 1, status: "done", sort_order: 100, version: 9 });
    const server = serveBoard([...backlog, ...active]);

    mockedMoveGoal.mockImplementation(async () => {
      server.settlesOn([elsewhere, backlog[1], active[0]]);
      throw new api.ApiError("goal was modified elsewhere", 412, elsewhere);
    });

    renderPage();

    await user.click(await screen.findByRole("button", { name: /G1 · Goal 1/ }));
    await user.click(screen.getByRole("button", { name: "Move to Active" }));

    expect(await screen.findByText("Updated elsewhere · refreshed")).toBeInTheDocument();

    // G1 sits where the server says. G2 and G3 sit where they always were — the optimistic
    // renumbering that made room for G1 has been undone.
    await waitFor(() => expect(cardTitles("Done")).toHaveLength(1));
    expect(cardTitles("Backlog")).toHaveLength(1);
    expect(cardTitles("Backlog")[0]).toContain("Goal 2");
    expect(cardTitles("Active")).toHaveLength(1);
    expect(cardTitles("Active")[0]).toContain("Goal 3");

    // A 412 describes the moved card only, so this is the one path that does need a read.
    await waitFor(() => expect(mockedGetBoard).toHaveBeenCalledTimes(2));
  });

  it("stamps a completion on the card when it lands in Done and clears it on the way back", async () => {
    const user = userEvent.setup();

    const done = goal({
      id: 1,
      status: "done",
      sort_order: 100,
      version: 2,
      completed_at: "2026-10-31T12:00:00Z",
    });

    const server = serveBoard([goal({ id: 1, status: "active" })]);
    mockedMoveGoal.mockImplementation(async () => {
      server.settlesOn([done]);
      return { goal: done, reordered: [done] };
    });

    renderPage();

    await user.click(await screen.findByRole("button", { name: /G1 · Goal 1/ }));
    await user.click(screen.getByRole("button", { name: "Move to Done" }));

    expect(await screen.findByText(/completed 2026-10-31/)).toBeInTheDocument();

    // And back out again: the server clears the stamp, so the card must too.
    const reopened = goal({
      id: 1,
      status: "active",
      sort_order: 100,
      version: 3,
      completed_at: null,
    });
    mockedMoveGoal.mockImplementation(async () => {
      server.settlesOn([reopened]);
      return { goal: reopened, reordered: [reopened] };
    });

    await user.click(screen.getByRole("button", { name: /G1 · Goal 1/ }));
    await user.click(screen.getByRole("button", { name: "Move to Active" }));

    await waitFor(() => expect(screen.queryByText(/completed/)).not.toBeInTheDocument());
  });

  // The only conflict this can hit is the phone and the laptop open at once. The card visibly
  // jumping back reads as a bug, so the toast says it wasn't.
  it("takes server truth and explains itself when a move conflicts", async () => {
    const user = userEvent.setup();

    // The other device already moved G1 to Done; this device still thinks it is in Backlog.
    const elsewhere = goal({ id: 1, status: "done", sort_order: 100, version: 5 });
    const server = serveBoard([
      goal({ id: 1, status: "backlog" }),
      goal({ id: 2, status: "active" }),
    ]);

    mockedMoveGoal.mockImplementation(async () => {
      server.settlesOn([elsewhere, goal({ id: 2, status: "active" })]);
      throw new api.ApiError("goal was modified elsewhere", 412, elsewhere);
    });

    renderPage();

    await user.click(await screen.findByRole("button", { name: /G1 · Goal 1/ }));
    await user.click(screen.getByRole("button", { name: "Move to Active" }));

    expect(await screen.findByText("Updated elsewhere · refreshed")).toBeInTheDocument();

    // The card sits where the server says it does, not where the optimistic update put it.
    await waitFor(() => expect(cardTitles("Done")).toHaveLength(1));
    expect(cardTitles("Active")).toHaveLength(1);
    expect(cardTitles("Active")[0]).toContain("Goal 2");
  });

  it("rolls the board back and reports the error when a move fails outright", async () => {
    const user = userEvent.setup();

    serveBoard([goal({ id: 1, status: "backlog" })]);
    mockedMoveGoal.mockRejectedValue(new api.ApiError("database is locked", 500));

    renderPage();

    await user.click(await screen.findByRole("button", { name: /G1 · Goal 1/ }));
    await user.click(screen.getByRole("button", { name: "Move to Active" }));

    expect(await screen.findByText("database is locked")).toBeInTheDocument();
    await waitFor(() => expect(cardTitles("Backlog")).toHaveLength(1));
    expect(cardTitles("Active")).toHaveLength(0);
  });

  it("shows done-means in the detail sheet rather than on the card face", async () => {
    const user = userEvent.setup();

    mockedGetBoard.mockResolvedValue(
      board([
        goal({
          id: 2,
          status: "active",
          title: "Reliability proven",
          done_means: "RELIABILITY.md: 5 experiments with graphs and 1 pprof fix",
        }),
      ]),
    );

    renderPage();

    const card = await screen.findByRole("button", { name: /G2 · Reliability proven/ });
    expect(card).not.toHaveTextContent("RELIABILITY.md");

    await user.click(card);
    expect(await screen.findByText(/RELIABILITY.md: 5 experiments/)).toBeInTheDocument();
  });

  it("deletes a goal behind a confirmation", async () => {
    const user = userEvent.setup();

    serveBoard([goal({ id: 1, status: "backlog" })]);
    mockedDeleteGoal.mockResolvedValue(undefined);

    renderPage();

    await user.click(await screen.findByRole("button", { name: /G1 · Goal 1/ }));

    // One tap arms it; the destructive action needs a second.
    await user.click(screen.getByRole("button", { name: /^Delete$/ }));
    expect(mockedDeleteGoal).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: /Delete goal/ }));
    await waitFor(() => expect(mockedDeleteGoal).toHaveBeenCalledWith(1));
  });

  it("does not send a request when the move would change nothing", async () => {
    const user = userEvent.setup();

    mockedGetBoard.mockResolvedValue(board([goal({ id: 1, status: "active" })]));

    renderPage();

    await user.click(await screen.findByRole("button", { name: /G1 · Goal 1/ }));

    // The card's own column is disabled in the sheet, so there is nothing to press.
    expect(screen.getByRole("button", { name: "Move to Active" })).toBeDisabled();
    expect(mockedMoveGoal).not.toHaveBeenCalled();
  });
});
