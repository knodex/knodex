/**
 * Project card component for displaying project information in a list
 */
import { Users, Shield, Edit, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Project } from "@/types/project";

interface ProjectCardProps {
  project: Project;
  onEdit?: (project: Project) => void;
  onDelete?: (projectName: string) => void;
  onClick?: (project: Project) => void;
  canManage?: boolean;
}

export function ProjectCard({
  project,
  onEdit,
  onDelete,
  onClick,
  canManage = false,
}: ProjectCardProps) {
  const roleCount = project.roles?.length || 0;
  const destinationCount = project.destinations?.length || 0;

  return (
    <div
      className="flex items-center justify-between p-4 rounded-lg border border-border hover:bg-secondary/50 transition-colors cursor-pointer"
      onClick={() => onClick?.(project)}
      data-testid="project-card"
    >
      <div className="flex items-center gap-4 flex-1 min-w-0">
        {/* Icon */}
        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 flex-shrink-0">
          <Shield className="h-5 w-5 text-primary" />
        </div>

        {/* Project Info */}
        <div className="flex-1 min-w-0">
          <Tooltip>
            <TooltipTrigger asChild>
              <div className="font-medium text-sm truncate">{project.name}</div>
            </TooltipTrigger>
            <TooltipContent>
              <p>{project.name}</p>
            </TooltipContent>
          </Tooltip>
          {project.description && (
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="text-xs text-muted-foreground truncate">
                  {project.description}
                </div>
              </TooltipTrigger>
              <TooltipContent>
                <p>{project.description}</p>
              </TooltipContent>
            </Tooltip>
          )}
          <div className="text-xs text-muted-foreground mt-1">
            Created {new Date(project.createdAt).toLocaleDateString()}
            {project.createdBy && ` by ${project.createdBy}`}
          </div>
        </div>

        {/* Badges */}
        <div className="flex items-center gap-2 flex-shrink-0">
          <Badge variant="secondary" className="flex items-center gap-1">
            <Users className="h-3 w-3" />
            <span>{roleCount} role{roleCount !== 1 ? "s" : ""}</span>
          </Badge>
          {destinationCount > 0 && (
            <Badge variant="outline" className="text-xs">
              {destinationCount} destination{destinationCount !== 1 ? "s" : ""}
            </Badge>
          )}
        </div>
      </div>

      {/* Actions */}
      {canManage && (
        <div
          className="flex items-center gap-2 ml-4"
          onClick={(e) => e.stopPropagation()}
        >
          {onEdit && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => onEdit(project)}
                  data-testid="edit-project-btn"
                >
                  <Edit className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p>Edit</p>
              </TooltipContent>
            </Tooltip>
          )}
          {onDelete && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => onDelete(project.name)}
                  className="text-destructive hover:text-destructive hover:bg-destructive/10"
                  data-testid="delete-project-btn"
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p>Delete</p>
              </TooltipContent>
            </Tooltip>
          )}
        </div>
      )}
    </div>
  );
}
