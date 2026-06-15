// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo, useState } from "react";
import { toast } from "sonner";
import { ApiError } from "@/api/client";
import { useModelConfigs, usePatchAgentModel } from "@/hooks/useModelConfigs";
import type { AgentModel } from "@/api/agents";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

/** The minimal agent identity the modal needs (name + namespace + bound config). */
export interface EditableAgent {
  name: string;
  namespace: string;
  /** Resolved current model — kept for display/copy only (NOT for preselect). */
  model?: AgentModel | null;
  /** Bound ModelConfig name — pre-selects the EXACT current config by name. */
  modelConfig?: string;
}

interface AgentModelEditModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  agent: EditableAgent | null;
}

/**
 * Edits ONLY which ModelConfig an agent runs on. systemMessage, tools, and type
 * are curated agent behaviour and are never editable here — the server enforces
 * this (it patches a single field) and the modal never surfaces them. Repointing
 * requires a ModelConfig to already exist in the agent's namespace.
 */
export function AgentModelEditModal({ open, onOpenChange, agent }: AgentModelEditModalProps) {
  const { data, isLoading, isError } = useModelConfigs(
    open ? agent?.namespace : undefined,
    open ? agent?.name : undefined
  );
  const patch = usePatchAgentModel();
  // null ⇒ untouched: the effective value falls back to the pre-selected match.
  // The modal is unmounted on close (parent conditional render), so this resets
  // naturally per open — no effect needed.
  const [touched, setTouched] = useState<string | null>(null);

  const configs = useMemo(() => data?.modelConfigs ?? [], [data]);

  // Pre-select the agent's bound ModelConfig by EXACT name — never by reverse-
  // matching {provider, model}, which would silently pick the wrong config when
  // two share a provider+model pair (and a different API-key Secret). Falls back
  // to unselected only if the bound config is absent from the namespace pool
  // (renamed/deleted). Derived in render, not stored — no effect needed.
  const preselected = useMemo(() => {
    const current = agent?.modelConfig;
    return current && configs.some((c) => c.name === current) ? current : "";
  }, [configs, agent?.modelConfig]);

  const selected = touched ?? preselected;

  const handleSave = async () => {
    if (!agent || !selected) return;
    try {
      await patch.mutateAsync({
        namespace: agent.namespace,
        name: agent.name,
        modelConfig: selected,
      });
      toast.success(`Model updated for "${agent.name}"`);
      onOpenChange(false);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 404) {
          toast.error("That model configuration no longer exists");
        } else if (err.status === 403) {
          toast.error("You don't have permission to edit this agent");
        } else {
          toast.error(err.message);
        }
      } else {
        toast.error("Failed to update the agent model");
      }
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Change model</DialogTitle>
          <DialogDescription>
            {agent ? (
              <>
                Repoint <span className="font-medium text-foreground">{agent.name}</span> at a
                different model configuration. Its system message and tools are unchanged.
              </>
            ) : (
              "Select a model configuration."
            )}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2 py-2">
          <Label htmlFor="agent-model-config">Model configuration</Label>
          <Select
            value={selected}
            onValueChange={setTouched}
            disabled={isLoading || isError || configs.length === 0}
          >
            <SelectTrigger id="agent-model-config" data-testid="model-config-select">
              <SelectValue
                placeholder={
                  isLoading
                    ? "Loading model configurations…"
                    : isError
                    ? "Failed to load model configurations"
                    : configs.length === 0
                    ? "No model configurations in this namespace"
                    : "Select a model configuration"
                }
              />
            </SelectTrigger>
            <SelectContent>
              {configs.map((c) => (
                <SelectItem key={c.name} value={c.name}>
                  {c.name}
                  {(c.provider || c.model) && (
                    <span className="text-muted-foreground">
                      {" "}
                      — {[c.provider, c.model].filter(Boolean).join(" · ")}
                    </span>
                  )}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {!isLoading && !isError && configs.length === 0 && (
            <p className="text-xs text-muted-foreground">
              Create a ModelConfig in{" "}
              <span className="font-mono">{agent?.namespace}</span> before changing the model.
            </p>
          )}
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={patch.isPending}
          >
            Cancel
          </Button>
          <Button type="button" onClick={handleSave} disabled={patch.isPending || !selected}>
            {patch.isPending ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
