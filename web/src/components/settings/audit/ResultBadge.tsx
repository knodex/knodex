// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Badge } from "@/components/ui/badge";

/** Badge color based on audit event result */
export function ResultBadge({ result }: { result: string }) {
  const colorClass =
    result === "success"
      ? "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400"
      : result === "denied"
        ? "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400"
        : result === "error"
          ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400"
          : "";

  return (
    <Badge variant="outline" className={colorClass}>
      {result}
    </Badge>
  );
}
