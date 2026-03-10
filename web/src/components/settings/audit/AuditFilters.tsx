// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useEffect, useRef } from "react";
import { X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useProjects } from "@/hooks/useProjects";

const RESULT_OPTIONS = ["success", "denied", "error"] as const;

// TODO: Consider fetching available actions/resources from a /facets API endpoint
// to avoid drift when new action or resource types are added to the backend.
const ACTION_OPTIONS = [
  "login",
  "login_failed",
  "logout",
  "create",
  "update",
  "delete",
  "get",
  "list",
  "denied",
] as const;

const RESOURCE_OPTIONS = [
  "auth",
  "projects",
  "instances",
  "rgds",
  "repositories",
  "settings",
  "sso",
] as const;

import { isoToLocalDatetime } from "./utils";

const DEBOUNCE_MS = 400;

export interface AuditFiltersProps {
  userId: string;
  action: string;
  resource: string;
  project: string;
  result: string;
  from: string;
  to: string;
  onFilterChange: (key: string, value: string) => void;
  onClearFilters: () => void;
}

export function AuditFilters({
  userId,
  action,
  resource,
  project,
  result,
  from,
  to,
  onFilterChange,
  onClearFilters,
}: AuditFiltersProps) {
  // Fetch projects for dropdown
  const { data: projects, isLoading: projectsLoading } = useProjects();

  // Local state for debounced text inputs
  const [localUserId, setLocalUserId] = useState(userId);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  // Sync local state when props change (e.g. clear filters)
  useEffect(() => { setLocalUserId(userId); }, [userId]);

  const hasFilters = userId || action || resource || project || result || from || to;

  // Debounced update for text inputs
  const updateFilterDebounced = useCallback(
    (key: string, value: string) => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        onFilterChange(key, value);
      }, DEBOUNCE_MS);
    },
    [onFilterChange]
  );

  // Cleanup debounce timer on unmount
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  return (
    <div className="flex flex-wrap items-end gap-3">
      {/* User filter */}
      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">User</label>
        <Input
          placeholder="Filter by user..."
          value={localUserId}
          onChange={(e) => {
            setLocalUserId(e.target.value);
            updateFilterDebounced("userId", e.target.value);
          }}
          className="h-9 w-[160px]"
        />
      </div>

      {/* Action filter */}
      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">Action</label>
        <Select
          value={action || "__all__"}
          onValueChange={(v) => onFilterChange("action", v === "__all__" ? "" : v)}
        >
          <SelectTrigger className="h-9 w-[140px]">
            <SelectValue placeholder="All actions" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All actions</SelectItem>
            {ACTION_OPTIONS.map((a) => (
              <SelectItem key={a} value={a}>
                {a}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Resource filter */}
      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">Resource</label>
        <Select
          value={resource || "__all__"}
          onValueChange={(v) => onFilterChange("resource", v === "__all__" ? "" : v)}
        >
          <SelectTrigger className="h-9 w-[140px]">
            <SelectValue placeholder="All resources" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All resources</SelectItem>
            {RESOURCE_OPTIONS.map((r) => (
              <SelectItem key={r} value={r}>
                {r}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Project filter */}
      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">Project</label>
        <Select
          value={project || "__all__"}
          onValueChange={(v) => onFilterChange("project", v === "__all__" ? "" : v)}
          disabled={projectsLoading}
        >
          <SelectTrigger className="h-9 w-[160px]">
            <SelectValue placeholder={projectsLoading ? "Loading..." : "All projects"} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All projects</SelectItem>
            {projects?.items?.map((p) => (
              <SelectItem key={p.name} value={p.name}>
                {p.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Result filter */}
      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">Result</label>
        <Select
          value={result || "__all__"}
          onValueChange={(v) => onFilterChange("result", v === "__all__" ? "" : v)}
        >
          <SelectTrigger className="h-9 w-[130px]">
            <SelectValue placeholder="All results" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">All results</SelectItem>
            {RESULT_OPTIONS.map((r) => (
              <SelectItem key={r} value={r}>
                {r}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Date range: from */}
      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">From</label>
        <Input
          type="datetime-local"
          value={from ? isoToLocalDatetime(from) : ""}
          onChange={(e) => {
            const v = e.target.value;
            onFilterChange("from", v ? new Date(v).toISOString() : "");
          }}
          className="h-9 w-[180px]"
        />
      </div>

      {/* Date range: to */}
      <div className="space-y-1">
        <label className="text-xs font-medium text-muted-foreground">To</label>
        <Input
          type="datetime-local"
          value={to ? isoToLocalDatetime(to) : ""}
          onChange={(e) => {
            const v = e.target.value;
            onFilterChange("to", v ? new Date(v).toISOString() : "");
          }}
          className="h-9 w-[180px]"
        />
      </div>

      {/* Clear filters */}
      {hasFilters && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onClearFilters}
          className="h-9"
        >
          <X className="h-4 w-4 mr-1" />
          Clear
        </Button>
      )}
    </div>
  );
}
