import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect, useSyncExternalStore } from "react";
import { afterEach, describe, expect, it } from "vitest";

import { AppShell } from "@/features/shell/app-shell";
import { api } from "@/lib/api/endpoints";
import { apiError, installApiFake } from "@/test/api-fake";
import { navigation, router } from "@/test/navigation";
import { renderWithClient } from "@/test/render";

import { queryKeys } from "./keys";
import { Providers } from "./providers";

function DashboardProbe() {
  const q = useQuery({ queryKey: queryKeys.dashboard, queryFn: ({ signal }) => api.dashboard(signal) });
  return <p>{q.isError ? "failed" : q.isSuccess ? "loaded" : "loading"}</p>;
}

function OtherExpiredRequests() {
  const session = useQuery({ queryKey: queryKeys.session, queryFn: () => api.session() });
  const { mutate, isError } = useMutation({ mutationFn: () => api.analyze() });
  useEffect(() => { mutate(); }, [mutate]);
  return <p>{session.isError && isError ? "other requests failed" : "pending"}</p>;
}

afterEach(() => {
  window.history.replaceState(null, "", "/");
});

describe("global 401 handling", () => {
  it("sends an expired session to the login page once", async () => {
    installApiFake({
      "GET /api/v1/dashboard/summary": apiError(401, "unauthenticated", "Authentication is required."),
      "GET /api/v1/auth/session": apiError(401, "unauthenticated", "Authentication is required."),
      "POST /api/v1/ai/analyze": apiError(401, "unauthenticated", "Authentication is required."),
    });
    render(
      <Providers>
        <OtherExpiredRequests />
        <DashboardProbe />
      </Providers>,
    );

    await screen.findAllByText("failed");
    await screen.findByText("other requests failed");
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/login?expired=1"));
    expect(router.replace).toHaveBeenCalledTimes(1);
  });

  it("never redirects while already on the login page", async () => {
    window.history.replaceState(null, "", "/login");
    installApiFake({ "GET /api/v1/dashboard/summary": apiError(401, "unauthenticated", "Authentication is required.") });
    render(
      <Providers>
        <DashboardProbe />
      </Providers>,
    );

    await screen.findByText("failed");
    expect(router.replace).not.toHaveBeenCalled();
  });
});

describe("logout", () => {
  it("offers session recovery after the API was unavailable", async () => {
    const fake = installApiFake({ "GET /api/v1/auth/session": apiError(503, "unavailable", "The service is unavailable.") });
    const user = userEvent.setup();
    renderWithClient(<AppShell><p>content</p></AppShell>);
    await screen.findByText(/Session details unavailable/);
    fake.set("GET /api/v1/auth/session", { body: { authenticated: true, username: "operator" } });
    await user.click(screen.getByRole("button", { name: "Retry session" }));
    expect(await screen.findByText("operator")).toBeInTheDocument();
  });
  it("ends the session and returns to the login page", async () => {
    const fake = installApiFake({
      "GET /api/v1/auth/session": { body: { authenticated: true, username: "operator" } },
      "POST /api/v1/auth/logout": { status: 204 },
    });
    const user = userEvent.setup();
    const { rerender } = render(
      <Providers>
        <AppShell>
          <p>content</p>
        </AppShell>
      </Providers>,
    );

    expect(await screen.findByText("operator")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Dashboard" })).toHaveAttribute("aria-current", "page");
    await user.click(screen.getByRole("button", { name: "Sign out" }));

    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/login"));
    expect(fake.count("POST", "/api/v1/auth/logout")).toBe(1);
    // The shell must not refetch the session after logging out (it would 401).
    expect(fake.count("GET", "/api/v1/auth/session")).toBe(1);

    // Once the login page is shown, cached product data is gone.
    navigation.pathname = "/login";
    rerender(
      <Providers>
        <CacheSize />
      </Providers>,
    );
    await waitFor(() => expect(screen.getByTestId("cache-size")).toHaveTextContent("0"));
  });
});

function CacheSize() {
  const cache = useQueryClient().getQueryCache();
  const size = useSyncExternalStore(
    (onChange) => cache.subscribe(onChange),
    () => cache.getAll().length,
  );
  return <p data-testid="cache-size">{size}</p>;
}
