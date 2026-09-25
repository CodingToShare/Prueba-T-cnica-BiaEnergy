import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { SESSION_COOKIE } from "@/lib/session-cookie";

// The root resolves intentionally: to the dashboard when a session cookie is
// present, otherwise to the login page. The cookie is only a routing hint;
// the backend validates every request.
export default async function RootPage() {
  const store = await cookies();
  redirect(store.has(SESSION_COOKIE) ? "/dashboard" : "/login");
}
