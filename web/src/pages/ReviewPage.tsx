import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, CalendarDays, CheckCircle2, FileText, Loader2, Save } from "lucide-react";
import type { FormEvent } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "wouter";

import {
  createMetric,
  getCheckpoint,
  getDashboard,
  getMetricDefs,
  getMetrics,
  getReviews,
  saveCheckpoint,
  upsertReview,
  type Checkpoint,
  type DailyReview,
  type GameEnvelope,
  type Metric,
  type MetricDef,
  type UpsertReviewInput,
} from "../api/client";
import { FieldGuide } from "../components/FieldGuide";
import { LevelUpMoment } from "../components/LevelUpMoment";
import { gameCopy, gameToast } from "../copy/game";
import { reviewCopy } from "../copy/review";
import { addDays, currentLisbonDate, formatLisbonDayHeading, formatLisbonTime } from "../lib/date";
import { gameEventsQueryKey, gameProfileQueryKey, gameSkillsQueryKey } from "../lib/queryKeys";

const otherMetricValue = "__other__";

export function ReviewPage() {
  const queryClient = useQueryClient();
  const today = useMemo(() => currentLisbonDate(), []);
  const reviewRange = useMemo(() => ({ from: addDays(today, -60), to: today }), [today]);
  const reviewsQueryKey = useMemo(() => ["reviews", reviewRange] as const, [reviewRange]);
  const [selectedDate, setSelectedDate] = useState(today);
  const [learned, setLearned] = useState("");
  const [issue, setIssue] = useState("");
  const [nextStep, setNextStep] = useState("");
  const [minutes, setMinutes] = useState("");
  const [metricChoice, setMetricChoice] = useState("");
  const [otherMetricName, setOtherMetricName] = useState("");
  const [metricValue, setMetricValue] = useState("");
  const [metricUnit, setMetricUnit] = useState("");
  const [metricNote, setMetricNote] = useState("");
  const [checkpointAnswers, setCheckpointAnswers] = useState<string[]>([]);
  const [toast, setToast] = useState<string | null>(null);
  const [levelUp, setLevelUp] = useState<NonNullable<GameEnvelope["level_up"]> | null>(null);

  const dashboardQuery = useQuery({
    queryKey: ["dashboard", []],
    queryFn: () => getDashboard([]),
  });

  const reviewsQuery = useQuery({
    queryKey: reviewsQueryKey,
    queryFn: () => getReviews(reviewRange),
  });

  const metricDefsQuery = useQuery({
    queryKey: ["metric-defs"],
    queryFn: getMetricDefs,
  });

  const metricDefs = useMemo(() => metricDefsQuery.data ?? [], [metricDefsQuery.data]);

  useEffect(() => {
    if (metricChoice === "" && metricDefs[0]) {
      setMetricChoice(metricDefs[0].name);
    }
  }, [metricChoice, metricDefs]);

  const selectedMetricDef = metricDefs.find((definition) => definition.name === metricChoice);
  const metricName =
    metricChoice === otherMetricValue ? otherMetricName.trim() : (selectedMetricDef?.name ?? "");

  useEffect(() => {
    setMetricUnit(selectedMetricDef?.unit ?? "");
    setMetricValue("");
    setMetricNote("");
  }, [metricChoice, selectedMetricDef?.unit]);

  const metricsQuery = useQuery({
    queryKey: ["metrics", metricName],
    queryFn: () => getMetrics({ name: metricName, limit: 5 }),
    enabled: metricName !== "",
  });

  const reviews = useMemo(() => reviewsQuery.data ?? [], [reviewsQuery.data]);
  const reviewByDate = useMemo(
    () => new Map(reviews.map((review) => [review.date, review])),
    [reviews],
  );
  const selectedReview = reviewByDate.get(selectedDate);

  useEffect(() => {
    setLearned(selectedReview?.learned ?? "");
    setIssue(selectedReview?.issue ?? "");
    setNextStep(selectedReview?.next ?? "");
    setMinutes(selectedReview?.minutes == null ? "" : String(selectedReview.minutes));
  }, [selectedDate, selectedReview]);

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

  const reviewMutation = useMutation({
    mutationFn: (input: UpsertReviewInput) => upsertReview(input),
    onSuccess: (review) => {
      queryClient.setQueryData<DailyReview[]>(reviewsQueryKey, (current) =>
        mergeReviews(current ?? [], review),
      );
      void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
      handleGame(review.game);
      if (!review.game) {
        setToast(reviewCopy.daily.saved);
      }
    },
    onError: (error) => {
      setToast(error instanceof Error ? error.message : "Review save failed");
    },
  });

  const metricMutation = useMutation({
    mutationFn: (input: Parameters<typeof createMetric>[0]) => createMetric(input),
    onSuccess: (metric) => {
      queryClient.setQueryData<Metric[]>(["metrics", metric.name], (current) =>
        mergeMetrics(current ?? [], metric),
      );
      setMetricValue("");
      setMetricNote("");
      handleGame(metric.game);
      if (!metric.game) {
        setToast(reviewCopy.metrics.saved);
      }
    },
    onError: (error) => {
      setToast(error instanceof Error ? error.message : "Metric save failed");
    },
  });

  const checkpointWeek = checkpointWeekFor(dashboardQuery.data?.week.code ?? null);
  const checkpointQuery = useQuery({
    queryKey: ["checkpoint", checkpointWeek],
    queryFn: () => {
      if (!checkpointWeek) {
        throw new Error("checkpoint week is not active");
      }
      return getCheckpoint(checkpointWeek);
    },
    enabled: checkpointWeek !== null,
  });

  useEffect(() => {
    if (!checkpointQuery.data) {
      return;
    }

    setCheckpointAnswers(
      checkpointQuery.data.answers ??
        Array.from({ length: checkpointQuery.data.questions.length }, () => ""),
    );
  }, [checkpointQuery.data]);

  const checkpointMutation = useMutation({
    mutationFn: ({ week, answers }: { week: string; answers: string[] }) =>
      saveCheckpoint(week, answers),
    onSuccess: (checkpoint) => {
      queryClient.setQueryData<Checkpoint>(["checkpoint", checkpoint.week], checkpoint);
      setCheckpointAnswers(
        checkpoint.answers ?? Array.from({ length: checkpoint.questions.length }, () => ""),
      );
      handleGame(checkpoint.game);
      if (!checkpoint.game) {
        setToast(reviewCopy.checkpoint.saved);
      }
    },
    onError: (error) => {
      setToast(error instanceof Error ? error.message : "Checkpoint save failed");
    },
  });

  const canSaveReview =
    !reviewMutation.isPending &&
    [learned, issue, nextStep].some((value) => value.trim().length > 0);
  const canSaveMetric =
    !metricMutation.isPending &&
    metricName !== "" &&
    metricValue.trim() !== "" &&
    Number.isFinite(Number(metricValue));

  const saveReview = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSaveReview) {
      return;
    }

    reviewMutation.mutate(buildReviewInput(selectedDate, learned, issue, nextStep, minutes));
  };

  const saveMetric = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSaveMetric) {
      return;
    }

    const cleanUnit = cleanOptional(metricUnit) ?? selectedMetricDef?.unit ?? undefined;
    metricMutation.mutate({
      name: metricName,
      value: Number(metricValue),
      unit: cleanUnit,
      note: cleanOptional(metricNote),
    });
  };

  return (
    <section className="min-h-screen px-4 py-5 md:px-10 md:py-8">
      <div className="mx-auto grid max-w-6xl gap-5 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="flex min-w-0 flex-col gap-5">
          <header className="flex flex-col gap-2">
            <p className="text-sm font-medium uppercase text-cyan-300">
              {reviewCopy.daily.eyebrow}
            </p>
            <div className="flex flex-wrap items-end justify-between gap-3">
              <h1 className="text-3xl font-semibold text-white md:text-4xl">
                {reviewCopy.daily.heading}
              </h1>
              <p className="text-sm text-zinc-400">{formatLisbonDayHeading(selectedDate)}</p>
            </div>
          </header>

          <DailyReviewForm
            selectedDate={selectedDate}
            today={today}
            reviewedDates={reviews.map((review) => review.date)}
            showEmptyExample={!reviewsQuery.isLoading && reviews.length === 0}
            learned={learned}
            issue={issue}
            nextStep={nextStep}
            minutes={minutes}
            saving={reviewMutation.isPending}
            canSave={canSaveReview}
            onDateChange={setSelectedDate}
            onLearnedChange={setLearned}
            onIssueChange={setIssue}
            onNextStepChange={setNextStep}
            onMinutesChange={setMinutes}
            onSubmit={saveReview}
          />

          {checkpointWeek ? (
            <CheckpointSection
              checkpoint={checkpointQuery.data}
              loading={checkpointQuery.isLoading}
              error={checkpointQuery.isError ? errorMessage(checkpointQuery.error) : null}
              answers={checkpointAnswers}
              saving={checkpointMutation.isPending}
              onAnswerChange={(index, answer) => {
                setCheckpointAnswers((current) =>
                  current.map((existing, existingIndex) =>
                    existingIndex === index ? answer : existing,
                  ),
                );
              }}
              onSubmit={(event) => {
                event.preventDefault();
                if (checkpointQuery.data) {
                  checkpointMutation.mutate({
                    week: checkpointQuery.data.week,
                    answers: checkpointAnswers,
                  });
                }
              }}
            />
          ) : null}
        </div>

        <aside className="flex min-w-0 flex-col gap-5">
          <WeekCompletion dashboard={dashboardQuery.data} loading={dashboardQuery.isLoading} />

          <MetricQuickAdd
            definitions={metricDefs}
            readings={metricsQuery.data ?? []}
            definitionsLoading={metricDefsQuery.isLoading}
            readingsLoading={metricsQuery.isLoading && metricName !== ""}
            selectedChoice={metricChoice}
            selectedDefinition={selectedMetricDef}
            otherMetricName={otherMetricName}
            metricValue={metricValue}
            metricUnit={metricUnit}
            metricNote={metricNote}
            saving={metricMutation.isPending}
            canSave={canSaveMetric}
            onChoiceChange={setMetricChoice}
            onOtherNameChange={setOtherMetricName}
            onValueChange={setMetricValue}
            onUnitChange={setMetricUnit}
            onNoteChange={setMetricNote}
            onSubmit={saveMetric}
          />
        </aside>
      </div>

      <div className="mx-auto mt-5 max-w-6xl border-t border-white/10 pt-4 md:hidden">
        <Link
          href="/plan"
          className="inline-flex min-h-10 items-center justify-center gap-2 rounded-md border border-white/10 bg-zinc-900 px-3 text-sm font-semibold text-zinc-100"
        >
          <FileText className="size-4 text-cyan-300" aria-hidden="true" />
          Plan
        </Link>
      </div>

      {toast ? <Toast message={toast} onClose={() => setToast(null)} /> : null}
      {levelUp ? <LevelUpMoment levelUp={levelUp} onClose={() => setLevelUp(null)} /> : null}
    </section>
  );
}

