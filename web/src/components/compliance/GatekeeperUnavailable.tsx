// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { ServerOff, AlertTriangle, ExternalLink, CheckCircle } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

interface GatekeeperUnavailableProps {
  message?: string;
}

export function GatekeeperUnavailable({
  message = "OPA Gatekeeper is not available in your cluster",
}: GatekeeperUnavailableProps) {
  return (
    <div className="flex items-center justify-center min-h-[400px] p-6">
      <Card className="max-w-md w-full">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4 rounded-full bg-orange-100 dark:bg-orange-900/30 p-4">
            <ServerOff className="h-8 w-8 text-orange-600 dark:text-orange-400" />
          </div>
          <CardTitle className="text-xl">Gatekeeper Not Available</CardTitle>
          <CardDescription className="text-base">
            {message}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-lg bg-muted/50 p-4 space-y-3">
            <h4 className="font-medium text-sm flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 text-orange-500" />
              Troubleshooting Steps:
            </h4>
            <ul className="text-sm text-muted-foreground space-y-2">
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">1.</span>
                <span>Verify OPA Gatekeeper is installed in your cluster</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">2.</span>
                <span>Check that gatekeeper-system namespace exists</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">3.</span>
                <span>Ensure the Gatekeeper controller pods are running</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">4.</span>
                <span>Verify ConstraintTemplate CRDs are installed</span>
              </li>
            </ul>
          </div>

          <div className="rounded-lg border bg-card p-4 space-y-2">
            <h4 className="font-medium text-sm flex items-center gap-2">
              <CheckCircle className="h-4 w-4 text-green-500" />
              Quick Check Commands:
            </h4>
            <div className="bg-muted rounded p-2 text-xs font-mono overflow-x-auto">
              <div className="text-muted-foreground"># Check Gatekeeper installation</div>
              <div>kubectl get pods -n gatekeeper-system</div>
              <div className="mt-2 text-muted-foreground"># Verify CRDs</div>
              <div>kubectl get crd constrainttemplates.templates.gatekeeper.sh</div>
            </div>
          </div>

          <div className="flex flex-col gap-2 pt-2">
            <Button className="w-full" asChild>
              <a
                href="https://open-policy-agent.github.io/gatekeeper/website/docs/install/"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-center gap-2"
              >
                Gatekeeper Installation Guide
                <ExternalLink className="h-4 w-4" />
              </a>
            </Button>
            <Button variant="outline" className="w-full" onClick={() => window.location.reload()}>
              Retry Connection
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export default GatekeeperUnavailable;
