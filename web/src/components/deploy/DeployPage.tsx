// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { FormProvider, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useLocation, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { AlertCircle, X } from "@/lib/icons";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { DeployFormSkeleton } from "./DeployFormSkeleton";
import {
  Sheet,
  SheetContent,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import * as VisuallyHiddenPrimitive from "@radix-ui/react-visually-hidden";
import { useRGD, useRGDSchema } from "@/hooks/useRGDs";
import { useProjects } from "@/hooks/useProjects";
import { useCurrentProject } from "@/hooks/useAuth";
import { buildFormSchema, getDefaultValues } from "@/lib/schema-to-zod";
import { validateInstanceName } from "@/lib/validate-instance-name";
import { buildTabsFromSchema, RESERVED_BASICS_KEYS } from "./buildTabsFromSchema";
import { createInstance } from "@/api/rgd";
import { buildInstanceRoute } from "@/lib/instancePath";
import { useComplianceValidation } from "./useComplianceValidation";
import type { CreateInstanceRequest } from "@/types/rgd";
import type { DeploymentMode } from "@/types/deployment";
import type { CatalogRGD, FormSchema } from "@/types/rgd";
import { DiscardDialog } from "@/components/deploy/discard-dialog";
import { DeployTabs } from "@/components/deploy/DeployTabs";
import { DocsButton } from "@/components/shared/DocsButton";
import { DeployActionFooter } from "@/components/deploy/DeployActionFooter";
import { GeneralTab } from "@/components/deploy/tabs/GeneralTab";
import { SchemaTab } from "@/components/deploy/tabs/SchemaTab";
import { ReviewTab } from "@/components/deploy/tabs/ReviewTab";

interface DeployPageProps {
  rgdName: string;
}

interface PrefillState {
  prefill?: boolean;
  instanceId?: string;
  namespace?: string;
}

function stripReserved(values: Record<string, unknown>): Record<string, unknown> {
  const copy = { ...values };
  for (const k of RESERVED_BASICS_KEYS) delete copy[k];
  return copy;
}

// ─────────────────────────────────────────────────────────────────────────────
// Drawer chrome shared by every render path (loading skeleton, error state,
// the main wizard). Keeps the prototype's eyebrow + title header + close
// affordance consistent so the panel "shape" never changes between states.
// ─────────────────────────────────────────────────────────────────────────────

interface DeployDrawerShellProps {
  eyebrow: string;
  title: string;
  onClose: () => void;
  closeDisabled?: boolean;
  /**
   * Optional content rendered to the right of the title (docs button, etc).
   * The close X is always rendered as the rightmost item by this shell.
   */
  headerAction?: React.ReactNode;
  /**
   * Optional band rendered between the header and the body (e.g. the DeployTabs
   * strip). When omitted, no separator is rendered.
   */
  belowHeader?: React.ReactNode;
  /**
   * Optional sticky footer (e.g. DeployActionFooter). Rendered outside the
   * scroll region so action affordances stay pinned.
   */
  footer?: React.ReactNode;
  children: React.ReactNode;
}

export function DeployDrawerShell({
  eyebrow,
  title,
  onClose,
  closeDisabled = false,
  headerAction,
  belowHeader,
  footer,
  children,
}: DeployDrawerShellProps) {
  const titleId = "deploy-drawer-title";
  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent
        side="right"
        aria-labelledby={titleId}
        // [&>button:last-child]:hidden suppresses the default Radix Dialog close
        // button (rendered by SheetContent) so the header's own close affordance
        // is the only one — see DeployDrawerShell header below.
        className="w-full p-0 sm:max-w-[640px] lg:max-w-[760px] flex flex-col gap-0 overflow-hidden [&>button:last-child]:hidden"
      >
        <VisuallyHiddenPrimitive.Root>
          <SheetTitle id={titleId}>
            {eyebrow} — {title}
          </SheetTitle>
          <SheetDescription>
            Configure and deploy a new instance of {title}.
          </SheetDescription>
        </VisuallyHiddenPrimitive.Root>

        <div className="flex items-start justify-between gap-4 border-b border-[var(--border-subtle)] px-6 pt-6 pb-5">
          <div className="min-w-0">
            <div className="text-[11px] font-medium uppercase tracking-[0.08em] text-muted-foreground">
              {eyebrow}
            </div>
            <div className="mt-1 truncate text-xl font-semibold text-foreground">
              {title}
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {headerAction}
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={onClose}
              disabled={closeDisabled}
              aria-label="Close"
              data-testid="deploy-header-close"
            >
              <X className="h-5 w-5" />
            </Button>
          </div>
        </div>

        {belowHeader && (
          <div className="border-b border-[var(--border-subtle)] px-6 py-3">
            {belowHeader}
          </div>
        )}

        <div className="flex-1 overflow-y-auto px-6 py-5">{children}</div>

        {footer}
      </SheetContent>
    </Sheet>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Outer loader — fetches data, shows skeleton, then mounts DeployPageContent
// once everything is available. This ensures useForm() receives the correct
// defaultValues on first render so form.reset() is never needed.
// ─────────────────────────────────────────────────────────────────────────────

export function DeployPage({ rgdName }: DeployPageProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const currentProject = useCurrentProject();

  const { data: rgd, isLoading: rgdLoading, isError: rgdError, error: rgdFetchError, refetch: refetchRgd } = useRGD(rgdName);
  const { data: schemaResponse, isLoading: schemaLoading, isError: schemaError, error: schemaFetchError, refetch: refetchSchema } = useRGDSchema(rgdName);
  const { data: projectsData, isLoading: projectsLoading } = useProjects();

  const schema = schemaResponse?.schema ?? null;

  // Loading + error states render INSIDE the drawer so the panel slides in
  // immediately on /deploy/{rgd} instead of waiting for the data fetch. The
  // close button on both intermediate states routes through navigate(-1) so
  // users land back where they came from (catalog or RGD detail).
  if (rgdLoading || schemaLoading || projectsLoading) {
    return (
      <DeployDrawerShell
        eyebrow="Deploy resource"
        title={rgdName}
        onClose={() => navigate(-1)}
      >
        <DeployFormSkeleton />
      </DeployDrawerShell>
    );
  }

  if (rgdError || schemaError) {
    const errorMsg = (rgdFetchError ?? schemaFetchError) instanceof Error
      ? (rgdFetchError ?? schemaFetchError)!.message
      : 'Unknown error';
    return (
      <DeployDrawerShell
        eyebrow="Deploy resource"
        title={rgdName}
        onClose={() => navigate(-1)}
      >
        <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
          <AlertCircle className="h-12 w-12 text-destructive" />
          <div className="text-center">
            <h2 className="text-lg font-semibold">Failed to load deployment configuration</h2>
            <p className="text-sm text-muted-foreground">{errorMsg}</p>
          </div>
          <button
            onClick={() => { void refetchRgd?.(); void refetchSchema?.(); }}
            className="text-sm underline"
          >
            Retry
          </button>
        </div>
      </DeployDrawerShell>
    );
  }

  if (!schema) {
    return (
      <DeployDrawerShell
        eyebrow="Deploy resource"
        title={rgdName}
        onClose={() => navigate(-1)}
      >
        <div className="flex flex-col items-center justify-center min-h-[400px] gap-4">
          <AlertCircle className="h-12 w-12 text-destructive" />
          <div className="text-center">
            <h2 className="text-lg font-semibold">Schema unavailable for RGD</h2>
            <p className="text-sm text-muted-foreground">
              The schema for &ldquo;{rgdName}&rdquo; could not be loaded.
            </p>
          </div>
          <Button onClick={() => navigate("/catalog")} variant="default">
            Back to Catalog
          </Button>
        </div>
      </DeployDrawerShell>
    );
  }

  const prefillState = (location.state as PrefillState | null) ?? null;
  const prefilling = prefillState?.prefill === true;

  const defaultValues: Record<string, unknown> = {
    ...getDefaultValues(schema.properties),
    instanceName: prefilling ? (prefillState?.instanceId ?? "") : "",
    namespace: prefilling ? (prefillState?.namespace ?? "") : "",
    project: currentProject ?? projectsData?.items?.[0]?.name ?? "",
    deploymentMode: "direct",
  };

  return (
    <DeployPageContent
      rgdName={rgdName}
      schema={schema}
      rgd={rgd ?? null}
      defaultValues={defaultValues}
    />
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Inner content — mounts only after data is loaded. useForm() gets the right
// defaultValues on first render, no reset() call needed.
// ─────────────────────────────────────────────────────────────────────────────

interface DeployPageContentProps {
  rgdName: string;
  schema: FormSchema;
  rgd: CatalogRGD | null;
  defaultValues: Record<string, unknown>;
}

function DeployPageContent({
  rgdName,
  schema,
  rgd,
  defaultValues,
}: DeployPageContentProps) {
  const navigate = useNavigate();
  const location = useLocation();

  const isClusterScoped = schema.isClusterScoped === true;
  const tabs = useMemo(() => buildTabsFromSchema(schema), [schema]);

  const zodSchema = useMemo(() => {
    const base = buildFormSchema(schema.properties, schema.required ?? []);
    // Extend with Basics-tab fields so zodResolver validates them (field-level
    // `register` validate options are ignored when a resolver is present).
    return base.extend({
      instanceName: z.string().refine(
        (v) => validateInstanceName(v) === "",
        (v) => ({ message: validateInstanceName(v) })
      ),
      project: z.string().min(1, "Project is required"),
      namespace: isClusterScoped
        ? z.string().optional()
        : z.string().min(1, "Namespace is required"),
      deploymentMode: z.string().optional(),
      repositoryId: z.string().optional(),
      gitBranch: z.string().optional(),
      gitPath: z.string().optional(),
    });
  }, [schema, isClusterScoped]);

  const form = useForm({
    resolver: zodResolver(zodSchema),
    defaultValues,
    mode: "onChange",
  });

  // Active tab via URL hash (kept in sync with react-router).
  const activeTabId = useMemo(() => {
    const fromHash = location.hash.replace(/^#/, "");
    if (fromHash && tabs.some((t) => t.id === fromHash)) return fromHash;
    return tabs[0]?.id ?? "general";
  }, [location.hash, tabs]);

  const setActiveTab = useCallback(
    (id: string) => {
      navigate(location.pathname + "#" + id, { replace: false });
    },
    [navigate, location.pathname]
  );

  const [visitedIds, setVisitedIds] = useState<Set<string>>(
    () => new Set([activeTabId])
  );
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- functional updater with early-return guard; only updates when tab is new; no cascade risk
    setVisitedIds((prev) => {
      if (prev.has(activeTabId)) return prev;
      const next = new Set(prev);
      next.add(activeTabId);
      return next;
    });
  }, [activeTabId]);

  // Validate the LEAVING tab whenever the active tab changes. Errors populate
  // in formState so the tab badge turns red on the tab the user just left —
  // and the inline error markers are visible when they navigate back to fix
  // missing required fields. Non-blocking: navigation still completes.
  const prevTabIdRef = useRef(activeTabId);
  useEffect(() => {
    const previous = prevTabIdRef.current;
    prevTabIdRef.current = activeTabId;
    if (previous === activeTabId) return;

    const leavingTab = tabs.find((t) => t.id === previous);
    if (!leavingTab) return;

    let fields: string[] = [];
    switch (leavingTab.kind) {
      case "general": {
        const knodex = ["instanceName", "project"];
        if (!isClusterScoped) knodex.push("namespace");
        fields = [...knodex, ...Object.keys(leavingTab.properties ?? {})];
        break;
      }
      case "schema":
        fields = Object.keys(leavingTab.properties ?? {}).map(
          (k) => `${leavingTab.id}.${k}`
        );
        break;
    }
    if (fields.length > 0) {
      void form.trigger(fields as Parameters<typeof form.trigger>[0]);
    }
  }, [activeTabId, tabs, form, isClusterScoped]);

  // Compliance + preflight — extracted to useComplianceValidation.
  const {
    complianceResult,
    complianceViolations,
    warningsAcknowledged,
    setWarningsAcknowledged,
    preflightValid,
    preflightMessage,
    isValidating,
    isPreflighting,
  } = useComplianceValidation({
    activeTabId,
    form,
    schema,
    isClusterScoped,
    rgdName,
    reservedKeys: RESERVED_BASICS_KEYS,
  });

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [justSubmitted, setJustSubmitted] = useState(false);

  const canDeploy =
    form.formState.isValid &&
    complianceResult !== "block" &&
    (complianceResult !== "warning" || warningsAcknowledged) &&
    preflightValid &&
    !isSubmitting;

  const handleDeploy = useCallback(async () => {
    if (isSubmitting) return;
    const mode = form.getValues("deploymentMode") as DeploymentMode | undefined;
    const isGitOpsMode = mode === "gitops" || mode === "hybrid";
    const namespace =
      (form.getValues("namespace") as string | undefined) ?? "";
    const request: CreateInstanceRequest = {
      name: form.getValues("instanceName") as string,
      namespace: isClusterScoped ? undefined : namespace || undefined,
      projectId: form.getValues("project") as string,
      rgdName,
      spec: stripReserved(form.getValues() as Record<string, unknown>),
      deploymentMode: mode,
      repositoryId: isGitOpsMode
        ? (form.getValues("repositoryId") as string | undefined) || undefined
        : undefined,
      gitBranch: isGitOpsMode
        ? (form.getValues("gitBranch") as string | undefined) || undefined
        : undefined,
      gitPath: isGitOpsMode
        ? (form.getValues("gitPath") as string | undefined) || undefined
        : undefined,
    };

    setIsSubmitting(true);
    try {
      const result = await createInstance(schema.group, schema.kind, request);
      setJustSubmitted(true);
      toast.success(`"${result.name}" deployed successfully`);
      navigate(
        buildInstanceRoute({
          apiVersion: `${result.apiGroup}/${result.version}`,
          namespace: result.namespace || undefined,
          kind: schema.kind,
          name: result.name,
        })
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : "Deployment failed";
      toast.error(message);
    } finally {
      setIsSubmitting(false);
    }
  }, [isSubmitting, form, isClusterScoped, rgdName, schema, navigate]);

  const activeIndex = Math.max(
    0,
    tabs.findIndex((t) => t.id === activeTabId)
  );
  const isOnFirst = activeIndex === 0;
  const isOnReview = tabs[activeIndex]?.kind === "review";

  const handlePrev = useCallback(() => {
    if (activeIndex > 0) setActiveTab(tabs[activeIndex - 1].id);
  }, [activeIndex, tabs, setActiveTab]);

  const handleGoToReview = useCallback(() => {
    const reviewTab = tabs.find((t) => t.kind === "review");
    if (reviewTab) setActiveTab(reviewTab.id);
  }, [tabs, setActiveTab]);

  const handleNext = useCallback(() => {
    if (activeIndex >= tabs.length - 1) return;
    // Validation of the leaving tab is handled by the useEffect on
    // `activeTabId` — Next is non-blocking so users can move forward and
    // come back to fix red-flagged tabs after seeing the Review summary.
    setActiveTab(tabs[activeIndex + 1].id);
  }, [activeIndex, tabs, setActiveTab]);

  const handleCancel = useCallback(() => {
    navigate(`/catalog/${encodeURIComponent(rgdName)}`);
  }, [navigate, rgdName]);

  const activeTab = tabs[activeIndex];

  return (
    <FormProvider {...form}>
      <DeployDrawerShell
        eyebrow="Deploy resource"
        title={rgd?.title ?? rgdName}
        onClose={handleCancel}
        closeDisabled={isSubmitting}
        headerAction={
          <DocsButton
            docsUrl={rgd?.docsUrl}
            rgdLabel={rgd?.title ?? rgdName}
          />
        }
        belowHeader={
          <DeployTabs
            tabs={tabs}
            activeId={activeTabId}
            onSelect={setActiveTab}
            visitedIds={visitedIds}
          />
        }
        footer={
          <DeployActionFooter
            onPrev={handlePrev}
            onNext={handleNext}
            onDeploy={handleDeploy}
            onGoToReview={handleGoToReview}
            isOnFirst={isOnFirst}
            isOnReview={isOnReview}
            canDeploy={canDeploy}
            isSubmitting={isSubmitting}
          />
        }
      >
        {activeTab?.kind === "general" && (
          <GeneralTab
            schema={schema}
            tab={activeTab}
            allowedDeploymentModes={rgd?.allowedDeploymentModes}
          />
        )}
        {activeTab?.kind === "schema" && <SchemaTab tab={activeTab} />}
        {activeTab?.kind === "review" && (
          <ReviewTab
            tabs={tabs}
            onEditTab={setActiveTab}
            complianceResult={complianceResult}
            complianceViolations={complianceViolations}
            warningsAcknowledged={warningsAcknowledged}
            setWarningsAcknowledged={setWarningsAcknowledged}
            preflightValid={preflightValid}
            preflightMessage={preflightMessage}
            isValidating={isValidating}
            isPreflighting={isPreflighting}
            isClusterScoped={isClusterScoped}
          />
        )}
      </DeployDrawerShell>

      <DiscardDialog
        hasUnsavedChanges={form.formState.isDirty && !justSubmitted}
      />
    </FormProvider>
  );
}
