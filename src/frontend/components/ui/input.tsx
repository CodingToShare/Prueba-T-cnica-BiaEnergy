import * as React from "react"
import { cn } from "cn"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        // Reference field: 3.2rem high, 12px radius, strong border, navy focus ring.
        "h-[3.2rem] w-full min-w-0 rounded-[12px] border border-border-strong bg-surface px-4 text-base text-foreground transition-[border-color,box-shadow] duration-150 outline-none placeholder:text-muted-foreground focus-visible:border-primary focus-visible:shadow-[0_0_0_0.25rem_var(--focus-ring)] focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive",
        className
      )}
      {...props}
    />
  )
}

export { Input }
