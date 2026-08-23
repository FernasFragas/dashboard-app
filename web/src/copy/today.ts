/**
 * Copy for the Today screen's week banner.
 *
 * Same rule as the Review field guide and Skills: strings live here, not inline in components.
 *
 * Before the plan starts the banner used to render nothing at all — a blank space above a
 * perfectly good task list, with no hint of why. `plan_complete` still does; that is the mirror
 * case and is deliberately left alone here.
 */
export const todayCopy = {
  banner: {
    /** Before the first week starts. The task list below already shows that week's work. */
    notStarted: {
      eyebrow: "Not started yet",
      /** e.g. "W1 · Golden set starts Monday" */
      title: (week: string, focus: string | null) => (focus ? `${week} · ${focus}` : week),
      countdown: (days: number, weekday: string) => {
        if (days <= 0) {
          return `Starts today.`;
        }
        if (days === 1) {
          return `Starts tomorrow, ${weekday}.`;
        }
        return `Starts ${weekday}, in ${days} days.`;
      },
      /** Says why there is a task list at all when no week is running. */
      note: "You're seeing the first week's tasks. Anything you add lands there.",
    },
  },
} as const;
