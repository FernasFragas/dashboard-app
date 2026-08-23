import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  CheckCircle2,
  FileText,
  Loader2,
  QrCode,
  Smartphone,
  Upload,
} from "lucide-react";
import type { ChangeEvent, FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { Link } from "wouter";

import {
  applyPlan,
  createPair,
  getPlan,
  getPairSVG,
  previewPlan,
  type ApplyPlanResponse,
  type PairResponse,
  type PlanAtRisk,
  type PlanChangeCounts,
  type PlanCounts,
  type PlanPreviewResponse,
} from "../api/client";
import { pairCopy } from "../copy/pair";
import { planCopy } from "../copy/plan";

export function PlanPage() {
  const queryClient = useQueryClient();
  const [source, setSource] = useState("");
  const [fileName, setFileName] = useState<string | null>(null);
  const [confirmName, setConfirmName] = useState("");
  const [preview, setPreview] = useState<PlanPreviewResponse | null>(null);
  const [applied, setApplied] = useState<ApplyPlanResponse | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const planQuery = useQuery({
    queryKey: ["plan"],
    queryFn: getPlan,
  });

  const previewMutation = useMutation({
    mutationFn: (markdown: string) => previewPlan(markdown),
    onSuccess: (result) => {
      setPreview(result);
      setConfirmName("");
      setApplied(null);
      setMessage(null);
    },
    onError: (error) => {
      setMessage(error instanceof Error ? error.message : "Preview failed");
    },
  });

  const applyMutation = useMutation({
    mutationFn: () => {
      if (!preview?.source_sha256) {
        throw new Error("Preview the plan first");
      }
      return applyPlan({
        source,
        source_sha256: preview.source_sha256,
        confirm_plan_name: confirmName,
      });
    },
    onSuccess: (result) => {
      setApplied(result);
      setMessage(null);
      setPreview(null);
      void queryClient.invalidateQueries({ queryKey: ["plan"] });
      void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
      void queryClient.invalidateQueries({ queryKey: ["categories"] });
      void queryClient.invalidateQueries({ queryKey: ["goals"] });
      void queryClient.invalidateQueries({ queryKey: ["skills"] });
      void queryClient.invalidateQueries({ queryKey: ["metric-defs"] });
    },
    onError: (error) => {
      setMessage(error instanceof Error ? error.message : "Apply failed");
    },
  });

  const cleanPreview = preview != null && preview.errors.length === 0 && preview.plan != null;
  const replacementNeedsConfirm = cleanPreview && preview.mode === "replace";
  const canApply =
    cleanPreview &&
    !applyMutation.isPending &&
    preview.source_sha256 != null &&
    (!replacementNeedsConfirm || confirmName === preview.plan?.name);

  const currentCounts = planQuery.data?.counts;
  const sourceLabel = useMemo(() => {
    if (fileName) {
      return fileName;
    }
    if (source.trim() !== "") {
      return "Pasted markdown";
    }
    return null;
  }, [fileName, source]);

  const submitPreview = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (source.trim() === "" || previewMutation.isPending) {
      return;
    }
    previewMutation.mutate(source);
  };

  const chooseFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }

    setFileName(file.name);
    setSource(await file.text());
    setPreview(null);
    setApplied(null);
    setMessage(null);
  };

  return (
    <section className="min-h-screen px-4 py-5 md:px-10 md:py-8">
      <div className="mx-auto flex max-w-6xl flex-col gap-5">
        <header className="flex flex-col gap-2">
          <p className="text-sm font-medium uppercase text-cyan-300">{planCopy.current.eyebrow}</p>
          <div className="flex flex-wrap items-end justify-between gap-3">
            <h1 className="text-3xl font-semibold text-white md:text-4xl">
              {planCopy.current.heading}
            </h1>
            {planQuery.isLoading ? (
              <Loader2 className="size-5 animate-spin text-zinc-400" aria-label="Loading plan" />
            ) : null}
          </div>
        </header>

        <section className="rounded-md border border-white/10 bg-white/[0.03] p-4">
          {planQuery.data?.plan ? (
            <div className="grid gap-4 md:grid-cols-[1fr_auto]">
              <div>
                <h2 className="text-xl font-semibold text-white">{planQuery.data.plan.name}</h2>
                <p className="mt-1 font-mono text-sm text-zinc-500">{planQuery.data.plan.id}</p>
                <p className="mt-2 text-sm text-zinc-400">
                  Loaded {formatDateTime(planQuery.data.plan.loaded_at)}
                </p>
                <p className="mt-1 text-sm text-zinc-500">
                  {planQuery.data.source
                    ? `Stored source ${shortHash(planQuery.data.source.sha256)}`
                    : planCopy.current.noSource}
                </p>
              </div>
              {currentCounts ? <CountsGrid counts={currentCounts} /> : null}
            </div>
          ) : (
            <p className="text-zinc-400">{planCopy.current.noPlan}</p>
          )}
        </section>

        <PairPhoneSection />

        <form
          className="rounded-md border border-white/10 bg-white/[0.03]"
          onSubmit={submitPreview}
        >
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/10 px-4 py-3">
            <div>
              <h2 className="text-lg font-semibold text-white">{planCopy.load.title}</h2>
              {sourceLabel ? <p className="mt-1 text-sm text-zinc-400">{sourceLabel}</p> : null}
            </div>
            <label className="inline-flex min-h-10 cursor-pointer items-center justify-center gap-2 rounded-md border border-white/10 bg-zinc-900 px-3 text-sm font-semibold text-zinc-100 hover:border-cyan-300">
              <Upload className="size-4 text-cyan-300" aria-hidden="true" />
              {planCopy.load.fileLabel}
              <input
                type="file"
                accept=".md,text/markdown,text/plain"
                className="sr-only"
                onChange={chooseFile}
              />
            </label>
          </div>

          <div className="space-y-3 p-4">
            <label className="block">
              <span className="text-sm font-medium text-zinc-300">{planCopy.load.sourceLabel}</span>
              <textarea
                value={source}
                rows={12}
                aria-label={planCopy.load.sourceLabel}
                onChange={(event) => {
                  setSource(event.target.value);
                  setFileName(null);
                  setPreview(null);
                  setApplied(null);
                  setMessage(null);
                }}
                placeholder={planCopy.load.sourcePlaceholder}
                className="mt-2 w-full resize-y rounded-md border border-white/10 bg-zinc-900 px-3 py-2 font-mono text-sm leading-6 text-white outline-none placeholder:text-zinc-600 focus:border-cyan-300"
              />
            </label>

            <button
              type="submit"
              disabled={source.trim() === "" || previewMutation.isPending}
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-md bg-cyan-300 px-4 font-semibold text-zinc-950 disabled:cursor-not-allowed disabled:bg-zinc-700 disabled:text-zinc-400"
            >
              {previewMutation.isPending ? (
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              ) : (
                <FileText className="size-4" aria-hidden="true" />
              )}
              {planCopy.load.preview}
            </button>
          </div>
        </form>

        {preview ? (
          <PreviewPanel
            preview={preview}
            confirmName={confirmName}
            canApply={canApply}
            applying={applyMutation.isPending}
            onConfirmName={setConfirmName}
            onApply={() => applyMutation.mutate()}
          />
        ) : null}

        {applied ? <AppliedPanel applied={applied} /> : null}

        {message ? (
          <div
            role="status"
            className="rounded-md border border-cyan-300/30 bg-cyan-300/10 p-3 text-sm text-cyan-100"
          >
            {message}
          </div>
        ) : null}
      </div>
    </section>
  );
}

