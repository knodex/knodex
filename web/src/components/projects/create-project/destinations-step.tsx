// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useId } from "react";
import { Plus, X, MapPin } from "@/lib/icons";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import type { Destination } from "@/types/project";

interface DestinationsStepProps {
  destinations: Destination[];
  onDestinationsChange: (destinations: Destination[]) => void;
  error?: string;
}

const NAMESPACE_RE = /^[a-z0-9]([a-z0-9*-]*[a-z0-9*])?$/;

export function DestinationsStep({
  destinations,
  onDestinationsChange,
  error,
}: DestinationsStepProps) {
  const inputId = useId();
  const [newNamespace, setNewNamespace] = useState("");
  const [inputError, setInputError] = useState("");

  const addDestination = useCallback(() => {
    const ns = newNamespace.trim();
    if (!ns) return;

    if (!NAMESPACE_RE.test(ns)) {
      setInputError("Lowercase letters, numbers, hyphens, and wildcards (*) only");
      return;
    }

    if (destinations.some((d) => d.namespace === ns)) {
      setInputError("This namespace is already added");
      return;
    }

    onDestinationsChange([...destinations, { namespace: ns }]);
    setNewNamespace("");
    setInputError("");
  }, [newNamespace, destinations, onDestinationsChange]);

  const removeDestination = useCallback(
    (index: number) => {
      onDestinationsChange(destinations.filter((_, i) => i !== index));
    },
    [destinations, onDestinationsChange],
  );

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter") {
        e.preventDefault();
        addDestination();
      }
    },
    [addDestination],
  );

  return (
    <div className="space-y-5" data-testid="destinations-step">
      <div className="space-y-1.5">
        <Label>
          Allowed Destinations <span className="text-[var(--brand-primary)]">*</span>
        </Label>
        <p className="text-xs text-[var(--text-muted)]">
          Kubernetes namespaces where this project can deploy resources.
          Use wildcards like <code className="px-1 py-0.5 rounded bg-white/[0.06] text-[var(--text-secondary)]">dev-*</code> for pattern matching.
        </p>
      </div>

      {/* Error from parent validation */}
      {error && (
        <p className="text-xs text-[var(--status-error)]">{error}</p>
      )}

      {/* Destination list */}
      {destinations.length > 0 && (
        <div className="space-y-1.5">
          {destinations.map((dest, index) => (
            <div
              key={index}
              className="flex items-center justify-between gap-2 rounded-md px-3 py-2.5 bg-white/[0.03] group"
            >
              <div className="flex items-center gap-2 min-w-0">
                <MapPin
                  className="h-3.5 w-3.5 shrink-0"
                  style={{ color: "var(--brand-primary)" }}
                />
                <span
                  className="text-sm font-mono"
                  style={{ color: "var(--text-primary)" }}
                >
                  {dest.namespace}
                </span>
              </div>
              <button
                type="button"
                onClick={() => removeDestination(index)}
                className="shrink-0 rounded-md p-1 transition-colors opacity-0 group-hover:opacity-100 hover:bg-[rgba(255,255,255,0.06)]"
                style={{ color: "var(--text-muted)" }}
                aria-label={`Remove ${dest.namespace}`}
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Add input */}
      <div className="space-y-1.5">
        <div className="flex gap-2">
          <Input
            id={inputId}
            value={newNamespace}
            onChange={(e) => {
              setNewNamespace(e.target.value);
              setInputError("");
            }}
            onKeyDown={handleKeyDown}
            placeholder="Namespace (e.g., my-project, dev-*)"
            autoComplete="off"
            spellCheck={false}
          />
          <button
            type="button"
            onClick={addDestination}
            disabled={!newNamespace.trim()}
            className="flex items-center gap-1.5 px-3 py-2 rounded-md text-sm font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
            style={{
              backgroundColor: newNamespace.trim()
                ? "rgba(255,255,255,0.06)"
                : "transparent",
              color: "var(--text-secondary)",
              border: "1px solid rgba(255,255,255,0.10)",
            }}
          >
            <Plus className="h-4 w-4" />
            Add
          </button>
        </div>
        {inputError && (
          <p className="text-xs text-[var(--status-error)]">{inputError}</p>
        )}
      </div>

      {/* Empty state hint */}
      {destinations.length === 0 && !error && (
        <div
          className="flex flex-col items-center gap-2 py-6 rounded-md border border-dashed"
          style={{ borderColor: "rgba(255,255,255,0.10)", color: "var(--text-muted)" }}
        >
          <MapPin className="h-5 w-5" />
          <p className="text-xs text-center">
            Add at least one namespace to define where this project can deploy.
          </p>
        </div>
      )}
    </div>
  );
}
