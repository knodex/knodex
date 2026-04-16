// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { toast } from "sonner";
import { Copy, Check } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";

interface YamlPreviewPanelProps {
  yamlPreview: string;
}

export function YamlPreviewPanel({ yamlPreview }: YamlPreviewPanelProps) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>Generated Constraint YAML</Label>
        <CopyButton text={yamlPreview} />
      </div>
      <div className="rounded-lg border bg-slate-950 text-slate-50 p-4 overflow-x-auto max-h-[400px]">
        <pre className="text-sm font-mono whitespace-pre">
          <code>{yamlPreview}</code>
        </pre>
      </div>
      <p className="text-xs text-muted-foreground">
        This is the Kubernetes resource that will be created.
      </p>
    </div>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      toast.success("Copied to clipboard");
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Failed to copy to clipboard");
    }
  };

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      onClick={handleCopy}
      className="flex items-center gap-1"
      data-testid="copy-yaml-btn"
    >
      {copied ? (
        <>
          <Check className="h-3.5 w-3.5" />
          Copied
        </>
      ) : (
        <>
          <Copy className="h-3.5 w-3.5" />
          Copy YAML
        </>
      )}
    </Button>
  );
}
