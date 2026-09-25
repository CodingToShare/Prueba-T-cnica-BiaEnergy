// The only place that calls fetch. Requests are same-origin relative URLs
// (/api/v1/...), forwarded to the Go API by the Next.js rewrite, so the
// HttpOnly session cookie travels automatically and no CORS is needed.

/** A failed API call, carrying the backend's standard error body when present. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId: string | null;

  constructor(status: number, code: string, message: string, requestId: string | null) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }

  get isUnauthorized(): boolean {
    return this.status === 401;
  }

  get isNotFound(): boolean {
    return this.status === 404;
  }
}

const NETWORK_MESSAGE = "The service could not be reached. Check your connection and try again.";
const UNEXPECTED_MESSAGE = "The service returned an unexpected response. Please try again.";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Builds an ApiError from any response body. The backend's standard shape is
 * `{"error":{"code","message","request_id"}}`; anything else (a proxy HTML
 * page, an empty body) becomes a generic, safe error. Raw bodies are never
 * shown to users.
 */
export function parseApiError(status: number, body: unknown): ApiError {
  if (isRecord(body) && isRecord(body.error)) {
    const { code, message, request_id: requestId } = body.error;
    if (typeof code === "string" && typeof message === "string") {
      return new ApiError(status, code, message, typeof requestId === "string" ? requestId : null);
    }
  }
  if (status === 502 || status === 503 || status === 504) {
    return new ApiError(status, "service_unavailable", NETWORK_MESSAGE, null);
  }
  return new ApiError(status, "unexpected_response", UNEXPECTED_MESSAGE, null);
}

async function readJson(response: Response): Promise<unknown> {
  const text = await response.text();
  if (text === "") {
    return undefined;
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return undefined;
  }
}

export interface RequestOptions {
  method?: "GET" | "POST";
  body?: unknown;
  signal?: AbortSignal;
}

/**
 * Performs a request and returns the parsed JSON body typed as T (the type
 * comes from the OpenAPI-generated schema). 204 resolves to undefined.
 */
export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers: HeadersInit = { Accept: "application/json" };
  let body: string | undefined;
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(options.body);
  }

  let response: Response;
  try {
    response = await fetch(path, {
      method: options.method ?? "GET",
      headers,
      body,
      credentials: "same-origin",
      cache: "no-store",
      signal: options.signal,
    });
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === "AbortError") {
      throw cause;
    }
    throw new ApiError(0, "network_error", NETWORK_MESSAGE, null);
  }

  if (!response.ok) {
    throw parseApiError(response.status, await readJson(response));
  }
  if (response.status === 204) {
    // Finish the response before logout navigates away; otherwise Chromium
    // can report the acknowledged request as aborted during navigation.
    await response.text();
    return undefined as T;
  }
  const data = await readJson(response);
  if (data === undefined) {
    throw new ApiError(response.status, "unexpected_response", UNEXPECTED_MESSAGE, null);
  }
  return data as T;
}

/** Builds a query string from defined values only. */
export function withQuery(path: string, params: Record<string, string | number | undefined | null>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      search.set(key, String(value));
    }
  }
  const query = search.toString();
  return query === "" ? path : `${path}?${query}`;
}
