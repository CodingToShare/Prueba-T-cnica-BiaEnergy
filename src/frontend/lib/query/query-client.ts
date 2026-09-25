import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";

import { ApiError } from "@/lib/api/client";

declare module "@tanstack/react-query" {
  interface Register {
    queryMeta: { handlesUnauthorized?: boolean };
    mutationMeta: { handlesUnauthorized?: boolean };
  }
}

export function isUnauthorized(error: unknown): boolean {
  return error instanceof ApiError && error.isUnauthorized;
}

/** 4xx answers are final: retrying cannot change them. */
function shouldRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
    return false;
  }
  return failureCount < 2;
}

/**
 * Creates the application's QueryClient. `onUnauthorized` runs when any
 * protected request answers 401 (expired or missing session), unless the
 * query or mutation handles 401 itself (login).
 */
export function makeQueryClient(onUnauthorized: () => void): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: (error, query) => {
        if (isUnauthorized(error) && query.meta?.handlesUnauthorized !== true) {
          onUnauthorized();
        }
      },
    }),
    mutationCache: new MutationCache({
      onError: (error, _variables, _context, mutation) => {
        if (isUnauthorized(error) && mutation.meta?.handlesUnauthorized !== true) {
          onUnauthorized();
        }
      },
    }),
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        retry: shouldRetry,
      },
      mutations: { retry: false },
    },
  });
}
