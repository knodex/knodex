// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { ExternalLink, Loader2, AlertCircle, RefreshCw, ArrowRight } from "@/lib/icons";
import { Link } from "react-router-dom";
import { useFormContext } from "react-hook-form";
import { useK8sResources, useRGDList } from "@/hooks/useRGDs";
import { cn } from "@/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface ExternalRefSelectorProps {
  name: string;
  apiVersion: string;
  kind: string;
  /** The deployment namespace selected at the top of the deploy form */
  deploymentNamespace?: string;
  /** When true, restrict resource listing to the deployment namespace. When false, list across all namespaces. */
  useInstanceNamespace?: boolean;
  /** Maps resource attributes to sub-field names (e.g., { name: "name", namespace: "namespace" }) */
  autoFillFields: Record<string, string>;
  label: string;
  description?: string;
  required?: boolean;
  error?: string;
}

/**
 * ExternalRefSelector renders a dropdown populated with existing K8s resources.
 * When a resource is selected, it auto-fills both name and namespace sub-fields
 * via react-hook-form's setValue.
 */
export function ExternalRefSelector({
  name,
  apiVersion,
  kind,
  deploymentNamespace,
  useInstanceNamespace = true,
  autoFillFields,
  label,
  description,
  required,
  error,
}: ExternalRefSelectorProps) {
  const { setValue, watch } = useFormContext();

  // When useInstanceNamespace is true (default), filter to the deployment namespace.
  // When false, query all namespaces — the resource may live in a shared namespace
  // different from where the instance is deployed (e.g., kubeconfig in eng-shared).
  const effectiveNamespace = useInstanceNamespace ? deploymentNamespace : undefined;
  const isReady = useInstanceNamespace ? !!deploymentNamespace : true;

  // Read the current name value from the form
  const currentName = (watch(`${name}.${autoFillFields.name}`) as string) || "";

  const {
    data: resources,
    isLoading,
    isError,
    error: fetchError,
    refetch,
    isFetching,
  } = useK8sResources(apiVersion, kind, effectiveNamespace, isReady);

  // Handle selection: auto-fill both name and namespace
  const handleChange = (selectedName: string) => {
    if (!selectedName) {
      // Clear both fields
      setValue(`${name}.${autoFillFields.name}`, "");
      setValue(`${name}.${autoFillFields.namespace}`, "");
      return;
    }

    const resource = resources?.find((r) => r.name === selectedName);
    if (!resource) {
      if (import.meta.env.DEV) {
        console.warn(`ExternalRefSelector: resource "${selectedName}" not found in loaded resources`);
      }
      return;
    }
    setValue(`${name}.${autoFillFields.name}`, resource.name);
    setValue(`${name}.${autoFillFields.namespace}`, resource.namespace);
  };

  // Only resolve the required Kind to an RGD when there are no resources to show the "Deploy one now" link
  const isEmpty = !isLoading && !isError && isReady && (!resources || resources.length === 0);
  // Extract API group from apiVersion (e.g., "containerservice.azure.com/v1" → "containerservice.azure.com")
  const producesGroup = useMemo(() => {
    if (!apiVersion) return undefined;
    const parts = apiVersion.split("/");
    // Core group (e.g., "v1") has no group prefix
    return parts.length >= 2 ? parts[0] : undefined;
  }, [apiVersion]);
  const { data: matchingRgds, isLoading: isResolvingRgd, isError: isRgdError } = useRGDList(
    isEmpty ? { producesKind: kind, producesGroup, pageSize: 10 } : undefined
  );

  // Compute the deploy link URL (only relevant when empty)
  const deployUrl = useMemo(() => {
    if (!isEmpty) return null;
    const items = matchingRgds?.items;
    if (!items || items.length === 0) return null;
    if (items.length === 1) {
      return `/catalog/${encodeURIComponent(items[0].name)}`;
    }
    return `/catalog?producesKind=${encodeURIComponent(kind)}`;
  }, [isEmpty, matchingRgds, kind]);

  // Check if error is a 403 Forbidden
  const isForbiddenError =
    fetchError &&
    typeof fetchError === "object" &&
    "response" in fetchError &&
    (fetchError as { response?: { status?: number } }).response?.status === 403;

  return (
    <div className="space-y-1.5" data-testid={`field-${name}`}>
      <label
        htmlFor={name}
        className="text-sm font-medium text-foreground flex items-center gap-2"
      >
        <ExternalLink className="h-3.5 w-3.5 text-status-warning" />
        {label}
        {required && <span className="text-destructive">*</span>}
        <span className="text-xs font-normal text-muted-foreground">
          ({kind})
        </span>
      </label>

      {description && (
        <p className="text-xs text-muted-foreground">{description}</p>
      )}

      <div className="relative">
        <select
          id={name}
          value={currentName}
          onChange={(e) => handleChange(e.target.value)}
          disabled={isLoading || isError || !isReady}
          data-testid={`input-${name}`}
          className={cn(
            "w-full px-3 py-2 text-sm rounded-md border bg-background appearance-none",
            "focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary",
            "disabled:opacity-50 disabled:cursor-not-allowed",
            error ? "border-destructive" : "border-border"
          )}
        >
          {!isReady ? (
            <option value="">Select a deployment namespace to view available {kind}s</option>
          ) : isLoading ? (
            <option value="">Loading {kind}s...</option>
          ) : isError ? (
            <option value="">Failed to load {kind}s</option>
          ) : !resources || resources.length === 0 ? (
            <option value="">
              No {kind}s found{effectiveNamespace ? ` in ${effectiveNamespace}` : ""}
            </option>
          ) : (
            <>
              <option value="">Select a {kind}...</option>
              {resources.map((resource) => (
                <option key={`${resource.namespace}/${resource.name}`} value={resource.name}>
                  {resource.namespace ? `${resource.namespace}/` : ""}
                  {resource.name}
                </option>
              ))}
            </>
          )}
        </select>

        {/* Loading/Refresh indicator */}
        <div className="absolute right-8 top-1/2 -translate-y-1/2">
          {isLoading || isFetching ? (
            <Loader2 className="h-4 w-4 text-muted-foreground animate-spin" />
          ) : isError ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  onClick={() => refetch()}
                  className="p-1 text-muted-foreground hover:text-primary transition-colors"
                >
                  <RefreshCw className="h-4 w-4" />
                </button>
              </TooltipTrigger>
              <TooltipContent>
                <p>Retry loading resources</p>
              </TooltipContent>
            </Tooltip>
          ) : null}
        </div>
      </div>

      {/* Error messages */}
      {isError && (
        <div className="flex items-center gap-1.5 text-xs text-destructive" data-testid={`error-${name}`}>
          <AlertCircle className="h-3.5 w-3.5" />
          {isForbiddenError ? (
            <span data-testid={`error-forbidden-${name}`}>
              Permission denied. The dashboard service account may not have
              access to list {kind} resources.
            </span>
          ) : (
            <span data-testid={`error-fetch-${name}`}>Failed to load {kind} resources. Click refresh to retry.</span>
          )}
        </div>
      )}

      {error && <p className="text-xs text-destructive">{error}</p>}

      {/* Resource count hint */}
      {!isLoading && !isError && resources && resources.length > 0 && isReady && (
        <p className="text-xs text-muted-foreground">
          {resources.length} {kind}
          {resources.length !== 1 ? "s" : ""} available{effectiveNamespace ? ` in ${effectiveNamespace}` : ""}
        </p>
      )}

      {/* Deploy one now link — shown when no resources found */}
      {isEmpty && deployUrl && (
        <Link
          to={deployUrl}
          className="inline-flex items-center gap-1 text-xs text-primary hover:text-primary/80 transition-colors"
          data-testid={`deploy-link-${name}`}
        >
          Deploy one now
          <ArrowRight className="h-3 w-3" />
        </Link>
      )}
      {isEmpty && isResolvingRgd && (
        <p className="text-xs text-muted-foreground" data-testid={`resolving-rgd-${name}`}>
          Checking catalog…
        </p>
      )}
      {isEmpty && !deployUrl && !isResolvingRgd && (matchingRgds || isRgdError) && (
        <p className="text-xs text-muted-foreground" data-testid={`no-rgd-${name}`}>
          No RGD produces this resource
        </p>
      )}
    </div>
  );
}
