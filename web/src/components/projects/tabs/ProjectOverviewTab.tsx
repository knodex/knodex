// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Project Overview Tab — Vercel/Dokploy-style flat property rows.
 * No card wrappers. Divider lines. Inline editing for description.
 */
import { useState } from "react";
import { Edit, Save, X, Loader2, MapPin, Users } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { Project, UpdateProjectRequest } from "@/types/project";

interface ProjectOverviewTabProps {
  project: Project;
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  isUpdating: boolean;
  canManage: boolean;
}

function PropertyRow({ label, children, className }: { label: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={`flex items-center justify-between py-3 border-b border-border ${className || ""}`}>
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm text-foreground">{children}</span>
    </div>
  );
}

export function ProjectOverviewTab({
  project,
  onUpdate,
  isUpdating,
  canManage,
}: ProjectOverviewTabProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [description, setDescription] = useState(project.description || "");

  const handleSave = async () => {
    await onUpdate({ description });
    setIsEditing(false);
  };

  const handleCancel = () => {
    setDescription(project.description || "");
    setIsEditing(false);
  };

  const roleCount = project.roles?.length || 0;
  const destCount = project.destinations?.length || 0;

  return (
    <div className="space-y-8">
      {/* Project Properties */}
      <section>
        <div className="flex items-center justify-between mb-1">
          <h3 className="text-sm font-medium text-foreground">Project Details</h3>
          {canManage && !isEditing && (
            <Button variant="ghost" size="sm" onClick={() => setIsEditing(true)} className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground">
              <Edit className="h-3 w-3 mr-1" />
              Edit
            </Button>
          )}
          {isEditing && (
            <div className="flex gap-1.5">
              <Button variant="ghost" size="sm" onClick={handleCancel} disabled={isUpdating} className="h-7 px-2 text-xs">
                <X className="h-3 w-3 mr-1" />
                Cancel
              </Button>
              <Button size="sm" onClick={handleSave} disabled={isUpdating} className="h-7 px-2 text-xs">
                {isUpdating ? <Loader2 className="h-3 w-3 mr-1 animate-spin" /> : <Save className="h-3 w-3 mr-1" />}
                Save
              </Button>
            </div>
          )}
        </div>

        <div className="border-t border-border">
          <PropertyRow label="Name">
            <code className="font-mono text-xs px-1.5 py-0.5 rounded bg-secondary">{project.name}</code>
          </PropertyRow>

          <div className={`flex ${isEditing ? "flex-col gap-2 py-3" : "items-center justify-between py-3"} border-b border-border`}>
            <span className="text-sm text-muted-foreground">Description</span>
            {isEditing ? (
              <Textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Add a description..."
                rows={2}
                className="text-sm"
              />
            ) : (
              <span className="text-sm text-foreground max-w-[400px] text-right">
                {project.description || <span className="text-muted-foreground italic">No description</span>}
              </span>
            )}
          </div>

          <PropertyRow label="Roles">
            <span className="inline-flex items-center gap-1.5">
              <Users className="h-3 w-3 text-muted-foreground" />
              {roleCount}
            </span>
          </PropertyRow>

          <PropertyRow label="Destinations" className="border-b-0">
            <span className="inline-flex items-center gap-1.5">
              <MapPin className="h-3 w-3 text-muted-foreground" />
              {destCount}
            </span>
          </PropertyRow>
        </div>
      </section>
    </div>
  );
}
