import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { AppShell } from "@/features/shell/app-shell";
import { SESSION_COOKIE } from "@/lib/session-cookie";

// Authenticated product area. Without a session cookie there is nothing to
// show, so the server redirects straight to the login page. With a cookie
// (valid or not), the client asks the backend; a 401 then returns to login.
export default async function ProductLayout({ children }: LayoutProps<"/">) {
  const store = await cookies();
  if (!store.has(SESSION_COOKIE)) {
    redirect("/login");
  }
  return <AppShell>{children}</AppShell>;
}
