// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Link } from "react-router-dom";
import {
  ArrowRight,
  Bot,
  Boxes,
  CheckCircle2,
  ExternalLink,
  FileCode,
  RefreshCw,
  Sparkles,
  type LucideIcon,
} from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { useAgentsStatus } from "@/hooks/useAgentsStatus";
import { useAgents, useModels, useAgentTemplates } from "@/hooks/useAgents";
import type { AgentsStatusResponse } from "@/api/agents";

// Install snippet verified against https://kagent.dev/docs (installation guide,
// 2026-06-06). The kagent chart additionally takes provider flags
// (e.g. --set providers.default=openAI) — the docs link covers the variants.
const KAGENT_INSTALL_SNIPPET = `helm install kagent-crds oci://ghcr.io/kagent-dev/kagent/helm/kagent-crds \\
  --namespace kagent --create-namespace
helm install kagent oci://ghcr.io/kagent-dev/kagent/helm/kagent \\
  --namespace kagent`;

/** Loading skeleton for the Overview presence check — mirrors the ready-state
 *  stat grid + quickstart so the swap to content causes no layout shift. */
function AgentsLoadingSkeleton() {
  return (
    <div className="space-y-8" data-testid="agents-loading">
      <div className="grid gap-3 sm:grid-cols-3">
        <Skeleton className="h-[72px] w-full rounded-xl" />
        <Skeleton className="h-[72px] w-full rounded-xl" />
        <Skeleton className="h-[72px] w-full rounded-xl" />
      </div>
      <div className="space-y-2">
        <Skeleton className="h-4 w-28" />
        <Skeleton className="h-[76px] w-full rounded-xl" />
        <Skeleton className="h-[76px] w-full rounded-xl" />
      </div>
    </div>
  );
}

/** Onboarding state: kagent definitively not installed. */
function AgentsOnboarding({ status }: { status: AgentsStatusResponse }) {
  // Surface which check failed so a half-installed kagent
  // ("CRD found, controller not responding") is debuggable from the UI.
  const checkDetails: string[] = [];
  if (status.crdPresent !== null) {
    checkDetails.push(
      status.crdPresent
        ? "Agent CRD (agents.kagent.dev): found"
        : "Agent CRD (agents.kagent.dev): not found"
    );
  }
  if (status.controllerHealthy !== null) {
    checkDetails.push(
      status.controllerHealthy
        ? "kagent controller: healthy"
        : "kagent controller: not responding"
    );
  }

  return (
    <div className="space-y-6" data-testid="agents-onboarding">
      <EmptyState
        icon={Bot}
        title="AI Agents not yet available"
        description="The kagent operator isn't detected in this cluster. Install it to activate the Agents workspace."
        action={
          <div className="space-y-4 text-left">
            <pre className="rounded-md border border-border bg-secondary/50 p-4 text-xs overflow-x-auto">
              <code>{KAGENT_INSTALL_SNIPPET}</code>
            </pre>
            {checkDetails.length > 0 && (
              <ul className="text-xs text-muted-foreground space-y-1" data-testid="agents-check-details">
                {checkDetails.map((detail) => (
                  <li key={detail}>{detail}</li>
                ))}
              </ul>
            )}
            <div className="flex justify-center">
              <Button variant="outline" size="sm" asChild>
                <a href="https://kagent.dev/docs" target="_blank" rel="noopener noreferrer">
                  <ExternalLink className="h-4 w-4" />
                  View kagent docs
                </a>
              </Button>
            </div>
          </div>
        }
      />
    </div>
  );
}

/** Degraded state: presence indeterminate — designed retry card, not a crash. */
function AgentsDegraded({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="space-y-6" data-testid="agents-degraded">
      <div className="flex flex-col items-center justify-center min-h-[40vh] gap-4 rounded-lg border border-border">
        <div className="text-center space-y-2 px-6">
          <p className="text-lg font-medium text-foreground">Agent status unavailable</p>
          <p className="text-sm text-muted-foreground max-w-md">
            {message || "The kagent presence check could not complete. This is usually transient."}
          </p>
        </div>
        <button
          onClick={onRetry}
          className="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
        >
          <RefreshCw className="h-4 w-4" />
          Retry
        </button>
      </div>
    </div>
  );
}

type StatTone = "accent" | "primary" | "muted";

