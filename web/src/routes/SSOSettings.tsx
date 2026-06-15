// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useMemo } from "react";
import { KeyRound, Plus, ShieldAlert, Pencil, Trash2, Loader2, AlertTriangle, Check } from "@/lib/icons";
import { AxiosError } from "axios";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { useCanI } from "@/hooks/useCanI";
import {
  useSSOProviders,
  useCreateSSOProvider,
  useUpdateSSOProvider,
  useDeleteSSOProvider,
} from "@/hooks/useSSOProviders";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type {
  SSOProvider,
  CreateSSOProviderRequest,
  UpdateSSOProviderRequest,
  TokenEndpointAuthMethod,
} from "@/types/sso";

/** Form state for create/edit */
interface ProviderFormData {
  name: string;
  issuerURL: string;
  clientID: string;
  clientSecret: string;
  redirectURL: string;
  scopes: string;
  tokenEndpointAuthMethod: TokenEndpointAuthMethod;
  authorizationURL: string;
  tokenURL: string;
  jwksURL: string;
}

const DEFAULT_SCOPES = "openid,profile,email";

function emptyForm(): ProviderFormData {
  return {
    name: "",
    issuerURL: "",
    clientID: "",
    clientSecret: "",
    redirectURL: "",
    scopes: DEFAULT_SCOPES,
    tokenEndpointAuthMethod: "client_secret_basic",
    authorizationURL: "",
    tokenURL: "",
    jwksURL: "",
  };
}

function providerToForm(p: SSOProvider): ProviderFormData {
  return {
    name: p.name,
    issuerURL: p.issuerURL,
    clientID: p.clientID,
    clientSecret: "", // secret is write-only — never returned from API
    redirectURL: p.redirectURL,
    scopes: p.scopes.join(","),
    tokenEndpointAuthMethod: p.tokenEndpointAuthMethod ?? "client_secret_basic",
    authorizationURL: p.authorizationURL ?? "",
    tokenURL: p.tokenURL ?? "",
    jwksURL: p.jwksURL ?? "",
  };
}

