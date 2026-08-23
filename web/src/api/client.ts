export class ApiError extends Error {
  readonly status: number;
  readonly current: unknown;

  constructor(message: string, status: number, current?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.current = current;
  }
}

export interface HealthResponse {
  status: string;
  version: string;
}

export interface Category {
  id: string;
  label: string;
  icon: string;
  sort_order: number;
}

export interface CategoryCount {
  category_id: string;
  label: string;
  icon: string;
  count: number;
}

export interface Project {
  id: string;
  label: string;
  sort_order: number;
}

export interface PlanIdentity {
  id: string;
  name: string;
  loaded_at?: string;
}

export interface PlanCounts {
  weeks: number;
  goals: number;
  tasks: number;
  skills: number;
  projects: number;
}

export interface PlanSource {
  id: number;
  plan_id: string;
  sha256: string;
  source: string;
  loaded_at: string;
}

export interface PlanStatusResponse {
  plan: PlanIdentity | null;
  source: PlanSource | null;
  counts: PlanCounts;
}

export interface PlanChangeCounts {
  added?: number;
  updated?: number;
  removed?: number;
  unchanged?: number;
  orphaned?: number;
}

export interface PlanDiff {
  weeks: PlanChangeCounts;
  tasks: PlanChangeCounts;
  goals: PlanChangeCounts;
}

export interface PlanAtRisk {
  task_completions: number;
  goal_positions: number;
  checkpoint_answers: number;
  log_goal_links: number;
  log_skill_links: number;
  retired_categories: number;
}

export interface PlanPreviewError {
  line?: number;
  message: string;
  excerpt?: string;
}

export interface PlanPreviewResponse {
  source_sha256?: string;
  plan?: PlanIdentity;
  current_plan?: PlanIdentity | null;
  mode?: "initial" | "additive" | "replace";
  parsed?: PlanCounts;
  changes?: PlanDiff;
  at_risk?: PlanAtRisk;
  errors: PlanPreviewError[];
}

export interface ApplyPlanInput {
  source: string;
  source_sha256: string;
  confirm_plan_name?: string;
}

export interface ApplyPlanResponse {
  plan: PlanIdentity;
  mode: "initial" | "additive" | "replace";
  counts: PlanCounts;
  seed: Record<string, number>;
  source: PlanSource;
  backup_path: string | null;
}

export interface PairResponse {
  url: string;
  svg_url: string;
  expires_at: string;
  expires_in_seconds: number;
}

export interface Goal {
  id: number;
  code: string | null;
  title: string;
  done_means: string | null;
  project: string;
  phase: string | null;
  status: "backlog" | "active" | "done";
  target: string | null;
  sort_order: number;
  log_count: number;
  version: number;
  created_at: string;
  completed_at: string | null;
}

export interface GoalsResponse {
  goals: Goal[];
  /** Goals currently in the Active column. */
  active_count: number;
  /** The WIP limit. Exceeding it is a warning only — the API never blocks a move. */
  active_limit: number;
}

/**
 * The result of a Kanban move.
 *
 * `reordered` holds every card whose sort_order changed — the target column, and on a
 * cross-column move the column the card left as well. The server renumbers both in one
 * transaction, so applying this payload is the whole update: no follow-up read, and the client
 * never computes ordering itself.
 */
export interface GoalMoveResponse {
  goal: Goal;
  reordered: Goal[];
  game?: GameEnvelope;
}

export interface Task {
  id: number;
  week: string;
  title: string;
  project: ProjectID;
  goal_id: number | null;
  steps: string[];
  done_means: string | null;
  status: "todo" | "done";
  done_at: string | null;
  skill_ids: number[];
  sort_order: number;
  version: number;
  created_at: string;
}

export interface CreateTaskInput {
  week: string;
  title: string;
  project: ProjectID;
  goal_id?: number;
  steps?: string[];
  done_means?: string;
  skill_ids: number[];
}

export interface PlanWeek {
  code: string | null;
  phase: string | null;
  focus: string | null;
  start_date: string | null;
  end_date: string | null;
  state: "not_started" | "active" | "plan_complete";
}

export interface TaskWeek {
  code: string;
  start_date: string;
  end_date: string;
  focus: string | null;
}

