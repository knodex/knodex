// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * TeamDrawer — create/edit drawer for Team management.
 *
 * Encapsulates the Sheet-based form (name, description, OIDC groups) used by the
 * TeamsSettings page.
 */
import { useState, useCallback } from "react";
import { Loader2 } from "@/lib/icons";
import { GroupTypeahead } from "@/components/teams/GroupTypeahead";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { TeamFormValues, ObservedGroup } from "@/types/team";

const teamNameRegex = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

interface TeamDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** "create" renders an empty form; "edit" prefills from initialValues with immutable name. */
  mode: "create" | "edit";
  /** Pre-filled values for edit mode. Ignored in create mode. */
  initialValues?: TeamFormValues;
  /** Called with validated form values when the user submits. May throw on server error. */
  onSubmit: (values: TeamFormValues) => Promise<void>;
  isSubmitting: boolean;
  /** Server-side error to display inside the form (e.g. name conflict). */
  errorMessage?: string;
  /** When true, renders oidcGroups as read-only Badge chips instead of the GroupTypeahead. */
  readOnlyGroups?: boolean;
  /** When true, hides the OIDC groups section entirely and drops its validation (e.g. a managed create where the group is provisioned server-side). */
  hideGroups?: boolean;
  /** When true, hides the description field (e.g. a managed create that does not persist a description). */
  hideDescription?: boolean;
  /** Replaces the default SheetTitle content — use to inject a custom header for a specific edition. */
  headerSlot?: React.ReactNode;
  /** Content injected at the very top of the form body, above the name field (e.g. a create-kind selector). */
  topSlot?: React.ReactNode;
  /** Extra content injected after the OIDC groups section (e.g. a managed-team roster panel). */
  children?: React.ReactNode;
  /** Observed-group suggestions for the GroupTypeahead typeahead. */
  suggestions?: ObservedGroup[];
  /** Whether the operator has mutation permission. Controls form field/button state. */
  canEdit?: boolean;
}

export function TeamDrawer({
  open,
  onOpenChange,
  mode,
  initialValues,
  onSubmit,
  isSubmitting,
  errorMessage,
  readOnlyGroups = false,
  hideGroups = false,
  hideDescription = false,
  headerSlot,
  topSlot,
  children,
  suggestions = [],
  canEdit = true,
}: TeamDrawerProps) {
  const [form, setForm] = useState<TeamFormValues>(
    () =>
      initialValues
        ? { ...initialValues }
        : { name: "", description: "", oidcGroups: [] }
  );
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});

  const isCreate = mode === "create";

  const validate = useCallback((): Record<string, string> => {
    const errors: Record<string, string> = {};
    if (isCreate) {
      if (!form.name) {
        errors.name = "Name is required";
      } else if (!teamNameRegex.test(form.name)) {
        errors.name =
          "Must be lowercase letters, numbers, and hyphens (DNS-1123 subdomain)";
      } else if (form.name.length > 253) {
        errors.name = "Name must be 253 characters or fewer";
      }
    }
    if (!readOnlyGroups && !hideGroups && form.oidcGroups.length === 0) {
      errors.oidcGroups = "At least one OIDC group is required";
    }
    return errors;
  }, [form, isCreate, readOnlyGroups, hideGroups]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const errors = validate();
      setFormErrors(errors);
      if (Object.keys(errors).length > 0) return;
      await onSubmit({ ...form });
    },
    [form, validate, onSubmit]
  );

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="w-full sm:max-w-md flex flex-col gap-0 p-0 border-l-0"
        data-testid="team-form-drawer"
      >
        <SheetHeader className="px-6 py-4 border-b border-border/60">
          {headerSlot ?? (
            <SheetTitle>{isCreate ? "New team" : "Edit team"}</SheetTitle>
          )}
        </SheetHeader>

        {/* noValidate: native HTML5 validation is disabled so our custom validation runs consistently. */}
        <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0" noValidate>
          <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
            {topSlot}

            {errorMessage && (
              <p className="text-sm text-destructive" role="alert">
                {errorMessage}
              </p>
            )}

            <div className="space-y-2">
              <Label htmlFor="team-name">
                Team name{" "}
                {isCreate && (
                  <span className="text-destructive" aria-hidden>
                    *
                  </span>
                )}
              </Label>
              <Input
                id="team-name"
                value={form.name}
                onChange={(e) =>
                  setForm((f) => ({ ...f, name: e.target.value }))
                }
                placeholder="e.g. platform-engineering"
                disabled={!isCreate || isSubmitting}
                required={isCreate}
                className="font-mono text-sm"
                data-testid="team-name-input"
              />
              {!isCreate && (
                <p className="text-xs text-muted-foreground">
                  Team name is immutable.
                </p>
              )}
              {formErrors.name && (
                <p className="text-sm text-destructive">{formErrors.name}</p>
              )}
            </div>

            {!hideDescription && (
              <div className="space-y-2">
                <Label htmlFor="team-description">Description</Label>
                <Textarea
                  id="team-description"
                  value={form.description}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, description: e.target.value }))
                  }
                  placeholder="What this team is responsible for"
                  disabled={isSubmitting}
                  rows={3}
                  data-testid="team-description-input"
                />
              </div>
            )}

            {!hideGroups && (
            <div className="space-y-3 pt-2">
              <div className="space-y-1.5">
                <p className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                  OIDC group mappings
                </p>
                <p className="text-xs text-muted-foreground">
                  Anyone in a linked identity-provider group joins this team
                  automatically on next sign-in.
                </p>
              </div>

              {readOnlyGroups ? (
                <div
                  className="flex flex-wrap gap-2"
                  data-testid="readonly-groups"
                >
                  {form.oidcGroups.length > 0 ? (
                    form.oidcGroups.map((g) => (
                      <Badge
                        key={g}
                        variant="secondary"
                        className="font-mono text-xs"
                        data-testid={`group-chip-${g}`}
                      >
                        {g}
                      </Badge>
                    ))
                  ) : (
                    <p className="text-sm text-muted-foreground italic">
                      No groups assigned
                    </p>
                  )}
                </div>
              ) : (
                <>
                  <GroupTypeahead
                    selected={form.oidcGroups}
                    onChange={(groups) =>
                      setForm((f) => ({ ...f, oidcGroups: groups }))
                    }
                    suggestions={suggestions}
                    canEdit={canEdit}
                    isLoading={isSubmitting}
                  />
                  {formErrors.oidcGroups && (
                    <p className="text-sm text-destructive">
                      {formErrors.oidcGroups}
                    </p>
                  )}
                </>
              )}
            </div>
            )}

            {children}
          </div>

          <SheetFooter className="px-6 py-4 border-t border-border/60 sm:justify-end gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="accent"
              disabled={!canEdit || isSubmitting}
              data-testid="team-save-button"
            >
              {isSubmitting && (
                <Loader2 className="h-4 w-4 mr-1 animate-spin" />
              )}
              {isCreate ? "Create team" : "Save changes"}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}
