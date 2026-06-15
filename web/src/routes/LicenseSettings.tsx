// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo, useState } from "react";
import { AxiosError } from "axios";
import { toast } from "sonner";
import { ShieldCheck, ShieldAlert, Loader2, AlertTriangle, CheckCircle2, Check, Clock } from "@/lib/icons";
import { isEnterprise } from "@/hooks/useCompliance";
import { useLicenseStatus, useUpdateLicense } from "@/hooks/useLicense";
import { formatActiveUsers, SeatThresholdBanner } from "@/components/settings/seat-usage";
import { useCanI } from "@/hooks/useCanI";
import { EnterpriseRequired } from "@/components/compliance";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

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
  const { allowed: canUpdateRaw, isLoading: isLoadingPermission, isError: isErrorPermission } = useCanI("settings", "update");
  const canUpdate = canUpdateRaw === true;

  const [dialogOpen, setDialogOpen] = useState(false);

  // 403 Access Denied handling
  const is403Error = error && (error as AxiosError)?.response?.status === 403;

  const statusConfig = getStatusConfig(licenseStatus?.status);

  if (is403Error) {
    return (
      <div>
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

  const license = licenseStatus?.license;
  const actionLabel = license ? "Renew license" : "Activate license";
  // Header always renders so the loading skeleton sits below the same title/
  // badge/action strip as the populated state — avoids the strip flashing in
  // after the query settles.
  const canShowAction = !isLoading && (isErrorPermission || canUpdate);

  return (
    <div>
      <Card>
        <div className="flex items-center justify-between gap-3 px-6 pt-6 pb-4 border-b">
          <h3 className="text-lg font-semibold leading-none tracking-tight">
            License status
          </h3>
          <div className="flex items-center gap-2">
            {licenseStatus && (
              <Badge
                variant="outline"
                className={cn("font-medium", statusConfig.badgeClass)}
              >
                {statusConfig.label}
              </Badge>
            )}
            {isLoading || isLoadingPermission ? (
              <Skeleton className="h-9 w-32" />
            ) : canShowAction ? (
              <Button variant="outline" onClick={() => setDialogOpen(true)}>
                {actionLabel}
              </Button>
            ) : null}
          </div>
        </div>
        <CardContent className="pt-6">
          {isLoading ? (
            // Skeleton mirrors the populated state: status banner strip on
            // top, an optional seat-threshold strip (STORY-465 AC #19 — the
            // strip CAN appear at warn/exceeded so we reserve space for it
            // instead of letting it pop in), then a 2-column grid of label /
            // value pairs (Customer, Edition, Expires, Active users, Features
            // toggle). Prior version was three flat bars at random widths
            // that gave no hint of the real shape.
            <div className="space-y-5">
              <div className="flex items-start gap-3 p-4 rounded-md border">
                <Skeleton className="h-5 w-5 rounded shrink-0 mt-0.5" />
                <div className="min-w-0 flex-1 space-y-2">
                  <Skeleton className="h-3.5 w-48" />
                  <Skeleton className="h-3 w-2/3" />
                </div>
              </div>
              {/* Threshold-banner placeholder — same height as the populated
                  banner so the layout doesn't jump when warn/exceeded lands. */}
              <div className="flex items-start gap-3 p-4 rounded-md border">
                <Skeleton className="h-5 w-5 rounded shrink-0 mt-0.5" />
                <div className="min-w-0 flex-1 space-y-2">
                  <Skeleton className="h-3.5 w-40" />
                  <Skeleton className="h-3 w-3/4" />
                </div>
              </div>
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-5">
                {/* 5 cells: Customer, Edition, Expires, Active users, Features-toggle */}
                {Array.from({ length: 5 }).map((_, i) => (
                  <div key={i} className="space-y-2">
                    <Skeleton className="h-3 w-20" />
                    <Skeleton className="h-4 w-32" />
                  </div>
                ))}
              </dl>
            </div>
          ) : error ? (
            <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md">
              <p className="text-sm text-destructive">
                Failed to load license status:{" "}
                {error instanceof Error ? error.message : "Unknown error"}
              </p>
            </div>
          ) : license ? (
            <div className="space-y-5">
              {/* Status banner — fixed headline + a friendly one-liner derived
                  from the license, rather than the raw API message. */}
              <div className={cn("flex items-start gap-3 p-4 rounded-md border", statusConfig.bgClass)}>
                <statusConfig.icon className={cn("h-5 w-5 mt-0.5 shrink-0", statusConfig.iconClass)} />
                <div className="min-w-0">
                  <p className={cn("text-sm font-medium", statusConfig.textClass)}>
                    {statusConfig.headline}
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">
                    {statusConfig.subline(license.customer, license.expiresAt)}
                    {licenseStatus.status === "grace_period" && licenseStatus.gracePeriodEnd && (
                      <> · grace period ends {formatDate(licenseStatus.gracePeriodEnd)}</>
                    )}
                  </p>
                </div>
              </div>

              {/* Seat-threshold banner (STORY-465 AC #15/#16/#17/#18). Hidden
                  on "ok" and during cold start (lastUpdated == "") so the
                  page is quiet by default. The visual treatment mirrors the
                  grace_period status banner: same AlertTriangle icon, amber/
                  destructive token tints. */}
              <SeatThresholdBanner seats={licenseStatus.seats} />

              {/* License details */}
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-5 text-sm">
                <div>
                  <dt className="text-xs text-muted-foreground">Customer</dt>
                  <dd className="font-medium mt-1">{license.customer}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">Edition</dt>
                  <dd className="font-medium mt-1 capitalize">{license.edition}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">Expires</dt>
                  <dd className="font-medium mt-1">{formatDate(license.expiresAt)}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted-foreground">Active users</dt>
                  <dd className="font-medium mt-1" data-testid="license-seats-active-users">
                    {formatActiveUsers(license.maxUsers, licenseStatus.seats)}
                  </dd>
                </div>
                {license.features.length > 0 && (
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-muted-foreground mb-2">Features</dt>
                    <dd className="flex flex-wrap gap-1.5">
                      {license.features.map((feature) => (
                        <Badge
                          key={feature}
                          variant="secondary"
                          className="font-normal lowercase"
                        >
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

        </CardContent>
      </Card>

      <LicenseTokenDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        mode={license ? "renew" : "activate"}
      />
    </div>
  );
}

interface LicenseTokenDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: "renew" | "activate";
}

/**
 * Modal for pasting a license JWT.
 *
 * The "Verify & preview" button decodes the JWT payload client-side so the
 * admin can see what they're about to install (customer / expiry / features)
 * before applying. This is **preview only** — the backend re-verifies the
 * signature and full claim set on `Apply license`, so a tampered preview
 * can't actually install an invalid license.
 */
function LicenseTokenDialog({ open, onOpenChange, mode }: LicenseTokenDialogProps) {
  const updateLicense = useUpdateLicense();
  const [token, setToken] = useState("");
  const [preview, setPreview] = useState<LicensePreview | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const title = mode === "renew" ? "Renew license" : "Activate license";
  const submitLabel = mode === "renew" ? "Apply license" : "Activate license";

  // Reset local state whenever the dialog closes so the next open starts
  // clean — keeps stale previews from leaking across attempts.
  const handleOpenChange = useCallback(
    (next: boolean) => {
      if (!next) {
        setToken("");
        setPreview(null);
        setPreviewError(null);
      }
      onOpenChange(next);
    },
    [onOpenChange],
  );

  // Re-running preview after editing the token would mislead the user, so
  // clear it on any change and let them click Verify & preview again.
  const trimmedToken = useMemo(() => token.trim(), [token]);

  const handleVerify = useCallback(() => {
    setPreviewError(null);
    const result = decodeLicenseJwt(trimmedToken);
    if ("error" in result) {
      setPreview(null);
      setPreviewError(result.error);
      return;
    }
    setPreview(result);
  }, [trimmedToken]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!trimmedToken) return;
      try {
        await updateLicense.mutateAsync(trimmedToken);
        toast.success(mode === "renew" ? "License renewed successfully" : "License activated successfully");
        handleOpenChange(false);
      } catch (err) {
        const msg =
          (err as AxiosError<{ message?: string }>)?.response?.data?.message ||
          (err as Error).message ||
          "Failed to apply license";
        toast.error(msg);
      }
    },
    [trimmedToken, updateLicense, mode, handleOpenChange],
  );

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[640px]">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="sr-only">
            Paste the signed license JWT to {mode === "renew" ? "renew" : "activate"} your enterprise license.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="license-token">
              License JWT <span className="text-destructive">*</span>
            </Label>
            <textarea
              id="license-token"
              value={token}
              onChange={(e) => {
                setToken(e.target.value);
                if (preview || previewError) {
                  setPreview(null);
                  setPreviewError(null);
                }
              }}
              placeholder="eyJhbGciOiJSUzI1NiIsImtpZCI6Imxxc…"
              className="flex min-h-[140px] max-h-[260px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 font-mono overflow-y-auto resize-none"
              disabled={updateLicense.isPending}
              autoComplete="new-password"
              spellCheck={false}
              // The textarea IS the modal's primary action — every user open
              // this dialog with a token already on their clipboard, so
              // landing focus on it is the obvious next step (matches the
              // "Create/Edit Secret" dialogs). Radix's focus trap keeps
              // keyboard navigation predictable after the initial focus.
              // eslint-disable-next-line jsx-a11y/no-autofocus
              autoFocus
            />
            <p className="text-xs text-muted-foreground">
              Paste the signed token you received from sales@knodex.io.
            </p>
          </div>

          <div>
            <Button
              type="button"
              variant="outline"
              onClick={handleVerify}
              disabled={updateLicense.isPending || !trimmedToken}
            >
              Verify &amp; preview
            </Button>
            {previewError && (
              <p className="mt-2 text-sm text-destructive">{previewError}</p>
            )}
            {preview && (
              <div className="mt-3 rounded-md border bg-muted/30 p-3 text-sm">
                <p className="text-xs font-medium text-muted-foreground mb-2">
                  Token preview (not yet applied)
                </p>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2">
                  {preview.customer && (
                    <div>
                      <dt className="text-xs text-muted-foreground">Customer</dt>
                      <dd className="font-medium">{preview.customer}</dd>
                    </div>
                  )}
                  {preview.edition && (
                    <div>
                      <dt className="text-xs text-muted-foreground">Edition</dt>
                      <dd className="font-medium capitalize">{preview.edition}</dd>
                    </div>
                  )}
                  {preview.expiresAt && (
                    <div>
                      <dt className="text-xs text-muted-foreground">Expires</dt>
                      <dd className="font-medium">{formatDate(preview.expiresAt)}</dd>
                    </div>
                  )}
                  {typeof preview.maxUsers === "number" && (
                    <div>
                      <dt className="text-xs text-muted-foreground">Max users</dt>
                      <dd className="font-medium">
                        {preview.maxUsers === 0 ? "Unlimited" : preview.maxUsers}
                      </dd>
                    </div>
                  )}
                  {preview.features && preview.features.length > 0 && (
                    <div className="sm:col-span-2">
                      <dt className="text-xs text-muted-foreground mb-1">Features</dt>
                      <dd className="flex flex-wrap gap-1.5">
                        {preview.features.map((feature) => (
                          <Badge key={feature} variant="secondary" className="font-normal lowercase">
                            {feature}
                          </Badge>
                        ))}
                      </dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={updateLicense.isPending}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="accent"
              disabled={updateLicense.isPending || !trimmedToken}
            >
              {updateLicense.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Check className="h-4 w-4" />
              )}
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

interface LicensePreview {
  customer?: string;
  edition?: string;
  expiresAt?: string;
  maxUsers?: number;
  features?: string[];
}

/**
 * Best-effort client-side decode of the license JWT payload — used by the
 * "Verify & preview" button only. We do NOT verify the signature here; the
 * backend does that on apply. The function accepts the standard 3-segment
 * JWT shape and reads either snake_case (`expires_at`, `max_users`) or the
 * camelCase aliases the backend normally returns.
 */
function decodeLicenseJwt(token: string): LicensePreview | { error: string } {
  if (!token) return { error: "Paste a token to preview." };
  const parts = token.split(".");
  if (parts.length !== 3) {
    return { error: "Token doesn't look like a JWT (expected three dot-separated segments)." };
  }
  try {
    // base64url → base64 → JSON. atob ignores `=` padding mismatch, but we
    // pad anyway so weird browsers don't choke.
    const b64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = b64 + "=".repeat((4 - (b64.length % 4)) % 4);
    const json = JSON.parse(atob(padded)) as Record<string, unknown>;
    const features = Array.isArray(json.features)
      ? (json.features.filter((f): f is string => typeof f === "string"))
      : undefined;
    return {
      customer: typeof json.customer === "string" ? json.customer : undefined,
      edition: typeof json.edition === "string" ? json.edition : undefined,
      expiresAt:
        typeof json.expiresAt === "string"
          ? json.expiresAt
          : typeof json.expires_at === "string"
            ? (json.expires_at as string)
            : undefined,
      maxUsers:
        typeof json.maxUsers === "number"
          ? json.maxUsers
          : typeof json.max_users === "number"
            ? (json.max_users as number)
            : undefined,
      features,
    };
  } catch {
    return { error: "Couldn't decode the token payload — make sure it's the full JWT." };
  }
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

/**
 * Status-specific UI configuration.
 *
 * `badgeClass` is applied to a `variant="outline"` Badge so each status gets
 * the same outline silhouette as the screenshot's teal "Active" pill, tinted
 * per state. `headline` + `subline()` produce the in-banner copy — the raw
 * API `message` is bypassed in favour of a consistent two-line shape:
 *   "License is active" / "Activated for {customer} · expires {date}."
 */
function getStatusConfig(status?: string) {
  const expires = (date: string) => `expires ${formatDate(date)}`;
  switch (status) {
    case "valid":
      return {
        label: "Active",
        icon: CheckCircle2,
        badgeClass:
          "border-emerald-500/40 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
        bgClass: "bg-emerald-500/5 border-emerald-500/30",
        iconClass: "text-emerald-600 dark:text-emerald-400",
        textClass: "text-foreground",
        headline: "License is active",
        subline: (customer: string, expiresAt: string) =>
          `Activated for ${customer} · ${expires(expiresAt)}.`,
      };
    case "grace_period":
      return {
        label: "Grace period",
        icon: Clock,
        badgeClass:
          "border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400",
        bgClass: "bg-amber-500/5 border-amber-500/30",
        iconClass: "text-amber-600 dark:text-amber-400",
        textClass: "text-foreground",
        headline: "License is in grace period",
        subline: (customer: string, expiresAt: string) =>
          `Activated for ${customer} · expired ${formatDate(expiresAt)}.`,
      };
    case "expired":
      return {
        label: "Expired",
        icon: AlertTriangle,
        badgeClass:
          "border-destructive/40 bg-destructive/10 text-destructive",
        bgClass: "bg-destructive/5 border-destructive/30",
        iconClass: "text-destructive",
        textClass: "text-foreground",
        headline: "License has expired",
        subline: (customer: string, expiresAt: string) =>
          `Activated for ${customer} · expired ${formatDate(expiresAt)}.`,
      };
    default:
      return {
        label: "No license",
        icon: ShieldCheck,
        badgeClass: "text-muted-foreground",
        bgClass: "bg-muted/40 border-border",
        iconClass: "text-muted-foreground",
        textClass: "text-foreground",
        headline: "No license installed",
        subline: (customer: string, _expiresAt: string) =>
          customer
            ? `Customer: ${customer}.`
            : "Activate an enterprise license to unlock all features.",
      };
  }
}

export default LicenseSettings;
