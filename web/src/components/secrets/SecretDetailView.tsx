// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
import { ArrowLeft, Eye, EyeOff, Pencil, Trash2, Clock, Tag, Copy, Check, Download, Loader2 } from "@/lib/icons";
import { useNavigate } from "react-router-dom";
import { useCanI } from "@/hooks/useCanI";
import { useCurrentProject } from "@/hooks/useAuth";
import { formatDateTime } from "@/lib/date";
import { toast } from "sonner";
import { getSecret } from "@/api/secrets";
import { getSafeErrorMessage } from "@/lib/errors";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import type { SecretDetail } from "@/types/secret";
import { EditSecretDialog } from "./EditSecretDialog";
import { DeleteSecretDialog } from "./DeleteSecretDialog";

interface SecretDetailViewProps {
  name: string;
  namespace: string;
}

export function SecretDetailView({ name, namespace }: SecretDetailViewProps) {
  const project = useCurrentProject() ?? "";
  const navigate = useNavigate();
  const [secretData, setSecretData] = useState<SecretDetail | null>(null);
  const [isLoadingValues, setIsLoadingValues] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [visibleKeys, setVisibleKeys] = useState<Set<string>>(new Set());
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);

  const { allowed: canUpdate } = useCanI("secrets", "update", project || "-");
  const { allowed: canDelete } = useCanI("secrets", "delete", project || "-");

  const handleLoadValues = useCallback(async () => {
    setIsLoadingValues(true);
    setLoadError(null);
    try {
      const data = await getSecret(name, project, namespace);
      setSecretData(data);
    } catch (err) {
      setLoadError(getSafeErrorMessage(err));
    } finally {
      setIsLoadingValues(false);
    }
  }, [name, project, namespace]);

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

  const handleCopy = async (key: string, value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopiedKey(key);
      setTimeout(() => setCopiedKey(null), 2000);
    } catch {
      toast.error("Failed to copy — clipboard access is blocked in this environment");
    }
  };

  const entries = secretData ? Object.entries(secretData.data) : null;

  return (
    <section className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" aria-label="Back to secrets" onClick={() => navigate(`/secrets`)}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">{name}</h1>
            <p className="text-sm text-muted-foreground">{namespace}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {canUpdate && (
            <Button
              variant="outline"
              onClick={() => setIsEditOpen(true)}
              disabled={!secretData}
              title={!secretData ? "Load values first" : undefined}
            >
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

      {/* Metadata — shown after values are loaded (createdAt/labels require API data) */}
      {secretData && (
        <div className="flex items-center gap-6 text-sm text-muted-foreground">
          <div className="flex items-center gap-1.5">
            <Clock className="h-4 w-4" />
            <span>Created {formatDateTime(secretData.createdAt)}</span>
          </div>
          {secretData.labels && Object.keys(secretData.labels).length > 0 && (
            <div className="flex items-center gap-1.5">
              <Tag className="h-4 w-4" />
              <div className="flex gap-1">
                {Object.entries(secretData.labels).map(([k, v]) => (
                  <Badge key={k} variant="secondary" className="text-xs">
                    {k}={v}
                  </Badge>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Key-Value Data */}
      <div className="border rounded-lg">
        <div className="px-4 py-3 border-b bg-muted/50 flex items-center justify-between">
          <h2 className="text-sm font-medium">
            {entries
              ? `Data (${entries.length} ${entries.length === 1 ? "key" : "keys"})`
              : "Data"}
          </h2>
          {!secretData && !isLoadingValues && (
            <Button variant="outline" size="sm" onClick={handleLoadValues}>
              <Download className="h-4 w-4 mr-2" />
              {loadError ? "Retry" : "Load values"}
            </Button>
          )}
        </div>

        {/* Loading state */}
        {isLoadingValues && (
          <div className="px-4 py-8 text-center text-sm text-muted-foreground flex items-center justify-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading secret values…
          </div>
        )}

        {/* Error state */}
        {loadError && !isLoadingValues && (
          <div className="p-4">
            <Alert variant="destructive" showIcon>
              <AlertDescription>{loadError}</AlertDescription>
            </Alert>
          </div>
        )}

        {/* Not loaded yet — prompt */}
        {!secretData && !isLoadingValues && !loadError && (
          <div className="px-4 py-8 text-center text-sm text-muted-foreground">
            Click "Load values" to fetch secret data. This will be recorded in the audit log.
          </div>
        )}

        {/* Loaded data */}
        {entries && (
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
                  <div className="flex items-center gap-1">
                    {visibleKeys.has(key) && (
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleCopy(key, value)}
                        aria-label={`Copy ${key}`}
                      >
                        {copiedKey === key ? (
                          <Check className="h-4 w-4 text-status-success" />
                        ) : (
                          <Copy className="h-4 w-4" />
                        )}
                      </Button>
                    )}
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
                </div>
              ))
            )}
          </div>
        )}
      </div>

      {/* Edit Dialog — only mounted when user has update permission AND values are loaded */}
      {canUpdate && secretData && (
        <EditSecretDialog
          open={isEditOpen}
          onOpenChange={setIsEditOpen}
          secret={secretData}
        />
      )}

      {/* Delete Dialog — only mounted when user has delete permission */}
      {canDelete && (
        <DeleteSecretDialog
          open={isDeleteOpen}
          onOpenChange={setIsDeleteOpen}
          secretName={name}
          secretNamespace={namespace}
        />
      )}
    </section>
  );
}
