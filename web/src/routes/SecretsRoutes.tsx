// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useParams } from "react-router-dom";
import { useSearchParams } from "react-router-dom";
import { SecretsPage } from "@/components/secrets";
import { SecretDetailView } from "@/components/secrets/SecretDetailView";
import { useSecret } from "@/hooks/useSecrets";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { getSafeErrorMessage } from "@/lib/errors";
import { Skeleton } from "@/components/ui/skeleton";

export function SecretsRoute() {
  return <SecretsPage />;
}

export function SecretDetailRoute() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const [searchParams] = useSearchParams();
  const project = searchParams.get("project") ?? "";

  if (!namespace || !name) {
    return (
      <Alert variant="destructive" showIcon className="max-w-md mx-auto mt-12">
        <AlertTitle>Invalid route</AlertTitle>
        <AlertDescription>Missing namespace or name in URL.</AlertDescription>
      </Alert>
    );
  }

  if (!project) {
    return (
      <Alert variant="destructive" showIcon className="max-w-md mx-auto mt-12">
        <AlertTitle>Missing project</AlertTitle>
        <AlertDescription>Project parameter is required.</AlertDescription>
      </Alert>
    );
  }

  return <SecretDetailContent name={name} namespace={namespace} project={project} />;
}

function SecretDetailContent({
  name,
  namespace,
  project,
}: {
  name: string;
  namespace: string;
  project: string;
}) {
  const { data, isLoading, isError, error } = useSecret(name, project, namespace);

  if (isLoading) {
    return (
      <section className="space-y-6">
        <div className="flex items-center gap-3">
          <Skeleton className="h-10 w-10 rounded" />
          <div>
            <Skeleton className="h-7 w-48" />
            <Skeleton className="h-4 w-24 mt-1" />
          </div>
        </div>
      </section>
    );
  }

  if (isError || !data) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Alert variant="destructive" showIcon className="max-w-md">
          <AlertTitle>Failed to load secret</AlertTitle>
          <AlertDescription>{getSafeErrorMessage(error)}</AlertDescription>
        </Alert>
      </div>
    );
  }

  return <SecretDetailView secret={data} project={project} />;
}
