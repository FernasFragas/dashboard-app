import { describe, expect, it } from "vitest";

import { daysBetween } from "./date";

// The banner's countdown depends entirely on this, and it must not drift with daylight saving:
// both ends are Lisbon calendar dates, and Lisbon changes offset on 2026-10-25.
describe("daysBetween", () => {
  it("counts whole days forward", () => {
    expect(daysBetween("2026-08-22", "2026-08-24")).toBe(2);
    expect(daysBetween("2026-08-23", "2026-08-24")).toBe(1);
    expect(daysBetween("2026-08-24", "2026-08-24")).toBe(0);
  });

  it("goes negative once the date is past", () => {
    expect(daysBetween("2026-08-25", "2026-08-24")).toBe(-1);
  });

  it("is unaffected by the daylight-saving change", () => {
    expect(daysBetween("2026-10-24", "2026-10-26")).toBe(2);
  });

  it("returns 0 rather than NaN for an unparseable key", () => {
    expect(daysBetween("nonsense", "2026-08-24")).toBe(0);
  });
});
