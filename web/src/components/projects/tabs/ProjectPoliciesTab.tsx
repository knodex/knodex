/**
 * Project Policies Tab - View and manage Casbin policies
 */
import { useState } from "react";
import { FileCode, Info, AlertCircle, Copy, Check } from "lucide-react";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { Project } from "@/types/project";

interface ProjectPoliciesTabProps {
  project: Project;
  canManage: boolean;
}

// Parse a Casbin policy string into components
function parsePolicy(policy: string): { subject: string; action: string; resource: string; effect?: string } | null {
  // Format: p, subject, resource, action
  // or: p, subject, resource, action, effect
  const parts = policy.split(",").map((p) => p.trim());
  if (parts.length < 4) return null;

  return {
    subject: parts[1],
    resource: parts[2],
    action: parts[3],
    effect: parts[4],
  };
}

export function ProjectPoliciesTab({
  project,
  canManage,
}: ProjectPoliciesTabProps) {
  const [viewMode, setViewMode] = useState<"visual" | "raw">("visual");
  const [copied, setCopied] = useState(false);

  // Collect policies from all roles
  const allPolicies: { role: string; policy: string }[] = [];
  project.roles?.forEach((role) => {
    role.policies?.forEach((policy) => {
      allPolicies.push({ role: role.name, policy });
    });
  });

  const handleCopyPolicies = () => {
    const policyText = allPolicies.map((p) => p.policy).join("\n");
    navigator.clipboard.writeText(policyText);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-medium">Policy Management</h3>
          <p className="text-sm text-muted-foreground">
            View and manage Casbin policies that define access control rules.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Tabs value={viewMode} onValueChange={(v) => setViewMode(v as "visual" | "raw")}>
            <TabsList className="h-8">
              <TabsTrigger value="visual" className="text-xs px-3 py-1">
                Visual
              </TabsTrigger>
              <TabsTrigger value="raw" className="text-xs px-3 py-1">
                Raw
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      </div>

      {/* Info Banner */}
      <Card className="border-status-info/20 bg-status-info/5">
        <CardContent className="py-3">
          <div className="flex items-start gap-3">
            <Info className="h-5 w-5 text-status-info mt-0.5" />
            <div className="text-sm">
              <p className="font-medium text-foreground">
                About Casbin Policies
              </p>
              <p className="text-muted-foreground mt-1">
                Policies follow the format: <code className="bg-status-info/10 px-1 rounded">p, subject, resource, action</code>
              </p>
              <ul className="mt-2 space-y-1 text-muted-foreground">
                <li><strong>subject:</strong> Who (role, user, or group)</li>
                <li><strong>resource:</strong> What (RGD, instance, namespace)</li>
                <li><strong>action:</strong> How (get, list, create, update, delete, deploy)</li>
              </ul>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Policy Display */}
      {allPolicies.length === 0 ? (
        <Card>
          <CardContent className="py-12">
            <div className="text-center">
              <FileCode className="h-12 w-12 mx-auto mb-3 text-muted-foreground opacity-50" />
              <p className="text-lg font-medium">No policies defined</p>
              <p className="text-sm text-muted-foreground mt-2 max-w-md mx-auto">
                Policies are defined within roles. Add roles with policies to see them here.
              </p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <FileCode className="h-5 w-5" />
                  Project Policies
                </CardTitle>
                <CardDescription>
                  {allPolicies.length} polic{allPolicies.length !== 1 ? "ies" : "y"} across {project.roles?.length || 0} roles
                </CardDescription>
              </div>
              <Button variant="outline" size="sm" onClick={handleCopyPolicies}>
                {copied ? (
                  <>
                    <Check className="h-4 w-4 mr-2" />
                    Copied
                  </>
                ) : (
                  <>
                    <Copy className="h-4 w-4 mr-2" />
                    Copy All
                  </>
                )}
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            {viewMode === "visual" ? (
              <div className="space-y-4">
                {project.roles?.map((role) => (
                  <div key={role.name}>
                    <div className="flex items-center gap-2 mb-2">
                      <Badge variant="secondary">{role.name}</Badge>
                      <span className="text-xs text-muted-foreground">
                        {role.policies?.length || 0} policies
                      </span>
                    </div>
                    {role.policies && role.policies.length > 0 ? (
                      <div className="space-y-2 pl-4 border-l-2 border-muted">
                        {role.policies.map((policy, idx) => {
                          const parsed = parsePolicy(policy);
                          return (
                            <div
                              key={idx}
                              className="p-3 bg-secondary rounded-lg text-sm"
                            >
                              {parsed ? (
                                <div className="grid grid-cols-3 gap-4">
                                  <div>
                                    <span className="text-xs text-muted-foreground block">
                                      Subject
                                    </span>
                                    <code className="text-foreground">{parsed.subject}</code>
                                  </div>
                                  <div>
                                    <span className="text-xs text-muted-foreground block">
                                      Resource
                                    </span>
                                    <code className="text-foreground">{parsed.resource}</code>
                                  </div>
                                  <div>
                                    <span className="text-xs text-muted-foreground block">
                                      Action
                                    </span>
                                    <code className="text-foreground">{parsed.action}</code>
                                  </div>
                                </div>
                              ) : (
                                <code>{policy}</code>
                              )}
                            </div>
                          );
                        })}
                      </div>
                    ) : (
                      <p className="text-sm text-muted-foreground italic pl-4">
                        No policies for this role
                      </p>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <div className="bg-secondary rounded-lg p-4 font-mono text-sm">
                <pre className="whitespace-pre-wrap">
                  {allPolicies.map((p) => p.policy).join("\n") || "# No policies defined"}
                </pre>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Policy Editor Notice */}
      {canManage && (
        <Card className="border-status-warning/20 bg-status-warning/5">
          <CardContent className="py-3">
            <div className="flex items-start gap-3">
              <AlertCircle className="h-5 w-5 text-status-warning mt-0.5" />
              <div className="text-sm">
                <p className="font-medium text-foreground">
                  Policy Editor Coming Soon
                </p>
                <p className="text-muted-foreground mt-1">
                  Advanced policy editing with validation and conflict detection will be available in a future update.
                  For now, policies can be managed through the Roles tab or via the API.
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