function DailyReviewForm({
  selectedDate,
  today,
  reviewedDates,
  showEmptyExample,
  learned,
  issue,
  nextStep,
  minutes,
  saving,
  canSave,
  onDateChange,
  onLearnedChange,
  onIssueChange,
  onNextStepChange,
  onMinutesChange,
  onSubmit,
}: {
  selectedDate: string;
  today: string;
  reviewedDates: string[];
  showEmptyExample: boolean;
  learned: string;
  issue: string;
  nextStep: string;
  minutes: string;
  saving: boolean;
  canSave: boolean;
  onDateChange: (date: string) => void;
  onLearnedChange: (value: string) => void;
  onIssueChange: (value: string) => void;
  onNextStepChange: (value: string) => void;
  onMinutesChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <section className="rounded-md border border-white/10 bg-white/[0.03]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/10 px-4 py-3">
        <div>
          <h2 className="text-lg font-semibold text-white">{reviewCopy.daily.title}</h2>
          <p className="mt-1 text-sm text-zinc-400">{reviewCopy.daily.guideTitle}</p>
        </div>
        <div className="flex flex-wrap items-start gap-3">
          <label className="flex min-w-48 items-center gap-2 text-sm text-zinc-300">
            <CalendarDays className="size-4 text-cyan-300" aria-hidden="true" />
            <input
              type="date"
              list="reviewed-dates"
              max={today}
              value={selectedDate}
              onChange={(event) => onDateChange(event.target.value)}
              className="min-h-10 rounded-md border border-white/10 bg-zinc-900 px-3 text-sm text-white outline-none focus:border-cyan-300"
              aria-label={reviewCopy.daily.dateLabel}
            />
            <datalist id="reviewed-dates">
              {reviewedDates.map((date) => (
                <option key={date} value={date} />
              ))}
            </datalist>
          </label>
          <FieldGuide label={reviewCopy.daily.guideLabel}>
            <div className="space-y-2">
              <p>{reviewCopy.daily.fields.learned.guide}</p>
              <p>{reviewCopy.daily.fields.issue.guide}</p>
              <p>{reviewCopy.daily.fields.next.guide}</p>
              <p>{reviewCopy.daily.fields.minutes.guide}</p>
            </div>
          </FieldGuide>
        </div>
      </div>

      {reviewedDates.length > 0 ? (
        <div className="flex flex-wrap gap-2 border-b border-white/10 px-4 py-3">
          {reviewedDates.map((date) => (
            <button
              key={date}
              type="button"
              className={[
                "min-h-9 rounded-md border px-2 text-xs font-medium",
                date === selectedDate
                  ? "border-emerald-300 bg-emerald-300 text-zinc-950"
                  : "border-white/10 bg-zinc-950 text-zinc-300",
              ].join(" ")}
              aria-label={`Existing review ${date}`}
              onClick={() => onDateChange(date)}
            >
              {date.slice(5)}
            </button>
          ))}
        </div>
      ) : null}

      {showEmptyExample ? <DailyReviewExample /> : null}

      <form className="space-y-4 p-4" onSubmit={onSubmit}>
        <ReviewTextArea
          copy={reviewCopy.daily.fields.learned}
          value={learned}
          onChange={onLearnedChange}
        />
        <ReviewTextArea
          copy={reviewCopy.daily.fields.issue}
          value={issue}
          onChange={onIssueChange}
        />
        <ReviewTextArea
          copy={reviewCopy.daily.fields.next}
          value={nextStep}
          onChange={onNextStepChange}
        />

        <label className="block max-w-40">
          <span className="text-sm font-medium text-zinc-300">
            {reviewCopy.daily.fields.minutes.label}
          </span>
          <span className="mt-1 block text-xs leading-5 text-zinc-500">
            {reviewCopy.daily.fields.minutes.helper}
          </span>
          <input
            type="number"
            min="0"
            inputMode="numeric"
            value={minutes}
            aria-label={reviewCopy.daily.fields.minutes.label}
            onChange={(event) => onMinutesChange(event.target.value)}
            className="mt-2 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
          />
        </label>

        <button
          type="submit"
          disabled={!canSave}
          className="inline-flex min-h-11 items-center justify-center gap-2 rounded-md bg-cyan-300 px-4 font-semibold text-zinc-950 disabled:cursor-not-allowed disabled:bg-zinc-700 disabled:text-zinc-400"
        >
          {saving ? (
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
          ) : (
            <Save className="size-4" aria-hidden="true" />
          )}
          {reviewCopy.daily.save}
        </button>
      </form>
    </section>
  );
}