// accent == violet (the kagent/agent brand color), primary == teal (models),
// muted == neutral (templates) — so each building block is colour-coded.
const STAT_TONE: Record<StatTone, string> = {
  accent: "bg-accent/10 text-accent",
  primary: "bg-primary/10 text-primary",
  muted: "bg-muted text-muted-foreground",
};

/**
 * One building-block tile: a whole-card link to its workspace tab. Degrades the
 * count to a dash on a query error rather than crashing the page — a failed
 * count is informational, not fatal (AC #1).
 */
function StatCard({
  testId,
  to,
  icon: Icon,
  label,
  count,
  isLoading,
  isError,
  tone,
}: {
  testId: string;
  to: string;
  icon: LucideIcon;
  label: string;
  count: number | undefined;
  isLoading: boolean;
  isError: boolean;
  tone: StatTone;
}) {
  return (
    <Link
      to={to}
      data-testid={testId}
      aria-label={
        isError ? `${label}: count unavailable. Open ${label}.` : `${count ?? 0} ${label}. Open ${label}.`
      }
      className={cn(
        "group flex items-center gap-4 rounded-xl border border-border bg-card p-4",
        "transition-colors hover:border-foreground/20 hover:bg-secondary/40",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      )}
    >
      <span className={cn("flex h-10 w-10 shrink-0 items-center justify-center rounded-lg", STAT_TONE[tone])}>
        <Icon className="h-5 w-5" aria-hidden="true" />
      </span>
      <div className="min-w-0 space-y-1">
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</p>
        {isLoading ? (
          <Skeleton className="h-7 w-10" />
        ) : (
          <p className="text-2xl font-semibold leading-none tabular-nums text-foreground">
            {isError ? "—" : (count ?? 0)}
          </p>
        )}
      </div>
      <ArrowRight
        className="ml-auto h-4 w-4 shrink-0 text-muted-foreground/40 transition-all group-hover:translate-x-0.5 group-hover:text-foreground"
        aria-hidden="true"
      />
    </Link>
  );
}

interface QuickstartStep {
  testId: string;
  to: string;
  icon: LucideIcon;
  title: string;
  description: string;
  done: boolean;
}

/** One quickstart step row. Reflects progress: done (check, muted), active
 *  (the first incomplete step — primary ring) or pending (neutral). */
function StepCard({ step, active }: { step: QuickstartStep; active: boolean }) {
  const { to, icon: Icon, title, description, done, testId } = step;
  return (
    <Link
      to={to}
      data-testid={testId}
      className={cn(
        "group flex items-start gap-3 rounded-xl border p-4 transition-colors",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        done
          ? "border-border/60 hover:bg-secondary/30"
          : active
            ? "border-primary/40 bg-primary/5 hover:bg-primary/10"
            : "border-border hover:bg-secondary/40"
      )}
    >
      <span
        className={cn(
          "flex h-8 w-8 shrink-0 items-center justify-center rounded-lg",
          done
            ? "text-green-600 dark:text-green-400"
            : active
              ? "bg-primary/15 text-primary"
              : "bg-muted text-muted-foreground"
        )}
      >
        {done ? <CheckCircle2 className="h-5 w-5" aria-hidden="true" /> : <Icon className="h-4 w-4" aria-hidden="true" />}
      </span>
      <div className="min-w-0 space-y-0.5">
        <p className={cn("text-sm font-medium", done ? "text-muted-foreground" : "text-foreground")}>
          {title}
          {done && (
            <span className="ml-2 text-xs font-normal text-green-600 dark:text-green-400">Done</span>
          )}
        </p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      <ArrowRight
        className="ml-auto mt-0.5 h-4 w-4 shrink-0 text-muted-foreground/40 transition-all group-hover:translate-x-0.5 group-hover:text-foreground"
        aria-hidden="true"
      />
    </Link>
  );
}

/**
 * Progress-aware quickstart. Steps are ordered by dependency (a model before an
 * agent) and self-mark as Done once that resource exists, so the panel guides a
 * first run without nagging a populated workspace. Once both are done it yields
 * a single forward CTA into the Agents tab.
 */
