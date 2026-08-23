import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, Loader2, ScrollText, Target, X } from "lucide-react";
import { useState } from "react";

import {
  getGameSkills,
  getSkill,
  getSkills,
  type GameSkill,
  type Skill,
  type Task,
} from "../api/client";
import { GameSkillBar } from "../components/GameSkillBar";
import { skillsCopy } from "../copy/skills";
import { gameSkillsQueryKey, skillsQueryKey } from "../lib/queryKeys";

export function SkillsPage() {
  const [selectedID, setSelectedID] = useState<number | null>(null);

  const skillsQuery = useQuery({
    queryKey: skillsQueryKey,
    queryFn: getSkills,
  });

  const gameSkillsQuery = useQuery({
    queryKey: gameSkillsQueryKey,
    queryFn: getGameSkills,
  });

  const skills = skillsQuery.data?.skills ?? [];
  const unlinked = skillsQuery.data?.unlinked_tasks ?? [];
  const gameSkills = new Map(
    (gameSkillsQuery.data?.skills ?? []).map((skill) => [skill.skill_id, skill]),
  );

  return (
    <section className="min-h-screen px-4 py-5 md:px-10 md:py-8">
      <div className="mx-auto flex max-w-5xl flex-col gap-5">
        <header className="flex flex-col gap-2">
          <p className="text-sm font-medium uppercase text-cyan-300">{skillsCopy.tab.eyebrow}</p>
          <h1 className="text-3xl font-semibold text-white md:text-4xl">{skillsCopy.tab.title}</h1>
          <p className="max-w-2xl text-sm text-zinc-400">{skillsCopy.tab.intro}</p>
        </header>

        {skillsQuery.isLoading ? <LoadingState /> : null}
        {skillsQuery.isError ? <ErrorState message={errorMessage(skillsQuery.error)} /> : null}

        {unlinked.length > 0 ? (
          <p
            role="status"
            className="flex items-center gap-2 rounded-md border border-amber-300/40 bg-amber-300/10 px-4 py-3 text-sm text-amber-100"
          >
            <AlertTriangle className="size-4 shrink-0" aria-hidden="true" />
            {unlinked.length} {unlinked.length === 1 ? "task" : "tasks"}{" "}
            {skillsCopy.picker.needsSkill}
            {" — "}
            {skillsCopy.picker.needsSkillHelper}
          </p>
        ) : null}

        {skillsQuery.data && skills.length === 0 ? (
          <p className="rounded-md border border-white/10 bg-white/[0.03] p-5 text-sm text-zinc-300">
            {skillsCopy.tab.empty}
          </p>
        ) : null}

        {skills.length > 0 ? (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {skills.map((skill) => (
              <SkillCard
                key={skill.id}
                skill={skill}
                gameSkill={gameSkills.get(skill.id)}
                onSelect={() => setSelectedID(skill.id)}
              />
            ))}
          </div>
        ) : null}
      </div>

      {selectedID !== null ? (
        <SkillDetailSheet id={selectedID} onClose={() => setSelectedID(null)} />
      ) : null}
    </section>
  );
}

function SkillCard({
  skill,
  gameSkill,
  onSelect,
}: {
  skill: Skill;
  gameSkill?: GameSkill;
  onSelect: () => void;
}) {
  const percent = skill.task_count === 0 ? 0 : (skill.task_done / skill.task_count) * 100;

  return (
    <button
      type="button"
      onClick={onSelect}
      aria-label={`${skill.name}: ${skillsCopy.card.progress(skill.task_done, skill.task_count)}`}
      className="flex min-h-11 flex-col gap-3 rounded-md border border-white/10 bg-white/[0.03] p-4 text-left transition hover:border-white/25"
    >
      <div className="flex items-start justify-between gap-2">
        <span className="text-base font-medium text-white">{skill.name}</span>
        {skill.target_tier ? (
          <span className="inline-flex shrink-0 items-center gap-1 rounded-sm bg-cyan-300/15 px-2 py-1 text-xs font-medium text-cyan-200">
            <Target className="size-3" aria-hidden="true" />
            {skill.target_tier}
          </span>
        ) : null}
      </div>

      <div>
        <div
          className="h-1.5 w-full overflow-hidden rounded-full bg-white/10"
          role="progressbar"
          aria-label={`${skill.name} task completion`}
          aria-valuenow={skill.task_done}
          aria-valuemin={0}
          aria-valuemax={skill.task_count}
        >
          <div className="h-full rounded-full bg-emerald-300" style={{ width: `${percent}%` }} />
        </div>
        <p className="mt-2 text-xs text-zinc-400">
          {skill.task_count === 0
            ? skillsCopy.card.noTasks
            : skillsCopy.card.progress(skill.task_done, skill.task_count)}
        </p>
      </div>

      {gameSkill ? <GameSkillBar skill={gameSkill} /> : null}

      <p className="flex items-center justify-between text-xs text-zinc-400">
        <span className="inline-flex items-center gap-1">
          <ScrollText className="size-3.5" aria-hidden="true" />
          {skill.evidence_count}
        </span>
        <span>
          {skill.last_activity ? skill.last_activity.slice(0, 10) : skillsCopy.card.neverTouched}
        </span>
      </p>
    </button>
  );
}

