// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
import { toast } from "sonner";
import { useCreateModel } from "@/hooks/useAgents";
import { useProjectNamespaces } from "@/hooks/useNamespaces";
import { useProjects } from "@/hooks/useProjects";
import { useCurrentProject } from "@/hooks/useAuth";
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

interface CreateModelDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// K8s DNS-1123 subdomain: lowercase alphanumeric, hyphens, dots; max 253 chars.
// Same rule the server validates the model name against (IsValidDNS1123Subdomain).
const DNS_1123_REGEX = /^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$/;
const MAX_NAME_LENGTH = 253;

// Canonical kagent ModelConfig.spec.provider enum values (kagent 0.9.6
// v1alpha2) — the CRD enum is strict and capitalized; lowercase is rejected.
const PROVIDERS = ["OpenAI", "AzureOpenAI", "Anthropic", "Gemini", "Ollama", "Bedrock"] as const;

export function CreateModelDialog({ open, onOpenChange }: CreateModelDialogProps) {
  const globalProject = useCurrentProject();
  const [project, setProject] = useState(globalProject ?? "");
  const [name, setName] = useState("");
  const [namespace, setNamespace] = useState("");
  const [provider, setProvider] = useState<string>("OpenAI");
  const [model, setModel] = useState("gpt-4o");
  const [apiKey, setApiKey] = useState("");
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});

  const { data: projectsData, isLoading: projectsLoading } = useProjects();
  const projects = projectsData?.items ?? [];

  const { data: namespacesData, isLoading: namespacesLoading, isError: namespacesError } =
    useProjectNamespaces(project || undefined);
  const namespaces = namespacesData?.namespaces ?? [];

  const createMutation = useCreateModel();

  const resetForm = useCallback(() => {
    setProject(globalProject ?? "");
    setName("");
    setNamespace("");
    setProvider("OpenAI");
    setModel("gpt-4o");
    setApiKey(""); // never retain the credential after close
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
    } else if (trimmedName.length > MAX_NAME_LENGTH) {
      errors.name = `Name must be at most ${MAX_NAME_LENGTH} characters`;
    } else if (!DNS_1123_REGEX.test(trimmedName)) {
      errors.name = "Name must be lowercase alphanumeric, hyphens, or dots (e.g. my-model)";
    }
    if (!namespace) {
      errors.namespace = "Namespace is required";
    }
    if (!provider) {
      errors.provider = "Provider is required";
    }
    if (!model.trim()) {
      errors.model = "Model is required";
    }
    if (!apiKey) {
      errors.apiKey = "API key is required";
    }
    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  }, [project, name, namespace, provider, model, apiKey]);

  const handleSubmit = useCallback(async () => {
    if (!validate()) return;
    try {
      await createMutation.mutateAsync({
        name: name.trim(),
        provider,
        model: model.trim(),
        namespace,
        apiKey,
      });
      toast.success(`Model "${name.trim()}" created successfully`);
      handleOpenChange(false);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 409) {
          toast.error(`Model "${name.trim()}" already exists`);
        } else if (err.status === 403) {
          toast.error("Permission denied");
        } else {
          toast.error(err.message);
        }
      } else {
        toast.error("Failed to create model");
      }
    }
  }, [validate, createMutation, name, provider, model, namespace, apiKey, handleOpenChange]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[600px] max-h-[90vh] flex flex-col overflow-hidden">
        <DialogHeader>
          <DialogTitle>Create Model</DialogTitle>
          <DialogDescription>
            Create a model config so your agents have a provider/model to run on.
          </DialogDescription>
        </DialogHeader>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            handleSubmit();
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <div className="flex-1 space-y-4 overflow-y-auto py-2 pr-1">
            {/* Project */}
            <div className="space-y-2">
              <Label htmlFor="model-project">Project</Label>
              <Select
                value={project}
                onValueChange={(value) => {
                  setProject(value);
                  setNamespace("");
                }}
                disabled={projectsLoading}
              >
                <SelectTrigger id="model-project">
                  <SelectValue placeholder={projectsLoading ? "Loading projects…" : "Select project"} />
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

            {/* Namespace */}
            <div className="space-y-2">
              <Label htmlFor="model-namespace">Namespace</Label>
              <Select
                value={namespace}
                onValueChange={setNamespace}
                disabled={!project || namespacesLoading || namespacesError}
              >
                <SelectTrigger id="model-namespace">
                  <SelectValue
                    placeholder={
                      namespacesLoading
                        ? "Loading namespaces…"
                        : namespacesError
                          ? "Failed to load namespaces"
                          : "Select namespace"
                    }
                  />
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

            {/* Name */}
            <div className="space-y-2">
              <Label htmlFor="model-name">Name</Label>
              <Input
                id="model-name"
                placeholder="my-model"
                value={name}
                onChange={(e) => setName(e.target.value)}
                maxLength={MAX_NAME_LENGTH}
              />
              <p className="text-xs text-muted-foreground">
                Lowercase letters, numbers, hyphens, and dots only.
              </p>
              {validationErrors.name && (
                <p className="text-sm text-destructive">{validationErrors.name}</p>
              )}
            </div>

            {/* Provider */}
            <div className="space-y-2">
              <Label htmlFor="model-provider">Provider</Label>
              <Select value={provider} onValueChange={setProvider}>
                <SelectTrigger id="model-provider">
                  <SelectValue placeholder="Select provider" />
                </SelectTrigger>
                <SelectContent>
                  {PROVIDERS.map((p) => (
                    <SelectItem key={p} value={p}>
                      {p}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {validationErrors.provider && (
                <p className="text-sm text-destructive">{validationErrors.provider}</p>
              )}
            </div>

            {/* Model */}
            <div className="space-y-2">
              <Label htmlFor="model-model">Model</Label>
              <Input
                id="model-model"
                placeholder="gpt-4o"
                value={model}
                onChange={(e) => setModel(e.target.value)}
              />
              {validationErrors.model && (
                <p className="text-sm text-destructive">{validationErrors.model}</p>
              )}
            </div>

            {/* API key */}
            <div className="space-y-2">
              <Label htmlFor="model-apikey">API key</Label>
              <Input
                id="model-apikey"
                type="password"
                autoComplete="off"
                placeholder="Provider API key"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                Stored as a Kubernetes Secret. Never displayed again after creation.
              </p>
              {validationErrors.apiKey && (
                <p className="text-sm text-destructive">{validationErrors.apiKey}</p>
              )}
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
            <Button type="submit" disabled={createMutation.isPending}>
              {createMutation.isPending ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default CreateModelDialog;
