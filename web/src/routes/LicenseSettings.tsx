// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { Link } from "react-router-dom";
import { AxiosError } from "axios";
import { toast } from "sonner";
import { ArrowLeft, ShieldCheck, ShieldAlert, Loader2, AlertTriangle, CheckCircle2, Clock } from "@/lib/icons";
import { isEnterprise } from "@/hooks/useCompliance";
import { useLicenseStatus, useUpdateLicense } from "@/hooks/useLicense";
import { useCanI } from "@/hooks/useCanI";
import { EnterpriseRequired } from "@/components/compliance";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";

/**
 * License Settings page — View license status and activate/renew license.
 *
 * Enterprise-only feature gated by __ENTERPRISE__ build-time constant.
 * Admins can paste a JWT token to activate or renew the license.
 */
export function LicenseSettings() {
  // Enterprise gate
  if (!isEnterprise()) {
    return (
      <EnterpriseRequired
        feature="License Management"
        description="View and manage your enterprise license, including activation status, expiry dates, and license renewal."
      />
    );
  }

  return <LicenseSettingsContent />;
}

function LicenseSettingsContent() {
  const { data: licenseStatus, isLoading, error } = useLicenseStatus();
  const updateLicense = useUpdateLicense();
  const { allowed: canUpdateRaw, isLoading: isLoadingPermission, isError: isErrorPermission } = useCanI("settings", "update");
  const canUpdate = canUpdateRaw === true;

  const [token, setToken] = useState("");
  const [showForm, setShowForm] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = token.trim();
    if (!trimmed) return;

    try {
      await updateLicense.mutateAsync(trimmed);
      toast.success("License activated successfully");
      setToken("");
      setShowForm(false);
    } catch (err) {
      const msg =
        (err as AxiosError<{ message?: string }>)?.response?.data?.message ||
        (err as Error).message ||
        "Failed to activate license";
      toast.error(msg);
    }
  };

  // 403 Access Denied handling
  const is403Error = error && (error as AxiosError)?.response?.status === 403;

  const statusConfig = getStatusConfig(licenseStatus?.status);

  if (is403Error) {
    return (
      <div className="py-6">
        <div className="mb-8">
          <Link
            to="/settings"
            className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to Settings
          </Link>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <ShieldCheck className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-sm font-medium text-foreground">License</h2>
              <p className="text-muted-foreground">View and manage your enterprise license</p>
            </div>
          </div>
        </div>

        <Card>
          <CardContent className="pt-6">
            <div className="text-center py-12 text-muted-foreground">
              <ShieldAlert className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p className="text-sm font-medium">Access Denied</p>
              <p className="text-xs mt-2">
                You do not have permission to view license settings.
                <br />
                Contact your administrator if you believe this is an error.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="py-6">
      {/* Header */}
      <div className="mb-8">
        <Link
          to="/settings"
          className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Settings
        </Link>
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <ShieldCheck className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-sm font-medium text-foreground">License</h2>
            <p className="text-muted-foreground">View and manage your enterprise license</p>
          </div>
        </div>
      </div>

      {/* License Status Card */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <ShieldCheck className="h-5 w-5" />
                License Status
              </CardTitle>
              <CardDescription className="mt-1">
                Current license information and expiry
              </CardDescription>
            </div>
            {licenseStatus && (
              <Badge variant={statusConfig.variant}>{statusConfig.label}</Badge>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              <Skeleton className="h-4 w-1/3" />
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-4 w-1/4" />
            </div>
          ) : error ? (
            <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md">
              <p className="text-sm text-destructive">
                Failed to load license status:{" "}
                {error instanceof Error ? error.message : "Unknown error"}
              </p>
            </div>
          ) : licenseStatus?.license ? (
            <div className="space-y-4">
              {/* Status banner */}
              <div className={`flex items-start gap-3 p-3 rounded-md ${statusConfig.bgClass}`}>
                <statusConfig.icon className={`h-5 w-5 mt-0.5 ${statusConfig.iconClass}`} />
                <div>
                  <p className={`text-sm font-medium ${statusConfig.textClass}`}>
                    {licenseStatus.message}
                  </p>
                  {licenseStatus.status === "grace_period" && licenseStatus.gracePeriodEnd && (
                    <p className="text-xs text-muted-foreground mt-1">
                      Grace period ends: {formatDate(licenseStatus.gracePeriodEnd)}
                    </p>
                  )}
                </div>
              </div>

              {/* License details */}
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
                <div>
                  <dt className="text-muted-foreground">Customer</dt>
                  <dd className="font-medium mt-0.5">{licenseStatus.license.customer}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Edition</dt>
                  <dd className="font-medium mt-0.5 capitalize">{licenseStatus.license.edition}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Expires</dt>
                  <dd className="font-medium mt-0.5">{formatDate(licenseStatus.license.expiresAt)}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Max Users</dt>
                  <dd className="font-medium mt-0.5">{licenseStatus.license.maxUsers === 0 ? "Unlimited" : licenseStatus.license.maxUsers}</dd>
                </div>
                {licenseStatus.license.features.length > 0 && (
                  <div className="sm:col-span-2">
                    <dt className="text-muted-foreground mb-1">Features</dt>
                    <dd className="flex flex-wrap gap-1">
                      {licenseStatus.license.features.map((feature) => (
                        <Badge key={feature} variant="secondary" className="text-xs">
                          {feature}
                        </Badge>
                      ))}
                    </dd>
                  </div>
                )}
              </dl>
            </div>
          ) : (
            <div className="text-center py-8 text-muted-foreground">
              <ShieldCheck className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p className="text-sm font-medium">No license installed</p>
              <p className="text-xs mt-2">
                Activate an enterprise license to unlock all features.
              </p>
            </div>
          )}

          {/* Activate / Renew button */}
          {!showForm && !isLoading && isLoadingPermission && (
            <div className="mt-6 pt-4 border-t">
              <Skeleton className="h-9 w-32" />
            </div>
          )}
          {!showForm && !isLoading && !isLoadingPermission && (isErrorPermission || canUpdate) && (
            <div className="mt-6 pt-4 border-t">
              <Button variant="outline" onClick={() => setShowForm(true)}>
                {licenseStatus?.license ? "Renew License" : "Activate License"}
              </Button>
            </div>
          )}

          {/* Token paste form */}
          {showForm && !isLoading && (isErrorPermission || canUpdate) && (
            <form onSubmit={handleSubmit} className="mt-6 pt-4 border-t space-y-4">
              <div className="space-y-2">
                <Label htmlFor="license-token">License Token (JWT)</Label>
                <textarea
                  id="license-token"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder="Paste your license JWT token here..."
                  className="flex min-h-[100px] max-h-[200px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 font-mono overflow-y-auto resize-none"
                  disabled={updateLicense.isPending}
                  autoComplete="new-password"
                />
                <p className="text-xs text-muted-foreground">
                  Paste the JWT token provided by your Knodex sales representative.
                </p>
              </div>
              <div className="flex items-center gap-3">
                <Button type="submit" disabled={updateLicense.isPending || !token.trim()}>
                  {updateLicense.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  Activate
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    setShowForm(false);
                    setToken("");
                  }}
                  disabled={updateLicense.isPending}
                >
                  Cancel
                </Button>
              </div>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

/** Format ISO date string to human-readable format */
function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;
  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

/** Get status-specific UI configuration */
function getStatusConfig(status?: string) {
  switch (status) {
    case "valid":
      return {
        label: "Active",
        variant: "default" as const,
        icon: CheckCircle2,
        bgClass: "bg-green-500/10 border border-green-500/20",
        iconClass: "text-green-600 dark:text-green-400",
        textClass: "text-green-700 dark:text-green-300",
      };
    case "grace_period":
      return {
        label: "Grace Period",
        variant: "secondary" as const,
        icon: Clock,
        bgClass: "bg-amber-500/10 border border-amber-500/20",
        iconClass: "text-amber-600 dark:text-amber-400",
        textClass: "text-amber-700 dark:text-amber-300",
      };
    case "expired":
      return {
        label: "Expired",
        variant: "destructive" as const,
        icon: AlertTriangle,
        bgClass: "bg-destructive/10 border border-destructive/20",
        iconClass: "text-destructive",
        textClass: "text-destructive",
      };
    default:
      return {
        label: "No License",
        variant: "outline" as const,
        icon: ShieldCheck,
        bgClass: "bg-muted/50",
        iconClass: "text-muted-foreground",
        textClass: "text-muted-foreground",
      };
  }
}

export default LicenseSettings;
