import { vi } from "vitest";

// A small controlled fake of the HTTP boundary: tests register responses per
// "METHOD /path" (query string ignored) and can inspect every call. Nothing
// inside TanStack Query or the app is mocked.

export interface FakeResponse {
  status?: number;
  body?: unknown;
}

export type Handler = (request: { url: URL; method: string; body: unknown }) => FakeResponse;

export interface ApiFake {
  calls: Array<{ method: string; url: URL; body: unknown }>;
  count: (method: string, path: string) => number;
  set: (route: string, handler: Handler | FakeResponse) => void;
}

export function installApiFake(routes: Record<string, Handler | FakeResponse>): ApiFake {
  const table = new Map<string, Handler | FakeResponse>(Object.entries(routes));
  const calls: ApiFake["calls"] = [];

  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(String(input), "http://localhost");
      const method = (init?.method ?? "GET").toUpperCase();
      const body = typeof init?.body === "string" ? (JSON.parse(init.body) as unknown) : undefined;
      calls.push({ method, url, body });
      const entry = table.get(`${method} ${url.pathname}`);
      if (entry === undefined) {
        return new Response(JSON.stringify({ error: { code: "not_found", message: "No fake route.", request_id: "test" } }), {
          status: 404,
          headers: { "Content-Type": "application/json" },
        });
      }
      const result = typeof entry === "function" ? entry({ url, method, body }) : entry;
      const status = result.status ?? 200;
      return new Response(status === 204 || result.body === undefined ? null : JSON.stringify(result.body), {
        status,
        headers: { "Content-Type": "application/json" },
      });
    }),
  );

  return {
    calls,
    count: (method, path) => calls.filter((c) => c.method === method && c.url.pathname === path).length,
    set: (route, handler) => table.set(route, handler),
  };
}

export function apiError(status: number, code: string, message: string): FakeResponse {
  return { status, body: { error: { code, message, request_id: "req-test" } } };
}
