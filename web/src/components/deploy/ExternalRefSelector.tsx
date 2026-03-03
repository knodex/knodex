import { ExternalLink, Loader2, AlertCircle, RefreshCw } from "lucide-react";
import { useFormContext } from "react-hook-form";
import { useK8sResources } from "@/hooks/useRGDs";
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
  autoFillFields,
  label,
  description,
  required,
  error,
}: ExternalRefSelectorProps) {
  const { setValue, watch } = useFormContext();

  // Always use the deployment namespace for filtering
  const effectiveNamespace = deploymentNamespace;

  // Read the current name value from the form
  const currentName = (watch(`${name}.${autoFillFields.name}`) as string) || "";

  const {
    data: resources,
    isLoading,
    isError,
    error: fetchError,
    refetch,
    isFetching,
  } = useK8sResources(apiVersion, kind, effectiveNamespace, !!effectiveNamespace);

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
      console.warn(`ExternalRefSelector: resource "${selectedName}" not found in loaded resources`);
      return;
    }
    setValue(`${name}.${autoFillFields.name}`, resource.name);
    setValue(`${name}.${autoFillFields.namespace}`, resource.namespace);
  };

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
          disabled={isLoading || isError || !effectiveNamespace}
          data-testid={`input-${name}`}
          className={cn(
            "w-full px-3 py-2 text-sm rounded-md border bg-background appearance-none",
            "focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary",
            "disabled:opacity-50 disabled:cursor-not-allowed",
            error ? "border-destructive" : "border-border"
          )}
        >
          {!effectiveNamespace ? (
            <option value="">Select a deployment namespace to view available {kind}s</option>
          ) : isLoading ? (
            <option value="">Loading {kind}s...</option>
          ) : isError ? (
            <option value="">Failed to load {kind}s</option>
          ) : !resources || resources.length === 0 ? (
            <option value="">
              No {kind}s found in {effectiveNamespace}
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
      {!isLoading && !isError && resources && resources.length > 0 && effectiveNamespace && (
        <p className="text-xs text-muted-foreground">
          {resources.length} {kind}
          {resources.length !== 1 ? "s" : ""} available in {effectiveNamespace}
        </p>
      )}
    </div>
  );
}
