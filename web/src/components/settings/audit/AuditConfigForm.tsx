import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { AlertTriangle, Loader2, Settings2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuditConfig, useUpdateAuditConfig } from "@/hooks/useAudit";

const auditConfigSchema = z.object({
  enabled: z.boolean(),
  retentionDays: z.coerce
    .number({ invalid_type_error: "Must be a number" })
    .int("Must be a whole number")
    .min(1, "Minimum 1 day")
    .max(3650, "Maximum 3650 days (10 years)"),
});

type AuditConfigFormValues = z.infer<typeof auditConfigSchema>;

export function AuditConfigForm() {
  const { data: config, isLoading: configLoading, error: configError, refetch: refetchConfig } = useAuditConfig();
  const updateConfig = useUpdateAuditConfig();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isDirty },
  } = useForm<AuditConfigFormValues>({
    resolver: zodResolver(auditConfigSchema),
    defaultValues: {
      enabled: true,
      retentionDays: 90,
    },
  });

  // Sync form when config loads or changes (e.g., after save + refetch)
  useEffect(() => {
    if (config) {
      reset({
        enabled: config.enabled,
        retentionDays: config.retentionDays,
      });
    }
  }, [config, reset]);

  const onSubmit = async (values: AuditConfigFormValues) => {
    try {
      // Preserve existing advanced settings (hidden from UI but kept in API)
      await updateConfig.mutateAsync({
        enabled: values.enabled,
        retentionDays: values.retentionDays,
        maxStreamLength: config?.maxStreamLength ?? 100000,
        excludeActions: config?.excludeActions ?? [],
        excludeResources: config?.excludeResources ?? [],
      });
      toast.success("Audit configuration saved");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to save configuration";
      toast.error(message);
    }
  };

  const hasChanges = isDirty;

  if (configLoading) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle className="text-base font-medium">
            Audit Configuration
          </CardTitle>
          <Settings2 className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (configError && !config) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle className="text-base font-medium">
            Audit Configuration
          </CardTitle>
          <Settings2 className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-2 p-3 rounded-md bg-destructive/10 border border-destructive/20 text-sm text-destructive">
            <AlertTriangle className="h-4 w-4 shrink-0" />
            <span className="flex-1">Failed to load audit configuration</span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetchConfig()}
              className="shrink-0"
            >
              Retry
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
        <CardTitle className="text-base font-medium">
          Audit Configuration
        </CardTitle>
        <Settings2 className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
          {/* General */}
          <div>
            <h4 className="text-sm font-medium mb-3">General</h4>
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                {...register("enabled")}
                className="h-4 w-4 rounded border-input accent-primary"
                disabled={updateConfig.isPending}
              />
              <div>
                <span className="text-sm font-medium">Enable Audit Trail</span>
                <p className="text-xs text-muted-foreground">
                  Record all user actions and security events
                </p>
              </div>
            </label>
          </div>

          {/* Retention */}
          <div>
            <h4 className="text-sm font-medium mb-3">Retention</h4>
            <div>
              <Label htmlFor="retentionDays">Retention Days</Label>
              <Input
                id="retentionDays"
                type="number"
                {...register("retentionDays")}
                disabled={updateConfig.isPending}
                className="mt-1.5 max-w-xs"
              />
              {errors.retentionDays && (
                <p className="mt-1 text-sm text-destructive">
                  {errors.retentionDays.message}
                </p>
              )}
              <p className="mt-1 text-xs text-muted-foreground">
                How long to keep audit events (1-3650 days)
              </p>
            </div>
          </div>

          {/* Submit */}
          <div className="flex justify-end pt-4 border-t">
            <Button
              type="submit"
              disabled={updateConfig.isPending || !hasChanges}
            >
              {updateConfig.isPending && (
                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              )}
              Save Configuration
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
