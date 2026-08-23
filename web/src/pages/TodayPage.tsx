import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Award, Check, Loader2, Plus, ShieldCheck, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  ApiError,
  createTask,
  createLog,
  getCategories,
  getDashboard,
  getGameProfile,
  getGameSkills,
  getGoalsBoard,
  getProjects,
  getSkills,
  type Category,
  type CreateTaskInput,
  type DashboardResponse,
  type GameEnvelope,
  type GameProfile,
  type GameSkill,
  type Goal,
  type Project,
  type ProjectID,
  type Skill,
  type Task,
  updateTaskSkills,
  updateTaskStatus,
} from "../api/client";
import { GameSkillBar } from "../components/GameSkillBar";
import { LevelUpMoment } from "../components/LevelUpMoment";
import { chapterLabel, gameCopy, gameToast } from "../copy/game";
import { skillsCopy } from "../copy/skills";
import { todayCopy } from "../copy/today";
import { daysBetween, formatLisbonDayHeading } from "../lib/date";
import {
  gameEventsQueryKey,
  gameProfileQueryKey,
  gameSkillsQueryKey,
  goalsQueryKey,
  projectsQueryKey,
  skillsQueryKey,
} from "../lib/queryKeys";

const dashboardQueryRoot = ["dashboard"] as const;

