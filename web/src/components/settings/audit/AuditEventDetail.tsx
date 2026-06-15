// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import { Separator } from "@/components/ui/separator";
import { ResultBadge } from "./ResultBadge";
import type { AuditEvent } from "@/types/audit";

interface AuditEventDetailProps {
  event: AuditEvent | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function formatFullTimestamp(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso || "\u2014";
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    timeZoneName: "short",
  }).format(d);
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex justify-between py-2">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm font-medium text-right max-w-[60%] break-all">{value}</span>
    </div>
  );
}

export function AuditEventDetail({ event, open, onOpenChange }: AuditEventDetailProps) {
  // Always render the Sheet so Radix can animate open/close transitions.
  // Content is conditionally rendered inside.
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>Audit Event Details</SheetTitle>
          <SheetDescription>
            {event ? `${event.action} on ${event.resource}/${event.name}` : "No event selected"}
          </SheetDescription>
        </SheetHeader>

        {event && (
          <div className="mt-6 space-y-4">
            {/* Core Info */}
            <div>
              <h3 className="text-sm font-semibold mb-2">Event</h3>
              <div className="rounded-md border p-3 space-y-0 divide-y">
                <DetailRow label="ID" value={<code className="text-xs">{event.id}</code>} />
                <DetailRow label="Time" value={formatFullTimestamp(event.timestamp)} />
                <DetailRow label="Result" value={<ResultBadge result={event.result} />} />
              </div>
            </div>

            <Separator />

            {/* User Info */}
            <div>
              <h3 className="text-sm font-semibold mb-2">User</h3>
              <div className="rounded-md border p-3 space-y-0 divide-y">
                <DetailRow label="Email" value={event.userEmail} />
                <DetailRow label="User ID" value={event.userId} />
                <DetailRow label="Source IP" value={event.sourceIP} />
              </div>
            </div>

            <Separator />

            {/* Action Info */}
            <div>
              <h3 className="text-sm font-semibold mb-2">Action</h3>
              <div className="rounded-md border p-3 space-y-0 divide-y">
                <DetailRow label="Action" value={event.action} />
                <DetailRow label="Resource" value={event.resource} />
                <DetailRow label="Name" value={event.name} />
                {event.project && <DetailRow label="Project" value={event.project} />}
                {event.namespace && <DetailRow label="Namespace" value={event.namespace} />}
              </div>
            </div>

            <Separator />

            {/* Request Info */}
            <div>
              <h3 className="text-sm font-semibold mb-2">Request</h3>
              <div className="rounded-md border p-3 space-y-0 divide-y">
                <DetailRow
                  label="Request ID"
                  value={<code className="text-xs">{event.requestId}</code>}
                />
              </div>
            </div>

            {/* Details map */}
            {event.details && Object.keys(event.details).length > 0 && (
              <>
                <Separator />
                <div>
                  <h3 className="text-sm font-semibold mb-2">Details</h3>
                  <div className="rounded-md border p-3 space-y-0 divide-y">
                    {Object.entries(event.details).map(([key, value]) => (
                      <DetailRow
                        key={key}
                        label={key}
                        value={
                          typeof value === "object"
                            ? JSON.stringify(value, null, 2)
                            : String(value)
                        }
                      />
                    ))}
                  </div>
                </div>
              </>
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
