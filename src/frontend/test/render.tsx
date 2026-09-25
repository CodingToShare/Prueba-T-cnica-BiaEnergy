import { QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import type { ReactElement } from "react";
import { vi } from "vitest";

import { makeQueryClient } from "@/lib/query/query-client";

/** Renders with the application's real QueryClient policy (retries off for speed). */
export function renderWithClient(ui: ReactElement) {
  const onUnauthorized = vi.fn();
  const client = makeQueryClient(onUnauthorized);
  client.setDefaultOptions({ queries: { ...client.getDefaultOptions().queries, retry: false } });
  const result = render(
    <QueryClientProvider client={client}>{ui}</QueryClientProvider>,
  );
  return { ...result, client, onUnauthorized };
}
