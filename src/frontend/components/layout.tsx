import type { ReactNode } from "react";

/** Page title region of the product shell. */
export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="mb-5 flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0">
        <h1 className="m-0 text-[1.45rem] font-extrabold tracking-[-0.01em] text-heading">{title}</h1>
        {description ? <p className="m-0 mt-1 text-[0.86rem] text-muted-foreground">{description}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2.5">{actions}</div> : null}
    </div>
  );
}

/** Reference panel: bordered surface with an optional header row. */
export function Panel({
  title,
  headingLevel = 2,
  actions,
  children,
  className = "",
  bodyClassName = "panel-body",
  labelledBy,
}: {
  title?: ReactNode;
  headingLevel?: 2 | 3;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
  labelledBy?: string;
}) {
  const Heading = headingLevel === 2 ? "h2" : "h3";
  return (
    <section className={`panel ${className}`} aria-labelledby={labelledBy}>
      {title !== undefined ? (
        <div className="panel-head">
          <Heading className="panel-title" id={labelledBy}>
            {title}
          </Heading>
          {actions}
        </div>
      ) : null}
      <div className={bodyClassName}>{children}</div>
    </section>
  );
}

/** Reference KPI card. */
export function KpiCard({
  label,
  value,
  unit,
  context,
  featured = false,
  children,
}: {
  label: string;
  value: ReactNode;
  unit?: string;
  context?: ReactNode;
  featured?: boolean;
  children?: ReactNode;
}) {
  return (
    <div className={`kpi ${featured ? "kpi-feature" : ""}`}>
      <div className="kpi-label">{label}</div>
      <div className="kpi-value">
        {value}
        {unit ? <small>{unit}</small> : null}
      </div>
      {context ? <div className="kpi-context">{context}</div> : null}
      {children}
    </div>
  );
}

/** Horizontal bar for a 0–1 value, with an accessible label. */
export function MeterBar({ value, label }: { value: number; label: string }) {
  const pct = Math.max(0, Math.min(100, Math.round(value * 100)));
  return (
    <div className="meter-bar" role="img" aria-label={label}>
      <span style={{ width: `${pct}%` }} />
    </div>
  );
}
