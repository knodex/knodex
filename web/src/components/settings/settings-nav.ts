// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { KeyRound, ScrollText, Settings as SettingsIcon, Shield, ShieldCheck, Users } from "@/lib/icons";
import type { LucideIcon } from "@/lib/icons";
import { isEnterprise } from "@/hooks/useCompliance";

/**
 * Settings menu model for the master-detail shell (SettingsLayout).
 *
 * Single source of truth for which sections appear and how the active item is
 * resolved. Enterprise items appear only under `isEnterprise()`.
 */
/**
 * Sidebar groups. `General` is ungrouped (rendered first, headerless). Empty
 * groups (e.g. BILLING in OSS/EE) are dropped by groupSettingsNavItems, so each
 * edition only shows the headers it has items for.
 */
export type SettingsGroupId = "access" | "billing" | "platform";

const GROUP_LABELS: Record<SettingsGroupId, string> = {
  access: "Access",
  billing: "Billing",
  platform: "Platform",
};

/** Render order of the labelled groups (after the ungrouped General item). */
const GROUP_ORDER: SettingsGroupId[] = ["access", "billing", "platform"];

export interface SettingsNavItem {
  id: string;
  label: string;
  icon: LucideIcon;
  to: string;
  /** Sidebar section. Omitted = ungrouped (the General hub, pinned to the top). */
  group?: SettingsGroupId;
  /** When true, the item is active only on an exact path match (the General hub). */
  exact?: boolean;
  /** Extra exact paths that should mark this item active (e.g. route aliases). */
  aliases?: string[];
}

/** An ordered sidebar section. `label` is null for the leading ungrouped items. */
export interface SettingsNavGroup {
  id: SettingsGroupId | null;
  label: string | null;
  items: SettingsNavItem[];
}

export function getSettingsNavItems(): SettingsNavItem[] {
  // Order here is the intra-group order — groupSettingsNavItems buckets by
  // `group` while preserving insertion order, so ACCESS reads
  // SSO → Role Templates → Users → Teams regardless of edition.
  const items: SettingsNavItem[] = [
    { id: "general", label: "General", icon: SettingsIcon, to: "/settings", exact: true },
    { id: "sso", label: "SSO Providers", icon: KeyRound, to: "/settings/sso", group: "access" },
  ];

  // OSS-core (Story 18.1): operator-managed catalog of reusable PROJECT-role
  // templates the project create/edit flow seeds from. Persisted in a ConfigMap;
  // visible on every edition (not isEnterprise-/cloud-gated).
  items.push({ id: "role-templates", label: "Role Templates", icon: Shield, to: "/settings/role-templates", group: "access" });

  // Users — the local identity.users roster at /settings/users.
  items.push({ id: "users", label: "Users", icon: Users, to: "/settings/users", group: "access" });

  // Teams — the federated TeamsSettings at /settings/teams (teams reference
  // existing external-IdP groups). The Team CRD + roles[].teams[] resolution is
  // standard product RBAC.
  items.push({ id: "teams", label: "Teams", icon: Users, to: "/settings/teams", group: "access" });

  if (isEnterprise()) {
    items.push({ id: "license", label: "License", icon: ShieldCheck, to: "/settings/license", group: "platform" });
    items.push({ id: "audit", label: "Audit", icon: ScrollText, to: "/settings/audit", group: "platform" });
  }

  return items;
}

/**
 * Buckets nav items into ordered sidebar sections for rendering. The ungrouped
 * items (General) lead in a headerless section; the labelled groups follow in
 * GROUP_ORDER. Insertion order is preserved within each group, and groups with
 * no items are omitted (so OSS/EE never render an empty BILLING header).
 */
export function groupSettingsNavItems(items: SettingsNavItem[]): SettingsNavGroup[] {
  const groups: SettingsNavGroup[] = [];
  const ungrouped = items.filter((i) => !i.group);
  if (ungrouped.length > 0) {
    groups.push({ id: null, label: null, items: ungrouped });
  }
  for (const gid of GROUP_ORDER) {
    const groupItems = items.filter((i) => i.group === gid);
    if (groupItems.length > 0) {
      groups.push({ id: gid, label: GROUP_LABELS[gid], items: groupItems });
    }
  }
  return groups;
}

/**
 * resolveActiveSettingsId returns the id of the single menu item that owns the
 * current path, or null. Resolution order avoids prefix collisions:
 *  1. an exact alias match wins (e.g. `/settings/billing/plan` → Plan);
 *  2. an exact `to` match;
 *  3. otherwise the item with the LONGEST segment-boundary prefix of the path
 *     (so `/settings/billing` never claims `/settings/billing/plan`, and a
 *     sub-route like the SSO form keeps its parent highlighted).
 * `exact` items (General → `/settings`) only ever match in step 2.
 */
export function resolveActiveSettingsId(pathname: string, items: SettingsNavItem[]): string | null {
  for (const item of items) {
    if (item.aliases?.includes(pathname)) return item.id;
  }
  for (const item of items) {
    if (pathname === item.to) return item.id;
  }
  let best: SettingsNavItem | null = null;
  for (const item of items) {
    if (item.exact) continue;
    if (pathname.startsWith(item.to + "/") && (!best || item.to.length > best.to.length)) {
      best = item;
    }
  }
  return best?.id ?? null;
}
