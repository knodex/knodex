// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo, useEffect } from "react";
import { FileDown, Loader2, Info } from "@/lib/icons";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useViolationHistoryCount,
  useExportViolationHistory,
} from "@/hooks/useCompliance";

interface ExportViolationHistoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Active constraint filter from the ViolationsPage (carried over) */
  constraintFilter?: string;
  /** Active resource filter from the ViolationsPage (carried over) */
  resourceFilter?: string;
}

/** Preset time range options */
const TIME_PRESETS = [
  { label: "Last 24 hours", value: "1" },
  { label: "Last 7 days", value: "7" },
  { label: "Last 30 days", value: "30" },
  { label: "Last 90 days", value: "90" },
] as const;

export function ExportViolationHistoryDialog({
  open,
  onOpenChange,
  constraintFilter,
  resourceFilter,
}: ExportViolationHistoryDialogProps) {
  const [selectedDays, setSelectedDays] = useState("30");
  const [enforcement, setEnforcement] = useState("all");

  // Reset enforcement filter when dialog opens
  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- intentional reset on dialog open
      setEnforcement("all");
    }
  }, [open]);

  const sinceDate = useMemo(() => {
    const days = parseInt(selectedDays, 10);
    const d = new Date();
    d.setDate(d.getDate() - days);
    return d.toISOString();
  }, [selectedDays]);

  const countParams = useMemo(
    () => ({
      since: sinceDate,
      enforcement: enforcement !== "all" ? enforcement : undefined,
      constraint: constraintFilter || undefined,
      resource: resourceFilter || undefined,
    }),
    [sinceDate, enforcement, constraintFilter, resourceFilter]
  );

  const { data: countData, isLoading: isCountLoading } =
    useViolationHistoryCount(countParams);

  const exportMutation = useExportViolationHistory();

  const handleExport = () => {
    exportMutation.mutate(
      {
        since: sinceDate,
        enforcement: enforcement !== "all" ? enforcement : undefined,
        constraint: constraintFilter || undefined,
        resource: resourceFilter || undefined,
      },
      {
        onSuccess: () => {
          toast.success("Violation history exported successfully");
          onOpenChange(false);
        },
        onError: (error) => {
          toast.error(
            error instanceof Error
              ? error.message
              : "Failed to export violation history"
          );
        },
      }
    );
  };

  const recordCount = countData?.count ?? 0;
  const retentionDays = countData?.retentionDays ?? 90;
  const selectedDaysNum = parseInt(selectedDays, 10);
  const exceedsRetention = selectedDaysNum > retentionDays;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FileDown className="h-5 w-5" />
            Export Violation History
          </DialogTitle>
          <DialogDescription>
            Download violation history as CSV for compliance auditing.
            Records are retained for {retentionDays} days.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          {/* Time range preset chips */}
          <div className="space-y-2">
            {/* eslint-disable-next-line jsx-a11y/label-has-associated-control */}
            <label className="text-sm font-medium" id="time-range-label">Time Range</label>
            <div className="flex flex-wrap gap-2">
              {TIME_PRESETS.map((preset) => (
                <Button
                  key={preset.value}
                  variant={selectedDays === preset.value ? "default" : "outline"}
                  size="sm"
                  onClick={() => setSelectedDays(preset.value)}
                  className="text-xs"
                >
                  {preset.label}
                </Button>
              ))}
            </div>
          </div>

          {/* Enforcement filter */}
          <div className="space-y-2">
            <label className="text-sm font-medium" htmlFor="enforcement-action-select">Enforcement Action</label>
            <Select value={enforcement} onValueChange={setEnforcement}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All</SelectItem>
                <SelectItem value="deny">Deny</SelectItem>
                <SelectItem value="warn">Warn</SelectItem>
                <SelectItem value="dryrun">Dry Run</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* Retention warning */}
          {exceedsRetention && (
            <div className="flex items-start gap-2 rounded-md bg-blue-50 dark:bg-blue-950/30 p-3 text-sm text-blue-800 dark:text-blue-300">
              <Info className="h-4 w-4 mt-0.5 shrink-0" />
              <span>
                History is retained for {retentionDays} days. Showing available data.
              </span>
            </div>
          )}

          {/* Preview count */}
          <div className="rounded-md bg-muted p-3 text-sm">
            {isCountLoading ? (
              <span className="text-muted-foreground">Counting records...</span>
            ) : (
              <span>
                <strong>{recordCount.toLocaleString()}</strong> records match
                your filters
              </span>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={exportMutation.isPending}
          >
            Cancel
          </Button>
          <Button
            onClick={handleExport}
            disabled={exportMutation.isPending || recordCount === 0}
          >
            {exportMutation.isPending ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Exporting...
              </>
            ) : (
              <>
                <FileDown className="mr-2 h-4 w-4" />
                Export CSV
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
