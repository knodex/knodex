// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
import { toast } from "sonner";
import { useCreateSecret } from "@/hooks/useSecrets";
import { useProjectNamespaces } from "@/hooks/useNamespaces";
import { ApiError } from "@/api/client";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { KeyValueEditor } from "./KeyValueEditor";
import { createPairId, type KeyValuePair } from "./keyValueTypes";
import { useCurrentProject } from "@/hooks/useAuth";
import { useProjects } from "@/hooks/useProjects";

interface CreateSecretDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// K8s DNS-1123 subdomain: lowercase alphanumeric, hyphens, dots; max 253 chars
const DNS_1123_REGEX = /^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$/;
const MAX_SECRET_NAME_LENGTH = 253;

const createInitialPairs = (): KeyValuePair[] => [{ id: createPairId(), key: "", value: "", visible: false }];

export function CreateSecretDialog({ open, onOpenChange }: CreateSecretDialogProps) {
  const globalProject = useCurrentProject();
  const [project, setProject] = useState(globalProject ?? "");
  const [name, setName] = useState("");
  const [namespace, setNamespace] = useState("");
  const [pairs, setPairs] = useState<KeyValuePair[]>(createInitialPairs);
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});

  const { data: projectsData, isLoading: projectsLoading } = useProjects();
  const projects = projectsData?.items ?? [];

  const { data: namespacesData, isLoading: namespacesLoading, isError: namespacesError } = useProjectNamespaces(project || undefined);
  const namespaces = namespacesData?.namespaces ?? [];

  const createMutation = useCreateSecret();

  const resetForm = useCallback(() => {
    setProject(globalProject ?? "");
    setName("");
    setNamespace("");
    setPairs(createInitialPairs());
    setValidationErrors({});
  }, [globalProject]);

  const handleOpenChange = useCallback(
    (isOpen: boolean) => {
      if (!isOpen) {
        resetForm();
      }
      onOpenChange(isOpen);
    },
    [onOpenChange, resetForm]
  );

  const validate = useCallback((): boolean => {
    const errors: Record<string, string> = {};

    if (!project) {
      errors.project = "Project is required";
    }
    const trimmedName = name.trim();
    if (!trimmedName) {
      errors.name = "Name is required";
    } else if (trimmedName.length > MAX_SECRET_NAME_LENGTH) {
      errors.name = `Name must be at most ${MAX_SECRET_NAME_LENGTH} characters`;
    } else if (!DNS_1123_REGEX.test(trimmedName)) {
      errors.name = "Name must be lowercase alphanumeric, hyphens, or dots (e.g. my-secret)";
    }
    if (!namespace) {
      errors.namespace = "Namespace is required";
    }

    // Validate key-value pairs
    const nonEmptyPairs = pairs.filter((p) => p.key.trim() || p.value.trim());
    if (nonEmptyPairs.length === 0) {
      errors.keys = "At least one key-value pair is required";
    } else {
      const keys = new Set<string>();
      for (const pair of nonEmptyPairs) {
        if (!pair.key.trim()) {
          errors.keys = "All keys must be non-empty";
          break;
        }
        if (keys.has(pair.key.trim())) {
          errors.keys = `Duplicate key: ${pair.key.trim()}`;
          break;
        }
        keys.add(pair.key.trim());
      }
    }

    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  }, [project, name, namespace, pairs]);

  const handleSubmit = useCallback(async () => {
    if (!validate()) return;

    // Validation already ensures all non-blank pairs have non-empty keys.
    // Skip any remaining all-blank rows (e.g., the initial empty placeholder row).
    const data: Record<string, string> = {};
    for (const pair of pairs) {
      if (pair.key.trim()) {
        data[pair.key.trim()] = pair.value;
      }
    }

    try {
      await createMutation.mutateAsync({ project, name: name.trim(), namespace, data });
      toast.success(`Secret "${name.trim()}" created successfully`);
      handleOpenChange(false);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === "BAD_REQUEST" && err.message.startsWith("Secret already exists:")) {
          toast.error(`Secret "${name.trim()}" already exists`);
        } else if (err.status === 403) {
          toast.error("Permission denied");
        } else {
          toast.error(err.message);
        }
      } else {
        toast.error("Failed to create secret");
      }
    }
  }, [validate, pairs, name, namespace, project, createMutation, handleOpenChange]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>Create Secret</DialogTitle>
          <DialogDescription>
            Create a new Kubernetes secret.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            handleSubmit();
          }}
        >
          <div className="space-y-4 py-2">
            {/* Project */}
            <div className="space-y-2">
              <Label htmlFor="secret-project">Project</Label>
              <Select
                value={project}
                onValueChange={(value) => {
                  setProject(value);
                  setNamespace("");
                }}
                disabled={projectsLoading}
              >
                <SelectTrigger id="secret-project">
                  <SelectValue placeholder={
                    projectsLoading ? "Loading projects…" : "Select project"
                  } />
                </SelectTrigger>
                <SelectContent>
                  {projects.map((p) => (
                    <SelectItem key={p.name} value={p.name}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {validationErrors.project && (
                <p className="text-sm text-destructive">{validationErrors.project}</p>
              )}
            </div>

            {/* Name */}
            <div className="space-y-2">
              <Label htmlFor="secret-name">Name</Label>
              <Input
                id="secret-name"
                placeholder="my-secret"
                value={name}
                onChange={(e) => setName(e.target.value)}
                maxLength={MAX_SECRET_NAME_LENGTH}
              />
              <p className="text-xs text-muted-foreground">
                Lowercase letters, numbers, hyphens, and dots only.
              </p>
              {validationErrors.name && (
                <p className="text-sm text-destructive">{validationErrors.name}</p>
              )}
            </div>

            {/* Namespace */}
            <div className="space-y-2">
              <Label htmlFor="secret-namespace">Namespace</Label>
              <Select value={namespace} onValueChange={setNamespace} disabled={!project || namespacesLoading || namespacesError}>
                <SelectTrigger id="secret-namespace">
                  <SelectValue placeholder={
                    namespacesLoading ? "Loading namespaces…" :
                    namespacesError ? "Failed to load namespaces" :
                    "Select namespace"
                  } />
                </SelectTrigger>
                <SelectContent>
                  {namespaces.map((ns) => (
                    <SelectItem key={ns} value={ns}>
                      {ns}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {namespacesError && (
                <p className="text-sm text-destructive">Failed to load namespaces. Please try again.</p>
              )}
              {validationErrors.namespace && (
                <p className="text-sm text-destructive">{validationErrors.namespace}</p>
              )}
            </div>

            {/* Key-Value Editor */}
            <div className="space-y-2">
              <Label>Data</Label>
              <KeyValueEditor
                pairs={pairs}
                onChange={setPairs}
                errors={validationErrors}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={createMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
