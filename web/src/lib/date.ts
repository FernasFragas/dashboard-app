const lisbonDateParts = new Intl.DateTimeFormat("en-US", {
  timeZone: "Europe/Lisbon",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

const lisbonTimeFormatter = new Intl.DateTimeFormat("en-US", {
  timeZone: "Europe/Lisbon",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

const dayHeadingFormatter = new Intl.DateTimeFormat("en-US", {
  weekday: "short",
  month: "short",
  day: "numeric",
});

const weekLabelFormatter = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "numeric",
});

export function lisbonDateKey(value: string | Date): string {
  const date = typeof value === "string" ? new Date(value) : value;
  const parts = lisbonDateParts.formatToParts(date);
  const year = parts.find((part) => part.type === "year")?.value;
  const month = parts.find((part) => part.type === "month")?.value;
  const day = parts.find((part) => part.type === "day")?.value;

  if (!year || !month || !day) {
    throw new Error("could not format Lisbon date");
  }

  return `${year}-${month}-${day}`;
}

export function currentLisbonDate(now = new Date()): string {
  return lisbonDateKey(now);
}

export function addDays(dateKey: string, days: number): string {
  const date = dateKeyToUTCNoon(dateKey);
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

export function formatLisbonDayHeading(dateKey: string): string {
  return dayHeadingFormatter.format(dateKeyToUTCNoon(dateKey));
}

export function formatLisbonTime(isoTimestamp: string): string {
  return lisbonTimeFormatter.format(new Date(isoTimestamp));
}

export function formatWeekOf(dateKey: string | null): string {
  if (!dateKey) {
    return "This week";
  }

  return `Week of ${weekLabelFormatter.format(dateKeyToUTCNoon(dateKey))}`;
}

function dateKeyToUTCNoon(dateKey: string): Date {
  const [year, month, day] = dateKey.split("-").map(Number);
  return new Date(Date.UTC(year, month - 1, day, 12));
}

/**
 * Whole days from one Lisbon date key to another, negative if `to` is in the past.
 *
 * Both keys are already Lisbon calendar dates, so this is plain date arithmetic — parsing them
 * as UTC midnight keeps daylight-saving out of it entirely.
 */
export function daysBetween(from: string, to: string): number {
  const start = Date.parse(`${from}T00:00:00Z`);
  const end = Date.parse(`${to}T00:00:00Z`);

  if (Number.isNaN(start) || Number.isNaN(end)) {
    return 0;
  }

  return Math.round((end - start) / 86_400_000);
}
