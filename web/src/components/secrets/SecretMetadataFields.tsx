// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { SecretRotation } from "@/types/secret";
import type { SecretMetadataFormValue } from "@/lib/secret-metadata";

interface SecretMetadataFieldsProps {
  value: SecretMetadataFormValue;
  onChange: (next: SecretMetadataFormValue) => void;
  /** Per-field validation errors keyed by `metadata:rotation`, `metadata:docsUrl`, `metadata:expiresAt`. */
  errors?: Record<string, string>;
  /** Render under a small section heading. The dialog supplies its own
   *  spacing; this component just contributes the field block. */
  idPrefix?: string;
}

/**
 * Reusable Rotation / Documentation URL / Expiration form section,
 * used by both Create and Edit dialogs. All three fields are optional —
 * an empty form value tells the server to leave the corresponding
 * label/annotation unset (Create) or cleared (Edit).
 */
export function SecretMetadataFields({
  value,
  onChange,
  errors = {},
  idPrefix = "secret-metadata",
}: SecretMetadataFieldsProps) {
  const set = <K extends keyof SecretMetadataFormValue>(key: K, v: SecretMetadataFormValue[K]) => {
    onChange({ ...value, [key]: v });
  };

  return (
    <div className="space-y-4 rounded-md border border-border bg-muted/30 p-4">
      <div>
        <h3 className="text-sm font-medium">Metadata</h3>
        <p className="text-xs text-muted-foreground">
          Rotation policy, documentation, and expiration are stored as labels and
          annotations on the underlying Kubernetes Secret.
        </p>
      </div>

      {/* Rotation */}
      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-rotation`}>Rotation</Label>
        <Select
          value={value.rotation || "none"}
          onValueChange={(v) => set("rotation", v === "none" ? "" : (v as SecretRotation))}
        >
          <SelectTrigger id={`${idPrefix}-rotation`}>
            <SelectValue placeholder="Select rotation policy" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">—</SelectItem>
            <SelectItem value="manual">Manual</SelectItem>
            <SelectItem value="auto">Auto</SelectItem>
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground">
          Stored as the <code className="font-mono">knodex.io/rotation</code> label.
        </p>
        {errors["metadata:rotation"] && (
          <p className="text-sm text-destructive">{errors["metadata:rotation"]}</p>
        )}
      </div>

      {/* Documentation URL */}
      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-docs-url`}>Documentation URL</Label>
        <Input
          id={`${idPrefix}-docs-url`}
          type="url"
          placeholder="https://wiki.example.com/secrets/my-secret"
          value={value.docsUrl}
          onChange={(e) => set("docsUrl", e.target.value)}
          maxLength={2048}
        />
        <p className="text-xs text-muted-foreground">
          Optional link to a runbook or owner doc. Must be http or https.
        </p>
        {errors["metadata:docsUrl"] && (
          <p className="text-sm text-destructive">{errors["metadata:docsUrl"]}</p>
        )}
      </div>

      {/* Expiration date */}
      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-expires`}>Expiration date</Label>
        <Input
          id={`${idPrefix}-expires`}
          type="date"
          value={value.expiresAtDate}
          onChange={(e) => set("expiresAtDate", e.target.value)}
        />
        <p className="text-xs text-muted-foreground">
          Stored as end-of-day UTC. Past dates are accepted — the secret will
          simply show as expired in the list.
        </p>
        {errors["metadata:expiresAt"] && (
          <p className="text-sm text-destructive">{errors["metadata:expiresAt"]}</p>
        )}
      </div>
    </div>
  );
}
