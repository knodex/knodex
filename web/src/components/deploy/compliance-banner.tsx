// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { AlertTriangle, ShieldAlert } from "@/lib/icons";
import type { ComplianceValidateViolation } from "@/api/compliance";

interface ComplianceBannerProps {
  violations: ComplianceValidateViolation[];
  result: "warning" | "block";
  onAcknowledge?: () => void;
}

export function ComplianceBanner({ violations, result, onAcknowledge }: ComplianceBannerProps) {
  const [acknowledged, setAcknowledged] = useState(false);

  const isBlock = result === "block";

  const handleAcknowledge = () => {
    setAcknowledged(true);
    onAcknowledge?.();
  };

  return (
    <div className="space-y-3">
      {violations.map((v, i) => (
        <div
          key={`${v.policy}-${i}`}
          role="alert"
          className="rounded-lg p-4 border"
          style={{
            backgroundColor: isBlock ? "rgba(244,63,94,0.1)" : "rgba(245,158,11,0.1)",
            borderColor: isBlock ? "rgba(244,63,94,0.5)" : "rgba(245,158,11,0.5)",
          }}
        >
          <div className="flex items-start gap-2">
            {isBlock ? (
              <ShieldAlert className="h-5 w-5 shrink-0 mt-0.5" style={{ color: "#f43f5e" }} />
            ) : (
              <AlertTriangle className="h-5 w-5 shrink-0 mt-0.5" style={{ color: "#f59e0b" }} />
            )}
            <div>
              <p className="font-medium text-sm" style={{ color: isBlock ? "#f43f5e" : "#f59e0b" }}>
                {v.policy}
              </p>
              <p className="text-sm mt-1" style={{ color: isBlock ? "#fda4af" : "#fcd34d" }}>
                {v.message}
              </p>
              {v.guidance && (
                <p className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
                  {v.guidance}
                </p>
              )}
            </div>
          </div>
        </div>
      ))}

      {!isBlock && !acknowledged && onAcknowledge && (
        <label className="flex items-center gap-2 text-sm cursor-pointer" style={{ color: "var(--text-secondary)" }}>
          <input
            type="checkbox"
            onChange={handleAcknowledge}
            className="rounded"
          />
          Acknowledge warnings and continue
        </label>
      )}
    </div>
  );
}
