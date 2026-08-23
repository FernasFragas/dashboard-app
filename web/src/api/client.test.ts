import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError, getHealth, getProjects } from "./client";

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
    const calls = fetchMock.mock.calls as unknown as Array<[string, RequestInit]>;
    const [path, init] = calls[0];
    expect(path).toBe("/api/health");
    expect(init.cache).toBe("no-store");
    expect(new Headers(init.headers).get("Accept")).toBe("application/json");
  });

  it("throws on non-2xx responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("nope", { status: 503 })),
    );

    await expect(getHealth()).rejects.toThrow("request failed with 503");
    await expect(getHealth()).rejects.toMatchObject({ status: 503 } satisfies Partial<ApiError>);
  });

  it("returns plan projects", async () => {
    const fetchMock = vi.fn(async () => {
      return new Response(
        JSON.stringify({ projects: [{ id: "ops", label: "Operations", sort_order: 10 }] }),
        {
          headers: { "Content-Type": "application/json" },
          status: 200,
        },
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(getProjects()).resolves.toEqual({
      projects: [{ id: "ops", label: "Operations", sort_order: 10 }],
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/projects", expect.any(Object));
  });
});
