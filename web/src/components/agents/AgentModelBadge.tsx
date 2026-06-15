// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Badge } from "@/components/ui/badge";
import type { AgentModel } from "@/api/agents";

/**
 * AgentModelBadge surfaces the AI model an agent runs on (Story 50.4) — the
 * reusable replacement for the removed "Powered by kagent" vendor badge. It
 * renders the server-resolved provider + model name as a neutral chip
 * ("OpenAI · gpt-4.1-mini"); when no model is resolved it renders NOTHING (no
 * empty chip), keeping the agent surface clean and fail-soft (AC #4). Only the
 * provider label and model name ever reach this component — keys, endpoints
 * and secret references stay server-side (NFR-A4/A7).
 */
export function AgentModelBadge({
  model,
  className,
}: {
  model?: AgentModel | null;
  className?: string;
}) {
  if (!model || (!model.provider && !model.name)) {
    return null;
  }
  const label = [model.provider, model.name].filter(Boolean).join(" · ");
  return (
    <Badge data-testid="agent-model-badge" variant="soft" className={className}>
      {label}
    </Badge>
  );
}
