// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useMemo } from "react";
import { KeyRound, ArrowLeft, Plus, ShieldAlert, Pencil, Trash2, Loader2, AlertTriangle } from "@/lib/icons";
import { Link } from "react-router-dom";
import { AxiosError } from "axios";
import { toast } from "sonner";
import { useCanI } from "@/hooks/useCanI";
import {
  useSSOProviders,
  useCreateSSOProvider,
  useUpdateSSOProvider,
  useDeleteSSOProvider,
} from "@/hooks/useSSOProviders";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
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

  const [view, setView] = useState<"list" | "form">("list");
  const [editingProvider, setEditingProvider] = useState<SSOProvider | null>(null);
  const [deletingProvider, setDeletingProvider] = useState<SSOProvider | null>(null);
  const [formData, setFormData] = useState<ProviderFormData>(emptyForm());
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});

  const { data: providers, isLoading, error } = useSSOProviders();
  const createMutation = useCreateSSOProvider();
  const updateMutation = useUpdateSSOProvider();
  const deleteMutation = useDeleteSSOProvider();

  const providerList = useMemo(() => providers || [], [providers]);

  // --- Navigation ---

  const openCreateForm = useCallback(() => {
    setEditingProvider(null);
    setFormData(emptyForm());
    setFormErrors({});
    setView("form");
  }, []);

  const openEditForm = useCallback((provider: SSOProvider) => {
    setEditingProvider(provider);
    setFormData(providerToForm(provider));
    setFormErrors({});
    setView("form");
  }, []);

  const closeForm = useCallback(() => {
    setView("list");
    setEditingProvider(null);
    setFormData(emptyForm());
    setFormErrors({});
  }, []);

  // --- Validation ---

  const validateForm = useCallback((isCreate: boolean): Record<string, string> => {
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
  }, [formData]);

  // --- Submit ---

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    const isCreate = !editingProvider;
    const errors = validateForm(isCreate);
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
          closeForm();
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
        { name: editingProvider!.name, request: req },
        {
          onSuccess: () => {
            toast.success(`SSO provider "${editingProvider!.name}" updated — changes take effect within seconds`);
            closeForm();
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
  }, [editingProvider, formData, validateForm, createMutation, updateMutation, closeForm]);

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

  // --- Form view ---

  if (view === "form") {
    const isCreate = !editingProvider;
    const isPending = createMutation.isPending || updateMutation.isPending;

    return (
      <div className="py-6">
        <div className="mb-8">
          <button
            onClick={closeForm}
            className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground mb-4"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to SSO Providers
          </button>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <KeyRound className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-sm font-medium text-foreground">
                {isCreate ? "Add SSO Provider" : `Edit ${editingProvider!.name}`}
              </h2>
              <p className="text-muted-foreground">
                {isCreate
                  ? "Configure a new OIDC authentication provider"
                  : "Update OIDC provider configuration"}
              </p>
            </div>
          </div>
        </div>

        <Card>
          <CardContent className="pt-6">
            <form onSubmit={handleSubmit} className="space-y-6">
              {/* Name (create only) */}
              {isCreate && (
                <div className="space-y-2">
                  <Label htmlFor="name">Name</Label>
                  <Input
                    id="name"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    placeholder="e.g. google, keycloak, auth0"
                    disabled={isPending}
                    autoComplete="off"
                  />
                  <p className="text-xs text-muted-foreground">
                    Lowercase letters, numbers, and hyphens only (DNS label format)
                  </p>
                  {formErrors.name && (
                    <p className="text-xs text-destructive">{formErrors.name}</p>
                  )}
                </div>
              )}

              {/* Issuer URL */}
              <div className="space-y-2">
                <Label htmlFor="issuerURL">Issuer URL</Label>
                <Input
                  id="issuerURL"
                  value={formData.issuerURL}
                  onChange={(e) => setFormData({ ...formData, issuerURL: e.target.value })}
                  placeholder="https://accounts.google.com"
                  disabled={isPending}
                  autoComplete="off"
                />
                <p className="text-xs text-muted-foreground">Must use HTTPS</p>
                {formErrors.issuerURL && (
                  <p className="text-xs text-destructive">{formErrors.issuerURL}</p>
                )}
              </div>

              {/* Client ID */}
              <div className="space-y-2">
                <Label htmlFor="clientID">Client ID</Label>
                <Input
                  id="clientID"
                  value={formData.clientID}
                  onChange={(e) => setFormData({ ...formData, clientID: e.target.value })}
                  placeholder="your-client-id"
                  disabled={isPending}
                  autoComplete="off"
                />
                {formErrors.clientID && (
                  <p className="text-xs text-destructive">{formErrors.clientID}</p>
                )}
              </div>

              {/* Token endpoint auth method */}
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
                  >
                    Public client (PKCE)
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">
                  Public clients (PKCE) skip the client secret entirely — recommended when the IdP issues no shared secret (e.g. Supabase Auth).
                </p>
              </div>

              {/* Client Secret — confidential clients only */}
              {formData.tokenEndpointAuthMethod === "client_secret_basic" && (
                <div className="space-y-2">
                  <Label htmlFor="clientSecret">
                    Client Secret{!isCreate && " (leave blank to keep existing)"}
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
                <Label htmlFor="redirectURL">Redirect URL</Label>
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
                />
                <p className="text-xs text-muted-foreground">
                  Comma-separated list of OIDC scopes
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

              {/* Actions */}
              <div className="flex items-center gap-3 pt-2">
                <Button type="submit" disabled={isPending}>
                  {isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  {isCreate ? "Create Provider" : "Save Changes"}
                </Button>
                <Button type="button" variant="outline" onClick={closeForm} disabled={isPending}>
                  Cancel
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      </div>
    );
  }

  // --- List view ---

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
            <KeyRound className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-sm font-medium text-foreground">SSO Providers</h2>
            <p className="text-muted-foreground">Manage OIDC authentication providers</p>
          </div>
        </div>
      </div>

      {/* Provider list */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <KeyRound className="h-5 w-5" />
                Providers
              </CardTitle>
              <CardDescription className="mt-1">
                {providerList.length} provider{providerList.length !== 1 ? "s" : ""} configured
              </CardDescription>
            </div>
            {isLoadingCreate ? (
              <Skeleton className="h-9 w-32" />
            ) : (isErrorCreate || canCreate) ? (
              <Button onClick={openCreateForm}>
                <Plus className="h-4 w-4 mr-2" />
                Add Provider
              </Button>
            ) : null}
          </div>
        </CardHeader>
        <CardContent>
          {error && !is403Error && (
            <div className="mb-4 p-3 bg-destructive/10 border border-destructive/20 rounded-md">
              <p className="text-sm text-destructive">
                Failed to load SSO providers:{" "}
                {error instanceof Error ? error.message : "Unknown error"}
              </p>
            </div>
          )}

          {isLoading ? (
            <div className="space-y-3">
              {[1, 2].map((i) => (
                <div key={i} className="flex items-center gap-4 p-4 border rounded-md">
                  <Skeleton className="h-10 w-10 rounded-lg" />
                  <div className="flex-1 space-y-2">
                    <Skeleton className="h-4 w-1/4" />
                    <Skeleton className="h-3 w-1/2" />
                  </div>
                </div>
              ))}
            </div>
          ) : providerList.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <KeyRound className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p className="text-sm font-medium">No SSO providers configured</p>
              <p className="text-xs mt-2">
                Add an OIDC provider to enable single sign-on authentication.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {providerList.map((provider) => (
                <div
                  key={provider.name}
                  className="flex items-center gap-4 p-4 border rounded-md hover:bg-muted/50 transition-colors"
                >
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                    <KeyRound className="h-5 w-5" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="font-medium text-foreground">{provider.name}</p>
                      {provider.tokenEndpointAuthMethod === "none" && (
                        <Badge variant="outline" className="text-xs" data-testid={`badge-public-${provider.name}`}>
                          Public (PKCE)
                        </Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground truncate">
                      {provider.issuerURL}
                    </p>
                    {provider.scopes.length > 0 && (
                      <div className="flex flex-wrap gap-1 mt-1">
                        {provider.scopes.map((scope) => (
                          <Badge key={scope} variant="secondary" className="text-xs">
                            {scope}
                          </Badge>
                        ))}
                      </div>
                    )}
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {(isLoadingUpdate || isErrorUpdate || canUpdate) && (
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => openEditForm(provider)}
                      >
                        <Pencil className="h-4 w-4" />
                        <span className="sr-only">Edit {provider.name}</span>
                      </Button>
                    )}
                    {(isLoadingDelete || isErrorDelete || canDelete) && (
                      <Button
                        size="sm"
                        variant="ghost"
                        className="text-destructive hover:text-destructive"
                        onClick={() => setDeletingProvider(provider)}
                      >
                        <Trash2 className="h-4 w-4" />
                        <span className="sr-only">Delete {provider.name}</span>
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

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
