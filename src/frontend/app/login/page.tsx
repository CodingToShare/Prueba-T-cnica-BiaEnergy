import type { Metadata } from "next";
import { Zap } from "lucide-react";

import { LoginForm } from "@/features/auth/login-form";

export const metadata: Metadata = { title: "Sign in" };

export default async function LoginPage({ searchParams }: PageProps<"/login">) {
  const params = await searchParams;
  const expired = params.expired === "1";

  return (
    <main className="grid min-h-dvh place-items-center px-4 py-10">
      <div className="w-full max-w-[420px]">
        <div className="mb-6 flex items-center gap-2.5 text-[1.2rem] font-extrabold tracking-[-0.01em] text-heading">
          <span className="grid size-9 place-items-center rounded-[10px] bg-navy text-mint" aria-hidden="true">
            <Zap className="size-5" strokeWidth={2.5} />
          </span>
          Bia Energy
        </div>
        <section className="panel" aria-labelledby="login-title">
          <div className="p-6 sm:p-7">
            <h1 id="login-title" className="m-0 text-[1.45rem] font-extrabold tracking-[-0.02em] text-heading">
              Sign in
            </h1>
            <p className="mb-6 mt-1 text-[0.9rem] text-muted-foreground">
              Energy management with explainable AI anomaly detection.
            </p>
            <LoginForm expired={expired} />
          </div>
        </section>
        <p className="mt-4 text-center text-[0.8rem] text-muted-foreground">
          Sign in with your demo credentials.
        </p>
      </div>
    </main>
  );
}
