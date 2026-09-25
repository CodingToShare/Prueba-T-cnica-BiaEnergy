"use client";

import { QueryClientProvider } from "@tanstack/react-query";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";

import { makeQueryClient } from "./query-client";

export const LOGIN_PATH = "/login";

/**
 * Routes a 401 to the login page at most once per visit. The action is
 * bound after mount (it needs the router), and the login page
 * itself is never redirected, so there is no redirect loop.
 */
class UnauthorizedRedirect {
  private redirecting = false;
  private action: (() => void) | null = null;

  bind(action: () => void) {
    this.action = action;
  }

  reset() {
    this.redirecting = false;
  }

  trigger() {
    if (this.redirecting || this.action === null || window.location.pathname === LOGIN_PATH) {
      return;
    }
    this.redirecting = true;
    this.action();
  }
}

/** Client-side providers: TanStack Query (remote state). */
export function Providers({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [unauthorized] = useState(() => new UnauthorizedRedirect());
  const [client] = useState(() => makeQueryClient(() => unauthorized.trigger()));

  useEffect(() => {
    unauthorized.bind(() => router.replace(`${LOGIN_PATH}?expired=1`));
  }, [unauthorized, router]);

  // Cached product data is dropped once the login page is shown (after
  // logout or an expired session). Clearing earlier, while the product views
  // are still mounted, would make them refetch without a session.
  useEffect(() => {
    if (pathname === LOGIN_PATH) {
      client.clear();
      unauthorized.reset();
    }
  }, [pathname, client, unauthorized]);

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}
