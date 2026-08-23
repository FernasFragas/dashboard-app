export const reviewCopy = {
  daily: {
    eyebrow: "Review",
    heading: "Daily closeout",
    title: "Daily 3 bullets",
    dateLabel: "Review date",
    save: "Save review",
    saved: "Review saved",
    guideLabel: "Daily review guidance",
    guideTitle: "Three bullets. Two minutes. The log is the record.",
    emptyExampleLabel: "Example",
    emptyExample: {
      learned: "ValidateAgainstEvidence rejects on citation index, not text match.",
      issue: "k6 run was CPU-bound on the laptop, so the latency number is not trustworthy.",
      next: "Move load generation to the other machine and rerun the mixed profile.",
    },
    fields: {
      learned: {
        label: "Learned today",
        helper:
          "One thing you understand now that you didn't this morning — a mechanism, a gotcha, a number. Not an activity.",
        placeholder:
          "pgxpool MaxConns above Neon's cap just queues — saw it in the pool wait metric",
        guide:
          'Good entries are specific enough that future-you can act on them without context. "Worked on the eval runner" is a task — it belongs in Today. "ValidateAgainstEvidence rejects on citation index, not text match" is a learning. If the day produced nothing, write what you\'d try differently — that counts.',
      },
      issue: {
        label: "Blocker / issue",
        helper:
          "What slowed or stopped you — technical, planning, or energy. The thing you'd warn yesterday-you about.",
        placeholder:
          "vLLM OOMs at 8k ctx on the 24GB card — need 4k ctx or a bigger card for the bake-off",
        guide:
          'Name it precisely enough that "Next step" can attack it. "None" is a valid, honest answer — don\'t invent friction. The same issue three days running is a signal to change the plan, not to push harder: raise it at Sunday review.',
      },
      next: {
        label: "Next step",
        helper:
          "The first action of your next session. One verb, one action, startable in under 2 minutes.",
        placeholder: "Add -runs=5 flag to cmd/evalrun and rerun the golden set",
        guide:
          'This field kills session start-up cost. Write it as an instruction to tomorrow-you: a file, a command, a target. "Continue eval work" fails the test. "Wire rejection-rate calc into evalrun\'s summary table" passes.',
      },
      minutes: {
        label: "Focused minutes",
        helper:
          "Minutes of actual focused work — not elapsed time. Rough is fine. Blank on rest days.",
        guide:
          "Feeds the weekly load view against the 2–3h/day target. Under-counting beats flattering yourself — the number is for pacing, not judgment. An empty Sunday is the plan working, not failing.",
      },
    },
  },
  weekPct: {
    title: "Week completion",
    helper:
      "Done ÷ all tasks for the current plan week. A pace signal, not a scorecard — low by Thursday means apply the slip order (guardrail #4), not extend hours.",
    loading: "Loading completion",
  },
  metrics: {
    title: "Metric quick-add",
    guideLabel: "Metric guidance",
    guideTitle: "Only measured numbers.",
    helper:
      "From a run, a dashboard, or a bill. If you estimated it, it's a Learned bullet, not a metric.",
    save: "Save metric",
    saved: "Metric saved",
    loadingDefinitions: "Loading metrics",
    loadingReadings: "Loading readings",
    noReadings: "No readings yet",
    lastReadings: "Last readings",
    otherOption: "other...",
    fields: {
      metric: {
        label: "Metric",
        helper: "Pick a catalogue metric so its definition and measurement method travel with it.",
      },
      otherName: {
        label: "Metric name",
        helper: "Use a stable name you would recognize in a time series.",
        placeholder: "Provider fallback success rate",
      },
      value: {
        label: "Value",
        helper: "Exactly as measured. Same load profile every time, or the series is garbage.",
        placeholder: "0.74",
      },
      unit: {
        label: "Unit",
        helper: "Prefilled for catalogue metrics; editable only for a new metric.",
        placeholder: "s, %, ms",
      },
      note: {
        label: "Metric note",
        helper: "One line of context so the number survives 3 months: profile, config, commit.",
        placeholder: "mixed k6 profile · 2 concurrent writers · commit a1b2c3",
      },
    },
  },
  checkpoint: {
    title: "Checkpoint",
    save: "Save checkpoint",
    saved: "Checkpoint saved",
    loading: "Loading checkpoint",
    guideLabel: "Checkpoint guidance",
    helper: "Written answers. 45 minutes. No editing afterwards — the log is the record.",
    answerPlaceholder: "0.31 rejection rate, CI-gated",
  },
} as const;
