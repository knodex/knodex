// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useFormContext, Controller } from "react-hook-form";
import { Loader2, Link, AlertCircle, Database } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * Represents a dependency reference in a form field
 */
export interface DependencyFieldInfo {
  /** The form field path that references another RGD */
  fieldPath: string;
  /** The target RGD name being referenced */
  targetRGD: string;
  /** The target RGD namespace */
  targetNamespace?: string;
  /** The CEL expression that creates this reference */
  expression?: string;
}

/**
 * Represents an available instance that can be selected
 */
export interface AvailableInstance {
  name: string;
  namespace: string;
  status?: "ready" | "pending" | "error";
  createdAt?: string;
}

interface DependencyFieldSelectorProps {
  name: string;
  label: string;
  description?: string;
  required?: boolean;
  error?: string;
  dependencyInfo: DependencyFieldInfo;
  instances: AvailableInstance[];
  isLoading?: boolean;
  loadError?: string;
}

export function DependencyFieldSelector({
  name,
  label,
  description,
  required,
  error,
  dependencyInfo,
  instances,
  isLoading,
  loadError,
}: DependencyFieldSelectorProps) {
  const { control } = useFormContext();

  // Group instances by namespace
  const instancesByNamespace = useMemo(() => {
    const grouped: Record<string, AvailableInstance[]> = {};
    for (const instance of instances) {
      if (!grouped[instance.namespace]) {
        grouped[instance.namespace] = [];
      }
      grouped[instance.namespace].push(instance);
    }
    return grouped;
  }, [instances]);

  const hasInstances = instances.length > 0;

  return (
    <div className="space-y-1.5">
      <label
        htmlFor={name}
        className="text-sm font-medium text-foreground flex items-center gap-1.5"
      >
        <Link className="h-3.5 w-3.5 text-primary" />
        {label}
        {required && <span className="text-destructive">*</span>}
      </label>

      {/* Dependency info badge */}
      <div className="flex items-center gap-2 mb-1.5">
        <span className="text-xs font-mono text-muted-foreground bg-primary/10 text-primary px-2 py-0.5 rounded">
          References: {dependencyInfo.targetRGD}
        </span>
      </div>

      {isLoading ? (
        <div className="flex items-center gap-2 px-3 py-2 text-sm text-muted-foreground border border-border rounded-md bg-secondary/30">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading available instances...
        </div>
      ) : loadError ? (
        <div className="flex items-center gap-2 px-3 py-2 text-sm text-destructive border border-destructive/50 rounded-md bg-destructive/10">
          <AlertCircle className="h-4 w-4" />
          {loadError}
        </div>
      ) : !hasInstances ? (
        <div className="flex flex-col gap-1 px-3 py-2 text-sm border border-border rounded-md bg-secondary/30">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Database className="h-4 w-4" />
            No instances of {dependencyInfo.targetRGD} available
          </div>
          <p className="text-xs text-muted-foreground">
            Deploy an instance of <span className="font-mono">{dependencyInfo.targetRGD}</span> first.
          </p>
        </div>
      ) : (
        <Controller
          name={name}
          control={control}
          render={({ field }) => (
            <div className="relative">
              <select
                {...field}
                id={name}
                className={cn(
                  "w-full px-3 py-2 text-sm rounded-md border bg-background appearance-none",
                  "focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary",
                  "pr-10",
                  error ? "border-destructive" : "border-border"
                )}
              >
                <option value="">Select an instance...</option>
                {Object.entries(instancesByNamespace).map(([namespace, nsInstances]) => (
                  <optgroup key={namespace} label={namespace}>
                    {nsInstances.map((instance) => (
                      <option key={`${namespace}/${instance.name}`} value={instance.name}>
                        {instance.name}
                        {instance.status && ` (${instance.status})`}
                      </option>
                    ))}
                  </optgroup>
                ))}
              </select>
              <div className="absolute inset-y-0 right-0 flex items-center pr-3 pointer-events-none">
                <svg
                  className="h-4 w-4 text-muted-foreground"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 9l-7 7-7-7"
                  />
                </svg>
              </div>
            </div>
          )}
        />
      )}

      {description && (
        <p className="text-xs text-muted-foreground">{description}</p>
      )}

      {error && <p className="text-xs text-destructive">{error}</p>}

      {dependencyInfo.expression && (
        <p className="text-xs font-mono text-muted-foreground/70 truncate">
          CEL: {dependencyInfo.expression}
        </p>
      )}
    </div>
  );
}

/**
 * Hook to detect dependency fields from form schema and dependency graph
 */
// eslint-disable-next-line react-refresh/only-export-components -- Hook is co-located with component for cohesion
export function useDependencyFields(
  dependencies: Array<{ field?: string; to: string; expression?: string }> | undefined
): Map<string, DependencyFieldInfo> {
  return useMemo(() => {
    const map = new Map<string, DependencyFieldInfo>();

    if (!dependencies) {
      return map;
    }

    // Extract field references from dependency edges
    for (const dep of dependencies) {
      if (dep.field) {
        // Parse the field path from CEL expression if available
        // e.g., "${db-postgres-rds.spec.dbSubnetGroupName}" -> references "db-postgres-rds"
        map.set(dep.field, {
          fieldPath: dep.field,
          targetRGD: dep.to,
          expression: dep.expression,
        });
      }
    }

    return map;
  }, [dependencies]);
}
