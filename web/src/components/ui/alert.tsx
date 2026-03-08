// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { AlertCircle, AlertTriangle, CheckCircle2, Info, RefreshCw } from "lucide-react"

import { cn } from "@/lib/utils"
import { Button } from "./button"

const alertVariants = cva(
  "relative w-full rounded-lg border p-4 [&>svg~*]:pl-7 [&>svg+div]:translate-y-[-3px] [&>svg]:absolute [&>svg]:left-4 [&>svg]:top-4",
  {
    variants: {
      variant: {
        default: "bg-background text-foreground [&>svg]:text-foreground",
        destructive:
          "border-destructive/50 text-destructive dark:border-destructive [&>svg]:text-destructive bg-destructive/5",
        warning:
          "border-warning/50 text-warning-foreground dark:border-warning [&>svg]:text-warning bg-warning/5",
        success:
          "border-success/50 text-success-foreground dark:border-success [&>svg]:text-success bg-success/5",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

const variantIcons = {
  default: Info,
  destructive: AlertCircle,
  warning: AlertTriangle,
  success: CheckCircle2,
}

interface AlertProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof alertVariants> {
  /** Show appropriate icon based on variant */
  showIcon?: boolean
  /** Callback for retry button */
  onRetry?: () => void
  /** Whether retry action is in progress */
  isRetrying?: boolean
}

const Alert = React.forwardRef<HTMLDivElement, AlertProps>(
  ({ className, variant, showIcon = false, onRetry, isRetrying, children, ...props }, ref) => {
    const Icon = variantIcons[variant || "default"]

    return (
      <div
        ref={ref}
        role="alert"
        className={cn(alertVariants({ variant }), className)}
        {...props}
      >
        {showIcon && <Icon className="h-4 w-4" />}
        <div className={cn("flex-1", onRetry && "pr-20")}>
          {children}
        </div>
        {onRetry && (
          <Button
            variant="outline"
            size="sm"
            onClick={onRetry}
            disabled={isRetrying}
            className="absolute right-4 top-4 gap-1.5"
          >
            <RefreshCw className={cn("h-3.5 w-3.5", isRetrying && "animate-spin")} />
            {isRetrying ? "Retrying..." : "Retry"}
          </Button>
        )}
      </div>
    )
  }
)
Alert.displayName = "Alert"

const AlertTitle = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h5
    ref={ref}
    className={cn("mb-1 font-medium leading-none tracking-tight", className)}
    {...props}
  />
))
AlertTitle.displayName = "AlertTitle"

const AlertDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn("text-sm [&_p]:leading-relaxed", className)}
    {...props}
  />
))
AlertDescription.displayName = "AlertDescription"

// eslint-disable-next-line react-refresh/only-export-components -- alertVariants exported for consumer customization
export { Alert, AlertTitle, AlertDescription, alertVariants }
