// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * PolicyRulesTable - ArgoCD-aligned policy rules editor
 * Implements a structured UI for editing Casbin policy rules
 *
 * ArgoCD Policy Format:
 * p, <subject>, <resource>, <action>, <object>, <effect>
 *
 * Where:
 * - subject: proj:{project}:{role}
 * - resource: projects, rgds, instances, repositories, settings
 * - action: get, create, update, delete, list, * (wildcard)
 * - object: Pattern like {project}/* or *
 * - effect: allow or deny
 *
 * Editing model: every row is inline-editable at once. Changes commit
 * immediately to the parent via onPoliciesChange, so the surrounding form's own
 * Save button persists the whole set — there is no per-row save step. "Add
 * Policy" appends a row you can edit straight away, and you can add several
 * before saving.
 */
import { useState, useCallback } from "react";
import { Plus, Trash2 } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from "@/components/ui/table";
import { ListTableHeader } from "@/components/ui/list-table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import {
  RESOURCES,
  ACTIONS,
  parsePolicyString,
  formatPolicyString,
  RGDS_ALL_CATEGORIES,
  rgdsObjectToCategory,
  categoryToRgdsObject,
} from "@/lib/policy-utils";
import type { PolicyRule } from "@/lib/policy-utils";
import { useCategories } from "@/hooks/useCategories";

interface PolicyRulesTableProps {
  /** Project ID for policy subject formatting */
  projectId: string;
  /** Role name for policy subject formatting */
  roleName: string;
  /** Current policy strings (raw Casbin format) */
  policies: string[];
  /** Callback when policies are updated */
  onPoliciesChange: (policies: string[]) => void;
  /** Whether the user can edit policies */
  canEdit: boolean;
  /** Whether the component is in a loading/saving state */
  isLoading?: boolean;
}

