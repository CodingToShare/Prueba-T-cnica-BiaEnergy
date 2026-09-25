"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";

import { errorMessage } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api/client";
import { api } from "@/lib/api/endpoints";
import { queryKeys } from "@/lib/query/keys";

/**
 * Demo login. The credential is sent once to POST /api/v1/auth/login; the
 * backend answers with an HttpOnly session cookie. Nothing is stored in the
 * browser, and the password is never logged or kept after submit.
 */
export function LoginForm({ expired = false }: { expired?: boolean }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const errorId = useId();
  const passwordRef = useRef<HTMLInputElement>(null);

  const login = useMutation({
    mutationFn: (credentials: { username: string; password: string }) =>
      api.login(credentials.username, credentials.password),
    meta: { handlesUnauthorized: true },
    onSuccess: (session) => {
      queryClient.clear();
      queryClient.setQueryData(queryKeys.session, session);
      router.replace("/dashboard");
    },
    onSettled: () => setPassword(""),
  });

  useEffect(() => {
    if (login.isError) passwordRef.current?.focus();
  }, [login.isError]);

  const invalidCredentials = login.error instanceof ApiError && login.error.isUnauthorized;

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (login.isPending || username.trim() === "" || password === "") {
      return;
    }
    login.mutate({ username: username.trim(), password });
  }

  return (
    <form onSubmit={onSubmit} noValidate className="flex flex-col gap-5" aria-describedby={login.isError ? errorId : undefined}>
      {expired && !login.isError ? (
        <div className="alert alert-informational" role="status">
          <span>Your session has ended. Please sign in again.</span>
        </div>
      ) : null}
      {login.isError ? (
        <div id={errorId} className="alert alert-error" role="alert">
          <span>
            <strong>Sign-in failed.</strong> {errorMessage(login.error)}
          </span>
        </div>
      ) : null}

      <div className="flex flex-col gap-2">
        <Label htmlFor="username">Username</Label>
        <Input
          id="username"
          name="username"
          autoComplete="username"
          required
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          aria-invalid={invalidCredentials || undefined}
          aria-describedby={login.isError ? errorId : undefined}
          disabled={login.isPending}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          ref={passwordRef}
          name="password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          aria-invalid={invalidCredentials || undefined}
          aria-describedby={login.isError ? errorId : undefined}
          disabled={login.isPending}
        />
      </div>

      <Button type="submit" size="lg" disabled={login.isPending || username.trim() === "" || password === ""} aria-busy={login.isPending}>
        {login.isPending ? "Signing in…" : "Sign in"}
      </Button>
    </form>
  );
}
