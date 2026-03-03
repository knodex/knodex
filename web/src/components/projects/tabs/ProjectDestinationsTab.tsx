/**
 * Project Destinations Tab - Manage project destination namespaces
 * Allows adding/removing destination namespaces with validation and impact warnings
 */
import { useState } from "react";
import { Plus, Trash2, MapPin, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { AxiosError } from "axios";
import { useQuery } from "@tanstack/react-query";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { listInstances } from "@/api/rgd";
import { ConfirmNamespaceRemovalDialog } from "@/components/projects/ConfirmNamespaceRemovalDialog";
import { toUserFriendlyError } from "@/lib/errors";
import type { Project, Destination, UpdateProjectRequest } from "@/types/project";

interface ProjectDestinationsTabProps {
  project: Project;
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  isUpdating: boolean;
  canManage: boolean;
}

// DNS-1123 label: lowercase alphanumeric and hyphens, start/end with alphanumeric
const DNS_1123_PATTERN = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

function isValidNamespace(value: string): boolean {
  if (!value || value.length > 63) return false;
  // Lone wildcard
  if (value === "*") return true;
  // Wildcard patterns: starts or ends with * (matches server's IsWildcard logic)
  // Accepts suffix (dev-*), prefix (*-prod), and combined (*) patterns
  if (value.startsWith("*") || value.endsWith("*")) {
    const nonWildcard = value.replace(/\*/g, "");
    return nonWildcard.length === 0 || /^[a-z0-9-]+$/.test(nonWildcard);
  }
  // Middle-only wildcards are invalid (server rejects these too)
  if (value.includes("*")) return false;
  // Exact DNS-1123 label
  return DNS_1123_PATTERN.test(value);
}

export function ProjectDestinationsTab({
  project,
  onUpdate,
  isUpdating,
  canManage,
}: ProjectDestinationsTabProps) {
  const [newNamespace, setNewNamespace] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);

  // Removal dialog state
  const [pendingRemoval, setPendingRemoval] = useState<{
    destination: Destination;
    index: number;
  } | null>(null);
  const [isRemoving, setIsRemoving] = useState(false);

  const destinations = project.destinations || [];
  const isLastDestination = destinations.length <= 1;

  // Query instances in the namespace being removed (only when dialog is open)
  const namespaceToCheck = pendingRemoval?.destination.namespace || "";
  const isWildcard = namespaceToCheck.includes("*");
  const { data: instanceData, isLoading: isLoadingInstances } = useQuery({
    queryKey: ["instances", { namespace: namespaceToCheck }],
    queryFn: () => listInstances({ namespace: namespaceToCheck }),
    enabled: !!pendingRemoval && !isWildcard,
    staleTime: 30_000,
  });

  // For wildcard patterns, we show a generic warning since we can't easily resolve them client-side
  const instanceCount = pendingRemoval
    ? isWildcard
      ? null // We don't know the count for wildcards
      : (instanceData?.totalCount ?? null)
    : null;

  const handleAdd = async () => {
    const trimmed = newNamespace.trim();

    if (!trimmed) return;

    // Client-side validation
    if (!isValidNamespace(trimmed)) {
      setValidationError(
        "Invalid namespace format. Use lowercase letters, numbers, hyphens, or wildcard patterns (e.g., dev-*, *)."
      );
      return;
    }

    // Duplicate check
    if (destinations.some((d) => d.namespace === trimmed)) {
      setValidationError(`Namespace "${trimmed}" already exists in this project.`);
      return;
    }

    setValidationError(null);
    const updatedDestinations = [...destinations, { namespace: trimmed }];
    try {
      await onUpdate({ destinations: updatedDestinations });
      setNewNamespace("");
    } catch (err) {
      const axiosError = err as AxiosError<{ message?: string; details?: Record<string, string> }>;
      const responseData = axiosError?.response?.data;
      const errorMessage = toUserFriendlyError(
        responseData?.message || (err as Error).message || "Failed to add destination"
      );
      toast.error(errorMessage);
    }
  };

  const handleRemoveClick = (index: number) => {
    const destination = destinations[index];
    // Open confirmation dialog with instance count check
    setPendingRemoval({ destination, index });
  };

  const handleConfirmRemoval = async () => {
    if (!pendingRemoval) return;

    setIsRemoving(true);
    const updatedDestinations = destinations.filter(
      (_, i) => i !== pendingRemoval.index
    );
    try {
      await onUpdate({ destinations: updatedDestinations });
      setPendingRemoval(null);
    } catch (err) {
      const axiosError = err as AxiosError<{ message?: string; details?: Record<string, string> }>;
      const responseData = axiosError?.response?.data;
      const errorMessage = toUserFriendlyError(
        responseData?.message || (err as Error).message || "Failed to remove destination"
      );
      toast.error(errorMessage);
    } finally {
      setIsRemoving(false);
    }
  };

  const handleCancelRemoval = () => {
    setPendingRemoval(null);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleAdd();
    }
  };

  if (destinations.length === 0 && !canManage) {
    return (
      <Card>
        <CardContent className="py-12">
          <div className="text-center">
            <MapPin className="h-12 w-12 mx-auto mb-3 text-muted-foreground opacity-50" />
            <p className="text-lg font-medium">No destinations configured</p>
            <p className="text-sm text-muted-foreground mt-2">
              This project has no destination namespaces defined.
            </p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <MapPin className="h-5 w-5" />
                Destination Namespaces
              </CardTitle>
              <p className="text-sm text-muted-foreground mt-1">
                Kubernetes namespaces where this project can deploy resources.
                Supports wildcard patterns like <code>dev-*</code>.
              </p>
            </div>
            <Badge variant="outline">
              {destinations.length} destination{destinations.length !== 1 ? "s" : ""}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Destination List */}
          {destinations.length === 0 ? (
            <div className="text-center py-6">
              <MapPin className="h-8 w-8 mx-auto mb-2 text-muted-foreground opacity-50" />
              <p className="text-sm text-muted-foreground">No destinations configured</p>
            </div>
          ) : (
            <div className="space-y-2">
              {destinations.map((dest, index) => (
                <div
                  key={`${dest.namespace}-${index}`}
                  className="flex items-center gap-2 p-3 bg-secondary rounded-md"
                >
                  <MapPin className="h-4 w-4 text-muted-foreground shrink-0" />
                  <div className="flex-1 min-w-0">
                    <code className="text-sm font-medium">{dest.namespace || "*"}</code>
                    {dest.name && (
                      <span className="text-sm text-muted-foreground ml-2">
                        ({dest.name})
                      </span>
                    )}
                    {dest.namespace?.includes("*") && (
                      <Badge variant="outline" className="ml-2 text-xs">
                        wildcard
                      </Badge>
                    )}
                  </div>
                  {canManage && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleRemoveClick(index)}
                            disabled={isLastDestination || isUpdating}
                            className="text-destructive hover:text-destructive hover:bg-destructive/10"
                            aria-label="Remove destination"
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </span>
                      </TooltipTrigger>
                      {isLastDestination && (
                        <TooltipContent>
                          At least one destination is required
                        </TooltipContent>
                      )}
                    </Tooltip>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Add Destination */}
          {canManage && (
            <div className="pt-2 border-t">
              <div className="flex gap-2">
                <Input
                  value={newNamespace}
                  onChange={(e) => {
                    setNewNamespace(e.target.value);
                    if (validationError) setValidationError(null);
                  }}
                  onKeyDown={handleKeyDown}
                  placeholder="Add namespace (e.g., production, dev-*)"
                  disabled={isUpdating}
                  className="flex-1"
                />
                <Button
                  variant="outline"
                  onClick={handleAdd}
                  disabled={!newNamespace.trim() || isUpdating}
                  aria-label="Add destination"
                >
                  <Plus className="h-4 w-4 mr-1" />
                  Add
                </Button>
              </div>
              {validationError && (
                <p className="mt-2 text-sm text-destructive flex items-center gap-1">
                  <AlertTriangle className="h-3 w-3" />
                  {validationError}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Confirmation Dialog for Namespace Removal */}
      {pendingRemoval && (
        <ConfirmNamespaceRemovalDialog
          isOpen={true}
          namespace={pendingRemoval.destination.namespace || "*"}
          instanceCount={isWildcard ? null : instanceCount}
          isLoadingCount={!isWildcard && isLoadingInstances}
          onConfirm={handleConfirmRemoval}
          onCancel={handleCancelRemoval}
          isRemoving={isRemoving}
        />
      )}
    </div>
  );
}