function PairPhoneSection() {
  const [pair, setPair] = useState<PairResponse | null>(null);
  const [qrSVG, setQRSVG] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [error, setError] = useState<string | null>(null);

  const pairMutation = useMutation({
    mutationFn: async () => {
      const response = await createPair(currentRoute());
      const svg = await getPairSVG(response.svg_url);
      return { response, svg };
    },
    onSuccess: ({ response, svg }) => {
      setPair(response);
      setQRSVG(svg);
      setNow(Date.now());
      setError(null);
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : "Pairing failed");
    },
  });

  useEffect(() => {
    if (!pair) {
      return undefined;
    }

    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [pair]);

  const secondsLeft = pair ? Math.max(0, Math.ceil((Date.parse(pair.expires_at) - now) / 1000)) : 0;
  const expired = pair != null && secondsLeft <= 0;
  const isLocalhost = typeof window !== "undefined" && localhost(window.location.hostname);

  return (
    <section className="rounded-md border border-white/10 bg-white/[0.03] p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <Smartphone className="size-5 text-cyan-300" aria-hidden="true" />
            <h2 className="text-lg font-semibold text-white">{pairCopy.title}</h2>
          </div>
          <p className="mt-1 text-sm text-zinc-400">{pairCopy.helper}</p>
        </div>
        <button
          type="button"
          onClick={() => pairMutation.mutate()}
          disabled={pairMutation.isPending}
          className="inline-flex min-h-10 items-center justify-center gap-2 rounded-md bg-cyan-300 px-3 text-sm font-semibold text-zinc-950 disabled:cursor-not-allowed disabled:bg-zinc-700 disabled:text-zinc-400"
        >
          {pairMutation.isPending ? (
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
          ) : (
            <QrCode className="size-4" aria-hidden="true" />
          )}
          {pairMutation.isPending
            ? pairCopy.loading
            : pair
              ? pairCopy.buttonRefresh
              : pairCopy.buttonIdle}
        </button>
      </div>

      {isLocalhost ? (
        <div className="mt-3 rounded-md border border-amber-300/30 bg-amber-300/10 px-3 py-2 text-sm text-amber-100">
          {pairCopy.localhost}
        </div>
      ) : null}

      {pair && qrSVG ? (
        <div className="mt-4 grid gap-4 md:grid-cols-[minmax(0,16rem)_1fr]">
          <div
            className={[
              "rounded-md border border-white/10 bg-white p-3 transition",
              expired ? "opacity-35 grayscale" : "",
            ].join(" ")}
          >
            <img
              src={`data:image/svg+xml;utf8,${encodeURIComponent(qrSVG)}`}
              alt="Pair phone QR"
              className="aspect-square w-full"
            />
          </div>
          <div className="min-w-0">
            <p className="text-sm font-semibold text-white">
              {expired ? pairCopy.expired : pairCopy.expiresIn(secondsLeft)}
            </p>
            <p className="mt-3 text-xs uppercase text-zinc-500">{pairCopy.urlLabel}</p>
            <p className="mt-1 select-all break-all rounded-md border border-white/10 bg-zinc-950 px-3 py-2 font-mono text-xs text-zinc-200">
              {pair.url}
            </p>
            <p className="mt-2 text-sm text-zinc-500">{pairCopy.fallback}</p>
          </div>
        </div>
      ) : null}

      {error ? (
        <p className="mt-3 rounded-md border border-red-300/30 bg-red-300/10 px-3 py-2 text-sm text-red-100">
          {error}
        </p>
      ) : null}
    </section>
  );
}

