import { AlertTriangle, SearchX, Sparkles, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/client";

/** Skeleton rows while a view loads; the shell stays in place. */
export function LoadingState({ label, rows = 4 }: { label: string; rows?: number }) {
  return (
    <div className="state" aria-busy="true" role="status">
      <span className="sr-only">{label}</span>
      <Skeleton className="mx-auto mb-3 h-3 w-3/5" />
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className="my-2 h-3" style={{ width: `${95 - i * 8}%` }} />
      ))}
    </div>
  );
}

function StateCard({
  icon: Icon,
  tone,
  title,
  message,
  action,
  role,
  headingLevel = 2,
}: {
  icon: LucideIcon;
  tone: "primary" | "muted" | "error";
  title: string;
  message: ReactNode;
  action?: ReactNode;
  role?: "alert" | "status";
  headingLevel?: 1 | 2;
}) {
  const Heading = headingLevel === 1 ? "h1" : "h2";
  const glyph =
    tone === "primary"
      ? "bg-primary-soft text-primary"
      : tone === "error"
        ? "bg-destructive-soft text-destructive-text"
        : "bg-surface-muted text-muted-foreground";
  return (
    <div className="state" role={role}>
      <div className={`state-glyph ${glyph}`} aria-hidden="true">
        <Icon className="size-5" />
      </div>
      <Heading>{title}</Heading>
      <p>{message}</p>
      {action}
    </div>
  );
}

export function EmptyState({
  title,
  message,
  action,
  variant = "empty",
}: {
  title: string;
  message: ReactNode;
  action?: ReactNode;
  variant?: "empty" | "not-analyzed";
}) {
  return (
    <StateCard
      icon={variant === "not-analyzed" ? Sparkles : SearchX}
      tone={variant === "not-analyzed" ? "primary" : "muted"}
      title={title}
      message={message}
      action={action}
      role="status"
    />
  );
}

/** Safe, user-facing message for any error (never raw bodies or internals). */
export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.message;
  }
  return "Something went wrong. Please try again.";
}

export function ErrorState({
  title,
  error,
  onRetry,
  action,
}: {
  title: string;
  error: unknown;
  onRetry?: () => void;
  action?: ReactNode;
}) {
  return (
    <StateCard
      icon={AlertTriangle}
      tone="error"
      title={title}
      message={errorMessage(error)}
      role="alert"
      action={
        action ??
        (onRetry ? (
          <Button variant="secondary" size="sm" onClick={onRetry}>
            Retry
          </Button>
        ) : undefined)
      }
    />
  );
}

export function NotFoundState({ title, message, action, headingLevel = 2 }: { title: string; message: string; action: ReactNode; headingLevel?: 1 | 2 }) {
  return <StateCard icon={SearchX} tone="muted" title={title} message={message} action={action} role="status" headingLevel={headingLevel} />;
}
