// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Project Overview Tab - Display and edit project metadata
 */
import { useState } from "react";
import { Edit, Save, X, Loader2 } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { Project, UpdateProjectRequest } from "@/types/project";

interface ProjectOverviewTabProps {
  project: Project;
  onUpdate: (updates: Partial<UpdateProjectRequest>) => Promise<void>;
  isUpdating: boolean;
  canManage: boolean;
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

  return (
    <div className="space-y-6">
      {/* Basic Information */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Basic Information</CardTitle>
            {canManage && !isEditing && (
              <Button variant="outline" size="sm" onClick={() => setIsEditing(true)}>
                <Edit className="h-4 w-4 mr-2" />
                Edit
              </Button>
            )}
            {isEditing && (
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleCancel}
                  disabled={isUpdating}
                >
                  <X className="h-4 w-4 mr-2" />
                  Cancel
                </Button>
                <Button size="sm" onClick={handleSave} disabled={isUpdating}>
                  {isUpdating ? (
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  ) : (
                    <Save className="h-4 w-4 mr-2" />
                  )}
                  Save
                </Button>
              </div>
            )}
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <Label className="text-muted-foreground">Project Name</Label>
            <p className="font-mono">{project.name}</p>
          </div>

          <div>
            <Label className="text-muted-foreground">Description</Label>
            {isEditing ? (
              <Textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Add a description..."
                className="mt-1"
                rows={3}
              />
            ) : (
              <p className="text-sm">
                {project.description || (
                  <span className="text-muted-foreground italic">No description</span>
                )}
              </p>
            )}
          </div>

          <div>
            <Label className="text-muted-foreground">Resource Version</Label>
            <p className="font-mono text-sm text-muted-foreground">
              {project.resourceVersion}
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Metadata */}
      <Card>
        <CardHeader>
          <CardTitle>Metadata</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label className="text-muted-foreground">Created At</Label>
              <p className="text-sm">
                {new Date(project.createdAt).toLocaleString()}
              </p>
            </div>
            {project.createdBy && (
              <div>
                <Label className="text-muted-foreground">Created By</Label>
                <p className="text-sm">{project.createdBy}</p>
              </div>
            )}
            {project.updatedAt && (
              <div>
                <Label className="text-muted-foreground">Last Updated</Label>
                <p className="text-sm">
                  {new Date(project.updatedAt).toLocaleString()}
                </p>
              </div>
            )}
            {project.updatedBy && (
              <div>
                <Label className="text-muted-foreground">Updated By</Label>
                <p className="text-sm">{project.updatedBy}</p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Summary Stats */}
      <Card>
        <CardHeader>
          <CardTitle>Summary</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4 text-center">
            <div className="p-4 bg-secondary rounded-lg">
              <p className="text-2xl font-bold">{project.roles?.length || 0}</p>
              <p className="text-sm text-muted-foreground">Roles</p>
            </div>
            <div className="p-4 bg-secondary rounded-lg">
              <p className="text-2xl font-bold">
                {project.destinations?.length || 0}
              </p>
              <p className="text-sm text-muted-foreground">Destinations</p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