function ReviewTextArea({
  copy,
  value,
  onChange,
}: {
  copy: {
    label: string;
    helper: string;
    placeholder: string;
  };
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="block">
      <span className="text-sm font-medium text-zinc-300">{copy.label}</span>
      <span className="mt-1 block text-xs leading-5 text-zinc-500">{copy.helper}</span>
      <textarea
        value={value}
        aria-label={copy.label}
        onChange={(event) => onChange(event.target.value)}
        placeholder={copy.placeholder}
        rows={3}
        className="mt-2 w-full resize-y rounded-md border border-white/10 bg-zinc-900 px-3 py-2 text-base leading-6 text-white outline-none placeholder:text-zinc-600 focus:border-cyan-300"
      />
    </label>
  );
}

function DailyReviewExample() {
  return (
    <div className="border-b border-white/10 px-4 py-3">
      <div className="rounded-md border border-dashed border-cyan-300/40 bg-cyan-300/5 px-3 py-3 text-sm text-zinc-300">
        <p className="text-xs font-semibold uppercase text-cyan-200">
          {reviewCopy.daily.emptyExampleLabel}
        </p>
        <dl className="mt-2 grid gap-2">
          <div>
            <dt className="text-zinc-500">{reviewCopy.daily.fields.learned.label}</dt>
            <dd>{reviewCopy.daily.emptyExample.learned}</dd>
          </div>
          <div>
            <dt className="text-zinc-500">{reviewCopy.daily.fields.issue.label}</dt>
            <dd>{reviewCopy.daily.emptyExample.issue}</dd>
          </div>
          <div>
            <dt className="text-zinc-500">{reviewCopy.daily.fields.next.label}</dt>
            <dd>{reviewCopy.daily.emptyExample.next}</dd>
          </div>
        </dl>
      </div>
    </div>
  );
}

