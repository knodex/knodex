import { AlertTriangle, BarChart3, Calendar, ShieldX, Users } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { AuditStats as AuditStatsType } from "@/types/audit";

interface AuditStatsProps {
  stats?: AuditStatsType;
  isLoading?: boolean;
  error?: Error | null;
}

interface StatCardProps {
  title: string;
  value: number | string;
  subtitle?: string;
  icon: typeof BarChart3;
  variant?: "default" | "warning" | "danger";
  isLoading?: boolean;
}

function StatCard({
  title,
  value,
  subtitle,
  icon: Icon,
  variant = "default",
  isLoading,
}: StatCardProps) {
  const variantClasses = {
    default: "text-foreground",
    warning: "text-yellow-600 dark:text-yellow-400",
    danger: "text-red-600 dark:text-red-400",
  };

  return (
    <Card className="relative overflow-hidden transition-all hover:shadow-md">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-20" />
            {subtitle !== undefined && <Skeleton className="h-4 w-32" />}
          </div>
        ) : (
          <>
            <div className={cn("text-3xl font-bold", variantClasses[variant])}>
              {typeof value === "number" ? value.toLocaleString() : value}
            </div>
            {subtitle && (
              <p className="text-xs text-muted-foreground mt-1">{subtitle}</p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}

function TopUsersCard({
  topUsers,
  isLoading,
}: {
  topUsers?: AuditStatsType["topUsers"];
  isLoading?: boolean;
}) {
  return (
    <Card className="relative overflow-hidden transition-all hover:shadow-md">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Top Users Today
        </CardTitle>
        <Users className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-5 w-3/4" />
          </div>
        ) : topUsers && topUsers.length > 0 ? (
          <div className="space-y-1.5">
            {topUsers.slice(0, 3).map((user) => (
              <div
                key={user.userId}
                className="flex items-center justify-between text-sm"
              >
                <span className="truncate text-muted-foreground max-w-[70%]">
                  {user.userId}
                </span>
                <span className="font-medium tabular-nums">
                  {user.count.toLocaleString()}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No activity today</p>
        )}
      </CardContent>
    </Card>
  );
}

export function AuditStats({ stats, isLoading, error }: AuditStatsProps) {
  const deniedAttempts = stats?.deniedAttempts ?? 0;
  const deniedVariant = deniedAttempts > 0 ? "danger" : "default";

  if (error && !isLoading) {
    return (
      <div className="flex items-center gap-2 p-3 rounded-md bg-destructive/10 border border-destructive/20 text-sm text-destructive">
        <AlertTriangle className="h-4 w-4 shrink-0" />
        <span>Failed to load audit statistics</span>
      </div>
    );
  }

  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      <StatCard
        title="Total Events"
        value={stats?.totalEvents ?? 0}
        subtitle="Total tracked events"
        icon={BarChart3}
        isLoading={isLoading}
      />
      <StatCard
        title="Events Today"
        value={stats?.eventsToday ?? 0}
        subtitle="Since midnight UTC"
        icon={Calendar}
        isLoading={isLoading}
      />
      <TopUsersCard topUsers={stats?.topUsers} isLoading={isLoading} />
      <StatCard
        title="Denied Attempts"
        value={deniedAttempts}
        subtitle="Access denied today"
        icon={ShieldX}
        variant={deniedVariant}
        isLoading={isLoading}
      />
    </div>
  );
}
