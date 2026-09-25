"use client";

import { AlertTriangle, LayoutDashboard, LogOut, Menu, Gauge, Zap } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetDescription, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { useLogout, useSession } from "@/features/auth/session";

const NAV = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/meters", label: "Meters", icon: Gauge },
  { href: "/anomalies", label: "AI Anomalies", icon: AlertTriangle },
] as const;

function isActive(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

function Brand() {
  return (
    <div className="flex items-center gap-2.5 text-[1.05rem] font-extrabold tracking-[-0.01em] text-primary-foreground">
      <span className="grid size-[30px] place-items-center rounded-[9px] bg-mint text-navy-900" aria-hidden="true">
        <Zap className="size-4" strokeWidth={2.5} />
      </span>
      Bia Energy
    </div>
  );
}

function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname();
  return (
    <nav aria-label="Main navigation">
      <div className="mx-2.5 mb-1.5 mt-4 text-[0.66rem] uppercase tracking-[0.08em] text-sidebar-muted">Operations</div>
      <ul className="m-0 list-none p-0">
        {NAV.map(({ href, label, icon: Icon }) => {
          const active = isActive(pathname, href);
          return (
            <li key={href}>
              <Link
                href={href}
                onClick={onNavigate}
                aria-current={active ? "page" : undefined}
                className={`mb-0.5 flex min-h-11 items-center gap-2.5 rounded-[11px] px-3 text-[0.9rem] font-semibold no-underline transition-colors ${
                  active
                    ? "bg-white/12 text-primary-foreground"
                    : "text-sidebar-text hover:bg-white/6 hover:text-primary-foreground"
                }`}
              >
                <Icon className={`size-[18px] ${active ? "text-mint" : "opacity-70"}`} aria-hidden="true" />
                {label}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}

function SessionBlock() {
  const session = useSession();
  const logout = useLogout();
  return (
    <div className="mt-auto border-t border-white/10 px-2.5 pt-4 text-[0.8rem] text-sidebar-text">
      {session.isError ? (
        <div role="alert">
          Session details unavailable.{" "}
          <button type="button" className="font-bold underline underline-offset-[3px]" disabled={session.isFetching}
            onClick={() => void session.refetch()}>Retry session</button>
        </div>
      ) : <div>
        Signed in as{" "}
        <span className="font-bold text-primary-foreground">{session.data?.username ?? "…"}</span>
      </div>}
      <button
        type="button"
        onClick={() => logout.mutate()}
        disabled={logout.isPending}
        className="mt-1.5 inline-flex min-h-10 cursor-pointer items-center gap-1.5 border-0 bg-transparent p-0 font-bold text-primary-foreground underline underline-offset-[3px] disabled:opacity-60"
      >
        <LogOut className="size-4" aria-hidden="true" />
        {logout.isPending ? "Signing out…" : "Sign out"}
      </button>
      {logout.isError ? (
        <p role="alert" className="mt-1 text-[0.78rem] font-semibold text-primary-foreground">
          Could not sign out. Please try again.
        </p>
      ) : null}
    </div>
  );
}

/**
 * Product shell of the reference: navy sidebar on desktop; at 900 px and
 * below a top bar with an accessible menu (Radix dialog/sheet).
 */
export function AppShell({ children }: { children: ReactNode }) {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <div className="min-h-dvh min-[901px]:grid min-[901px]:grid-cols-[232px_minmax(0,1fr)]">
      <a
        href="#main"
        className="absolute left-4 top-[-3rem] z-50 rounded-[10px] bg-primary px-4 py-2.5 font-bold text-primary-foreground focus:top-4"
      >
        Skip to content
      </a>

      <aside className="on-navy sticky top-0 hidden h-dvh flex-col bg-linear-to-b from-navy to-navy-900 px-4 py-5 min-[901px]:flex">
        <div className="px-2 pb-5 pt-1.5">
          <Brand />
        </div>
        <NavLinks />
        <SessionBlock />
      </aside>

      <header className="on-navy sticky top-0 z-40 flex items-center justify-between bg-navy px-4 py-2.5 min-[901px]:hidden">
        <Brand />
        <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
          <SheetTrigger asChild>
            <Button variant="ghost" size="sm" className="bg-white/12 text-primary-foreground hover:bg-white/20" aria-label="Open menu">
              <Menu className="size-5" aria-hidden="true" />
              Menu
            </Button>
          </SheetTrigger>
          <SheetContent
            side="left"
            className="on-navy flex w-[min(18rem,85vw)] flex-col border-0 bg-linear-to-b from-navy to-navy-900 px-4 py-5 text-sidebar-text [&_[data-slot=sheet-close]]:text-primary-foreground [&_[data-slot=sheet-close]]:hover:bg-white/10"
          >
            <SheetTitle className="sr-only">Navigation</SheetTitle>
            <SheetDescription className="sr-only">Main sections of Bia Energy</SheetDescription>
            <div className="px-2 pb-3 pt-1.5">
              <Brand />
            </div>
            <NavLinks onNavigate={() => setMenuOpen(false)} />
            <SessionBlock />
          </SheetContent>
        </Sheet>
      </header>

      <main id="main" className="min-w-0 bg-surface-muted px-4 py-5 sm:px-6 min-[901px]:px-7 min-[901px]:py-6">
        <div className="mx-auto w-full max-w-[1280px]">{children}</div>
      </main>
    </div>
  );
}