export interface DashboardResponse {
  today: string;
  week: PlanWeek;
  /**
   * The week the task list came from, with its dates and focus.
   *
   * Differs from `week` before the plan starts and after it ends — there is no active week
   * then, but the screen still shows the first or last week's tasks. Anything that writes a
   * task must use this, and the banner uses its dates to say when an upcoming week begins.
   */
  task_week: TaskWeek | null;
  rhythm: {
    label: string;
    slot: string;
  };
  tasks: Task[];
  completion: {
    done: number;
    total: number;
  };
  counters: CategoryCount[];
  streak: {
    days: number;
    counts_today: boolean;
  };
}

export interface CreateLogInput {
  category_id: string;
  title: string;
  note?: string;
  url?: string;
  goal_id?: number;
  occurred_at?: string;
  /** Optional evidence links; the quick-log path stays cheap by default. */
  skill_ids?: number[];
}

export interface LogEntry {
  id: number;
  category_id: string;
  category_label: string;
  icon: string;
  title: string;
  note: string | null;
  url: string | null;
  goal_id: number | null;
  goal_code: string | null;
  skill_ids?: number[];
  occurred_at: string;
  created_at: string;
}

export interface LogFeedResponse {
  entries: LogEntry[];
  next_cursor: string | null;
}

export interface LogSummaryResponse {
  range: "week" | "all";
  from: string | null;
  to: string | null;
  counts: CategoryCount[];
  total: number;
}

export interface DailyReview {
  date: string;
  learned: string | null;
  issue: string | null;
  next: string | null;
  minutes: number | null;
  created_at: string;
  updated_at: string;
}

export interface UpsertReviewInput {
  date?: string;
  learned?: string;
  issue?: string;
  next?: string;
  minutes?: number;
}

export interface Metric {
  id: number;
  name: string;
  value: number;
  unit: string | null;
  note: string | null;
  recorded_at: string;
}

export interface MetricDef {
  name: string;
  slug: string | null;
  unit: string | null;
  baseline: string | null;
  target: string | null;
  definition: string | null;
  how_to_measure: string | null;
  sort_order: number;
}

export interface CreateMetricInput {
  name: string;
  value: number;
  unit?: string;
  note?: string;
  recorded_at?: string;
}

export interface Checkpoint {
  week: string;
  questions: string[];
  helpers: string[] | null;
  answers: string[] | null;
  completed_at: string | null;
}

export interface Skill {
  id: number;
  code: string;
  name: string;
  description: string;
  /** The tagging rule, rendered beside every option so the description does the deciding. */
  associate_when: string;
  target_tier: string | null;
  sort_order: number;
  /** Derived at read time from the associations — never stored (ADR-007). */
  task_count: number;
  task_done: number;
  evidence_count: number;
  last_activity: string | null;
}

export interface SkillsResponse {
  skills: Skill[];
  /** Tasks with no skill at all: rows that predate M8. */
  unlinked_tasks: number[];
}

export interface SkillDetail extends Skill {
  tasks: Task[];
  evidence: LogEntry[];
}

export interface XPEvent {
  id: number;
  source_type: "task" | "log" | "goal" | "review" | "metric" | "achievement";
  source_id: number;
  amount: number;
  created_at: string;
  skill_ids?: number[];
  skill_codes?: string[];
  skill_names?: string[];
  source_label?: string;
}

export interface Achievement {
  id: number;
  code: string;
  name: string;
  description: string;
  sort_order: number;
  unlocked_at?: string;
}

export interface GameLevelUp {
  level: number;
  title: string;
  total_xp: number;
  next_threshold: number;
}

export interface GameEnvelope {
  xp_awarded: number;
  level_up?: GameLevelUp;
  unlocks: Achievement[];
  event?: XPEvent;
}

export interface GameProfile {
  total_xp: number;
  level: number;
  title: string;
  current_threshold: number;
  next_level: number;
  next_threshold: number;
  progress_xp: number;
  progress_required: number;
  streak: {
    days: number;
    counts_today: boolean;
    shielded: boolean;
  };
  badges: Achievement[];
}

export interface GameSkill {
  skill_id: number;
  code: string;
  name: string;
  target_tier: string | null;
  xp: number;
  tier: string;
  tier_min: number;
  next_tier_xp: number;
  next_tier?: string;
}

export interface TaskWriteResponse extends Task {
  game?: GameEnvelope;
}

export interface LogWriteResponse extends LogEntry {
  game?: GameEnvelope;
}

export interface ReviewWriteResponse extends DailyReview {
  game?: GameEnvelope;
}

export interface MetricWriteResponse extends Metric {
  game?: GameEnvelope;
}

