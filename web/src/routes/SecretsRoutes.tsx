// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useParams } from "react-router-dom";
import { SecretsPage } from "@/components/secrets";
import { SecretDetailView } from "@/components/secrets/SecretDetailView";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";

export function SecretsRoute() {
  return <SecretsPage />;
}

export function SecretDetailRoute() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();

  if (!namespace || !name) {
    return (
      <Alert variant="destructive" showIcon className="max-w-md mx-auto mt-12">
        <AlertTitle>Invalid route</AlertTitle>
        <AlertDescription>Missing namespace or name in URL.</AlertDescription>
      </Alert>
    );
  }

  return <SecretDetailView name={name} namespace={namespace} />;
}
