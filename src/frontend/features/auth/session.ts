"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

import { api } from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query/keys";

/**
 * The authenticated session, from GET /api/v1/auth/session. The session is
 * an HttpOnly cookie managed by the backend; JavaScript never reads it.
 */
export function useSession() {
  return useQuery({
    queryKey: queryKeys.session,
    queryFn: ({ signal }) => api.session(signal),
    staleTime: 5 * 60_000,
  });
}

/**
 * Logs out and returns to the login page, where the providers drop every
 * cached product query.
 */
export function useLogout() {
  const router = useRouter();
  return useMutation({
    mutationFn: () => api.logout(),
    onSuccess: () => router.replace("/login"),
  });
}