function WeekCompletion({
  dashboard,
  loading,
}: {
  dashboard: Awaited<ReturnType<typeof getDashboard>> | undefined;
  loading: boolean;
}) {
  const done = dashboard?.completion.done ?? 0;
  const total = dashboard?.completion.total ?? 0;
  const percentage = total > 0 ? Math.round((done / total) * 100) : 0;

  return (
    <section className="rounded-md border border-white/10 bg-white/[0.03] p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-semibold uppercase text-zinc-400">{reviewCopy.weekPct.title}</p>
        {loading ? (
          <Loader2
            className="size-4 animate-spin text-zinc-400"
            aria-label={reviewCopy.weekPct.loading}
          />
        ) : null}
      </div>
      <p className="mt-2 text-2xl font-semibold text-white">
        {done}/{total}
      </p>
      <div
        className="mt-3 h-3 overflow-hidden rounded-sm bg-zinc-800"
        role="progressbar"
        aria-label="Week completion"
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={done}
      >
        <div className="h-full bg-emerald-300" style={{ width: `${percentage}%` }} />
      </div>
      <p className="mt-2 text-xs leading-5 text-zinc-500">{reviewCopy.weekPct.helper}</p>
    </section>
  );
}

function MetricQuickAdd({
  definitions,
  readings,
  definitionsLoading,
  readingsLoading,
  selectedChoice,
  selectedDefinition,
  otherMetricName,
  metricValue,
  metricUnit,
  metricNote,
  saving,
  canSave,
  onChoiceChange,
  onOtherNameChange,
  onValueChange,
  onUnitChange,
  onNoteChange,
  onSubmit,
}: {
  definitions: MetricDef[];
  readings: Metric[];
  definitionsLoading: boolean;
  readingsLoading: boolean;
  selectedChoice: string;
  selectedDefinition: MetricDef | undefined;
  otherMetricName: string;
  metricValue: string;
  metricUnit: string;
  metricNote: string;
  saving: boolean;
  canSave: boolean;
  onChoiceChange: (value: string) => void;
  onOtherNameChange: (value: string) => void;
  onValueChange: (value: string) => void;
  onUnitChange: (value: string) => void;
  onNoteChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  const unitLocked = selectedDefinition?.unit != null && selectedDefinition.unit !== "";

  return (
    <section className="rounded-md border border-white/10 bg-white/[0.03]">
      <div className="flex items-start justify-between gap-3 border-b border-white/10 px-4 py-3">
        <div>
          <h2 className="text-lg font-semibold text-white">{reviewCopy.metrics.title}</h2>
          <p className="mt-1 text-sm text-zinc-400">{reviewCopy.metrics.helper}</p>
        </div>
        <div className="flex items-start gap-2">
          <Activity className="mt-3 size-5 text-cyan-300" aria-hidden="true" />
          <FieldGuide label={reviewCopy.metrics.guideLabel}>
            <p>{reviewCopy.metrics.guideTitle}</p>
          </FieldGuide>
        </div>
      </div>

      <form className="space-y-3 p-4" onSubmit={onSubmit}>
        <label className="block">
          <span className="text-sm font-medium text-zinc-300">
            {reviewCopy.metrics.fields.metric.label}
          </span>
          <span className="mt-1 block text-xs leading-5 text-zinc-500">
            {reviewCopy.metrics.fields.metric.helper}
          </span>
          <select
            value={selectedChoice}
            aria-label={reviewCopy.metrics.fields.metric.label}
            onChange={(event) => onChoiceChange(event.target.value)}
            className="mt-2 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
          >
            {definitionsLoading ? (
              <option value="">{reviewCopy.metrics.loadingDefinitions}</option>
            ) : null}
            {definitions.map((definition) => (
              <option key={definition.name} value={definition.name}>
                {definition.name}
              </option>
            ))}
            <option value={otherMetricValue}>{reviewCopy.metrics.otherOption}</option>
          </select>
        </label>

        {selectedChoice === otherMetricValue ? (
          <label className="block">
            <span className="text-sm font-medium text-zinc-300">
              {reviewCopy.metrics.fields.otherName.label}
            </span>
            <span className="mt-1 block text-xs leading-5 text-zinc-500">
              {reviewCopy.metrics.fields.otherName.helper}
            </span>
            <input
              value={otherMetricName}
              aria-label={reviewCopy.metrics.fields.otherName.label}
              onChange={(event) => onOtherNameChange(event.target.value)}
              placeholder={reviewCopy.metrics.fields.otherName.placeholder}
              className="mt-2 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none placeholder:text-zinc-600 focus:border-cyan-300"
            />
          </label>
        ) : null}

        {selectedDefinition ? (
          <div className="rounded-md border border-white/10 bg-zinc-950 px-3 py-2 text-sm text-zinc-300">
            <p className="font-medium text-white">{selectedDefinition.name}</p>
            {selectedDefinition.definition ? (
              <p className="mt-1">{selectedDefinition.definition}</p>
            ) : null}
            {selectedDefinition.how_to_measure ? (
              <p className="mt-1 text-zinc-400">{selectedDefinition.how_to_measure}</p>
            ) : null}
            <p className="mt-1 text-xs text-zinc-500">
              {selectedDefinition.baseline ? `baseline ${selectedDefinition.baseline}` : ""}
              {selectedDefinition.baseline && selectedDefinition.target ? " · " : ""}
              {selectedDefinition.target ? `target ${selectedDefinition.target}` : ""}
            </p>
          </div>
        ) : null}

        <div className="grid gap-3 sm:grid-cols-[1fr_7rem] lg:grid-cols-1">
          <label className="block">
            <span className="text-sm font-medium text-zinc-300">
              {reviewCopy.metrics.fields.value.label}
            </span>
            <span className="mt-1 block text-xs leading-5 text-zinc-500">
              {reviewCopy.metrics.fields.value.helper}
            </span>
            <input
              type="number"
              step="any"
              inputMode="decimal"
              value={metricValue}
              aria-label={reviewCopy.metrics.fields.value.label}
              onChange={(event) => onValueChange(event.target.value)}
              placeholder={reviewCopy.metrics.fields.value.placeholder}
              className="mt-2 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none placeholder:text-zinc-600 focus:border-cyan-300"
            />
          </label>

          <label className="block">
            <span className="text-sm font-medium text-zinc-300">
              {reviewCopy.metrics.fields.unit.label}
            </span>
            <span className="mt-1 block text-xs leading-5 text-zinc-500">
              {reviewCopy.metrics.fields.unit.helper}
            </span>
            <input
              value={metricUnit}
              aria-label={reviewCopy.metrics.fields.unit.label}
              readOnly={unitLocked}
              onChange={(event) => onUnitChange(event.target.value)}
              placeholder={reviewCopy.metrics.fields.unit.placeholder}
              className={[
                "mt-2 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none placeholder:text-zinc-600 focus:border-cyan-300",
                unitLocked ? "cursor-not-allowed text-zinc-400" : "",
              ].join(" ")}
            />
          </label>
        </div>

        <label className="block">
          <span className="text-sm font-medium text-zinc-300">
            {reviewCopy.metrics.fields.note.label}
          </span>
          <span className="mt-1 block text-xs leading-5 text-zinc-500">
            {reviewCopy.metrics.fields.note.helper}
          </span>
          <input
            value={metricNote}
            aria-label={reviewCopy.metrics.fields.note.label}
            onChange={(event) => onNoteChange(event.target.value)}
            placeholder={reviewCopy.metrics.fields.note.placeholder}
            className="mt-2 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none placeholder:text-zinc-600 focus:border-cyan-300"
          />
        </label>

        <button
          type="submit"
          disabled={!canSave}
          className="inline-flex min-h-11 w-full items-center justify-center gap-2 rounded-md bg-cyan-300 px-4 font-semibold text-zinc-950 disabled:cursor-not-allowed disabled:bg-zinc-700 disabled:text-zinc-400"
        >
          {saving ? (
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
          ) : (
            <Save className="size-4" aria-hidden="true" />
          )}
          {reviewCopy.metrics.save}
        </button>
      </form>

      <div className="border-t border-white/10 px-4 py-3 text-sm text-zinc-300">
        {readingsLoading ? (
          <span>
            <Loader2 className="mr-2 inline size-4 animate-spin" aria-hidden="true" />
            {reviewCopy.metrics.loadingReadings}
          </span>
        ) : readings.length > 0 ? (
          <span>
            {reviewCopy.metrics.lastReadings}: {readings.map(readingText).join(" · ")}
          </span>
        ) : (
          <span>{reviewCopy.metrics.noReadings}</span>
        )}
      </div>
    </section>
  );
}

function CheckpointSection({
  checkpoint,
  loading,
  error,
  answers,
  saving,
  onAnswerChange,
  onSubmit,
}: {
  checkpoint: Checkpoint | undefined;
  loading: boolean;
  error: string | null;
  answers: string[];
  saving: boolean;
  onAnswerChange: (index: number, answer: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <section className="rounded-md border border-amber-300/40 bg-amber-300/10">
      <div className="flex items-start justify-between gap-3 border-b border-amber-300/20 px-4 py-3">
        <div>
          <h2 className="text-lg font-semibold text-white">
            {checkpoint ? `${checkpoint.week} ${gameCopy.bossReview}` : gameCopy.bossReview}
          </h2>
          <p className="mt-1 text-sm text-amber-100/80">{reviewCopy.checkpoint.helper}</p>
        </div>
        <div className="flex items-start gap-2">
          <CheckCircle2 className="mt-3 size-5 text-amber-200" aria-hidden="true" />
          <FieldGuide label={reviewCopy.checkpoint.guideLabel}>
            <p>{reviewCopy.checkpoint.helper}</p>
          </FieldGuide>
        </div>
      </div>

      {loading ? (
        <div className="p-4 text-sm text-zinc-300">
          <Loader2 className="mr-2 inline size-4 animate-spin" aria-hidden="true" />
          {reviewCopy.checkpoint.loading}
        </div>
      ) : null}

      {error ? <div className="p-4 text-sm text-red-100">{error}</div> : null}

      {checkpoint ? (
        <form className="space-y-4 p-4" onSubmit={onSubmit}>
          {checkpoint.questions.map((question, index) => (
            <label key={question} className="block">
              <span className="text-sm font-medium text-zinc-200">{question}</span>
              {checkpoint.helpers?.[index] ? (
                <span className="mt-1 block text-xs leading-5 text-amber-100/70">
                  {checkpoint.helpers[index]}
                </span>
              ) : null}
              <textarea
                value={answers[index] ?? ""}
                aria-label={question}
                onChange={(event) => onAnswerChange(index, event.target.value)}
                placeholder={reviewCopy.checkpoint.answerPlaceholder}
                rows={2}
                className="mt-2 w-full resize-y rounded-md border border-white/10 bg-zinc-950 px-3 py-2 text-base leading-6 text-white outline-none placeholder:text-zinc-600 focus:border-amber-200"
              />
            </label>
          ))}

          <button
            type="submit"
            disabled={saving}
            className="inline-flex min-h-11 items-center justify-center gap-2 rounded-md bg-amber-200 px-4 font-semibold text-zinc-950 disabled:cursor-wait disabled:bg-zinc-700 disabled:text-zinc-400"
          >
            {saving ? (
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            ) : (
              <Save className="size-4" aria-hidden="true" />
            )}
            {reviewCopy.checkpoint.save}
          </button>
        </form>
      ) : null}
    </section>
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

function buildReviewInput(
  date: string,
  learned: string,
  issue: string,
  nextStep: string,
  minutes: string,
): UpsertReviewInput {
  const input: UpsertReviewInput = { date };
  const cleanLearned = cleanOptional(learned);
  const cleanIssue = cleanOptional(issue);
  const cleanNext = cleanOptional(nextStep);
  const cleanMinutes = cleanOptional(minutes);

  if (cleanLearned) {
    input.learned = cleanLearned;
  }
  if (cleanIssue) {
    input.issue = cleanIssue;
  }
  if (cleanNext) {
    input.next = cleanNext;
  }
  if (cleanMinutes) {
    input.minutes = Number(cleanMinutes);
  }

  return input;
}

function cleanOptional(value: string): string | undefined {
  const clean = value.trim();
  return clean === "" ? undefined : clean;
}

function mergeReviews(current: DailyReview[], next: DailyReview): DailyReview[] {
  return [next, ...current.filter((review) => review.date !== next.date)].sort((a, b) =>
    b.date.localeCompare(a.date),
  );
}

function mergeMetrics(current: Metric[], next: Metric): Metric[] {
  return [next, ...current.filter((metric) => metric.id !== next.id)].slice(0, 5);
}

function checkpointWeekFor(week: string | null): string | null {
  return week === "W12" || week === "B7" ? week : null;
}

function readingText(metric: Metric): string {
  const unit = metric.unit ?? "";
  const note = metric.note ? ` (${metric.note})` : "";
  return `${metric.value}${unit} ${formatLisbonTime(metric.recorded_at)}${note}`;
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Request failed";
}