export interface CheckpointWriteResponse extends Checkpoint {
  game?: GameEnvelope;
}

export type ProjectID = string;

interface APIEnvelope {
  error?: string;
  current?: unknown;
}

const tokenStorageKey = "dashboard.token";

export async function getHealth(): Promise<HealthResponse> {
  return apiFetch<HealthResponse>("/api/health", { cache: "no-store" });
}

export async function getDashboard(projects: ProjectID[]): Promise<DashboardResponse> {
  const params = new URLSearchParams();
  if (projects.length > 0) {
    params.set("project", projects.join(","));
  }

  const query = params.toString();
  return apiFetch<DashboardResponse>(`/api/dashboard${query ? `?${query}` : ""}`);
}

export async function getCategories(): Promise<Category[]> {
  const response = await apiFetch<{ categories: Category[] }>("/api/categories");
  return response.categories;
}

export async function getProjects(): Promise<{ projects: Project[] }> {
  return apiFetch<{ projects: Project[] }>("/api/projects");
}

export async function getPlan(): Promise<PlanStatusResponse> {
  return apiFetch<PlanStatusResponse>("/api/plan");
}

export async function previewPlan(source: string): Promise<PlanPreviewResponse> {
  return apiFetch<PlanPreviewResponse>("/api/plan/preview", {
    method: "POST",
    headers: { "Content-Type": "text/markdown; charset=utf-8" },
    body: source,
  });
}

