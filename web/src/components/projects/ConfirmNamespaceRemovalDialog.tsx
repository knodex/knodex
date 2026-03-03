/**
 * Confirmation dialog for removing a destination namespace from a project.
 * Shows impact warning when instances exist in the namespace being removed.
 */
import { AlertTriangle, Loader2 } from "lucide-react";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";

interface ConfirmNamespaceRemovalDialogProps {
  isOpen: boolean;
  namespace: string;
  instanceCount: number | null; // null = still loading
  isLoadingCount: boolean;
  onConfirm: () => void;
  onCancel: () => void;
  isRemoving?: boolean;
}

export function ConfirmNamespaceRemovalDialog({
  isOpen,
  namespace,
  instanceCount,
  isLoadingCount,
  onConfirm,
  onCancel,
  isRemoving = false,
}: ConfirmNamespaceRemovalDialogProps) {
  const hasInstances = instanceCount !== null && instanceCount > 0;
  const isWildcard = namespace.includes("*");

  return (
    <AlertDialog open={isOpen} onOpenChange={(open) => !open && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2 text-destructive">
            <AlertTriangle className="h-5 w-5" />
            Remove Destination Namespace
          </AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-4">
              <p>
                Are you sure you want to remove the namespace{" "}
                <code className="font-semibold text-foreground">{namespace}</code>{" "}
                from this project?
              </p>

              {isLoadingCount && (
                <div className="flex items-center gap-2 p-3 bg-muted rounded-md text-sm">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Checking for affected instances...
                </div>
              )}

              {hasInstances && (
                <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-md text-sm space-y-2">
                  <p className="font-medium text-destructive">
                    Warning: {instanceCount} active instance{instanceCount !== 1 ? "s" : ""} found
                    {isWildcard ? " in namespaces matching this pattern" : " in this namespace"}
                  </p>
                  <p className="text-muted-foreground">
                    Removing this namespace will cause project members to lose
                    access to {instanceCount} instance{instanceCount !== 1 ? "s" : ""} deployed
                    in matching namespaces. RGDs scoped to this project may also
                    lose visibility for users who only have access through this
                    namespace.
                  </p>
                </div>
              )}

              {isWildcard && !isLoadingCount && instanceCount === null && (
                <div className="p-3 bg-amber-500/10 border border-amber-500/20 rounded-md text-sm space-y-2">
                  <p className="font-medium text-amber-700 dark:text-amber-500">
                    Wildcard pattern — impact unknown
                  </p>
                  <p className="text-muted-foreground">
                    Instance impact cannot be determined for wildcard patterns.
                    Namespaces matching <code>{namespace}</code> may contain
                    active instances. Removing this destination will cause
                    project members to lose access to any instances in matching
                    namespaces. RGDs scoped to this project may also lose
                    visibility.
                  </p>
                </div>
              )}

              {!isLoadingCount && !hasInstances && !isWildcard && (
                <p className="text-sm text-muted-foreground">
                  No active instances were found in{" "}
                  <code>{namespace}</code>. This removal should be safe.
                </p>
              )}
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button
            variant="outline"
            onClick={onCancel}
            disabled={isRemoving}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={onConfirm}
            disabled={isLoadingCount || isRemoving}
          >
            {isRemoving && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
            Remove Anyway
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
