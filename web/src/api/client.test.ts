import { afterEach, describe, expect, it, vi } from "vitest";

import { getHealth } from "./client";

describe("getHealth", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns the health response", async () => {
    const fetchMock = vi.fn(async () => {
      return new Response(JSON.stringify({ status: "ok", version: "test-build" }), {
        headers: { "Content-Type": "application/json" },
        status: 200,
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(getHealth()).resolves.toEqual({ status: "ok", version: "test-build" });
    expect(fetchMock).toHaveBeenCalledWith("/api/health", {
      cache: "no-store",
      headers: {
        Accept: "application/json",
      },
    });
  });

  it("throws on non-2xx responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("nope", { status: 503 })),
    );

    await expect(getHealth()).rejects.toThrow("health check failed with 503");
  });
});