export async function applyPlan(input: ApplyPlanInput): Promise<ApplyPlanResponse> {
  return apiFetch<ApplyPlanResponse>("/api/plan/apply", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function createPair(route: string): Promise<PairResponse> {
  return apiFetch<PairResponse>("/api/pair", {
    method: "POST",
    body: JSON.stringify({ route }),
  });
}

export async function getPairSVG(path: string): Promise<string> {
  return apiFetchText(path, {
    headers: { Accept: "image/svg+xml" },
  });
}

/**
 * The goals, plus the active count and limit behind the Kanban warning badge.
 *
 * One function for one endpoint: the Today picker reads `.goals` from the same response the
 * board uses, so both share a cache entry and a log linked to a goal refreshes its count.
 */
export async function getGoalsBoard(): Promise<GoalsResponse> {
  return apiFetch<GoalsResponse>("/api/goals");
}

/**
 * Move a goal to `position` (0-based) within the `status` column.
 *
 * Both the desktop drag and the phone "Move to…" sheet come through here, so there is one
 * request shape and one reconciliation path.
 */
export async function moveGoal(
  goal: Goal,
  status: Goal["status"],
  position: number,
): Promise<GoalMoveResponse> {
  return apiFetch<GoalMoveResponse>(`/api/goals/${goal.id}`, {
    method: "PATCH",
    headers: {
      "If-Match": `W/"${goal.id}-${goal.version}"`,
    },
    body: JSON.stringify({ status, position }),
  });
}

export async function getSkills(): Promise<SkillsResponse> {
  return apiFetch<SkillsResponse>("/api/skills");
}

export async function getSkill(id: number): Promise<SkillDetail> {
  return apiFetch<SkillDetail>(`/api/skills/${id}`);
}

export async function getGameProfile(): Promise<GameProfile> {
  return apiFetch<GameProfile>("/api/game/profile");
}

export async function getGameSkills(): Promise<{ skills: GameSkill[] }> {
  return apiFetch<{ skills: GameSkill[] }>("/api/game/skills");
}

export async function getGameEvents(limit = 20): Promise<{ events: XPEvent[] }> {
  return apiFetch<{ events: XPEvent[] }>(`/api/game/events?limit=${limit}`);
}

/**
 * Replace a task's skills. Sending none is refused with 422 by the API: every task builds at
 * least one skill, so the UI disables save before it can get here.
 */
export async function updateTaskSkills(task: Task, skillIDs: number[]): Promise<TaskWriteResponse> {
  return apiFetch<TaskWriteResponse>(`/api/tasks/${task.id}`, {
    method: "PATCH",
    headers: {
      "If-Match": `W/"${task.id}-${task.version}"`,
    },
    body: JSON.stringify({ skill_ids: skillIDs }),
  });
}

export async function createTask(input: CreateTaskInput): Promise<Task> {
  return apiFetch<Task>("/api/tasks", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function deleteGoal(id: number): Promise<void> {
  await apiFetch<void>(`/api/goals/${id}`, { method: "DELETE" });
}

export async function updateTaskStatus(
  task: Task,
  status: Task["status"],
): Promise<TaskWriteResponse> {
  return apiFetch<TaskWriteResponse>(`/api/tasks/${task.id}`, {
    method: "PATCH",
    headers: {
      "If-Match": `W/"${task.id}-${task.version}"`,
    },
    body: JSON.stringify({ status }),
  });
}

export async function createLog(input: CreateLogInput): Promise<LogWriteResponse> {
  return apiFetch<LogWriteResponse>("/api/logs", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getLogs({
  categories = [],
  cursor,
  limit = 50,
}: {
  categories?: string[];
  cursor?: string | null;
  limit?: number;
} = {}): Promise<LogFeedResponse> {
  const params = new URLSearchParams();
  if (categories.length > 0) {
    params.set("category", categories.join(","));
  }
  if (cursor) {
    params.set("cursor", cursor);
  }
  params.set("limit", String(limit));

  return apiFetch<LogFeedResponse>(`/api/logs?${params.toString()}`);
}

export async function deleteLog(id: number): Promise<void> {
  await apiFetch<void>(`/api/logs/${id}`, { method: "DELETE" });
}

export async function getLogSummary(range: "week" | "all" = "week"): Promise<LogSummaryResponse> {
  const params = new URLSearchParams({ range });
  return apiFetch<LogSummaryResponse>(`/api/logs/summary?${params.toString()}`);
}

export async function getReviews({
  from,
  to,
}: {
  from?: string;
  to?: string;
} = {}): Promise<DailyReview[]> {
  const params = new URLSearchParams();
  if (from) {
    params.set("from", from);
  }
  if (to) {
    params.set("to", to);
  }

  const query = params.toString();
  const response = await apiFetch<{ reviews: DailyReview[] }>(
    `/api/reviews${query ? `?${query}` : ""}`,
  );
  return response.reviews;
}

export async function upsertReview(input: UpsertReviewInput): Promise<ReviewWriteResponse> {
  return apiFetch<ReviewWriteResponse>("/api/reviews", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getMetrics({
  name,
  limit = 5,
}: {
  name?: string;
  limit?: number;
} = {}): Promise<Metric[]> {
  const params = new URLSearchParams();
  if (name) {
    params.set("name", name);
  }
  params.set("limit", String(limit));

  const response = await apiFetch<{ metrics: Metric[] }>(`/api/metrics?${params.toString()}`);
  return response.metrics;
}

export async function createMetric(input: CreateMetricInput): Promise<MetricWriteResponse> {
  return apiFetch<MetricWriteResponse>("/api/metrics", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getMetricDefs(): Promise<MetricDef[]> {
  const response = await apiFetch<{ metric_defs: MetricDef[] }>("/api/metric-defs");
  return response.metric_defs;
}

export async function getCheckpoint(week: string): Promise<Checkpoint> {
  return apiFetch<Checkpoint>(`/api/checkpoints/${week}`);
}

export async function saveCheckpoint(
  week: string,
  answers: string[],
): Promise<CheckpointWriteResponse> {
  return apiFetch<CheckpointWriteResponse>(`/api/checkpoints/${week}`, {
    method: "PUT",
    body: JSON.stringify({ answers }),
  });
}

async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");

  if (init.body != null && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const token = configuredToken();
  if (token !== "") {
    headers.set("X-Token", token);
  }

  const response = await fetch(path, {
    ...init,
    headers,
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const body = (await response.json().catch(() => undefined)) as APIEnvelope | T | undefined;

  if (!response.ok) {
    const envelope = body as APIEnvelope | undefined;
    throw new ApiError(
      envelope?.error ?? `request failed with ${response.status}`,
      response.status,
      envelope?.current,
    );
  }

  return body as T;
}

async function apiFetchText(path: string, init: RequestInit = {}): Promise<string> {
  const headers = new Headers(init.headers);
  const token = configuredToken();
  if (token !== "") {
    headers.set("X-Token", token);
  }

  const response = await fetch(path, {
    ...init,
    headers,
  });
  const body = await response.text();
  if (!response.ok) {
    throw new ApiError(body || `request failed with ${response.status}`, response.status);
  }
  return body;
}

function configuredToken(): string {
  const envToken = import.meta.env.VITE_DASHBOARD_TOKEN;
  if (typeof window === "undefined") {
    return envToken ?? "";
  }

  return window.localStorage.getItem(tokenStorageKey) ?? envToken ?? "";
}