function Quickstart({ hasModels, hasAgents }: { hasModels: boolean; hasAgents: boolean }) {
  const steps: QuickstartStep[] = [
    {
      testId: "agents-overview-quickstart-model",
      to: "/agents/models",
      icon: Boxes,
      title: "Add a model",
      description: "Connect a provider and API key — an agent needs a model to run on, so start here.",
      done: hasModels,
    },
    {
      // Agents are created by deploying a published template, so this points at
      // Templates (the deploy source), not the read-only Agents list.
      testId: "agents-overview-quickstart-agent",
      to: "/agents/templates",
      icon: Bot,
      title: "Deploy an agent",
      description: "Pick a published template and deploy it into a namespace you can reach.",
      done: hasAgents,
    },
  ];
  const completed = steps.filter((s) => s.done).length;
  const allDone = completed === steps.length;
  const activeIdx = steps.findIndex((s) => !s.done);

  return (
    <section className="space-y-3" aria-labelledby="agents-quickstart-heading">
      <div className="flex items-center justify-between">
        <h2 id="agents-quickstart-heading" className="text-sm font-semibold text-foreground">
          {allDone ? "You're all set" : "Quick start"}
        </h2>
        <span className="text-xs tabular-nums text-muted-foreground">
          {completed} of {steps.length} complete
        </span>
      </div>

      <div className="space-y-2">
        {steps.map((step, i) => (
          <StepCard key={step.testId} step={step} active={i === activeIdx} />
        ))}
      </div>

      {allDone && (
        <Link
          to="/agents/list"
          data-testid="agents-overview-start"
          className={cn(
            "group flex items-center justify-center gap-2 rounded-xl bg-primary px-4 py-3",
            "text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          )}
        >
          <Sparkles className="h-4 w-4" aria-hidden="true" />
          Open the Agents workspace
          <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" aria-hidden="true" />
        </Link>
      )}
    </section>
  );
}

/**
 * Ready-state body: building-block stat cards + progress-aware quickstart. Split
 * out as its own component so useAgents/useModels/useAgentTemplates are
 * conditionally MOUNTED (not conditionally called) — the queries must never fire
 * on a kagent-less cluster (Rules of Hooks + the hooks' "only mounted inside the
 * ready-state workspace" contract).
 */
function AgentsOverviewReady() {
  const agents = useAgents();
  const models = useModels();
  const templates = useAgentTemplates();

  const agentCount = agents.data?.agents.length;
  const modelCount = models.data?.models.length;
  const templateCount = templates.data?.items.length;

  return (
    <div className="space-y-8" data-testid="agents-overview-ready">
      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard
          testId="agents-overview-agent-count"
          to="/agents/list"
          icon={Bot}
          label="Agents"
          count={agentCount}
          isLoading={agents.isLoading}
          isError={agents.isError}
          tone="accent"
        />
        <StatCard
          testId="agents-overview-model-count"
          to="/agents/models"
          icon={Boxes}
          label="Models"
          count={modelCount}
          isLoading={models.isLoading}
          isError={models.isError}
          tone="primary"
        />
        <StatCard
          testId="agents-overview-template-count"
          to="/agents/templates"
          icon={FileCode}
          label="Templates"
          count={templateCount}
          isLoading={templates.isLoading}
          isError={templates.isError}
          tone="muted"
        />
      </div>

      <Quickstart hasModels={(modelCount ?? 0) > 0} hasAgents={(agentCount ?? 0) > 0} />
    </div>
  );
}

/**
 * Agents Overview tab (Story 53.2) — owns the kagent presence branches lifted
 * from the old AgentsPage: loading / not_installed onboarding / degraded retry.
 * The ready state shows live agent/model/template counts + a progress-aware
 * quickstart (Story 53.6).
 */
export function AgentsOverviewPage() {
  const { data, isLoading, isError, refetch } = useAgentsStatus();

  if (isLoading) {
    return <AgentsLoadingSkeleton />;
  }

  if (isError || !data || data.status === "degraded") {
    return (
      <AgentsDegraded
        message={data?.message ?? "Unable to reach the server for agent status."}
        onRetry={() => void refetch()}
      />
    );
  }

  if (data.status === "not_installed") {
    return <AgentsOnboarding status={data} />;
  }

  // Mount the ready body only here so the count queries never fire on a
  // not_installed/degraded cluster.
  return <AgentsOverviewReady />;
}

export default AgentsOverviewPage;