function SkillDetailSheet({ id, onClose }: { id: number; onClose: () => void }) {
  const detailQuery = useQuery({
    queryKey: [...skillsQueryKey, id],
    queryFn: () => getSkill(id),
  });

  const detail = detailQuery.data;

  // Grouped by week so the evidence trail reads in plan order, the way an interviewer would
  // ask about it.
  const byWeek = new Map<string, Task[]>();
  for (const task of detail?.tasks ?? []) {
    byWeek.set(task.week, [...(byWeek.get(task.week) ?? []), task]);
  }

  return (
    <div className="fixed inset-0 z-40 bg-black/55" role="dialog" aria-modal="true">
      <div className="absolute inset-x-0 bottom-0 max-h-[85vh] overflow-y-auto rounded-t-md border-t border-white/10 bg-zinc-950 p-4 shadow-2xl md:left-auto md:right-6 md:top-6 md:w-[32rem] md:rounded-md md:border">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-xs font-medium uppercase text-cyan-300">
              {detail?.target_tier
                ? `${skillsCopy.card.targetPrefix} ${detail.target_tier}`
                : skillsCopy.card.noTarget}
            </p>
            <h2 className="text-xl font-semibold text-white">{detail?.name ?? "Skill"}</h2>
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

        {detailQuery.isLoading ? <LoadingState /> : null}
        {detailQuery.isError ? <ErrorState message={errorMessage(detailQuery.error)} /> : null}

        {detail ? (
          <>
            <p className="mt-4 text-sm text-zinc-200">{detail.description}</p>

            {/* The affordance for correct tagging: it says when to pick this, not just what it is. */}
            <div className="mt-4 rounded-md border border-cyan-300/30 bg-cyan-300/5 p-3">
              <p className="text-xs font-semibold uppercase text-cyan-200">
                {skillsCopy.detail.associateWhenLabel}
              </p>
              <p className="mt-1 text-sm text-zinc-200">{detail.associate_when}</p>
            </div>

            <h3 className="mt-5 text-xs font-semibold uppercase text-zinc-400">
              {skillsCopy.detail.tasksHeading}
            </h3>
            {detail.tasks.length === 0 ? (
              <p className="mt-2 text-sm text-zinc-500">{skillsCopy.detail.noTasks}</p>
            ) : (
              <div className="mt-2 space-y-3">
                {[...byWeek.entries()].map(([week, tasks]) => (
                  <div key={week}>
                    <p className="text-xs font-medium text-zinc-500">{week}</p>
                    <ul className="mt-1 space-y-1">
                      {tasks.map((task) => (
                        <li
                          key={task.id}
                          className={[
                            "text-sm",
                            task.status === "done" ? "text-zinc-500 line-through" : "text-zinc-200",
                          ].join(" ")}
                        >
                          {task.title}
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            )}

            <h3 className="mt-5 text-xs font-semibold uppercase text-zinc-400">
              {skillsCopy.detail.evidenceHeading}
            </h3>
            {detail.evidence.length === 0 ? (
              <p className="mt-2 text-sm text-zinc-500">{skillsCopy.detail.noEvidence}</p>
            ) : (
              <ul className="mt-2 space-y-2">
                {detail.evidence.map((entry) => (
                  <li key={entry.id} className="text-sm text-zinc-200">
                    <span aria-hidden="true">{entry.icon} </span>
                    {entry.title}
                    <span className="ml-2 text-xs text-zinc-500">
                      {entry.occurred_at.slice(0, 10)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </>
        ) : null}
      </div>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="grid min-h-32 place-items-center rounded-md border border-white/10 bg-white/[0.03] text-zinc-300">
      <Loader2 className="mr-2 inline size-4 animate-spin" aria-hidden="true" />
      Loading
    </div>
  );
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="mt-4 rounded-md border border-red-300/30 bg-red-300/10 p-4 text-red-100">
      {message}
    </div>
  );
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Request failed";
}
