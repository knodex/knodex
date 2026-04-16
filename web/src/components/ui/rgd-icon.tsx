// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import {
  Database,
  HardDrive,
  Network,
  Server,
  MessageSquare,
  Activity,
  Shield,
  Box,
  Package,
  Cloud,
  Lock,
  Workflow,
} from "@/lib/icons";

const CATEGORY_ICONS: Record<string, typeof Database> = {
  database: Database,
  storage: HardDrive,
  networking: Network,
  network: Network,
  compute: Server,
  messaging: MessageSquare,
  monitoring: Activity,
  security: Shield,
  application: Package,
  app: Package,
  cloud: Cloud,
  auth: Lock,
  workflow: Workflow,
};

/**
 * Renders the appropriate Lucide icon for a category string.
 * Falls back to Box for unknown categories.
 */
export function CategoryIcon({
  category,
  className,
}: {
  category: string;
  className?: string;
}) {
  const Icon = CATEGORY_ICONS[category.toLowerCase().trim()] || Box;
  return <Icon className={className} />;
}

/**
 * Renders a custom brand icon from /api/v1/icons/{slug}.
 * Falls back to CategoryIcon on load error (unknown slug or network failure).
 */
export function RGDIcon({
  icon,
  category,
  className = "h-5 w-5",
}: {
  icon?: string;
  category: string;
  className?: string;
}) {
  const [prevIcon, setPrevIcon] = useState(icon);
  const [failed, setFailed] = useState(false);

  // Reset error state when the icon slug changes (React-approved render-time state adjustment)
  if (prevIcon !== icon) {
    setPrevIcon(icon);
    setFailed(false);
  }

  if (!icon || failed) {
    return <CategoryIcon category={category} className={className} />;
  }
  return (
    <img
      src={`/api/v1/icons/${icon}`}
      alt={`${icon} icon`}
      className={`${className} object-contain`}
      onError={() => setFailed(true)}
    />
  );
}
