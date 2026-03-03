/**
 * Project list component for displaying all projects
 */
import { Shield, Search } from "lucide-react";
import { useState, useMemo } from "react";
import { Input } from "@/components/ui/input";
import { ProjectCard } from "./ProjectCard";
import { ProjectCardSkeleton } from "./ProjectCardSkeleton";
import type { Project } from "@/types/project";

interface ProjectListProps {
  projects: Project[];
  onEdit?: (project: Project) => void;
  onDelete?: (projectName: string) => void;
  onClick?: (project: Project) => void;
  canManage?: boolean;
  isLoading?: boolean;
}

export function ProjectList({
  projects,
  onEdit,
  onDelete,
  onClick,
  canManage = false,
  isLoading = false,
}: ProjectListProps) {
  const [searchQuery, setSearchQuery] = useState("");

  const filteredProjects = useMemo(() => {
    if (!searchQuery.trim()) return projects;
    const query = searchQuery.toLowerCase();
    return projects.filter(
      (project) =>
        project.name.toLowerCase().includes(query) ||
        project.description?.toLowerCase().includes(query)
    );
  }, [projects, searchQuery]);

  if (isLoading) {
    return (
      <div className="space-y-3">
        <ProjectCardSkeleton />
        <ProjectCardSkeleton />
        <ProjectCardSkeleton />
      </div>
    );
  }

  if (projects.length === 0) {
    return (
      <div className="text-center py-12 text-muted-foreground">
        <Shield className="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p className="text-sm font-medium">No projects configured</p>
        <p className="text-xs mt-2 max-w-md mx-auto">
          Projects define RBAC boundaries for your deployments.
          They control which repositories and namespaces users can access.
        </p>
        {canManage && (
          <p className="text-xs mt-2">
            Click "Create Project" to define your first project.
          </p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Search */}
      {projects.length > 3 && (
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search projects..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10"
          />
        </div>
      )}

      {/* Project List */}
      <div className="space-y-3">
        {filteredProjects.map((project) => (
          <ProjectCard
            key={project.name}
            project={project}
            onEdit={onEdit}
            onDelete={onDelete}
            onClick={onClick}
            canManage={canManage}
          />
        ))}
        {filteredProjects.length === 0 && searchQuery && (
          <div className="text-center py-8 text-muted-foreground">
            <p className="text-sm">No projects match "{searchQuery}"</p>
          </div>
        )}
      </div>
    </div>
  );
}
