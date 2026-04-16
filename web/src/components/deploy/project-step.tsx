// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { Project } from "@/types/project";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

interface ProjectStepProps {
  projects: Project[];
  selectedProject: string;
  onProjectChange: (project: string) => void;
  namespaces: string[];
  selectedNamespace: string;
  onNamespaceChange: (namespace: string) => void;
  isClusterScoped?: boolean;
}

export function ProjectStep({
  projects,
  selectedProject,
  onProjectChange,
  namespaces,
  selectedNamespace,
  onNamespaceChange,
  isClusterScoped,
}: ProjectStepProps) {
  return (
    <div className="space-y-6" data-testid="project-step">
      {/* Project selection */}
      <div className="space-y-2">
        <Label htmlFor="project-select">Project</Label>
        <Select value={selectedProject} onValueChange={onProjectChange}>
          <SelectTrigger id="project-select" data-testid="project-select">
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
          Select the project that will own this deployment.
        </p>
      </div>

      {/* Namespace selection — hidden for cluster-scoped RGDs */}
      {!isClusterScoped && (
        <div className="space-y-2">
          <Label htmlFor="namespace-select">Namespace</Label>
          <Select
            value={selectedNamespace}
            onValueChange={onNamespaceChange}
            disabled={!selectedProject || namespaces.length === 0}
          >
            <SelectTrigger id="namespace-select" data-testid="namespace-select">
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
            Target namespace for this deployment.
          </p>
        </div>
      )}
    </div>
  );
}
