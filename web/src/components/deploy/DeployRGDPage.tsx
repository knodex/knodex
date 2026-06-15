// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useState } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { AlertTriangle, CheckCircle } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useCanI } from "@/hooks/useCanI";
import { validateInstanceName } from "@/lib/validate-instance-name";
import { createRGD } from "@/api/rgd";
import { ApiError } from "@/api/client";
import { parseSpec, isRGDSpec } from "@/components/agents/spec-extract";
import { DeployDrawerShell } from "@/components/deploy/DeployPage";

/** Router state carried from the RGD Builder's "Use this spec" action. */
interface DeployRGDState {
  specYaml?: string;
  requirement?: string;
  runId?: string;
}

export function DeployRGDPage() {
  const location = useLocation();
  const state = (location.state as DeployRGDState | null) ?? null;

  // Direct/refresh navigation without state: send the user back to the agents list.
  if (!state?.specYaml) {
    return <Navigate to="/agents/list" replace />;
  }

  return <DeployRGDContent specYaml={state.specYaml} runId={state.runId} />;
}

function DeployRGDContent({ specYaml, runId }: { specYaml: string; runId?: string }) {
  const navigate = useNavigate();

  const parsed = parseSpec(specYaml);
  const metadata =
    parsed && typeof parsed.metadata === "object" && !Array.isArray(parsed.metadata)
      ? (parsed.metadata as Record<string, unknown>)
      : null;
  const initialName = typeof metadata?.name === "string" ? metadata.name : "";

  const [name, setName] = useState(initialName);
  const [yamlText, setYamlText] = useState(specYaml);
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [created, setCreated] = useState<{ name: string } | null>(null);

  const { allowed: canCreate } = useCanI("rgds", "create");

  const handleSubmit = useCallback(async () => {
    if (submitting) return;
    setFormError(null);

    const nameError = validateInstanceName(name);
    if (nameError) {
      setFormError(`Name: ${nameError}`);
      return;
    }
    // Client-side fast feedback — the server's kind lock is the real boundary.
    const spec = parseSpec(yamlText);
    if (!spec || !isRGDSpec(spec)) {
      setFormError(
        "The YAML must be a valid kro.run/v1alpha1 ResourceGraphDefinition manifest."
      );
      return;
    }

    setSubmitting(true);
    try {
      const result = await createRGD({ name, yaml: yamlText, runId });
      setCreated({ name: result.name });
    } catch (error) {
      if (error instanceof ApiError && error.status === 403) {
        setFormError(
          "You are not authorized to create ResourceGraphDefinitions. Creating platform abstractions requires the rgds create permission (server administrators by default)."
        );
      } else if (error instanceof ApiError && error.status === 409) {
        setFormError(
          `A ResourceGraphDefinition named "${name}" already exists — rename it and try again.`
        );
      } else {
        setFormError(error instanceof Error ? error.message : "Failed to create the ResourceGraphDefinition");
      }
    } finally {
      setSubmitting(false);
    }
  }, [submitting, name, yamlText, runId]);

  const title = initialName || "Generated spec";

  const footer = !created ? (
    <div
      className="flex shrink-0 items-center justify-end gap-3 border-t bg-background px-6 py-4"
      style={{ borderColor: "rgba(255,255,255,0.08)" }}
    >
      <Button
        type="button"
        variant="outline"
        onClick={() => navigate(-1)}
        disabled={submitting}
        data-testid="deploy-rgd-cancel"
      >
        Cancel
      </Button>
      <Button
        type="button"
        onClick={handleSubmit}
        disabled={submitting}
        data-testid="deploy-rgd-submit"
      >
        {submitting ? "Deploying…" : "Deploy RGD"}
      </Button>
    </div>
  ) : undefined;

  return (
    <DeployDrawerShell
      eyebrow="Deploy RGD"
      title={created ? created.name : title}
      onClose={() => navigate(-1)}
      closeDisabled={submitting}
      footer={footer}
    >
      {created ? (
        <SuccessPanel name={created.name} />
      ) : (
        <div className="space-y-5" data-testid="deploy-rgd-form">
          <p className="text-sm text-muted-foreground">
            Review the generated ResourceGraphDefinition, adjust anything you need —
            the spec is a starting point, not locked — then name it and deploy.
          </p>

          {canCreate === false && (
            <div
              className="rounded-lg border border-border/60 bg-muted/30 px-4 py-3 text-sm text-muted-foreground"
              data-testid="deploy-rgd-cani-notice"
            >
              Your account may not have permission to create ResourceGraphDefinitions —
              the server will have the final say when you submit.
            </div>
          )}

          <div className="space-y-1.5">
            <label htmlFor="deploy-rgd-name" className="text-sm font-medium text-foreground">
              Name
            </label>
            <Input
              id="deploy-rgd-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="my-rgd"
              disabled={submitting}
              data-testid="deploy-rgd-name"
            />
          </div>

          <div className="space-y-1.5">
            <label htmlFor="deploy-rgd-yaml" className="text-sm font-medium text-foreground">
              ResourceGraphDefinition YAML
            </label>
            <Textarea
              id="deploy-rgd-yaml"
              value={yamlText}
              onChange={(e) => setYamlText(e.target.value)}
              rows={20}
              className="font-mono text-xs"
              disabled={submitting}
              data-testid="deploy-rgd-yaml"
              aria-label="ResourceGraphDefinition YAML"
            />
          </div>

          {formError && (
            <div
              role="alert"
              data-testid="deploy-rgd-error"
              className="flex gap-3 rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm"
            >
              <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5 text-destructive" aria-hidden="true" />
              <p>{formError}</p>
            </div>
          )}
        </div>
      )}
    </DeployDrawerShell>
  );
}

function SuccessPanel({ name }: { name: string }) {
  return (
    <div className="space-y-4" data-testid="deploy-rgd-success">
      <div className="flex items-center gap-3">
        <CheckCircle className="h-6 w-6 text-green-600" aria-hidden="true" />
        <h2 className="text-lg font-semibold text-foreground">
          ResourceGraphDefinition {name} created
        </h2>
      </div>
      <p className="text-sm text-muted-foreground">
        The RGD now appears in the Catalog. Instance deployment becomes available
        once KRO reconciles the definition and registers its schema — this usually
        takes a few moments.
      </p>
      <div className="flex gap-2">
        <Button asChild data-testid="deploy-rgd-view-catalog">
          <Link to={`/catalog/${encodeURIComponent(name)}`}>View in Catalog</Link>
        </Button>
        <Button asChild variant="outline" data-testid="deploy-rgd-back-builder">
          <Link to="/agents/list">Back to Agents</Link>
        </Button>
      </div>
    </div>
  );
}
