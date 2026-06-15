// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { ShieldOff, Sparkles, ExternalLink } from "@/lib/icons";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

interface EnterpriseRequiredProps {
  feature?: string;
  description?: string;
}

export function EnterpriseRequired({
  feature = "This feature",
  description,
}: EnterpriseRequiredProps) {
  return (
    <div className="flex items-center justify-center min-h-[400px] p-6">
      <Card className="max-w-md w-full">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4 rounded-full bg-amber-100 dark:bg-amber-900/30 p-4">
            <ShieldOff className="h-8 w-8 text-amber-600 dark:text-amber-400" />
          </div>
          <CardTitle className="text-xl">Enterprise Feature</CardTitle>
          <CardDescription className="text-base">
            {feature} requires an Enterprise license
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {description && (
            <p className="text-sm text-muted-foreground text-center">
              {description}
            </p>
          )}

          <div className="rounded-lg bg-muted/50 p-4 space-y-3">
            <h4 className="font-medium text-sm flex items-center gap-2">
              <Sparkles className="h-4 w-4 text-primary" />
              Enterprise includes:
            </h4>
            <ul className="text-sm text-muted-foreground space-y-2">
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">•</span>
                <span>OPA Gatekeeper policy compliance monitoring</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">•</span>
                <span>Real-time violation tracking and alerts</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">•</span>
                <span>Advanced RBAC and audit logging</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-primary mt-0.5">•</span>
                <span>Priority support and SLA guarantees</span>
              </li>
            </ul>
          </div>

          <div className="flex flex-col gap-2 pt-2">
            <Button className="w-full" asChild>
              <a
                href="https://provops.dev/enterprise"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center justify-center gap-2"
              >
                Learn More About Enterprise
                <ExternalLink className="h-4 w-4" />
              </a>
            </Button>
            <Button variant="outline" className="w-full" asChild>
              <a
                href="mailto:sales@provops.dev"
                className="flex items-center justify-center gap-2"
              >
                Contact Sales
              </a>
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export default EnterpriseRequired;
