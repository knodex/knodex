// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface EmptyStateProps {
  /** Icon to display */
  icon: ReactNode;
  /** Main heading text */
  title: string;
  /** Supporting description text */
  description: string;
  /** Optional action button or link */
  action?: ReactNode;
  /** Optional variant for different visual styles */
  variant?: "default" | "success";
  /** Optional className for customization */
  className?: string;
}

/**
 * Empty state component for compliance list views
 * AC-SHARED-02: Empty state component for no results
 */
export function EmptyState({
  icon,
  title,
  description,
  action,
  variant = "default",
  className,
}: EmptyStateProps) {
  const iconBgClasses = {
    default: "bg-muted",
    success: "bg-green-100 dark:bg-green-900/30",
  };

  const titleClasses = {
    default: "text-foreground",
    success: "text-green-600 dark:text-green-400",
  };

  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center py-12 text-center",
        className
      )}
    >
      <div
        className={cn(
          "rounded-full p-4 mb-4",
          iconBgClasses[variant]
        )}
      >
        {icon}
      </div>
      <h3 className={cn("text-lg font-semibold", titleClasses[variant])}>
        {title}
      </h3>
      <p className="text-sm text-muted-foreground mt-1 max-w-[300px]">
        {description}
      </p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

export default EmptyState;