function PreviewPanel({
  preview,
  confirmName,
  canApply,
  applying,
  onConfirmName,
  onApply,
}: {
  preview: PlanPreviewResponse;
  confirmName: string;
  canApply: boolean;
  applying: boolean;
  onConfirmName: (value: string) => void;
  onApply: () => void;
}) {
  if (preview.errors.length > 0) {
    return (
      <section className="rounded-md border border-red-300/40 bg-red-300/10 p-4">
        <div className="flex items-center gap-2">
          <AlertTriangle className="size-5 text-red-200" aria-hidden="true" />
          <h2 className="text-lg font-semibold text-white">{planCopy.preview.errors}</h2>
        </div>
        <div className="mt-3 grid gap-2">
          {preview.errors.map((error, index) => (
            <div key={`${error.line ?? "unknown"}-${index}`} className="text-sm text-red-50">
              <p className="font-semibold">{error.line ? `Line ${error.line}` : "Plan error"}</p>
              <p className="mt-1 text-red-100">{error.message}</p>
              {error.excerpt ? (
                <pre className="mt-2 overflow-x-auto rounded-md bg-zinc-950 px-3 py-2 font-mono text-xs text-zinc-200">
                  {error.excerpt}
                </pre>
              ) : null}
            </div>
          ))}
        </div>
      </section>
    );
  }

  return (
    <section className="rounded-md border border-white/10 bg-white/[0.03]">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/10 px-4 py-3">
        <div>
          <h2 className="text-lg font-semibold text-white">{planCopy.preview.title}</h2>
          <p className="mt-1 text-sm text-zinc-400">
            {preview.plan?.name} · {preview.mode}
          </p>
        </div>
        <div className="inline-flex items-center gap-2 rounded-md border border-emerald-300/40 bg-emerald-300/10 px-3 py-2 text-sm font-semibold text-emerald-100">
          <CheckCircle2 className="size-4" aria-hidden="true" />
          {planCopy.preview.clean}
        </div>
      </div>

      <div className="grid gap-4 p-4 lg:grid-cols-[1fr_1fr]">
        {preview.parsed ? (
          <div>
            <h3 className="text-sm font-semibold uppercase text-zinc-400">
              {planCopy.preview.parsed}
            </h3>
            <CountsGrid counts={preview.parsed} />
          </div>
        ) : null}

        {preview.changes ? (
          <div>
            <h3 className="text-sm font-semibold uppercase text-zinc-400">
              {planCopy.preview.changes}
            </h3>
            <div className="mt-3 grid gap-2">
              <ChangeRow label="Weeks" counts={preview.changes.weeks} />
              <ChangeRow label="Tasks" counts={preview.changes.tasks} />
              <ChangeRow label="Goals" counts={preview.changes.goals} />
            </div>
          </div>
        ) : null}
      </div>

      {preview.mode === "replace" && preview.at_risk ? (
        <AtRiskBlock risk={preview.at_risk} />
      ) : null}

      <div className="border-t border-white/10 p-4">
        {preview.mode === "replace" ? (
          <label className="mb-3 block max-w-md">
            <span className="text-sm font-medium text-zinc-300">{planCopy.apply.confirmLabel}</span>
            <input
              value={confirmName}
              aria-label={planCopy.apply.confirmLabel}
              onChange={(event) => onConfirmName(event.target.value)}
              className="mt-2 min-h-11 w-full rounded-md border border-white/10 bg-zinc-900 px-3 text-base text-white outline-none focus:border-cyan-300"
            />
          </label>
        ) : null}

        <button
          type="button"
          disabled={!canApply}
          onClick={onApply}
          className={[
            "inline-flex min-h-11 items-center justify-center gap-2 rounded-md px-4 font-semibold disabled:cursor-not-allowed disabled:bg-zinc-700 disabled:text-zinc-400",
            preview.mode === "replace" ? "bg-red-300 text-zinc-950" : "bg-cyan-300 text-zinc-950",
          ].join(" ")}
        >
          {applying ? <Loader2 className="size-4 animate-spin" aria-hidden="true" /> : null}
          {applyButtonText(preview)}
        </button>
      </div>
    </section>
  );
}

