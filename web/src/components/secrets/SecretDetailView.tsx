// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { ArrowLeft, Eye, EyeOff, Pencil, Trash2, Clock, Tag } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useCanI } from "@/hooks/useCanI";
import { formatDateTime } from "@/lib/date";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import type { SecretDetail } from "@/types/secret";
import { EditSecretDialog } from "./EditSecretDialog";
import { DeleteSecretDialog } from "./DeleteSecretDialog";

interface SecretDetailViewProps {
  secret: SecretDetail;
  project: string;
  isLoading?: boolean;
}

export function SecretDetailView({ secret, project, isLoading }: SecretDetailViewProps) {
  const navigate = useNavigate();
  const [visibleKeys, setVisibleKeys] = useState<Set<string>>(new Set());
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);

  const { allowed: canUpdate } = useCanI("secrets", "update", project || "-");
  const { allowed: canDelete } = useCanI("secrets", "delete", project || "-");

  const toggleKeyVisibility = (key: string) => {
    setVisibleKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  if (isLoading) {
    return <SecretDetailSkeleton />;
  }

  const entries = Object.entries(secret.data);

  return (
    <section className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => navigate(`/secrets?project=${encodeURIComponent(project)}`)}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">{secret.name}</h1>
            <p className="text-sm text-muted-foreground">{secret.namespace}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {canUpdate && (
            <Button variant="outline" onClick={() => setIsEditOpen(true)}>
              <Pencil className="h-4 w-4 mr-2" />
              Edit
            </Button>
          )}
          {canDelete && (
            <Button variant="destructive" onClick={() => setIsDeleteOpen(true)}>
              <Trash2 className="h-4 w-4 mr-2" />
              Delete
            </Button>
          )}
        </div>
      </div>

      {/* Metadata */}
      <div className="flex items-center gap-6 text-sm text-muted-foreground">
        <div className="flex items-center gap-1.5">
          <Clock className="h-4 w-4" />
          <span>Created {formatDateTime(secret.createdAt)}</span>
        </div>
        {secret.labels && Object.keys(secret.labels).length > 0 && (
          <div className="flex items-center gap-1.5">
            <Tag className="h-4 w-4" />
            <div className="flex gap-1">
              {Object.entries(secret.labels).map(([k, v]) => (
                <Badge key={k} variant="secondary" className="text-xs">
                  {k}={v}
                </Badge>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Key-Value Data */}
      <div className="border rounded-lg">
        <div className="px-4 py-3 border-b bg-muted/50">
          <h2 className="text-sm font-medium">
            Data ({entries.length} {entries.length === 1 ? "key" : "keys"})
          </h2>
        </div>
        <div className="divide-y">
          {entries.length === 0 ? (
            <div className="px-4 py-8 text-center text-sm text-muted-foreground">
              No data keys in this secret.
            </div>
          ) : (
            entries.map(([key, value]) => (
              <div key={key} className="flex items-center justify-between px-4 py-3">
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-mono font-medium">{key}</p>
                  <p className="text-sm font-mono text-muted-foreground mt-1 break-all">
                    {visibleKeys.has(key) ? value : "••••••••"}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => toggleKeyVisibility(key)}
                  aria-label={visibleKeys.has(key) ? `Hide ${key}` : `Show ${key}`}
                >
                  {visibleKeys.has(key) ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </Button>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Edit Dialog — only mounted when user has update permission */}
      {canUpdate && (
        <EditSecretDialog
          open={isEditOpen}
          onOpenChange={setIsEditOpen}
          secret={secret}
          project={project}
        />
      )}

      {/* Delete Dialog — only mounted when user has delete permission */}
      {canDelete && (
        <DeleteSecretDialog
          open={isDeleteOpen}
          onOpenChange={setIsDeleteOpen}
          secretName={secret.name}
          secretNamespace={secret.namespace}
          project={project}
        />
      )}
    </section>
  );
}

function SecretDetailSkeleton() {
  return (
    <section className="space-y-6">
      <div className="flex items-center gap-3">
        <Skeleton className="h-10 w-10 rounded" />
        <div>
          <Skeleton className="h-7 w-48" />
          <Skeleton className="h-4 w-24 mt-1" />
        </div>
      </div>
      <Skeleton className="h-4 w-64" />
      <div className="border rounded-lg">
        <div className="px-4 py-3 border-b">
          <Skeleton className="h-4 w-20" />
        </div>
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex items-center justify-between px-4 py-3 border-b last:border-0">
            <div>
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-32 mt-1" />
            </div>
            <Skeleton className="h-8 w-8" />
          </div>
        ))}
      </div>
    </section>
  );
}
