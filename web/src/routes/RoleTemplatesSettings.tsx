// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * RoleTemplatesSettings — Settings → Role Templates page (Story 18.1).
 *
 * Lets operators create/edit/delete a reusable catalog of PROJECT-role
 * templates (admin/developer/operator/...) that the project create/edit flow
 * seeds from. Templates are persisted server-side in the knodex-role-templates
 * ConfigMap and surfaced via /v1/settings/role-templates.
 *
 * Templates are UI SEEDS only: applying one copies its (placeholder-bearing)
 * policies into Project.spec.roles[]. Editing/deleting a template does NOT
 * retroactively change roles already embedded in existing projects, and there
 * is no reconcile loop. The single Casbin enforcement layer is unchanged
 * (NFR-T1) — a template never participates in Enforce().
 *
 * Policy strings keep their {project}/{role} placeholders. The PolicyRulesTable
 * is driven with projectId="{project}" / roleName="{role}" so it parses and
 * round-trips the placeholders verbatim (resolution to a concrete project/role
 * happens at apply time, client-side, in @/lib/role-presets).
 *
 * OSS-core: ships in every edition (Postgres/identity not involved; just a
 * ConfigMap). Operator-gating is enforced by the server (settings/* get|update);
 * a non-operator gets a 403 on the list call → Access Denied state. useCanI only
 * drives the UX edit/disable affordances; the server gate is authoritative.
 */
import { useState, useCallback, useMemo } from "react";
import { Plus, Trash2, Loader2, Shield, Search, Settings2, ShieldAlert } from "@/lib/icons";
import { AxiosError } from "axios";
import { toast } from "sonner";
import { ApiError } from "@/api/client";
import { useCanI } from "@/hooks/useCanI";
import {
  useRoleTemplates,
  useCreateRoleTemplate,
  useUpdateRoleTemplate,
  useDeleteRoleTemplate,
} from "@/hooks/useRoleTemplates";
import type { RoleTemplate } from "@/api/role-templates";
import { PolicyRulesTable } from "@/components/projects/PolicyRulesTable";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

// DNS-1123 label, matches the server-side name validation.
const templateNameRegex = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

// Placeholder subject tokens — PolicyRulesTable formats/parses around these so
// the stored {project}/{role} placeholders survive an edit round-trip.
const PLACEHOLDER_PROJECT = "{project}";
const PLACEHOLDER_ROLE = "{role}";

interface TemplateFormState {
  name: string;
  label: string;
  description: string;
  policies: string[];
}

function emptyForm(): TemplateFormState {
  return { name: "", label: "", description: "", policies: [] };
}

function matchesSearch(t: RoleTemplate, q: string): boolean {
  if (!q) return true;
  const needle = q.toLowerCase();
  return (
    t.name.toLowerCase().includes(needle) ||
    t.label.toLowerCase().includes(needle) ||
    (t.description?.toLowerCase().includes(needle) ?? false)
  );
}

export function RoleTemplatesSettings() {
  const { allowed: canUpdate } = useCanI("settings", "update");
  const editable = canUpdate === true;

  const { data, isLoading, error } = useRoleTemplates();
  const createMutation = useCreateRoleTemplate();
  const updateMutation = useUpdateRoleTemplate();
  const deleteMutation = useDeleteRoleTemplate();

  const [view, setView] = useState<"list" | "form">("list");
  const [editing, setEditing] = useState<RoleTemplate | null>(null);
  const [deleting, setDeleting] = useState<RoleTemplate | null>(null);
  const [form, setForm] = useState<TemplateFormState>(emptyForm());
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [query, setQuery] = useState("");

  const templates = useMemo(() => data ?? [], [data]);
  const filtered = useMemo(
    () => templates.filter((t) => matchesSearch(t, query.trim())),
    [templates, query],
  );
  const saving = createMutation.isPending || updateMutation.isPending;

  const openCreate = useCallback(() => {
    setEditing(null);
    setForm(emptyForm());
    setFormErrors({});
    setView("form");
  }, []);

  const openEdit = useCallback((tmpl: RoleTemplate) => {
    setEditing(tmpl);
    setForm({
      name: tmpl.name,
      label: tmpl.label,
      description: tmpl.description ?? "",
      policies: [...tmpl.policies],
    });
    setFormErrors({});
    setView("form");
  }, []);

  const closeForm = useCallback(() => {
    setView("list");
    setEditing(null);
    setForm(emptyForm());
    setFormErrors({});
  }, []);

  const validate = useCallback(
    (isCreate: boolean): Record<string, string> => {
      const errors: Record<string, string> = {};
      if (isCreate) {
        if (!form.name) {
          errors.name = "Name is required";
        } else if (!templateNameRegex.test(form.name)) {
          errors.name =
            "Must be lowercase letters, numbers, and hyphens (DNS-1123 label)";
        } else if (form.name.length > 63) {
          errors.name = "Name must be 63 characters or fewer";
        }
      }
      if (form.policies.length === 0) {
        errors.policies = "At least one policy is required";
      }
      return errors;
    },
    [form],
  );

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const isCreate = !editing;
      const errors = validate(isCreate);
      setFormErrors(errors);
      if (Object.keys(errors).length > 0) return;

      const payload: RoleTemplate = {
        name: isCreate ? form.name : editing.name,
        // Label falls back to the name so the catalog button always has a face.
        label: form.label.trim() || form.name,
        description: form.description || undefined,
        policies: form.policies,
      };

      try {
        if (isCreate) {
          await createMutation.mutateAsync(payload);
          toast.success(`Template "${payload.name}" created`);
        } else {
          await updateMutation.mutateAsync({ name: editing.name, template: payload });
          toast.success(`Template "${editing.name}" updated`);
        }
        closeForm();
      } catch (err) {
        toast.error(
          err instanceof AxiosError
            ? err.response?.data?.message || err.message
            : "Failed to save template",
        );
      }
    },
    [editing, validate, form, createMutation, updateMutation, closeForm],
  );

  const handleDelete = useCallback(async () => {
    if (!deleting) return;
    try {
      await deleteMutation.mutateAsync(deleting.name);
      toast.success(`Template "${deleting.name}" deleted`);
      setDeleting(null);
    } catch (err) {
      toast.error(
        err instanceof AxiosError
          ? err.response?.data?.message || err.message
          : "Failed to delete template",
      );
    }
  }, [deleting, deleteMutation]);

  const isCreate = !editing;
  const formOpen = view === "form";

  // 403 → operator-gate denial. The apiClient interceptor surfaces HTTP errors
  // as ApiError (with a numeric .status), not AxiosError, so check that first —
  // this mirrors UsersSettings. The AxiosError branch is a defensive fallback.
  const status =
    (error as ApiError | undefined)?.status ??
    (error instanceof AxiosError ? error.response?.status : undefined);
  if (status === 401 || status === 403) {
    return (
      <div data-testid="role-templates-access-denied">
        <Card>
          <CardContent className="pt-6">
            <div className="text-center py-12 text-muted-foreground">
              <ShieldAlert className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p className="text-sm font-medium">Access Denied</p>
              <p className="text-xs mt-2">
                You do not have permission to manage role templates.
                <br />
                Contact your administrator if you believe this is an error.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  const hasTemplates = !isLoading && !error && templates.length > 0;

  return (
    <div className="space-y-6" data-testid="role-templates-settings">
      {hasTemplates && (
        <div className="flex items-center gap-3">
          <div className="relative flex-1">
            <Search
              className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none"
              aria-hidden
            />
            <Input
              type="search"
              placeholder="Search templates"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="pl-9 h-10"
              aria-label="Search role templates"
              data-testid="role-templates-search-input"
            />
          </div>
          <Button
            onClick={openCreate}
            disabled={!editable}
            data-testid="create-role-template-button"
            className="h-10"
          >
            <Plus className="h-4 w-4 mr-1.5" />
            New template
          </Button>
        </div>
      )}

      {isLoading ? (
        <div className="space-y-3" data-testid="role-templates-loading">
          {Array.from({ length: 3 }).map((_, i) => (
            <Card key={i}>
              <CardContent className="flex items-start gap-4 px-6 py-5">
                <Skeleton className="h-12 w-12 rounded-xl shrink-0" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-5 w-44" />
                  <Skeleton className="h-3.5 w-72" />
                </div>
                <Skeleton className="h-9 w-24 rounded-md" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : error ? (
        <Card>
          <CardContent className="pt-6" data-testid="role-templates-error">
            <p className="text-sm text-destructive">
              Failed to load role templates
              {status ? ` (HTTP ${status})` : ""}:{" "}
              {error instanceof AxiosError
                ? error.response?.data?.message || error.message
                : error instanceof Error
                  ? error.message
                  : "Unknown error"}
            </p>
          </CardContent>
        </Card>
      ) : templates.length === 0 ? (
        <Card data-testid="role-templates-empty-state">
          <CardContent className="flex flex-col items-center justify-center gap-5 py-20 text-center">
            <div
              className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/15 text-primary ring-1 ring-primary/20"
              aria-hidden
            >
              <Shield className="h-8 w-8" />
            </div>
            <div className="space-y-1.5 max-w-md">
              <h3 className="text-lg font-semibold">No role templates</h3>
              <p className="text-sm text-muted-foreground">
                Role templates are reusable project-role presets your team can
                apply when creating a project. Create one to get started.
              </p>
            </div>
            <Button
              onClick={openCreate}
              disabled={!editable}
              size="lg"
              data-testid="create-role-template-button"
            >
              <Plus className="h-4 w-4 mr-1.5" />
              New template
            </Button>
          </CardContent>
        </Card>
      ) : filtered.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            <Search className="h-6 w-6 mx-auto mb-2 opacity-50" aria-hidden />
            No templates match{" "}
            <span className="font-mono text-foreground">"{query}"</span>.
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3" data-testid="role-templates-list">
          {filtered.map((tmpl) => (
            <Card key={tmpl.name} data-testid={`role-template-card-${tmpl.name}`}>
              <CardContent className="flex items-start gap-4 px-6 py-5">
                <div
                  className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-primary/15 text-primary ring-1 ring-primary/20"
                  aria-hidden
                >
                  <Shield className="h-6 w-6" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <h3 className="text-base font-semibold text-foreground truncate">
                      {tmpl.label || tmpl.name}
                    </h3>
                    <span className="font-mono text-xs text-muted-foreground">
                      {tmpl.name}
                    </span>
                  </div>
                  {tmpl.description && (
                    <p className="text-sm text-muted-foreground mt-1 line-clamp-2">
                      {tmpl.description}
                    </p>
                  )}
                  <p className="text-xs text-muted-foreground mt-2">
                    <span
                      className="text-foreground"
                      data-testid={`role-template-policy-count-${tmpl.name}`}
                    >
                      {tmpl.policies.length}
                    </span>{" "}
                    {tmpl.policies.length === 1 ? "policy" : "policies"}
                  </p>
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => openEdit(tmpl)}
                    disabled={!editable}
                    data-testid={`edit-role-template-${tmpl.name}`}
                    aria-label={`Edit ${tmpl.name}`}
                  >
                    <Settings2 className="h-4 w-4 mr-1.5" />
                    Edit
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-9 w-9 p-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                    onClick={() => setDeleting(tmpl)}
                    disabled={!editable}
                    aria-label={`Delete ${tmpl.name}`}
                    data-testid={`delete-role-template-${tmpl.name}`}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Sheet
        open={formOpen}
        onOpenChange={(open) => {
          if (!open) closeForm();
        }}
      >
        <SheetContent
          side="right"
          className="w-full sm:max-w-xl flex flex-col gap-0 p-0 border-l-0"
          data-testid="role-template-form-drawer"
        >
          <SheetHeader className="px-6 py-4 border-b border-border/60">
            <SheetTitle>{isCreate ? "New template" : "Edit template"}</SheetTitle>
          </SheetHeader>

          <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
            <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
              <div className="space-y-2">
                <Label htmlFor="template-name">
                  Name{" "}
                  <span className="text-destructive" aria-hidden>
                    *
                  </span>
                </Label>
                <Input
                  id="template-name"
                  value={form.name}
                  onChange={(e) =>
                    setForm((f) => ({
                      ...f,
                      name: e.target.value.toLowerCase().replace(/\s+/g, "-"),
                    }))
                  }
                  placeholder="e.g. operator"
                  disabled={!isCreate || saving}
                  required
                  className="font-mono text-sm"
                  data-testid="role-template-name-input"
                />
                {!isCreate && (
                  <p className="text-xs text-muted-foreground">
                    Template name is immutable.
                  </p>
                )}
                {formErrors.name && (
                  <p className="text-sm text-destructive">{formErrors.name}</p>
                )}
              </div>

              <div className="space-y-2">
                <Label htmlFor="template-label">Label</Label>
                <Input
                  id="template-label"
                  value={form.label}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, label: e.target.value }))
                  }
                  placeholder="Display name (defaults to the name)"
                  disabled={saving}
                  data-testid="role-template-label-input"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="template-description">Description</Label>
                <Textarea
                  id="template-description"
                  value={form.description}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, description: e.target.value }))
                  }
                  placeholder="What this role grants"
                  disabled={saving}
                  rows={2}
                  data-testid="role-template-description-input"
                />
              </div>

              <div className="space-y-2">
                <Label className="text-muted-foreground">
                  Policy Rules{" "}
                  <span className="text-destructive" aria-hidden>
                    *
                  </span>
                </Label>
                <p className="text-xs text-muted-foreground">
                  Policies keep <code>{"{project}"}</code> and{" "}
                  <code>{"{role}"}</code> placeholders — they are resolved when
                  the template is applied to a project.
                </p>
                <PolicyRulesTable
                  projectId={PLACEHOLDER_PROJECT}
                  roleName={PLACEHOLDER_ROLE}
                  policies={form.policies}
                  onPoliciesChange={(policies) =>
                    setForm((f) => ({ ...f, policies }))
                  }
                  canEdit={editable && !saving}
                  isLoading={saving}
                />
                {formErrors.policies && (
                  <p className="text-sm text-destructive">
                    {formErrors.policies}
                  </p>
                )}
              </div>
            </div>

            <SheetFooter className="px-6 py-4 border-t border-border/60 sm:justify-end gap-2">
              <Button type="button" variant="ghost" onClick={closeForm}>
                Cancel
              </Button>
              <Button
                type="submit"
                variant="accent"
                disabled={!editable || saving}
                data-testid="role-template-save-button"
              >
                {saving && <Loader2 className="h-4 w-4 mr-1 animate-spin" />}
                {isCreate ? "Create template" : "Save changes"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <AlertDialog
        open={!!deleting}
        onOpenChange={(open) => !open && setDeleting(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete template?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes the template{" "}
              <span className="font-mono">{deleting?.name}</span> from the
              catalog. Projects that already used it keep their copied roles —
              only the reusable preset is removed. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <Button variant="ghost" onClick={() => setDeleting(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
              data-testid="confirm-delete-role-template"
            >
              {deleteMutation.isPending && (
                <Loader2 className="h-4 w-4 mr-1 animate-spin" />
              )}
              Delete
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

export default RoleTemplatesSettings;