export function TodayPage() {
  const queryClient = useQueryClient();
  const [selectedProjects, setSelectedProjects] = useProjectFilter();
  const [selectedTaskID, setSelectedTaskID] = useState<number | null>(null);
  const [addingTask, setAddingTask] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const [selectedCategoryID, setSelectedCategoryID] = useState<string | null>(null);
  const [profileOpen, setProfileOpen] = useState(false);
  const [levelUp, setLevelUp] = useState<NonNullable<GameEnvelope["level_up"]> | null>(null);

  const projectsQuery = useQuery({
    queryKey: projectsQueryKey,
    queryFn: getProjects,
  });
  const projects = useMemo(() => projectsQuery.data?.projects ?? [], [projectsQuery.data]);
  const selectedKnownProjects = useMemo(
    () => filterKnownProjects(selectedProjects, projects),
    [selectedProjects, projects],
  );

  useEffect(() => {
    if (!projectsQuery.data || selectedKnownProjects.length === selectedProjects.length) {
      return;
    }
    setSelectedProjects(selectedKnownProjects);
  }, [projectsQuery.data, selectedKnownProjects, selectedProjects.length, setSelectedProjects]);

  const dashboardQuery = useQuery({
    queryKey: [...dashboardQueryRoot, selectedKnownProjects],
    queryFn: () => getDashboard(selectedKnownProjects),
    enabled: projectsQuery.isSuccess || selectedProjects.length === 0,
  });

  const categoriesQuery = useQuery({
    queryKey: ["categories"],
    queryFn: getCategories,
  });

  const goalsQuery = useQuery({
    queryKey: goalsQueryKey,
    queryFn: getGoalsBoard,
  });

  const skillsQuery = useQuery({
    queryKey: skillsQueryKey,
    queryFn: getSkills,
  });

  const gameProfileQuery = useQuery({
    queryKey: gameProfileQueryKey,
    queryFn: getGameProfile,
  });

  const gameSkillsQuery = useQuery({
    queryKey: gameSkillsQueryKey,
    queryFn: getGameSkills,
  });

  const handleGame = useCallback(
    (game?: GameEnvelope) => {
      if (!game) {
        return;
      }

      const message = gameToast(game);
      if (message) {
        setToast(message);
      }
      if (game.level_up) {
        setLevelUp(game.level_up);
      }

      void queryClient.invalidateQueries({ queryKey: gameProfileQueryKey });
      void queryClient.invalidateQueries({ queryKey: gameSkillsQueryKey });
      void queryClient.invalidateQueries({ queryKey: gameEventsQueryKey });
    },
    [queryClient],
  );

  const taskMutation = useMutation({
    mutationFn: ({ task, nextStatus }: { task: Task; nextStatus: Task["status"] }) =>
      updateTaskStatus(task, nextStatus),
    onMutate: async ({ task, nextStatus }) => {
      const key = [...dashboardQueryRoot, selectedProjects] as const;
      await queryClient.cancelQueries({ queryKey: dashboardQueryRoot });
      const previous = queryClient.getQueryData<DashboardResponse>(key);
      const optimisticTask = {
        ...task,
        status: nextStatus,
        done_at: nextStatus === "done" ? new Date().toISOString() : null,
        version: task.version + 1,
      };

      queryClient.setQueryData<DashboardResponse>(key, (current) =>
        current ? replaceTask(current, optimisticTask) : current,
      );

      return { key, previous };
    },
    onError: (error, _vars, context) => {
      if (error instanceof ApiError && error.status === 412) {
        const currentTask = coerceTask(error.current);
        if (currentTask) {
          queryClient.setQueryData<DashboardResponse>(
            context?.key ?? dashboardQueryRoot,
            (current) => (current ? replaceTask(current, currentTask) : current),
          );
        }
        setToast("Updated elsewhere · refreshed");
        void queryClient.invalidateQueries({ queryKey: dashboardQueryRoot });
        return;
      }

      if (context?.previous) {
        queryClient.setQueryData(context.key, context.previous);
      }
      setToast(error instanceof Error ? error.message : "Task update failed");
    },
    onSuccess: (task, _vars, context) => {
      queryClient.setQueryData<DashboardResponse>(context?.key ?? dashboardQueryRoot, (current) =>
        current ? replaceTask(current, task) : current,
      );
      handleGame(task.game);
    },
  });

  const taskSkillsMutation = useMutation({
    mutationFn: ({ task, skillIDs }: { task: Task; skillIDs: number[] }) =>
      updateTaskSkills(task, skillIDs),
    onError: (error) => {
      if (error instanceof ApiError && error.status === 412) {
        const currentTask = coerceTask(error.current);
        if (currentTask) {
          queryClient.setQueryData<DashboardResponse>(
            [...dashboardQueryRoot, selectedProjects],
            (current) => (current ? replaceTask(current, currentTask) : current),
          );
        }
        setToast("Updated elsewhere · refreshed");
        void queryClient.invalidateQueries({ queryKey: dashboardQueryRoot });
        return;
      }

      setToast(error instanceof Error ? error.message : "Skill update failed");
    },
    onSuccess: (task) => {
      queryClient.setQueryData<DashboardResponse>(
        [...dashboardQueryRoot, selectedProjects],
        (current) => (current ? replaceTask(current, task) : current),
      );
      void queryClient.invalidateQueries({ queryKey: skillsQueryKey });
      handleGame(task.game);
      if (!task.game) {
        setToast("Skills saved");
      }
    },
  });

  const createTaskMutation = useMutation({
    mutationFn: (input: CreateTaskInput) => createTask(input),
    onSuccess: (task) => {
      setAddingTask(false);
      queryClient.setQueryData<DashboardResponse>(
        [...dashboardQueryRoot, selectedProjects],
        (current) => (current ? appendTask(current, task) : current),
      );
      void queryClient.invalidateQueries({ queryKey: dashboardQueryRoot });
      void queryClient.invalidateQueries({ queryKey: skillsQueryKey });
      setToast("Task added");
    },
    onError: (error) => {
      setToast(error instanceof Error ? error.message : "Task add failed");
    },
  });

  const logMutation = useMutation({
    mutationFn: createLog,
    onSuccess: (entry) => {
      setSelectedCategoryID(null);
      void queryClient.invalidateQueries({ queryKey: dashboardQueryRoot });
      // An entry linked to a goal changes that goal's log_count - the Kanban's proof-of-motion
      // chip - so the shared goals entry has to go stale too.
      void queryClient.invalidateQueries({ queryKey: goalsQueryKey });
      void queryClient.invalidateQueries({ queryKey: skillsQueryKey });
      handleGame(entry.game);
      if (!entry.game) {
        setToast("Logged");
      }
    },
    onError: (error) => {
      setToast(error instanceof Error ? error.message : "Log failed");
    },
  });

  const categories = useMemo(() => categoriesQuery.data ?? [], [categoriesQuery.data]);
  const goals = goalsQuery.data?.goals ?? [];
  const skills = skillsQuery.data?.skills ?? [];
  const gameSkills = gameSkillsQuery.data?.skills ?? [];

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() !== "q" || isTypingTarget(event.target)) {
        return;
      }
      event.preventDefault();
      setSelectedCategoryID(categories[0]?.id ?? "application");
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [categories]);

  const dashboard = dashboardQuery.data;
  const selectedTask = dashboard?.tasks.find((task) => task.id === selectedTaskID) ?? null;
  // The week a new task is written to. Before the plan starts and after it ends there is no
  // active week, but the screen is showing the first or last week's tasks and adding to that
  // week is exactly right — gating on activeWeek left "Add task" greyed out with no reason.
  const addTaskWeek = dashboard?.task_week?.code ?? null;

  return (
    <section className="min-h-screen px-4 py-5 md:px-10 md:py-8">
      <div className="mx-auto flex max-w-5xl flex-col gap-5">
        <header className="flex flex-col gap-2">
          <p className="text-sm font-medium uppercase text-cyan-300">Today</p>
          <div className="flex flex-wrap items-end justify-between gap-3">
            <h1 className="text-3xl font-semibold text-white md:text-4xl">
              {dashboard?.today ?? "Dashboard"}
            </h1>
            <div className="flex items-center gap-2">
              {gameProfileQuery.data ? (
                <LevelRing profile={gameProfileQuery.data} onClick={() => setProfileOpen(true)} />
              ) : null}
              {dashboard ? (
                <StreakChip
                  days={dashboard.streak.days}
                  countsToday={dashboard.streak.counts_today}
                />
              ) : null}
            </div>
          </div>
        </header>

        {dashboardQuery.isLoading ? <LoadingState /> : null}
        {dashboardQuery.isError ? (
          <ErrorState message={errorMessage(dashboardQuery.error)} />
        ) : null}

        {dashboard ? (
          <>
            <TopSummary dashboard={dashboard} />

            <ProjectChips
              projects={projects}
              selected={selectedKnownProjects}
              onChange={setSelectedProjects}
            />

            <TaskChecklist
              skills={skills}
              tasks={dashboard.tasks}
              canAddTask={addTaskWeek !== null}
              onAddTask={() => setAddingTask(true)}
              onSelectTask={(task) => setSelectedTaskID(task.id)}
              onToggleTask={(task) => {
                taskMutation.mutate({
                  task,
                  nextStatus: task.status === "done" ? "todo" : "done",
                });
              }}
            />

            <Counters counts={dashboard.counters} />
          </>
        ) : null}
      </div>

      <QuickLogBar
        categories={categories}
        loading={categoriesQuery.isLoading}
        onSelect={(categoryID) => setSelectedCategoryID(categoryID)}
      />

      {selectedCategoryID ? (
        <QuickLogSheet
          categories={categories}
          goals={goals}
          skills={skills}
          selectedCategoryID={selectedCategoryID}
          saving={logMutation.isPending}
          onClose={() => setSelectedCategoryID(null)}
          onSave={(input) => logMutation.mutate(input)}
        />
      ) : null}

      {selectedTask ? (
        <TaskDetailSheet
          key={selectedTask.id}
          task={selectedTask}
          skills={skills}
          saving={taskSkillsMutation.isPending}
          onClose={() => setSelectedTaskID(null)}
          onSave={(skillIDs) => taskSkillsMutation.mutate({ task: selectedTask, skillIDs })}
        />
      ) : null}

      {addingTask && addTaskWeek ? (
        <AddTaskSheet
          week={addTaskWeek}
          projects={projects}
          skills={skills}
          goals={goals}
          saving={createTaskMutation.isPending}
          onClose={() => setAddingTask(false)}
          onSave={(input) => createTaskMutation.mutate(input)}
        />
      ) : null}

      {profileOpen && gameProfileQuery.data ? (
        <GameProfileSheet
          profile={gameProfileQuery.data}
          skills={gameSkills}
          onClose={() => setProfileOpen(false)}
        />
      ) : null}

      {levelUp ? <LevelUpMoment levelUp={levelUp} onClose={() => setLevelUp(null)} /> : null}

      {toast ? <Toast message={toast} onClose={() => setToast(null)} /> : null}
    </section>
  );
}

