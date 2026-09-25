import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "cn"
import { Slot } from "radix-ui"

// Reference buttons (design-system-v3.html): navy primary with a soft
// shadow, soft-navy secondary, outline, ghost, and destructive reserved for
// destructive actions. Visible focus ring, pressed offset, disabled state.
const buttonVariants = cva(
  "inline-flex shrink-0 cursor-pointer items-center justify-center gap-2 border border-transparent font-bold leading-none whitespace-nowrap no-underline transition-[transform,background-color,border-color,color,box-shadow] duration-150 outline-none select-none focus-visible:shadow-[0_0_0_0.25rem_var(--focus-ring)] focus-visible:outline-none active:translate-y-px disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 disabled:shadow-none aria-disabled:cursor-not-allowed aria-disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        primary:
          "bg-primary text-primary-foreground shadow-[var(--shadow-primary)] hover:bg-primary-hover active:bg-primary-active",
        secondary: "bg-primary-soft text-primary hover:bg-primary-soft-hover",
        outline: "border-border-strong bg-surface text-heading hover:border-subtle hover:bg-surface-muted",
        ghost: "bg-transparent text-primary hover:bg-primary-soft",
        destructive: "bg-destructive text-primary-foreground hover:brightness-95",
      },
      size: {
        sm: "h-[2.3rem] rounded-[10px] px-[0.95rem] text-[0.85rem]",
        md: "h-[2.85rem] rounded-[12px] px-[1.35rem] text-[0.95rem]",
        lg: "h-[3.2rem] rounded-[14px] px-[1.6rem] text-[1.05rem]",
        icon: "size-[2.6rem] rounded-[10px]",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "md",
    },
  }
)

function Button({
  className,
  variant = "primary",
  size = "md",
  asChild = false,
  type,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      type={asChild ? undefined : (type ?? "button")}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
