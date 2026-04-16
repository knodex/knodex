// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useId } from "react";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

interface ProjectStepProps {
  name: string;
  onNameChange: (name: string) => void;
  nameError?: string;
  description: string;
  onDescriptionChange: (description: string) => void;
}

export function ProjectStep({
  name,
  onNameChange,
  nameError,
  description,
  onDescriptionChange,
}: ProjectStepProps) {
  const nameId = useId();
  const descId = useId();

  return (
    <div className="space-y-5" data-testid="project-step">
      {/* Project Name */}
      <div className="space-y-1.5">
        <Label htmlFor={nameId}>
          Project Name <span className="text-[var(--brand-primary)]">*</span>
        </Label>
        <Input
          id={nameId}
          value={name}
          onChange={(e) => onNameChange(e.target.value)}
          placeholder="my-project"
          autoComplete="off"
          spellCheck={false}
          aria-describedby={nameError ? `${nameId}-error` : `${nameId}-hint`}
        />
        {nameError ? (
          <p id={`${nameId}-error`} className="text-xs text-[var(--status-error)]">
            {nameError}
          </p>
        ) : (
          <p id={`${nameId}-hint`} className="text-xs text-[var(--text-muted)]">
            DNS-compatible name (lowercase, alphanumeric, hyphens allowed)
          </p>
        )}
      </div>

      {/* Description */}
      <div className="space-y-1.5">
        <Label htmlFor={descId}>Description</Label>
        <Textarea
          id={descId}
          value={description}
          onChange={(e) => onDescriptionChange(e.target.value)}
          placeholder="A brief description of this project's purpose..."
          rows={3}
          aria-describedby={`${descId}-hint`}
        />
        <p id={`${descId}-hint`} className="text-xs text-[var(--text-muted)]">
          Optional — helps team members understand this project's purpose
        </p>
      </div>
    </div>
  );
}