function TopSummary({ dashboard }: { dashboard: DashboardResponse }) {
  const weekText =
    dashboard.week.state === "active" && dashboard.week.code
      ? chapterLabel(dashboard.week.code, dashboard.week.focus)
      : null;

  // Before the plan starts there is no active week, but the task list below is already showing
  // the first week's work. Rendering nothing left a blank panel above it with no explanation.
  const upcoming =
    dashboard.week.state === "not_started" && dashboard.task_week ? dashboard.task_week : null;

  return (
    <div className="grid gap-3 md:grid-cols-[1.15fr_0.85fr]">
      {weekText ? (
        <section className="rounded-md border border-cyan-300/40 bg-cyan-300/10 p-4">
          <p className="text-xs font-semibold uppercase text-cyan-200">{gameCopy.chapter}</p>
          <p className="mt-2 text-lg font-semibold text-white">{weekText}</p>
        </section>
      ) : null}

      {upcoming ? <UpcomingWeekBanner today={dashboard.today} week={upcoming} /> : null}

      <section className="rounded-md border border-white/10 bg-white/[0.03] p-4">
        <p className="text-xs font-semibold uppercase text-zinc-400">Rhythm</p>
        <p className="mt-2 text-lg font-medium text-white">
          {dashboard.rhythm.label} — {dashboard.rhythm.slot}
        </p>
      </section>
    </div>
  );
}

function UpcomingWeekBanner({
  today,
  week,
}: {
  today: string;
  week: NonNullable<DashboardResponse["task_week"]>;
}) {
  const days = daysBetween(today, week.start_date);
  const copy = todayCopy.banner.notStarted;

  return (
    <section className="rounded-md border border-amber-300/40 bg-amber-300/10 p-4">
      <p className="text-xs font-semibold uppercase text-amber-200">{copy.eyebrow}</p>
      <p className="mt-2 text-lg font-semibold text-white">{copy.title(week.code, week.focus)}</p>
      <p className="mt-1 text-sm text-amber-100">
        {copy.countdown(days, formatLisbonDayHeading(week.start_date))}
      </p>
      <p className="mt-2 text-xs text-zinc-300">{copy.note}</p>
    </section>
  );
}

function ProjectChips({
  projects,
  selected,
  onChange,
}: {
  projects: Project[];
  selected: ProjectID[];
  onChange: (projects: ProjectID[]) => void;
}) {
  return (
    <section className="flex flex-wrap gap-2" aria-label="Project filters">
      {projects.map((project) => {
        const active = selected.includes(project.id);

        return (
          <button
            key={project.id}
            type="button"
            aria-label={project.id}
            className={[
              "min-h-11 rounded-md border px-3 text-sm font-medium transition",
              active
                ? "border-emerald-300 bg-emerald-300 text-zinc-950"
                : "border-white/10 bg-white/[0.03] text-zinc-300",
            ].join(" ")}
            aria-pressed={active}
            onClick={() => onChange(toggleProject(selected, project.id))}
          >
            {project.label}
          </button>
        );
      })}
    </section>
  );
}