function parseScopes(raw: string): string[] {
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

/**
 * Visual identity for a provider row. We deliberately do not ship vendor
 * logos — instead each provider gets a colored monogram so a list of three
 * "key icons" stops reading as one repeated shape. Well-known IdPs are
 * pinned to recognizable tones; everything else is hashed into a small
 * palette so the same `name` always renders the same color.
 */
const PROVIDER_PALETTE: ReadonlyArray<{ bg: string; text: string; ring: string }> = [
  { bg: "bg-teal-500/15", text: "text-teal-300", ring: "ring-teal-500/30" },
  { bg: "bg-violet-500/15", text: "text-violet-300", ring: "ring-violet-500/30" },
  { bg: "bg-blue-500/15", text: "text-blue-300", ring: "ring-blue-500/30" },
  { bg: "bg-amber-500/15", text: "text-amber-300", ring: "ring-amber-500/30" },
  { bg: "bg-rose-500/15", text: "text-rose-300", ring: "ring-rose-500/30" },
  { bg: "bg-emerald-500/15", text: "text-emerald-300", ring: "ring-emerald-500/30" },
];

// `name` is the DNS-label provider key — we don't peek at `issuerURL` here
// because operators routinely point known IdPs (e.g. Okta) at custom domains.
const WELL_KNOWN_PROVIDER_TONES: Record<string, (typeof PROVIDER_PALETTE)[number]> = {
  google: PROVIDER_PALETTE[2],     // blue
  github: PROVIDER_PALETTE[0],     // teal — neutral readable on dark
  gitlab: PROVIDER_PALETTE[3],     // amber
  okta: PROVIDER_PALETTE[1],       // violet
  azure: PROVIDER_PALETTE[2],
  microsoft: PROVIDER_PALETTE[2],
  entra: PROVIDER_PALETTE[2],
  auth0: PROVIDER_PALETTE[4],      // rose
  keycloak: PROVIDER_PALETTE[5],   // emerald
  supabase: PROVIDER_PALETTE[5],
};

function getProviderVisual(name: string): { initial: string; tone: (typeof PROVIDER_PALETTE)[number] } {
  const trimmed = name.trim();
  const initial = (trimmed[0] ?? "?").toUpperCase();
  const key = trimmed.toLowerCase();
  const pinned = WELL_KNOWN_PROVIDER_TONES[key];
  if (pinned) return { initial, tone: pinned };
  // FNV-1a-ish hash so the chosen tone is stable across renders and across
  // sessions — operators expect the same provider to look the same every time.
  let h = 2166136261;
  for (let i = 0; i < key.length; i++) {
    h ^= key.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return { initial, tone: PROVIDER_PALETTE[(h >>> 0) % PROVIDER_PALETTE.length] };
}

/** Strip protocol + path so the issuer reads as a host first, full URL on hover. */
function formatIssuerHost(issuer: string): string {
  try {
    const u = new URL(issuer);
    return u.host + (u.pathname && u.pathname !== "/" ? u.pathname : "");
  } catch {
    return issuer;
  }
}

/**
 * SSO Settings page - Manage OIDC authentication providers
 *
 * Access control: Accessible to all authenticated users. Authorization is handled
 * by the API via Casbin permission checks. If the API returns 403, the page
 * displays an Access Denied message.
 */
export function SSOSettings() {
  const { allowed: canCreate, isLoading: isLoadingCreate, isError: isErrorCreate } = useCanI("settings", "create");
  const { allowed: canUpdate, isLoading: isLoadingUpdate, isError: isErrorUpdate } = useCanI("settings", "update");
  const { allowed: canDelete, isLoading: isLoadingDelete, isError: isErrorDelete } = useCanI("settings", "delete");

  // `editingProvider === undefined` means the dialog is closed.
  // `null` means open in create mode; an `SSOProvider` means open in edit mode.
  // Using a single state for "open + mode + payload" avoids the
  // open-flag-out-of-sync bugs that plagued the previous list/form-view split.
  const [editingProvider, setEditingProvider] = useState<SSOProvider | null | undefined>(undefined);
  const [deletingProvider, setDeletingProvider] = useState<SSOProvider | null>(null);

  const { data: providers, isLoading, error } = useSSOProviders();
  const deleteMutation = useDeleteSSOProvider();

  const providerList = useMemo(() => providers || [], [providers]);

  const openCreate = useCallback(() => setEditingProvider(null), []);
  const openEdit = useCallback((provider: SSOProvider) => setEditingProvider(provider), []);
  const closeDialog = useCallback(() => setEditingProvider(undefined), []);

  // --- Delete ---

  const handleDeleteConfirm = useCallback(async () => {
    if (!deletingProvider) return;
    await deleteMutation.mutateAsync(deletingProvider.name, {
      onSuccess: () => {
        toast.success(`SSO provider "${deletingProvider.name}" deleted — changes take effect within seconds`);
        setDeletingProvider(null);
      },
      onError: (err) => {
        const msg =
          (err as AxiosError<{ message?: string }>)?.response?.data?.message ||
          err.message ||
          "Failed to delete provider";
        toast.error(msg);
      },
    });
  }, [deletingProvider, deleteMutation]);

  const handleDeleteCancel = useCallback(() => setDeletingProvider(null), []);

  // --- 403 Access Denied ---

  const is403Error = useMemo(() => error && (error as AxiosError)?.response?.status === 403, [error]);

  if (is403Error) {
    return (
      <div>
        <div className="mb-8">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <KeyRound className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-sm font-medium text-foreground">SSO Providers</h2>
              <p className="text-muted-foreground">Manage OIDC authentication providers</p>
            </div>
          </div>
        </div>

        <Card>
          <CardContent className="pt-6">
            <div className="text-center py-12 text-muted-foreground">
              <ShieldAlert className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p className="text-sm font-medium">Access Denied</p>
              <p className="text-xs mt-2">
                You do not have permission to view SSO settings.
                <br />
                Contact your administrator if you believe this is an error.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  // --- List view ---

  return (
    <div>
      {/* Provider list. The card surfaces three jobs at a glance:
           (1) how many IdPs are wired up, (2) one-click "add another",
           (3) per-row identity that's actually distinguishable (monogram
           avatars), with auth method + scopes as quiet supporting metadata. */}
      <Card>
        <CardContent className="p-0">
          {/* Card header: count on the left, primary action on the right.
               Hairline separator below replaces the old per-row borders so the
               list reads as one continuous surface. */}
          <div className="flex items-center justify-between gap-4 px-6 py-4">
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">
                {isLoading
                  ? "Loading providers…"
                  : `${providerList.length} ${providerList.length === 1 ? "provider" : "providers"} configured`}
              </p>
              <p className="text-xs text-muted-foreground">
                OIDC identity providers users can sign in through
              </p>
            </div>
            {isLoadingCreate ? (
              <Skeleton className="h-9 w-32" />
            ) : isErrorCreate || canCreate ? (
              <Button onClick={openCreate}>
                <Plus className="h-4 w-4 mr-2" />
                Add Provider
              </Button>
            ) : null}
          </div>

          {error && !is403Error && (
            <div className="mx-6 mb-4 p-3 bg-destructive/10 border border-destructive/20 rounded-md">
              <p className="text-sm text-destructive">
                Failed to load SSO providers:{" "}
                {error instanceof Error ? error.message : "Unknown error"}
              </p>
            </div>
          )}

          {isLoading ? (
            <div className="divide-y divide-border border-t">
              {[1, 2].map((i) => (
                <div key={i} className="flex items-center gap-4 px-6 py-4">
                  <Skeleton className="h-11 w-11 rounded-lg" />
                  <div className="flex-1 space-y-2">
                    <Skeleton className="h-4 w-1/4" />
                    <Skeleton className="h-3 w-1/2" />
                  </div>
                </div>
              ))}
            </div>
          ) : providerList.length === 0 ? (
            <div className="border-t text-center py-12 px-6 text-muted-foreground">
              <KeyRound className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p className="text-sm font-medium">No SSO providers configured</p>
              <p className="text-xs mt-2">
                Add an OIDC provider to enable single sign-on authentication.
              </p>
            </div>
          ) : (
            <div className="divide-y divide-border border-t">
              {providerList.map((provider) => {
                const visual = getProviderVisual(provider.name);
                const issuerHost = formatIssuerHost(provider.issuerURL);
                return (
                  <div
                    key={provider.name}
                    className="group flex items-center gap-4 px-6 py-4 transition-colors hover:bg-muted/40"
                  >
                    {/* Monogram avatar — deterministic tone per provider name */}
                    <div
                      className={cn(
                        "flex h-11 w-11 shrink-0 items-center justify-center rounded-lg ring-1 font-semibold text-sm",
                        visual.tone.bg,
                        visual.tone.text,
                        visual.tone.ring,
                      )}
                      aria-hidden="true"
                    >
                      {visual.initial}
                    </div>

                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <p className="font-medium text-foreground truncate">{provider.name}</p>
                        {provider.tokenEndpointAuthMethod === "none" ? (
                          <Badge
                            variant="outline"
                            className="text-[10px] font-medium uppercase tracking-wide px-1.5 py-0 h-5 border-amber-500/40 text-amber-300 bg-amber-500/10"
                            data-testid={`badge-public-${provider.name}`}
                          >
                            Public · PKCE
                          </Badge>
                        ) : (
                          <Badge
                            variant="outline"
                            className="text-[10px] font-medium uppercase tracking-wide px-1.5 py-0 h-5 border-border/60 text-muted-foreground"
                          >
                            Confidential
                          </Badge>
                        )}
                      </div>
                      <p
                        className="font-mono text-xs text-muted-foreground truncate"
                        title={provider.issuerURL}
                      >
                        {issuerHost}
                      </p>
                      {provider.scopes.length > 0 && (
                        <div className="flex flex-wrap gap-1 mt-2">
                          {provider.scopes.map((scope) => (
                            <span
                              key={scope}
                              className="inline-flex items-center rounded-md bg-muted/60 px-1.5 py-0.5 text-[11px] font-mono text-muted-foreground"
                            >
                              {scope}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>

                    <div className="flex items-center gap-1 shrink-0 opacity-70 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
                      {(isLoadingUpdate || isErrorUpdate || canUpdate) && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-8 w-8 p-0 text-muted-foreground hover:text-foreground"
                          onClick={() => openEdit(provider)}
                        >
                          <Pencil className="h-4 w-4" />
                          <span className="sr-only">Edit {provider.name}</span>
                        </Button>
                      )}
                      {(isLoadingDelete || isErrorDelete || canDelete) && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-8 w-8 p-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                          onClick={() => setDeletingProvider(provider)}
                        >
                          <Trash2 className="h-4 w-4" />
                          <span className="sr-only">Delete {provider.name}</span>
                        </Button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Create/Edit dialog — same surface as License renew so admins encounter
           one consistent "settings change" modal pattern across the page.
           Mounted only when active, with a `key` derived from the provider
           name (or "__new__" for create), so React drops the form state on
           close and re-initializes on next open without a setState-in-effect
           reset. Re-opening the same provider replays its server-side values. */}
      {editingProvider !== undefined && (
        <SSOProviderDialog
          key={editingProvider ? editingProvider.name : "__new__"}
          provider={editingProvider}
          onClose={closeDialog}
        />
      )}

      {/* Delete confirmation dialog */}
      {deletingProvider && (
        <DeleteProviderDialog
          provider={deletingProvider}
          isOpen={!!deletingProvider}
          onConfirm={handleDeleteConfirm}
          onCancel={handleDeleteCancel}
          isDeleting={deleteMutation.isPending}
          error={deleteMutation.error}
        />
      )}
    </div>
  );
}

// --- Create / Edit dialog ---

interface SSOProviderDialogProps {
  /** `null` = create mode; an `SSOProvider` = edit mode. */
  provider: SSOProvider | null;
  onClose: () => void;
}

/**
 * Modal for adding or editing an OIDC SSO provider.
 *
 * Mirrors the License renew dialog: a scrollable form body with an outline
 * Cancel and an accent ✓ submit, so settings-mutating modals share one shape
 * across the page. Today only OIDC is wired up server-side, so there is no
 * Protocol selector — when SAML or pure OAuth 2.0 lands, this is the natural
 * place to grow it (above Provider name).
 *
 * The parent only mounts this component when a create/edit is active and
 * passes a stable `key`, so initial form state is seeded once in `useState`
 * — no setState-in-effect reset, and re-opening the same provider replays
 * its server-side values automatically.
 */
function SSOProviderDialog({ provider, onClose }: SSOProviderDialogProps) {
  const isCreate = provider === null;
  const createMutation = useCreateSSOProvider();
  const updateMutation = useUpdateSSOProvider();
  const isPending = createMutation.isPending || updateMutation.isPending;

  const [formData, setFormData] = useState<ProviderFormData>(() =>
    provider ? providerToForm(provider) : emptyForm(),
  );
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});

  // --- Validation ---

  const validateForm = useCallback((): Record<string, string> => {
    const errors: Record<string, string> = {};
    const nameRegex = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

    if (isCreate) {
      if (!formData.name) {
        errors.name = "Name is required";
      } else if (!nameRegex.test(formData.name)) {
        errors.name = "Must be lowercase letters, numbers, and hyphens only (DNS label format)";
      } else if (formData.name.length > 63) {
        errors.name = "Name must be 63 characters or fewer";
      }
    }

    if (!formData.issuerURL) {
      errors.issuerURL = "Issuer URL is required";
    } else {
      try {
        const u = new URL(formData.issuerURL);
        if (u.protocol !== "https:") {
          errors.issuerURL = "Issuer URL must use HTTPS";
        }
      } catch {
        errors.issuerURL = "Must be a valid URL";
      }
    }

    if (!formData.clientID) {
      errors.clientID = "Client ID is required";
    }

    if (isCreate && formData.tokenEndpointAuthMethod === "client_secret_basic" && !formData.clientSecret) {
      errors.clientSecret = "Client secret is required";
    }

    if (!formData.redirectURL) {
      errors.redirectURL = "Redirect URL is required";
    } else {
      try {
        new URL(formData.redirectURL);
      } catch {
        errors.redirectURL = "Must be a valid URL";
      }
    }

    // Explicit endpoint override — all three required together (or all blank).
    const explicit: Array<["authorizationURL" | "tokenURL" | "jwksURL", string]> = [
      ["authorizationURL", formData.authorizationURL.trim()],
      ["tokenURL", formData.tokenURL.trim()],
      ["jwksURL", formData.jwksURL.trim()],
    ];
    const explicitSet = explicit.filter(([, v]) => v !== "").length;
    if (explicitSet > 0 && explicitSet < 3) {
      errors.authorizationURL = "All three endpoint URLs must be provided together (or leave all blank to use discovery)";
    } else if (explicitSet === 3) {
      for (const [field, value] of explicit) {
        try {
          const u = new URL(value);
          if (u.protocol !== "https:") {
            errors[field] = "Must use HTTPS";
          }
        } catch {
          errors[field] = "Must be a valid URL";
        }
      }
    }

    return errors;
  }, [formData, isCreate]);

  // --- Submit ---

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    const errors = validateForm();
    setFormErrors(errors);
    if (Object.keys(errors).length > 0) return;

    const scopes = parseScopes(formData.scopes);

    const authzURL = formData.authorizationURL.trim();
    const tokenURL = formData.tokenURL.trim();
    const jwksURL = formData.jwksURL.trim();
    const explicitEndpointsSet = authzURL !== "" && tokenURL !== "" && jwksURL !== "";

    if (isCreate) {
      const req: CreateSSOProviderRequest = {
        name: formData.name,
        issuerURL: formData.issuerURL,
        clientID: formData.clientID,
        clientSecret: formData.tokenEndpointAuthMethod === "none" ? "" : formData.clientSecret,
        redirectURL: formData.redirectURL,
        scopes,
        tokenEndpointAuthMethod: formData.tokenEndpointAuthMethod,
        ...(explicitEndpointsSet && {
          authorizationURL: authzURL,
          tokenURL,
          jwksURL,
        }),
      };
      await createMutation.mutateAsync(req, {
        onSuccess: () => {
          toast.success(`SSO provider "${req.name}" created — changes take effect within seconds`);
          onClose();
        },
        onError: (err) => {
          const msg =
            (err as AxiosError<{ message?: string }>)?.response?.data?.message ||
            err.message ||
            "Failed to create provider";
          toast.error(msg);
        },
      });
    } else {
      const req: UpdateSSOProviderRequest = {
        issuerURL: formData.issuerURL,
        clientID: formData.clientID,
        redirectURL: formData.redirectURL,
        scopes,
        tokenEndpointAuthMethod: formData.tokenEndpointAuthMethod,
        // Always send so clearing the fields actually unsets them server-side.
        // Empty strings are treated as "use discovery" by the validator (all-three-or-none).
        authorizationURL: authzURL,
        tokenURL,
        jwksURL,
      };
      if (formData.tokenEndpointAuthMethod === "client_secret_basic" && formData.clientSecret) {
        req.clientSecret = formData.clientSecret;
      }
      await updateMutation.mutateAsync(
        { name: provider!.name, request: req },
        {
          onSuccess: () => {
            toast.success(`SSO provider "${provider!.name}" updated — changes take effect within seconds`);
            onClose();
          },
          onError: (err) => {
            const msg =
              (err as AxiosError<{ message?: string }>)?.response?.data?.message ||
              err.message ||
              "Failed to update provider";
            toast.error(msg);
          },
        }
      );
    }
  }, [isCreate, formData, validateForm, createMutation, updateMutation, provider, onClose]);

  // Radix calls onOpenChange with `false` on overlay/Escape; the parent
  // (which owns mount/unmount) decides what happens, so we just forward.
  const handleOpenChange = useCallback((next: boolean) => {
    if (!next) onClose();
  }, [onClose]);

  const title = isCreate ? "Add SSO provider" : `Edit ${provider!.name}`;
  const submitLabel = isCreate ? "Add provider" : "Save changes";

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[640px] max-h-[90vh] flex flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="sr-only">
            {isCreate
              ? "Configure a new OIDC authentication provider."
              : "Update this OIDC provider's configuration."}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="flex-1 space-y-5 overflow-y-auto py-2 pr-1">
            {/* Provider name — DNS-label slug, only editable on create */}
            {isCreate && (
              <div className="space-y-2">
                <Label htmlFor="name">
                  Provider name <span className="text-destructive">*</span>
                </Label>
                <Input
                  id="name"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  placeholder="e.g. okta, keycloak, auth0"
                  disabled={isPending}
                  autoComplete="off"
                />
                <p className="text-xs text-muted-foreground">
                  Lowercase letters, numbers, and hyphens only — used as the URL slug.
                </p>
                {formErrors.name && (
                  <p className="text-xs text-destructive">{formErrors.name}</p>
                )}
              </div>
            )}

            {/* Issuer URL */}
            <div className="space-y-2">
              <Label htmlFor="issuerURL">
                Issuer URL <span className="text-destructive">*</span>
              </Label>
              <Input
                id="issuerURL"
                value={formData.issuerURL}
                onChange={(e) => setFormData({ ...formData, issuerURL: e.target.value })}
                placeholder="https://acme.okta.com"
                disabled={isPending}
                autoComplete="off"
              />
              <p className="text-xs text-muted-foreground">
                The provider&apos;s OIDC discovery endpoint. Must use HTTPS.
              </p>
              {formErrors.issuerURL && (
                <p className="text-xs text-destructive">{formErrors.issuerURL}</p>
              )}
            </div>

            {/* Client ID */}
            <div className="space-y-2">
              <Label htmlFor="clientID">
                Client ID <span className="text-destructive">*</span>
              </Label>
              <Input
                id="clientID"
                value={formData.clientID}
                onChange={(e) => setFormData({ ...formData, clientID: e.target.value })}
                placeholder="0oa1abc2dEF3gHIJ4klm"
                disabled={isPending}
                autoComplete="off"
                className="font-mono"
              />
              {formErrors.clientID && (
                <p className="text-xs text-destructive">{formErrors.clientID}</p>
              )}
            </div>

            {/* Token endpoint auth method — confidential (secret) vs public (PKCE).
                  Kept as an explicit radio rather than "empty secret = PKCE" so the
                  choice is unambiguous when an admin lands on the form: blank-secret
                  conventions vary by IdP and we want the server-side method enum
                  set deliberately. */}
            <div className="space-y-2">
              <Label>Token endpoint authentication</Label>
              <div role="radiogroup" aria-label="Token endpoint authentication" className="flex flex-col gap-2 sm:flex-row">
                <Button
                  type="button"
                  role="radio"
                  aria-checked={formData.tokenEndpointAuthMethod === "client_secret_basic"}
                  variant={formData.tokenEndpointAuthMethod === "client_secret_basic" ? "default" : "outline"}
                  onClick={() => setFormData({ ...formData, tokenEndpointAuthMethod: "client_secret_basic" })}
                  disabled={isPending}
                  data-testid="auth-method-confidential"
                  className="flex-1"
                >
                  Confidential client (with secret)
                </Button>
                <Button
                  type="button"
                  role="radio"
                  aria-checked={formData.tokenEndpointAuthMethod === "none"}
                  variant={formData.tokenEndpointAuthMethod === "none" ? "default" : "outline"}
                  onClick={() => setFormData({ ...formData, tokenEndpointAuthMethod: "none", clientSecret: "" })}
                  disabled={isPending}
                  data-testid="auth-method-public"
                  className="flex-1"
                >
                  Public client (PKCE)
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                Public clients (PKCE) skip the client secret — recommended when the IdP issues no shared secret (e.g. Supabase Auth).
              </p>
            </div>

            {/* Client Secret — confidential clients only */}
            {formData.tokenEndpointAuthMethod === "client_secret_basic" && (
              <div className="space-y-2">
                <Label htmlFor="clientSecret">
                  Client secret{!isCreate && " (leave blank to keep existing)"}
                  {isCreate && <span className="text-destructive"> *</span>}
                </Label>
                <Input
                  id="clientSecret"
                  type="password"
                  value={formData.clientSecret}
                  onChange={(e) => setFormData({ ...formData, clientSecret: e.target.value })}
                  placeholder={isCreate ? "your-client-secret" : "••••••••"}
                  disabled={isPending}
                  autoComplete="new-password"
                />
                {formErrors.clientSecret && (
                  <p className="text-xs text-destructive">{formErrors.clientSecret}</p>
                )}
              </div>
            )}

            {/* Redirect URL */}
            <div className="space-y-2">
              <Label htmlFor="redirectURL">
                Redirect URL <span className="text-destructive">*</span>
              </Label>
              <Input
                id="redirectURL"
                value={formData.redirectURL}
                onChange={(e) => setFormData({ ...formData, redirectURL: e.target.value })}
                placeholder="https://your-app.example.com/api/v1/auth/oidc/callback"
                disabled={isPending}
                autoComplete="off"
              />
              {formErrors.redirectURL && (
                <p className="text-xs text-destructive">{formErrors.redirectURL}</p>
              )}
            </div>

            {/* Scopes */}
            <div className="space-y-2">
              <Label htmlFor="scopes">Scopes</Label>
              <Input
                id="scopes"
                value={formData.scopes}
                onChange={(e) => setFormData({ ...formData, scopes: e.target.value })}
                placeholder="openid,profile,email"
                disabled={isPending}
                autoComplete="off"
                className="font-mono"
              />
              <p className="text-xs text-muted-foreground">
                Comma-separated list of OIDC scopes.
              </p>
              {formErrors.scopes && (
                <p className="text-xs text-destructive">{formErrors.scopes}</p>
              )}
            </div>

            {/* Advanced: explicit OIDC endpoints (bypass discovery) */}
            <details className="rounded-md border p-3" data-testid="advanced-endpoints">
              <summary className="text-sm font-medium cursor-pointer select-none">
                Advanced: override OIDC endpoints
              </summary>
              <div className="mt-3 space-y-3">
                <p className="text-xs text-muted-foreground">
                  Leave blank to use <code className="text-xs">/.well-known/openid-configuration</code> discovery (default).
                  Fill in <strong>all three</strong> to bypass discovery — required for IdPs with incomplete discovery
                  documents (e.g., Supabase GoTrue, which omits <code className="text-xs">authorization_endpoint</code>).
                </p>
                <div className="space-y-2">
                  <Label htmlFor="authorizationURL">Authorization endpoint</Label>
                  <Input
                    id="authorizationURL"
                    value={formData.authorizationURL}
                    onChange={(e) => setFormData({ ...formData, authorizationURL: e.target.value })}
                    placeholder="https://idp.example.com/auth/v1/authorize"
                    disabled={isPending}
                    autoComplete="off"
                  />
                  {formErrors.authorizationURL && (
                    <p className="text-xs text-destructive">{formErrors.authorizationURL}</p>
                  )}
                </div>
                <div className="space-y-2">
                  <Label htmlFor="tokenURL">Token endpoint</Label>
                  <Input
                    id="tokenURL"
                    value={formData.tokenURL}
                    onChange={(e) => setFormData({ ...formData, tokenURL: e.target.value })}
                    placeholder="https://idp.example.com/auth/v1/token"
                    disabled={isPending}
                    autoComplete="off"
                  />
                  {formErrors.tokenURL && (
                    <p className="text-xs text-destructive">{formErrors.tokenURL}</p>
                  )}
                </div>
                <div className="space-y-2">
                  <Label htmlFor="jwksURL">JWKS endpoint</Label>
                  <Input
                    id="jwksURL"
                    value={formData.jwksURL}
                    onChange={(e) => setFormData({ ...formData, jwksURL: e.target.value })}
                    placeholder="https://idp.example.com/auth/v1/.well-known/jwks.json"
                    disabled={isPending}
                    autoComplete="off"
                  />
                  {formErrors.jwksURL && (
                    <p className="text-xs text-destructive">{formErrors.jwksURL}</p>
                  )}
                </div>
              </div>
            </details>
          </div>

          <DialogFooter className="pt-4">
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={isPending}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="accent"
              disabled={isPending}
            >
              {isPending ? (
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

// --- Delete confirmation dialog ---

interface DeleteProviderDialogProps {
  provider: SSOProvider;
  isOpen: boolean;
  onConfirm: () => Promise<void>;
  onCancel: () => void;
  isDeleting?: boolean;
  error?: Error | null;
}

function DeleteProviderDialog({
  provider,
  isOpen,
  onConfirm,
  onCancel,
  isDeleting = false,
  error,
}: DeleteProviderDialogProps) {
  const [confirmName, setConfirmName] = useState("");
  const isConfirmValid = confirmName === provider.name;

  const handleConfirm = async () => {
    if (!isConfirmValid) return;
    await onConfirm();
  };

  const handleCancel = () => {
    setConfirmName("");
    onCancel();
  };

  return (
    <AlertDialog open={isOpen} onOpenChange={(open) => !open && handleCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2 text-destructive">
            <AlertTriangle className="h-5 w-5" />
            Delete SSO Provider
          </AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-4">
              <p>
                Are you sure you want to delete the SSO provider{" "}
                <strong className="text-foreground">{provider.name}</strong>?
              </p>

              <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md text-sm">
                <p className="font-medium text-destructive mb-1">Warning</p>
                <p className="text-muted-foreground">
                  Users authenticating through this provider will no longer be able to sign in.
                  This action cannot be undone.
                </p>
              </div>

              {error && (
                <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md">
                  <p className="text-sm text-destructive">
                    {error.message || "Failed to delete provider"}
                  </p>
                </div>
              )}

              <div className="pt-2">
                <Label htmlFor="confirm-provider-name" className="text-sm">
                  Type <code className="text-destructive">{provider.name}</code> to confirm
                  deletion
                </Label>
                <Input
                  id="confirm-provider-name"
                  value={confirmName}
                  onChange={(e) => setConfirmName(e.target.value)}
                  placeholder={provider.name}
                  className="mt-2"
                  disabled={isDeleting}
                  autoComplete="off"
                />
              </div>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button variant="outline" onClick={handleCancel} disabled={isDeleting}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={handleConfirm}
            disabled={!isConfirmValid || isDeleting}
          >
            {isDeleting && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
            Delete Provider
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export default SSOSettings;
