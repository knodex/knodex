// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import type { EnforcementAction } from "@/types/compliance";
import { getEnforcementClassName } from "@/types/compliance";

interface EnforcementActionSelectorProps {
  value: EnforcementAction;
  onChange: (value: EnforcementAction) => void;
}

export function EnforcementActionSelector({ value, onChange }: EnforcementActionSelectorProps) {
  return (
    <div className="space-y-2">
      <Label className="flex items-center gap-1">
        Enforcement Action
        <span className="text-destructive">*</span>
      </Label>
      <Select
        value={value}
        onValueChange={(v) => onChange(v as EnforcementAction)}
      >
        <SelectTrigger data-testid="enforcement-action-select">
          <SelectValue placeholder="Select enforcement action" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="deny">
            <div className="flex items-center gap-2">
              <Badge variant="outline" className={getEnforcementClassName("deny")}>
                deny
              </Badge>
              <span className="text-muted-foreground">
                Block violating resources
              </span>
            </div>
          </SelectItem>
          <SelectItem value="warn">
            <div className="flex items-center gap-2">
              <Badge variant="outline" className={getEnforcementClassName("warn")}>
                warn
              </Badge>
              <span className="text-muted-foreground">
                Warn but allow resources
              </span>
            </div>
          </SelectItem>
          <SelectItem value="dryrun">
            <div className="flex items-center gap-2">
              <Badge variant="outline" className={getEnforcementClassName("dryrun")}>
                dryrun
              </Badge>
              <span className="text-muted-foreground">
                Log only, no enforcement
              </span>
            </div>
          </SelectItem>
        </SelectContent>
      </Select>
      <p className="text-xs text-muted-foreground">
        Controls how violations are handled
      </p>
    </div>
  );
}