function TaskChecklist({
  skills,
  tasks,
  canAddTask,
  onAddTask,
  onSelectTask,
  onToggleTask,
}: {
  skills: Skill[];
  tasks: Task[];
  canAddTask: boolean;
  onAddTask: () => void;
  onSelectTask: (task: Task) => void;
  onToggleTask: (task: Task) => void;
}) {
  return (
    <section className="rounded-md border border-white/10 bg-white/[0.03]">
      <div className="flex items-center justify-between gap-3 border-b border-white/10 px-4 py-3">
        <h2 className="text-lg font-semibold text-white">Tasks</h2>
        <button
          type="button"
          disabled={!canAddTask}
          className="inline-flex min-h-11 items-center gap-2 rounded-md border border-white/10 px-3 text-sm font-medium text-zinc-200 disabled:cursor-not-allowed disabled:text-zinc-600"
          onClick={onAddTask}
        >
          <Plus className="size-4" aria-hidden="true" />
          Add task
        </button>
      </div>

      {tasks.length === 0 ? (
        <p className="p-5 text-sm text-zinc-300">No tasks match this filter.</p>
      ) : (
        <div className="divide-y divide-white/10">
          {tasks.map((task) => {
            const done = task.status === "done";

            return (
              <article key={task.id} className="p-3">
                <div className="grid grid-cols-[44px_1fr] gap-3">
                  <button
                    type="button"
                    role="checkbox"
                    aria-checked={done}
                    aria-label={`${done ? "Mark todo" : "Mark done"}: ${task.title}`}
                    className={[
                      "grid size-11 place-items-center rounded-md border transition",
                      done
                        ? "border-emerald-300 bg-emerald-300 text-zinc-950"
                        : "border-white/20 bg-zinc-950 text-transparent",
                    ].join(" ")}
                    onClick={() => onToggleTask(task)}
                  >
                    <Check className="size-5" aria-hidden="true" />
                  </button>

                  <button
                    type="button"
                    className="min-h-11 min-w-0 text-left"
                    onClick={() => onSelectTask(task)}
                  >
                    <span
                      className={[
                        "block text-base font-medium text-white",
                        done ? "text-zinc-500 line-through" : "",
                      ].join(" ")}
                    >
                      {task.title}
                    </span>
                    <span className="mt-1 inline-flex rounded-sm bg-amber-300/15 px-2 py-1 text-xs font-medium text-amber-200">
                      {task.project}
                    </span>
                    <TaskSkillDots skills={skills} task={task} />
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}

function TaskSkillDots({ skills, task }: { skills: Skill[]; task: Task }) {
  // A task created before M8 has no links at all. Saying so is better than showing an empty
  // row that reads as "no skills apply".
  if (task.skill_ids.length === 0) {
    return (
      <span className="inline-flex items-center gap-1 rounded-sm bg-amber-300/15 px-2 py-1 text-xs font-medium text-amber-200">
        {skillsCopy.picker.needsSkill}
      </span>
    );
  }

  const linked = task.skill_ids
    .map((id) => skills.find((skill) => skill.id === id))
    .filter((skill): skill is Skill => skill !== undefined);

  return (
    <span className="mt-2 flex flex-wrap gap-1.5" aria-label={skillsCopy.picker.label}>
      {linked.map((skill, index) => (
        <span
          key={skill.id}
          title={skill.associate_when}
          className={["inline-block size-2.5 rounded-full", skillDotClass(index)].join(" ")}
        >
          <span className="sr-only">{skill.name}</span>
        </span>
      ))}
    </span>
  );
}

function TaskSkillChips({ skills, selectedIDs }: { skills: Skill[]; selectedIDs: number[] }) {
  const linked = selectedIDs
    .map((id) => skills.find((skill) => skill.id === id))
    .filter((skill): skill is Skill => skill !== undefined);

  if (linked.length === 0) {
    return (
      <p className="inline-flex rounded-sm bg-amber-300/15 px-2 py-1 text-xs font-medium text-amber-200">
        {skillsCopy.picker.needsSkill}
      </p>
    );
  }

  return (
    <p className="flex flex-wrap gap-1" aria-label={skillsCopy.picker.label}>
      {linked.map((skill) => (
        <span
          key={skill.id}
          title={skill.associate_when}
          className="inline-flex rounded-sm bg-cyan-300/15 px-2 py-1 text-xs font-medium text-cyan-200"
        >
          {skill.name}
        </span>
      ))}
    </p>
  );
}

function TaskDetailSheet({
  task,
  skills,
  saving,
  onClose,
  onSave,
}: {
  task: Task;
  skills: Skill[];
  saving: boolean;
  onClose: () => void;
  onSave: (skillIDs: number[]) => void;
}) {
  const [selectedSkillIDs, setSelectedSkillIDs] = useState(task.skill_ids);
  const canSave =
    selectedSkillIDs.length > 0 && !saving && !sameIDs(selectedSkillIDs, task.skill_ids);

  return (
    <div className="fixed inset-0 z-40 bg-black/55" role="dialog" aria-modal="true">
      <div className="absolute inset-x-0 bottom-0 max-h-[85vh] overflow-y-auto rounded-t-md border-t border-white/10 bg-zinc-950 p-4 shadow-2xl md:left-auto md:right-6 md:top-6 md:w-[32rem] md:rounded-md md:border">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-xs font-medium uppercase text-cyan-300">
              {task.week} · {task.project}
            </p>
            <h2 className="text-xl font-semibold text-white">{task.title}</h2>
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

        <div className="mt-4">
          <TaskSkillChips skills={skills} selectedIDs={selectedSkillIDs} />
        </div>

        <div className="mt-4">
          <SkillMultiSelect
            skills={skills}
            selectedIDs={selectedSkillIDs}
            label={skillsCopy.picker.label}
            helper={skillsCopy.picker.helper}
            required
            onToggle={(skillID) => setSelectedSkillIDs(toggleID(selectedSkillIDs, skillID))}
          />
        </div>

        {task.steps.length > 0 ? (
          <div className="mt-5">
            <p className="text-xs font-semibold uppercase text-zinc-400">Steps</p>
            <ol className="mt-2 list-decimal space-y-1 pl-5 text-sm text-zinc-300">
              {task.steps.map((step) => (
                <li key={step}>{step}</li>
              ))}
            </ol>
          </div>
        ) : null}

        {task.done_means ? (
          <div className="mt-5">
            <p className="text-xs font-semibold uppercase text-zinc-400">Done means</p>
            <p className="mt-2 border-l border-cyan-300/50 pl-3 text-sm text-zinc-200">
              {task.done_means}
            </p>
          </div>
        ) : null}

        <button
          type="button"
          disabled={!canSave}
          className="mt-5 flex min-h-11 w-full items-center justify-center gap-2 rounded-md bg-cyan-300 px-4 font-semibold text-zinc-950 disabled:cursor-not-allowed disabled:bg-zinc-700 disabled:text-zinc-400"
          onClick={() => onSave(selectedSkillIDs)}
        >
          {saving ? (
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
          ) : (
            <Check className="size-4" aria-hidden="true" />
          )}
          Save skills
        </button>
      </div>
    </div>
  );
}

function AddTaskSheet({
  week,
  projects,
  skills,
  goals,
  saving,
  onClose,
  onSave,
}: {
  week: string;
  projects: Project[];
  skills: Skill[];
  goals: Goal[];
  saving: boolean;
  onClose: () => void;
  onSave: (input: CreateTaskInput) => void;
}) {
  const titleRef = useRef<HTMLInputElement>(null);
  const [title, setTitle] = useState("");
  const [project, setProject] = useState<ProjectID>(projects[0]?.id ?? "");
  const [goalID, setGoalID] = useState("");
  const [selectedSkillIDs, setSelectedSkillIDs] = useState<number[]>([]);
  const defaultProject = projects[0]?.id ?? "";
  const canSave =
    project !== "" && title.trim().length > 0 && selectedSkillIDs.length > 0 && !saving;

  useEffect(() => {
    titleRef.current?.focus();
  }, []);

  useEffect(() => {
    if (project === "" && defaultProject !== "") {
      setProject(defaultProject);
      return;
    }
    if (project !== "" && !projects.some((candidate) => candidate.id === project)) {
      setProject(defaultProject);
    }
  }, [defaultProject, project, projects]);

  return (
    <div className="fixed inset-0 z-40 bg-black/55" role="dialog" aria-modal="true">
      <div className="absolute inset-x-0 bottom-0 max-h-[85vh] overflow-y-auto rounded-t-md border-t border-white/10 bg-zinc-950 p-4 shadow-2xl md:left-auto md:right-6 md:top-6 md:w-[32rem] md:rounded-md md:border">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-xs font-medium uppercase text-cyan-300">{week}</p>
            <h2 className="text-xl font-semibold text-white">Add task</h2>
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

        <form
          className="mt-4 space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            if (!canSave) {
              return;
            }

            onSave({
              week,
              title: title.trim(),
              project,
              goal_id: goalID === "" ? undefined : Number(goalID),
              skill_ids: selectedSkillIDs,
            });
          }}
        >
          <label className="block">
            <span className="text-sm font-medium text-zinc-300">Title</span>
            <input
              ref={titleRef}
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              className="mt-1 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
            />
          </label>

          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block">
              <span className="text-sm font-medium text-zinc-300">Project</span>
              <select
                value={project}
                onChange={(event) => setProject(event.target.value)}
                className="mt-1 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
              >
                {projects.map((value) => (
                  <option key={value.id} value={value.id}>
                    {value.label}
                  </option>
                ))}
              </select>
            </label>

            <label className="block">
              <span className="text-sm font-medium text-zinc-300">Goal</span>
              <select
                value={goalID}
                onChange={(event) => setGoalID(event.target.value)}
                className="mt-1 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
              >
                <option value="">None</option>
                {goals.map((goal) => (
                  <option key={goal.id} value={goal.id}>
                    {goal.code ? `${goal.code} · ` : ""}
                    {goal.title}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <SkillMultiSelect
            skills={skills}
            selectedIDs={selectedSkillIDs}
            label={skillsCopy.picker.label}
            helper={skillsCopy.picker.helper}
            required
            onToggle={(skillID) => setSelectedSkillIDs(toggleID(selectedSkillIDs, skillID))}
          />

          <button
            type="submit"
            disabled={!canSave}
            className="flex min-h-11 w-full items-center justify-center gap-2 rounded-md bg-cyan-300 px-4 font-semibold text-zinc-950 disabled:cursor-not-allowed disabled:bg-zinc-700 disabled:text-zinc-400"
          >
            {saving ? (
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            ) : (
              <Plus className="size-4" aria-hidden="true" />
            )}
            Add task
          </button>
        </form>
      </div>
    </div>
  );
}

function SkillMultiSelect({
  skills,
  selectedIDs,
  label,
  helper,
  required = false,
  onToggle,
}: {
  skills: Skill[];
  selectedIDs: number[];
  label: string;
  helper: string;
  required?: boolean;
  onToggle: (skillID: number) => void;
}) {
  return (
    <div>
      <p className="text-sm font-medium text-zinc-300">{label}</p>
      <p className="mt-1 text-xs leading-5 text-zinc-500">{helper}</p>
      {required && selectedIDs.length === 0 ? (
        <p className="mt-1 text-xs text-amber-200">{skillsCopy.picker.required}</p>
      ) : null}

      {skills.length === 0 ? (
        <p className="mt-2 rounded-md border border-white/10 bg-white/[0.03] p-3 text-sm text-zinc-400">
          {skillsCopy.tab.empty}
        </p>
      ) : (
        <div className="mt-2 grid gap-2">
          {skills.map((skill) => {
            const checked = selectedIDs.includes(skill.id);

            return (
              <label
                key={skill.id}
                className={[
                  "grid cursor-pointer grid-cols-[1.25rem_1fr] gap-3 rounded-md border p-3",
                  checked ? "border-cyan-300 bg-cyan-300/10" : "border-white/10 bg-white/[0.03]",
                ].join(" ")}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => onToggle(skill.id)}
                  className="mt-1 size-4 accent-cyan-300"
                />
                <span>
                  <span className="block text-sm font-medium text-white">{skill.name}</span>
                  <span className="mt-1 block text-xs leading-5 text-zinc-400">
                    {skill.associate_when}
                  </span>
                </span>
              </label>
            );
          })}
        </div>
      )}
    </div>
  );
}

function Counters({ counts }: { counts: DashboardResponse["counters"] }) {
  const nonzero = counts.filter((count) => count.count > 0);
  const text =
    nonzero.length > 0
      ? nonzero.map((count) => `${count.count} ${count.label.toLowerCase()}`).join(" · ")
      : "No logs yet";

  return (
    <section className="rounded-md border border-white/10 bg-white/[0.03] p-4">
      <p className="text-sm text-zinc-300">
        <span className="font-medium text-white">This week:</span> {text}
      </p>
    </section>
  );
}

function StreakChip({ days, countsToday }: { days: number; countsToday: boolean }) {
  return (
    <div
      className={[
        "min-h-11 rounded-md border px-4 py-2 text-sm font-semibold",
        countsToday
          ? "border-emerald-300 bg-emerald-300 text-zinc-950"
          : "border-white/10 bg-white/[0.04] text-zinc-400",
      ].join(" ")}
    >
      {days} day streak
    </div>
  );
}

function LevelRing({ profile, onClick }: { profile: GameProfile; onClick: () => void }) {
  const percent =
    profile.progress_required <= 0
      ? 0
      : Math.max(0, Math.min(100, (profile.progress_xp / profile.progress_required) * 100));

  return (
    <button
      type="button"
      aria-label={`Open ${gameCopy.profile}: level ${profile.level}, ${profile.title}`}
      className="grid size-14 shrink-0 place-items-center rounded-full p-1 text-zinc-950 shadow-lg shadow-black/25 outline-none focus-visible:ring-2 focus-visible:ring-cyan-300"
      style={{
        background: `conic-gradient(rgb(103 232 249) ${percent * 3.6}deg, rgba(255,255,255,0.12) 0deg)`,
      }}
      onClick={onClick}
    >
      <span className="grid size-full place-items-center rounded-full bg-zinc-950 text-sm font-bold text-white">
        {profile.level}
      </span>
    </button>
  );
}

function GameProfileSheet({
  profile,
  skills,
  onClose,
}: {
  profile: GameProfile;
  skills: GameSkill[];
  onClose: () => void;
}) {
  return (
    <div className="fixed inset-0 z-40 bg-black/55" role="dialog" aria-modal="true">
      <div className="absolute inset-x-0 bottom-0 max-h-[85vh] overflow-y-auto rounded-t-md border-t border-white/10 bg-zinc-950 p-4 shadow-2xl md:left-auto md:right-6 md:top-6 md:w-[32rem] md:rounded-md md:border">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-xs font-medium uppercase text-cyan-300">{gameCopy.profile}</p>
            <h2 className="text-xl font-semibold text-white">
              {gameCopy.level} {profile.level} · {profile.title}
            </h2>
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

        <div className="mt-4 grid gap-3 sm:grid-cols-3">
          <ProfileStat label={gameCopy.xp} value={String(profile.total_xp)} />
          <ProfileStat label="Streak" value={`${profile.streak.days}d`} />
          <ProfileStat label={gameCopy.nextLevel} value={`${profile.next_threshold} XP`} />
        </div>

        <div className="mt-4 rounded-md border border-white/10 bg-white/[0.03] p-3">
          <div className="flex items-center justify-between gap-2 text-xs">
            <span className="font-medium text-zinc-300">
              {profile.progress_xp}/{profile.progress_required} XP
            </span>
            {profile.streak.shielded ? (
              <span className="inline-flex items-center gap-1 text-cyan-200">
                <ShieldCheck className="size-3.5" aria-hidden="true" />
                {gameCopy.shielded}
              </span>
            ) : null}
          </div>
          <div
            className="mt-2 h-2 overflow-hidden rounded-full bg-white/10"
            role="progressbar"
            aria-label="Level progress"
            aria-valuemin={profile.current_threshold}
            aria-valuemax={profile.next_threshold}
            aria-valuenow={profile.total_xp}
          >
            <div
              className="h-full rounded-full bg-emerald-300"
              style={{
                width: `${
                  profile.progress_required <= 0
                    ? 0
                    : Math.max(
                        0,
                        Math.min(100, (profile.progress_xp / profile.progress_required) * 100),
                      )
                }%`,
              }}
            />
          </div>
        </div>

        <h3 className="mt-5 text-xs font-semibold uppercase text-zinc-400">{gameCopy.badges}</h3>
        {profile.badges.length === 0 ? (
          <p className="mt-2 rounded-md border border-white/10 bg-white/[0.03] p-3 text-sm text-zinc-400">
            {gameCopy.noBadges}
          </p>
        ) : (
          <div className="mt-2 grid gap-2 sm:grid-cols-2">
            {profile.badges.map((badge) => (
              <div
                key={badge.id}
                className="rounded-md border border-amber-300/30 bg-amber-300/10 p-3"
              >
                <p className="flex items-center gap-2 text-sm font-semibold text-amber-100">
                  <Award className="size-4" aria-hidden="true" />
                  {badge.name}
                </p>
                <p className="mt-1 text-xs leading-5 text-zinc-300">{badge.description}</p>
              </div>
            ))}
          </div>
        )}

        <h3 className="mt-5 text-xs font-semibold uppercase text-zinc-400">{gameCopy.skills}</h3>
        <div className="mt-2 space-y-3">
          {skills.map((skill) => (
            <div
              key={skill.skill_id}
              className="rounded-md border border-white/10 bg-white/[0.03] p-3"
            >
              <div className="mb-2 flex items-center justify-between gap-2">
                <p className="text-sm font-medium text-white">{skill.name}</p>
                {skill.target_tier ? (
                  <p className="text-xs font-medium text-cyan-200">target {skill.target_tier}</p>
                ) : null}
              </div>
              <GameSkillBar skill={skill} />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function ProfileStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-white/10 bg-white/[0.03] p-3">
      <p className="text-xs font-semibold uppercase text-zinc-400">{label}</p>
      <p className="mt-1 text-lg font-semibold text-white">{value}</p>
    </div>
  );
}

function QuickLogBar({
  categories,
  loading,
  onSelect,
}: {
  categories: Category[];
  loading: boolean;
  onSelect: (categoryID: string) => void;
}) {
  return (
    <section className="fixed inset-x-0 bottom-[4.75rem] z-20 px-3 md:bottom-5 md:left-auto md:right-5 md:w-[26rem]">
      <div className="mx-auto max-w-md rounded-md border border-white/10 bg-zinc-950/95 p-2 shadow-2xl shadow-black/40 backdrop-blur">
        {loading ? (
          <div className="grid min-h-24 place-items-center text-sm text-zinc-400">
            <Loader2 className="mr-2 inline size-4 animate-spin" aria-hidden="true" />
            Loading
          </div>
        ) : (
          <div className="grid grid-cols-4 gap-2">
            {categories.map((category) => (
              <button
                key={category.id}
                type="button"
                className="flex min-h-14 flex-col items-center justify-center rounded-md border border-white/10 bg-white/[0.03] px-1 text-center text-xs font-medium text-zinc-200"
                onClick={() => onSelect(category.id)}
              >
                <span className="text-lg" aria-hidden="true">
                  {category.icon}
                </span>
                <span>{shortCategoryLabel(category.label)}</span>
              </button>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

function QuickLogSheet({
  categories,
  goals,
  skills,
  selectedCategoryID,
  saving,
  onClose,
  onSave,
}: {
  categories: Category[];
  goals: Goal[];
  skills: Skill[];
  selectedCategoryID: string;
  saving: boolean;
  onClose: () => void;
  onSave: (input: Parameters<typeof createLog>[0]) => void;
}) {
  const titleRef = useRef<HTMLInputElement>(null);
  const selectedCategory = categories.find((category) => category.id === selectedCategoryID);
  const [title, setTitle] = useState("");
  const [noteOrURL, setNoteOrURL] = useState("");
  const [goalID, setGoalID] = useState("");
  const [occurredAt, setOccurredAt] = useState(toDateTimeLocal(new Date()));
  const [selectedSkillIDs, setSelectedSkillIDs] = useState<number[]>([]);

  useEffect(() => {
    titleRef.current?.focus();
  }, []);

  const canSave = title.trim().length > 0 && !saving;

  return (
    <div className="fixed inset-0 z-40 bg-black/55" role="dialog" aria-modal="true">
      <div className="absolute inset-x-0 bottom-0 max-h-[85vh] overflow-y-auto rounded-t-md border-t border-white/10 bg-zinc-950 p-4 shadow-2xl md:left-auto md:right-6 md:w-[28rem] md:rounded-md md:border">
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-xs font-medium uppercase text-cyan-300">Quick log</p>
            <h2 className="text-xl font-semibold text-white">
              {selectedCategory?.icon} {selectedCategory?.label ?? "Log"}
            </h2>
          </div>
          <button
            type="button"
            className="grid size-11 place-items-center rounded-md border border-white/10 text-zinc-300"
            aria-label="Close"
            onClick={onClose}
          >
            <X className="size-5" aria-hidden="true" />
          </button>
        </div>

        <form
          className="mt-4 space-y-3"
          onSubmit={(event) => {
            event.preventDefault();
            if (!canSave) {
              return;
            }
            onSave(
              buildLogInput(
                selectedCategoryID,
                title,
                noteOrURL,
                goalID,
                occurredAt,
                selectedSkillIDs,
              ),
            );
          }}
        >
          <label className="block">
            <span className="text-sm font-medium text-zinc-300">Title</span>
            <input
              ref={titleRef}
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              className="mt-1 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
            />
          </label>

          <label className="block">
            <span className="text-sm font-medium text-zinc-300">Note / URL</span>
            <input
              value={noteOrURL}
              onChange={(event) => setNoteOrURL(event.target.value)}
              className="mt-1 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
            />
          </label>

          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block">
              <span className="text-sm font-medium text-zinc-300">Goal</span>
              <select
                value={goalID}
                onChange={(event) => setGoalID(event.target.value)}
                className="mt-1 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
              >
                <option value="">None</option>
                {goals.map((goal) => (
                  <option key={goal.id} value={goal.id}>
                    {goal.code ? `${goal.code} · ` : ""}
                    {goal.title}
                  </option>
                ))}
              </select>
            </label>

            <label className="block">
              <span className="text-sm font-medium text-zinc-300">Occurred at</span>
              <input
                type="datetime-local"
                value={occurredAt}
                onChange={(event) => setOccurredAt(event.target.value)}
                className="mt-1 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
              />
            </label>
          </div>

          <SkillMultiSelect
            skills={skills}
            selectedIDs={selectedSkillIDs}
            label={skillsCopy.evidence.label}
            helper={skillsCopy.evidence.helper}
            onToggle={(skillID) => setSelectedSkillIDs(toggleID(selectedSkillIDs, skillID))}
          />

          <button
            type="submit"
            disabled={!canSave}
            className="flex min-h-11 w-full items-center justify-center gap-2 rounded-md bg-cyan-300 px-4 font-semibold text-zinc-950 disabled:cursor-not-allowed disabled:bg-zinc-700 disabled:text-zinc-400"
          >
            {saving ? (
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            ) : (
              <Plus className="size-4" aria-hidden="true" />
            )}
            Save
          </button>
        </form>
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

function useProjectFilter(): [ProjectID[], (projects: ProjectID[]) => void] {
  const [selected, setSelected] = useState<ProjectID[]>(() => parseProjectsFromURL());

  useEffect(() => {
    const onPopState = () => setSelected(parseProjectsFromURL());
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  const update = useCallback((next: ProjectID[]) => {
    setSelected(next);
    const params = new URLSearchParams(window.location.search);
    if (next.length > 0) {
      params.set("project", next.join(","));
    } else {
      params.delete("project");
    }
    const query = params.toString();
    window.history.replaceState(null, "", `${window.location.pathname}${query ? `?${query}` : ""}`);
  }, []);

  return [selected, update];
}

function parseProjectsFromURL(): ProjectID[] {
  const values = new URLSearchParams(window.location.search).get("project")?.split(",") ?? [];
  return values.map((value) => value.trim()).filter((value) => value !== "");
}

function toggleProject(selected: ProjectID[], project: ProjectID): ProjectID[] {
  if (selected.includes(project)) {
    return selected.filter((existing) => existing !== project);
  }
  return [...selected, project];
}

function filterKnownProjects(selected: ProjectID[], projects: Project[]): ProjectID[] {
  if (projects.length === 0) {
    return selected;
  }

  const known = new Set(projects.map((project) => project.id));
  return selected.filter((project) => known.has(project));
}

function replaceTask(dashboard: DashboardResponse, nextTask: Task): DashboardResponse {
  const existing = dashboard.tasks.find((task) => task.id === nextTask.id);
  const tasks = dashboard.tasks.map((task) => (task.id === nextTask.id ? nextTask : task));

  if (!existing) {
    return { ...dashboard, tasks };
  }

  const wasDone = existing.status === "done";
  const isDone = nextTask.status === "done";
  const done = dashboard.completion.done + (isDone ? 1 : 0) - (wasDone ? 1 : 0);

  return {
    ...dashboard,
    tasks,
    completion: {
      ...dashboard.completion,
      done,
    },
  };
}

function appendTask(dashboard: DashboardResponse, task: Task): DashboardResponse {
  if (dashboard.week.code !== task.week) {
    return dashboard;
  }

  return {
    ...dashboard,
    tasks: [...dashboard.tasks, task].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id),
    completion: {
      ...dashboard.completion,
      total: dashboard.completion.total + 1,
    },
  };
}

function coerceTask(value: unknown): Task | null {
  if (value && typeof value === "object" && "id" in value && "status" in value) {
    return value as Task;
  }
  return null;
}

function buildLogInput(
  categoryID: string,
  title: string,
  noteOrURL: string,
  goalID: string,
  occurredAt: string,
  skillIDs: number[],
) {
  const input: Parameters<typeof createLog>[0] = {
    category_id: categoryID,
    title: title.trim(),
    occurred_at: new Date(occurredAt).toISOString(),
  };

  const extra = noteOrURL.trim();
  if (extra.startsWith("http")) {
    input.url = extra;
  } else if (extra !== "") {
    input.note = extra;
  }

  if (goalID !== "") {
    input.goal_id = Number(goalID);
  }

  if (skillIDs.length > 0) {
    input.skill_ids = skillIDs;
  }

  return input;
}

function toggleID(ids: number[], id: number): number[] {
  if (ids.includes(id)) {
    return ids.filter((existing) => existing !== id);
  }
  return [...ids, id];
}

function sameIDs(a: number[], b: number[]): boolean {
  if (a.length !== b.length) {
    return false;
  }

  const seen = new Set(a);
  return b.every((id) => seen.has(id));
}

function skillDotClass(index: number): string {
  return ["bg-cyan-300", "bg-emerald-300", "bg-amber-300", "bg-fuchsia-300", "bg-sky-300"][
    index % 5
  ];
}

function toDateTimeLocal(date: Date): string {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

function shortCategoryLabel(label: string): string {
  return label
    .replace("Course module", "Module")
    .replace("Medium post", "Post")
    .replace("Benchmark/number", "Number");
}

function isTypingTarget(target: EventTarget | null): boolean {
  return (
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement
  );
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Request failed";
}
