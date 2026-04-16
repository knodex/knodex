// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useId } from "react";
import type { Project } from "@/types/project";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

interface TargetStepProps {
  instanceName: string;
  onInstanceNameChange: (name: string) => void;
  instanceNameError?: string;
  projects: Project[];
  selectedProject: string;
  onProjectChange: (project: string) => void;
  namespaces: string[];
  selectedNamespace: string;
  onNamespaceChange: (namespace: string) => void;
  isClusterScoped?: boolean;
}

export function TargetStep({
  instanceName,
  onInstanceNameChange,
  instanceNameError,
  projects,
  selectedProject,
  onProjectChange,
  namespaces,
  selectedNamespace,
  onNamespaceChange,
  isClusterScoped,
}: TargetStepProps) {
  const nameId = useId();
  const projectId = useId();
  const nsId = useId();

  return (
    <div className="space-y-5" data-testid="target-step">
      {/* Instance Name */}
      <div className="space-y-1.5">
        <Label htmlFor={nameId}>
          Instance Name <span className="text-[var(--brand-primary)]">*</span>
        </Label>
        <Input
          id={nameId}
          value={instanceName}
          onChange={(e) => onInstanceNameChange(e.target.value)}
          placeholder="my-instance"
          autoComplete="off"
          spellCheck={false}
          aria-describedby={instanceNameError ? `${nameId}-error` : `${nameId}-hint`}
        />
        {instanceNameError ? (
          <p id={`${nameId}-error`} className="text-xs text-[var(--status-error)]">
            {instanceNameError}
          </p>
        ) : (
          <p id={`${nameId}-hint`} className="text-xs text-[var(--text-muted)]">
            Lowercase, alphanumeric, and hyphens only
          </p>
        )}
      </div>

      {/* Project */}
      <div className="space-y-1.5">
        <Label htmlFor={projectId}>
          Project <span className="text-[var(--brand-primary)]">*</span>
        </Label>
        <Select value={selectedProject} onValueChange={onProjectChange}>
          <SelectTrigger id={projectId} data-testid="project-select">
            <SelectValue placeholder="Select a project" />
          </SelectTrigger>
          <SelectContent>
            {projects.map((p) => (
              <SelectItem key={p.name} value={p.name}>
                {p.name}
                {p.description && (
                  <span className="ml-2 text-[var(--text-muted)]">
                    — {p.description}
                  </span>
                )}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-[var(--text-muted)]">
          The project that will own this deployment
        </p>
      </div>

      {/* Namespace — hidden for cluster-scoped RGDs */}
      {!isClusterScoped && (
        <div className="space-y-1.5">
          <Label htmlFor={nsId}>
            Namespace <span className="text-[var(--brand-primary)]">*</span>
          </Label>
          <Select
            value={selectedNamespace}
            onValueChange={onNamespaceChange}
            disabled={!selectedProject || namespaces.length === 0}
          >
            <SelectTrigger id={nsId} data-testid="namespace-select">
              <SelectValue
                placeholder={
                  !selectedProject
                    ? "Select a project first"
                    : namespaces.length === 0
                      ? "No namespaces available"
                      : "Select a namespace"
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
          <p className="text-xs text-[var(--text-muted)]">
            Target namespace for this deployment
          </p>
        </div>
      )}
    </div>
  );
}