function CountsGrid({ counts }: { counts: PlanCounts }) {
  return (
    <dl className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-5">
      {[
        ["Weeks", counts.weeks],
        ["Goals", counts.goals],
        ["Tasks", counts.tasks],
        ["Skills", counts.skills],
        ["Projects", counts.projects],
      ].map(([label, value]) => (
        <div key={label} className="rounded-md border border-white/10 bg-zinc-950 px-3 py-2">
          <dt className="text-xs text-zinc-500">{label}</dt>
          <dd className="mt-1 text-xl font-semibold text-white">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

function ChangeRow({ label, counts }: { label: string; counts: PlanChangeCounts }) {
  const entries = [
    ["added", counts.added ?? 0],
    ["updated", counts.updated ?? 0],
    ["removed", counts.removed ?? 0],
    ["unchanged", counts.unchanged ?? 0],
    ["orphaned", counts.orphaned ?? 0],
  ].filter(([, value]) => Number(value) > 0);

  return (
    <div className="rounded-md border border-white/10 bg-zinc-950 px-3 py-2">
      <p className="text-sm font-semibold text-white">{label}</p>
      <p className="mt-1 text-sm text-zinc-400">
        {entries.length > 0
          ? entries.map(([name, value]) => `${value} ${name}`).join(" · ")
          : "No changes"}
      </p>
    </div>
  );
}

function AtRiskBlock({ risk }: { risk: PlanAtRisk }) {
  return (
    <div className="border-t border-red-300/20 bg-red-300/10 px-4 py-3">
      <div className="flex items-center gap-2">
        <AlertTriangle className="size-5 text-red-200" aria-hidden="true" />
        <h3 className="text-sm font-semibold uppercase text-red-100">{planCopy.preview.atRisk}</h3>
      </div>
      <dl className="mt-3 grid gap-2 text-sm text-red-50 sm:grid-cols-3">
        <RiskItem label="Done tasks" value={risk.task_completions} />
        <RiskItem label="Moved goals" value={risk.goal_positions} />
        <RiskItem label="Checkpoint answers" value={risk.checkpoint_answers} />
        <RiskItem label="Log goal links" value={risk.log_goal_links} />
        <RiskItem label="Log skill links" value={risk.log_skill_links} />
        <RiskItem label="Retired categories" value={risk.retired_categories} />
      </dl>
    </div>
  );
}

function RiskItem({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <dt className="text-red-200/80">{label}</dt>
      <dd className="text-lg font-semibold text-white">{value}</dd>
    </div>
  );
}

function AppliedPanel({ applied }: { applied: ApplyPlanResponse }) {
  return (
    <section className="rounded-md border border-emerald-300/30 bg-emerald-300/10 p-4">
      <h2 className="text-lg font-semibold text-white">{planCopy.apply.done}</h2>
      <p className="mt-1 text-sm text-emerald-100">
        {applied.plan.name} · {applied.mode}
      </p>
      {applied.backup_path ? (
        <p className="mt-2 break-all font-mono text-xs text-emerald-100">
          Backup: {applied.backup_path}
        </p>
      ) : null}
      <Link
        href="/"
        className="mt-4 inline-flex min-h-10 items-center justify-center rounded-md bg-emerald-300 px-3 text-sm font-semibold text-zinc-950"
      >
        Today
      </Link>
    </section>
  );
}

function applyButtonText(preview: PlanPreviewResponse): string {
  switch (preview.mode) {
    case "replace":
      return `Replace ${preview.current_plan?.name ?? "current plan"} with ${preview.plan?.name ?? "new plan"}`;
    case "additive":
      return planCopy.apply.additive;
    default:
      return planCopy.apply.initial;
  }
}

function shortHash(hash: string): string {
  return hash.slice(0, 12);
}

function formatDateTime(value: string | undefined): string {
  if (!value) {
    return "unknown";
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function currentRoute(): string {
  if (typeof window === "undefined") {
    return "/";
  }

  return `${window.location.pathname}${window.location.search}`;
}

function localhost(hostname: string): boolean {
  return hostname === "localhost" || hostname === "127.0.0.1" || hostname === "::1";
}
