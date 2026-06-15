// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import {
  Bot,
  Box,
  FolderOpen,
  GitBranch,
  KeyRound,
  LayoutGrid,
  ScrollText,
  Settings,
  ShieldCheck,
} from "@/lib/icons";
import type { LucideIcon } from "@/lib/icons";

export type NavItemId =
  | "agents"
  | "catalog"
  | "instances"
  | "projects"
  | "secrets"
  | "repositories"
  | "settings"
  | "compliance"
  | "audit";

export interface NavItemMeta {
  id: NavItemId;
  label: string;
  path: string;
  icon: LucideIcon;
}

export const NAV_ITEMS: Record<NavItemId, NavItemMeta> = {
  agents: { id: "agents", label: "Agents", path: "/agents", icon: Bot },
  catalog: { id: "catalog", label: "Catalog", path: "/catalog", icon: LayoutGrid },
  instances: { id: "instances", label: "Instances", path: "/instances", icon: Box },
  projects: { id: "projects", label: "Projects", path: "/projects", icon: FolderOpen },
  secrets: { id: "secrets", label: "Secrets", path: "/secrets", icon: KeyRound },
  repositories: { id: "repositories", label: "Repositories", path: "/repositories", icon: GitBranch },
  settings: { id: "settings", label: "Settings", path: "/settings", icon: Settings },
  compliance: { id: "compliance", label: "Compliance", path: "/compliance", icon: ShieldCheck },
  audit: { id: "audit", label: "Audit", path: "/audit", icon: ScrollText },
};
