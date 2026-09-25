import { describe, expect, it, vi } from "vitest";

import { apiError, installApiFake } from "@/test/api-fake";

import { ApiError, apiRequest, parseApiError, withQuery } from "./client";

describe("parseApiError", () => {
  it("reads the backend's standard error body", () => {
    const err = parseApiError(404, { error: { code: "meter_not_found", message: "Meter was not found.", request_id: "abc" } });
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(404);
    expect(err.code).toBe("meter_not_found");
    expect(err.message).toBe("Meter was not found.");
    expect(err.requestId).toBe("abc");
    expect(err.isNotFound).toBe(true);
  });

  it("never exposes unexpected bodies", () => {
    expect(parseApiError(500, "<html>stack trace</html>").message).not.toContain("stack");
    expect(parseApiError(502, undefined).code).toBe("service_unavailable");
    expect(parseApiError(500, { error: { code: 1 } }).code).toBe("unexpected_response");
  });
});

describe("apiRequest", () => {
  it("sends JSON with same-origin credentials and parses the response", async () => {
    const fake = installApiFake({ "POST /api/v1/auth/login": { body: { authenticated: true } } });
    const result = await apiRequest<{ authenticated: boolean }>("/api/v1/auth/login", { method: "POST", body: { username: "u" } });
    expect(result.authenticated).toBe(true);
    expect(fake.calls[0].body).toEqual({ username: "u" });
    const init = vi.mocked(fetch).mock.calls[0][1];
    expect(init?.credentials).toBe("same-origin");
    expect(new Headers(init?.headers).get("Content-Type")).toBe("application/json");
  });

  it("resolves 204 to undefined", async () => {
    installApiFake({ "POST /api/v1/auth/logout": { status: 204 } });
    await expect(apiRequest("/api/v1/auth/logout", { method: "POST" })).resolves.toBeUndefined();
  });

  it("throws ApiError for error statuses", async () => {
    installApiFake({ "GET /api/v1/meters": apiError(400, "invalid_query", "Bad sort.") });
    await expect(apiRequest("/api/v1/meters")).rejects.toMatchObject({ status: 400, code: "invalid_query", message: "Bad sort." });
  });

  it("turns network failures into a safe error", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    await expect(apiRequest("/api/v1/dashboard/summary")).rejects.toMatchObject({ status: 0, code: "network_error" });
  });
});

describe("withQuery", () => {
  it("drops empty values", () => {
    expect(withQuery("/x", { a: "1", b: undefined, c: null, d: "", e: 0 })).toBe("/x?a=1&e=0");
    expect(withQuery("/x", {})).toBe("/x");
  });
});
