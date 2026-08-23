/**
 * Copy for the Skills tab and the skill pickers.
 *
 * Same rule as the Review field guide: helper strings live here, not inline in components, so
 * the wording can be read and changed in one place. The one deliberate exception is a skill's
 * own description and `associate_when` line — those are data, served by the API, so a skill
 * added later carries its own explanation instead of waiting for a frontend edit.
 */
export const skillsCopy = {
  tab: {
    eyebrow: "Skills",
    title: "What the work is building",
    intro:
      "Derived from the tasks themselves — nothing here is typed in twice. A skill moves when a task linked to it is done.",
    empty: "No skills yet. Seed the plan and they appear.",
  },

  card: {
    /** Rendered under the done/total bar. */
    progress: (done: number, total: number) => `${done} of ${total} tasks done`,
    noTasks: "No tasks linked yet",
    evidence: (count: number) => `${count} ${count === 1 ? "entry" : "entries"} of evidence`,
    neverTouched: "Not started",
    targetPrefix: "Target",
    noTarget: "No target set",
  },

  detail: {
    associateWhenLabel: "Tag a task with this when…",
    tasksHeading: "Tasks",
    evidenceHeading: "Evidence",
    noEvidence: "No log entries cite this skill yet. Attach one from the quick-log sheet.",
    noTasks: "No tasks build this skill yet.",
  },

  picker: {
    label: "Skills",
    /** Shown under a task's skill multi-select. */
    helper: "Every task builds at least one skill — pick what this work trains.",
    /** The save button's disabled reason, and the API's 422 message in plain words. */
    required: "Pick at least one skill before saving.",
    saving: "Saving…",
    /** Badge on a task that predates the skills feature. */
    needsSkill: "needs skill",
    needsSkillHelper: "This task was created before skills existed. Pick what it trains.",
  },

  evidence: {
    /** Quick-log sheet's optional row. */
    label: "Skills (optional)",
    helper: "Attach this entry as evidence for a skill. Leave empty to log faster.",
  },
} as const;
