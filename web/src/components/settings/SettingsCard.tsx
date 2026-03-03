import type { LucideIcon } from "lucide-react";
import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";

interface SettingsCardProps {
  title: string;
  description: string;
  icon: LucideIcon;
  to: string;
  badge?: string | number;
  disabled?: boolean;
}

/**
 * SettingsCard - A clickable card component for settings navigation
 * Following ArgoCD's Settings page pattern with icon, title, description
 */
export function SettingsCard({
  title,
  description,
  icon: Icon,
  to,
  badge,
  disabled = false,
}: SettingsCardProps) {
  const content = (
    <Card
      className={cn(
        "group relative cursor-pointer transition-all duration-200",
        "hover:border-primary/50 hover:shadow-md",
        disabled && "opacity-50 cursor-not-allowed hover:border-border hover:shadow-sm"
      )}
    >
      <CardContent className="p-6">
        <div className="flex items-start gap-4">
          {/* Icon container */}
          <div
            className={cn(
              "flex h-12 w-12 shrink-0 items-center justify-center rounded-lg",
              "bg-primary/10 text-primary",
              "group-hover:bg-primary/20 transition-colors duration-200"
            )}
          >
            <Icon className="h-6 w-6" />
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <h3 className="font-semibold text-foreground">{title}</h3>
              {badge !== undefined && (
                <span className="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                  {badge}
                </span>
              )}
            </div>
            <p className="mt-1 text-sm text-muted-foreground line-clamp-2">
              {description}
            </p>
          </div>

          {/* Arrow indicator */}
          <div
            className={cn(
              "flex h-8 w-8 shrink-0 items-center justify-center",
              "text-muted-foreground/50 group-hover:text-primary transition-colors duration-200"
            )}
          >
            <svg
              className="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9 5l7 7-7 7"
              />
            </svg>
          </div>
        </div>
      </CardContent>
    </Card>
  );

  if (disabled) {
    return content;
  }

  return (
    <Link to={to} className="block">
      {content}
    </Link>
  );
}

export default SettingsCard;