export function PolicyRulesTable({
  projectId,
  roleName,
  policies,
  onPoliciesChange,
  canEdit,
  isLoading = false,
}: PolicyRulesTableProps) {
  // Local rules are the source of truth once mounted; we push every change up
  // to the parent but never read `policies` back (the initializer runs once).
  // Consumers that swap policies wholesale (e.g. applying a template) remount
  // this component, so the initializer re-runs with the new values.
  const [rules, setRules] = useState<PolicyRule[]>(() =>
    policies
      .map((p) => parsePolicyString(p, projectId, roleName))
      .filter((r): r is PolicyRule => r !== null)
  );

  // Categories drive the friendly RGD object picker. OSS endpoint, always
  // available; the server filters to the caller's visible categories and the
  // hook degrades to [] on error, so this never blocks editing.
  const { data: categories = [] } = useCategories();

  // Commit a new rule set: update local state and notify the parent form.
  const commit = useCallback(
    (next: PolicyRule[]) => {
      setRules(next);
      onPoliciesChange(
        next.map((r) => formatPolicyString(r, projectId, roleName))
      );
    },
    [onPoliciesChange, projectId, roleName]
  );

  // Update one field of one row. Changing the resource resets the object
  // because the object grammar differs per resource: rgds objects are category
  // scopes ("*" or "{slug}/*"), everything else is "{project}/*"-shaped.
  const updateField = useCallback(
    (index: number, field: keyof PolicyRule, value: string) => {
      commit(
        rules.map((r, i) => {
          if (i !== index) return r;
          if (field === "resource" && value !== r.resource) {
            return {
              ...r,
              resource: value,
              object: value === "rgds" ? RGDS_ALL_CATEGORIES : `${projectId}/*`,
            };
          }
          return { ...r, [field]: value };
        })
      );
    },
    [commit, rules, projectId]
  );

  // Append a new editable row (sensible instances/get default).
  const handleAddRule = useCallback(() => {
    commit([
      ...rules,
      {
        resource: "instances",
        action: "get",
        object: `${projectId}/*`,
        permission: "allow",
      },
    ]);
  }, [commit, rules, projectId]);

  // Delete a row.
  const handleDeleteRule = useCallback(
    (index: number) => {
      commit(rules.filter((_, i) => i !== index));
    },
    [commit, rules]
  );

  // Render the read-mode object cell. For rgds rows, show the category name
  // ("All categories" for the wildcard) instead of the raw "{slug}/*" object.
  const renderObject = useCallback(
    (rule: PolicyRule): string => {
      if (rule.resource !== "rgds") return rule.object;
      const slug = rgdsObjectToCategory(rule.object);
      if (slug === RGDS_ALL_CATEGORIES) return "All categories";
      return categories.find((c) => c.slug === slug)?.name ?? slug;
    },
    [categories]
  );

  return (
    <div className="space-y-4">
      <Table>
        <ListTableHeader>
          <TableRow>
            <TableHead className="w-[140px]">Resource</TableHead>
            <TableHead className="w-[120px]">Action</TableHead>
            <TableHead>Object</TableHead>
            <TableHead className="w-[100px]">Permission</TableHead>
            {canEdit && <TableHead className="w-[60px]">Actions</TableHead>}
          </TableRow>
        </ListTableHeader>
        <TableBody>
          {rules.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={canEdit ? 5 : 4}
                className="text-center text-muted-foreground py-8"
              >
                No policy rules defined.{" "}
                {canEdit && 'Click "Add Policy" to create one.'}
              </TableCell>
            </TableRow>
          ) : (
            rules.map((rule, index) => (
              <TableRow key={index}>
                {/* Resource */}
                <TableCell>
                  {canEdit ? (
                    <Select
                      value={rule.resource}
                      onValueChange={(v) => updateField(index, "resource", v)}
                      disabled={isLoading}
                    >
                      <SelectTrigger className="h-8">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {RESOURCES.map((r) => (
                          <SelectItem key={r} value={r}>
                            {r}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : (
                    <span className="font-mono text-sm">{rule.resource}</span>
                  )}
                </TableCell>

                {/* Action */}
                <TableCell>
                  {canEdit ? (
                    <Select
                      value={rule.action}
                      onValueChange={(v) => updateField(index, "action", v)}
                      disabled={isLoading}
                    >
                      <SelectTrigger className="h-8">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {ACTIONS.map((a) => (
                          <SelectItem key={a} value={a}>
                            {a === "*" ? "* (all)" : a}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : (
                    <span className="font-mono text-sm">
                      {rule.action === "*" ? "* (all)" : rule.action}
                    </span>
                  )}
                </TableCell>

                {/* Object */}
                <TableCell>
                  {canEdit ? (
                    rule.resource === "rgds" ? (
                      (() => {
                        // Category scope picker. Preserve an unknown/legacy slug
                        // (e.g. a category with no visible RGDs) as its own
                        // option so editing round-trips losslessly.
                        const current = rgdsObjectToCategory(rule.object);
                        const extra =
                          current !== RGDS_ALL_CATEGORIES &&
                          !categories.some((c) => c.slug === current)
                            ? current
                            : null;
                        return (
                          <Select
                            value={current}
                            onValueChange={(v) =>
                              updateField(
                                index,
                                "object",
                                categoryToRgdsObject(v)
                              )
                            }
                            disabled={isLoading}
                          >
                            <SelectTrigger className="h-8">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value={RGDS_ALL_CATEGORIES}>
                                All categories
                              </SelectItem>
                              {categories.map((c) => (
                                <SelectItem key={c.slug} value={c.slug}>
                                  {c.name}
                                </SelectItem>
                              ))}
                              {extra && (
                                <SelectItem value={extra}>{extra}</SelectItem>
                              )}
                            </SelectContent>
                          </Select>
                        );
                      })()
                    ) : (
                      <Input
                        value={rule.object}
                        onChange={(e) =>
                          updateField(index, "object", e.target.value)
                        }
                        placeholder={`${projectId}/*`}
                        className="h-8 font-mono text-sm"
                        disabled={isLoading}
                      />
                    )
                  ) : (
                    <span className="font-mono text-sm">
                      {renderObject(rule)}
                    </span>
                  )}
                </TableCell>

                {/* Permission */}
                <TableCell>
                  {canEdit ? (
                    <Select
                      value={rule.permission}
                      onValueChange={(v) =>
                        updateField(index, "permission", v)
                      }
                      disabled={isLoading}
                    >
                      <SelectTrigger className="h-8">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="allow">
                          <span className="text-status-success">Allow</span>
                        </SelectItem>
                        <SelectItem value="deny">
                          <span className="text-status-error">Deny</span>
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  ) : (
                    <span
                      className={cn(
                        "font-medium text-sm px-2 py-0.5 rounded",
                        rule.permission === "allow"
                          ? "bg-status-success/10 text-status-success"
                          : "bg-status-error/10 text-status-error"
                      )}
                    >
                      {rule.permission}
                    </span>
                  )}
                </TableCell>

                {/* Actions */}
                {canEdit && (
                  <TableCell>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDeleteRule(index)}
                      disabled={isLoading}
                      aria-label={`Delete rule ${index + 1}`}
                      className="h-7 w-7 p-0 text-destructive hover:text-destructive hover:bg-destructive/10"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>

      {/* Add Policy Button */}
      {canEdit && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={handleAddRule}
          disabled={isLoading}
          className="w-full"
        >
          <Plus className="h-4 w-4 mr-2" />
          Add Policy
        </Button>
      )}
    </div>
  );
}
