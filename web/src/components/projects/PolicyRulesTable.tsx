/**
 * PolicyRulesTable - ArgoCD-aligned policy rules editor
 * Implements a structured UI for editing Casbin policy rules
 *
 * ArgoCD Policy Format:
 * p, <subject>, <resource>, <action>, <object>, <effect>
 *
 * Where:
 * - subject: proj:{project}:{role}
 * - resource: projects, rgds, instances, applications, repositories, settings
 * - action: get, create, update, delete, list, * (wildcard)
 * - object: Pattern like {project}/* or *
 * - effect: allow or deny
 */
import { useState, useCallback } from "react";
import { Plus, Trash2, Check, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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
  validatePolicyRule,
} from "@/lib/policy-utils";
import type { PolicyRule } from "@/lib/policy-utils";
import { logger } from "@/lib/logger";

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
  // Parse policies into editable rules
  const [rules, setRules] = useState<PolicyRule[]>(() =>
    policies
      .map((p) => parsePolicyString(p, projectId, roleName))
      .filter((r): r is PolicyRule => r !== null)
  );

  // Track editing state
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editingRule, setEditingRule] = useState<PolicyRule | null>(null);

  // Handle adding a new rule
  const handleAddRule = useCallback(() => {
    const newRule: PolicyRule = {
      resource: "instances",
      action: "get",
      object: `${projectId}/*`,
      permission: "allow",
    };
    setRules([...rules, newRule]);
    setEditingIndex(rules.length);
    setEditingRule(newRule);
  }, [rules, projectId]);

  // Handle editing a rule
  const handleEditRule = useCallback((index: number) => {
    setEditingIndex(index);
    setEditingRule({ ...rules[index] });
  }, [rules]);

  // Handle saving an edited rule
  const handleSaveRule = useCallback(() => {
    if (editingIndex === null || !editingRule) return;

    const error = validatePolicyRule(editingRule);
    if (error) {
      logger.warn("[PolicyRulesTable] Validation error:", error);
      return;
    }

    const newRules = [...rules];
    newRules[editingIndex] = editingRule;
    setRules(newRules);
    setEditingIndex(null);
    setEditingRule(null);

    // Convert rules back to policy strings and notify parent
    const newPolicies = newRules.map((r) =>
      formatPolicyString(r, projectId, roleName)
    );
    onPoliciesChange(newPolicies);
  }, [editingIndex, editingRule, rules, projectId, roleName, onPoliciesChange]);

  // Handle canceling edit
  const handleCancelEdit = useCallback(() => {
    // If we were adding a new rule (last index), remove it
    if (editingIndex === rules.length - 1 && editingRule) {
      const originalPolicies = policies
        .map((p) => parsePolicyString(p, projectId, roleName))
        .filter((r): r is PolicyRule => r !== null);
      if (originalPolicies.length < rules.length) {
        setRules(originalPolicies);
      }
    }
    setEditingIndex(null);
    setEditingRule(null);
  }, [editingIndex, editingRule, rules.length, policies, projectId, roleName]);

  // Handle deleting a rule
  const handleDeleteRule = useCallback(
    (index: number) => {
      const newRules = rules.filter((_, i) => i !== index);
      setRules(newRules);

      // Convert rules back to policy strings and notify parent
      const newPolicies = newRules.map((r) =>
        formatPolicyString(r, projectId, roleName)
      );
      onPoliciesChange(newPolicies);
    },
    [rules, projectId, roleName, onPoliciesChange]
  );

  // Handle field changes during editing
  const handleFieldChange = useCallback(
    (field: keyof PolicyRule, value: string) => {
      if (!editingRule) return;
      setEditingRule({ ...editingRule, [field]: value });
    },
    [editingRule]
  );

  return (
    <div className="space-y-4">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-[140px]">Resource</TableHead>
            <TableHead className="w-[120px]">Action</TableHead>
            <TableHead>Object</TableHead>
            <TableHead className="w-[100px]">Permission</TableHead>
            {canEdit && <TableHead className="w-[80px]">Actions</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rules.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={canEdit ? 5 : 4}
                className="text-center text-muted-foreground py-8"
              >
                No policy rules defined. {canEdit && "Click \"Add Policy\" to create one."}
              </TableCell>
            </TableRow>
          ) : (
            rules.map((rule, index) => {
              const isEditing = editingIndex === index;

              return (
                <TableRow key={index}>
                  {/* Resource */}
                  <TableCell>
                    {isEditing && editingRule ? (
                      <Select
                        value={editingRule.resource}
                        onValueChange={(v) => handleFieldChange("resource", v)}
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
                    {isEditing && editingRule ? (
                      <Select
                        value={editingRule.action}
                        onValueChange={(v) => handleFieldChange("action", v)}
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
                    {isEditing && editingRule ? (
                      <Input
                        value={editingRule.object}
                        onChange={(e) =>
                          handleFieldChange("object", e.target.value)
                        }
                        placeholder={`${projectId}/*`}
                        className="h-8 font-mono text-sm"
                        disabled={isLoading}
                      />
                    ) : (
                      <span className="font-mono text-sm">{rule.object}</span>
                    )}
                  </TableCell>

                  {/* Permission */}
                  <TableCell>
                    {isEditing && editingRule ? (
                      <Select
                        value={editingRule.permission}
                        onValueChange={(v) =>
                          handleFieldChange("permission", v)
                        }
                        disabled={isLoading}
                      >
                        <SelectTrigger className="h-8">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="allow">
                            <span className="text-status-success">
                              Allow
                            </span>
                          </SelectItem>
                          <SelectItem value="deny">
                            <span className="text-status-error">
                              Deny
                            </span>
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
                      <div className="flex items-center gap-1">
                        {isEditing ? (
                          <>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={handleSaveRule}
                              disabled={isLoading}
                              className="h-7 w-7 p-0 text-status-success hover:text-status-success hover:bg-status-success/10"
                            >
                              <Check className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={handleCancelEdit}
                              disabled={isLoading}
                              className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground"
                            >
                              <X className="h-4 w-4" />
                            </Button>
                          </>
                        ) : (
                          <>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleEditRule(index)}
                              disabled={isLoading || editingIndex !== null}
                              className="h-7 w-7 p-0"
                            >
                              <span className="sr-only">Edit</span>
                              ✎
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleDeleteRule(index)}
                              disabled={isLoading || editingIndex !== null}
                              className="h-7 w-7 p-0 text-destructive hover:text-destructive hover:bg-destructive/10"
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          </>
                        )}
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              );
            })
          )}
        </TableBody>
      </Table>

      {/* Add Policy Button */}
      {canEdit && editingIndex === null && (
        <Button
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
