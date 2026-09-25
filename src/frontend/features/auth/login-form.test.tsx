import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { apiError, installApiFake } from "@/test/api-fake";
import { router } from "@/test/navigation";
import { renderWithClient } from "@/test/render";

import { LoginForm } from "./login-form";

describe("LoginForm", () => {
  it("has labelled fields and keeps submit disabled until both are filled", async () => {
    const fake = installApiFake({});
    const user = userEvent.setup();
    renderWithClient(<LoginForm />);

    const submit = screen.getByRole("button", { name: "Sign in" });
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText("Username"), "operator");
    expect(submit).toBeDisabled();
    await user.keyboard("{Enter}");
    expect(fake.count("POST", "/api/v1/auth/login")).toBe(0);
    await user.type(screen.getByLabelText("Password"), "secret");
    expect(submit).toBeEnabled();
    expect(screen.getByLabelText("Password")).toHaveAttribute("type", "password");
  });

  it("signs in, stores nothing in the browser and goes to the dashboard", async () => {
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => (release = resolve));
    const fake = installApiFake({ "POST /api/v1/auth/login": { body: { authenticated: true, username: "operator" } } });
    const fetchImpl = vi.mocked(fetch).getMockImplementation()!;
    vi.mocked(fetch).mockImplementation(async (...args) => {
      await gate;
      return fetchImpl(...args);
    });
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const user = userEvent.setup();
    renderWithClient(<LoginForm />);

    await user.type(screen.getByLabelText("Username"), " operator ");
    await user.type(screen.getByLabelText("Password"), "secret");
    await user.dblClick(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByRole("button", { name: "Signing in…" })).toBeDisabled();
    release();

    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/dashboard"));
    expect(fake.calls[0].body).toEqual({ username: "operator", password: "secret" });
    expect(fake.count("POST", "/api/v1/auth/login")).toBe(1);
    expect(setItem).not.toHaveBeenCalled();
  });

  it("shows an accessible error for invalid credentials and clears the password", async () => {
    installApiFake({ "POST /api/v1/auth/login": apiError(401, "invalid_credentials", "Invalid username or password.") });
    const user = userEvent.setup();
    const { onUnauthorized } = renderWithClient(<LoginForm />);

    await user.type(screen.getByLabelText("Username"), "operator");
    await user.type(screen.getByLabelText("Password"), "wrong");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Sign-in failed. Invalid username or password.");
    expect(screen.getByLabelText("Username")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Password")).toHaveValue("");
    await waitFor(() => expect(screen.getByLabelText("Password")).toHaveFocus());
    expect(router.replace).not.toHaveBeenCalled();
    // A failed login is not an expired session: no global redirect.
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it("explains an expired session", () => {
    installApiFake({});
    renderWithClient(<LoginForm expired />);
    expect(screen.getByRole("status")).toHaveTextContent("Your session has ended.");
  });
});
