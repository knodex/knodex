// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useEffect, useRef } from "react";
import { Search, X } from "@/lib/icons";
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
import { filterSelectClasses } from "@/components/ui/filter-bar";
import { cn } from "@/lib/utils";

const RESULT_OPTIONS = ["success", "denied", "error"] as const;

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
  const { data: projects, isLoading: projectsLoading } = useProjects();

  const [localUserId, setLocalUserId] = useState(userId);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => { setLocalUserId(userId); }, [userId]);

  const hasFilters = userId || action || resource || project || result || from || to;

  const updateFilterDebounced = useCallback(
    (key: string, value: string) => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        onFilterChange(key, value);
      }, DEBOUNCE_MS);
    },
    [onFilterChange]
  );

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  return (
    <div className="flex flex-wrap items-center gap-2">
      {/* User search */}
      <div className="relative">
        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
        <Input
          placeholder="Filter by user..."
          value={localUserId}
          onChange={(e) => {
            setLocalUserId(e.target.value);
            updateFilterDebounced("userId", e.target.value);
          }}
          className={cn("pl-8 pr-3 h-8 w-[150px] text-xs bg-transparent border border-[var(--border-default)] hover:border-[var(--border-hover)] transition-colors focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/30 placeholder:text-muted-foreground")}
        />
      </div>

      {/* Action */}
      <Select
        value={action || "__all__"}
        onValueChange={(v) => onFilterChange("action", v === "__all__" ? "" : v)}
      >
        <SelectTrigger className={cn(filterSelectClasses(!!action), "h-8 w-[120px] text-xs")}>
          <SelectValue placeholder="Action" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All actions</SelectItem>
          {ACTION_OPTIONS.map((a) => (
            <SelectItem key={a} value={a}>{a}</SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Resource */}
      <Select
        value={resource || "__all__"}
        onValueChange={(v) => onFilterChange("resource", v === "__all__" ? "" : v)}
      >
        <SelectTrigger className={cn(filterSelectClasses(!!resource), "h-8 w-[130px] text-xs")}>
          <SelectValue placeholder="Resource" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All resources</SelectItem>
          {RESOURCE_OPTIONS.map((r) => (
            <SelectItem key={r} value={r}>{r}</SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Project */}
      <Select
        value={project || "__all__"}
        onValueChange={(v) => onFilterChange("project", v === "__all__" ? "" : v)}
        disabled={projectsLoading}
      >
        <SelectTrigger className={cn(filterSelectClasses(!!project), "h-8 w-[140px] text-xs")}>
          <SelectValue placeholder={projectsLoading ? "Loading..." : "Project"} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All projects</SelectItem>
          {projects?.items?.map((p) => (
            <SelectItem key={p.name} value={p.name}>{p.name}</SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Result */}
      <Select
        value={result || "__all__"}
        onValueChange={(v) => onFilterChange("result", v === "__all__" ? "" : v)}
      >
        <SelectTrigger className={cn(filterSelectClasses(!!result), "h-8 w-[110px] text-xs")}>
          <SelectValue placeholder="Result" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All results</SelectItem>
          {RESULT_OPTIONS.map((r) => (
            <SelectItem key={r} value={r}>{r}</SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* From */}
      <Input
        type="datetime-local"
        value={from ? isoToLocalDatetime(from) : ""}
        onChange={(e) => {
          const v = e.target.value;
          onFilterChange("from", v ? new Date(v).toISOString() : "");
        }}
        className="h-8 w-[160px] text-xs bg-transparent border border-[var(--border-default)] hover:border-[var(--border-hover)] transition-colors focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/30"
        aria-label="From date"
      />

      {/* To */}
      <Input
        type="datetime-local"
        value={to ? isoToLocalDatetime(to) : ""}
        onChange={(e) => {
          const v = e.target.value;
          onFilterChange("to", v ? new Date(v).toISOString() : "");
        }}
        className="h-8 w-[160px] text-xs bg-transparent border border-[var(--border-default)] hover:border-[var(--border-hover)] transition-colors focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/30"
        aria-label="To date"
      />

      {/* Clear */}
      {hasFilters && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onClearFilters}
          className="h-8 px-2 text-xs"
        >
          <X className="h-3.5 w-3.5 mr-1" />
          Clear
        </Button>
      )}
    </div>
  );
}
