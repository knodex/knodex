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
import { PageHeader } from "@/components/layout/PageHeader";
import { PageSkeleton } from "@/components/ui/page-skeleton";
import { useRGD, useRGDSchema } from "@/hooks/useRGDs";
import { useProjects } from "@/hooks/useProjects";
import { useCurrentProject } from "@/hooks/useAuth";
import { buildFormSchema, getDefaultValues } from "@/lib/schema-to-zod";
import { validateInstanceName } from "@/lib/validate-instance-name";
import { buildTabsFromSchema, RESERVED_BASICS_KEYS } from "@/lib/build-tabs";
import { createInstance, preflightInstance } from "@/api/rgd";
import { buildInstanceRoute } from "@/lib/instancePath";
import {
  validateCompliance,
  type ComplianceValidateViolation,
} from "@/api/compliance";
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

/** Recursively sort object keys so JSON.stringify produces a stable hash. */
function sortKeysDeep(value: unknown): unknown {
  if (value === null || value === undefined) return value;
  if (Array.isArray(value)) return value.map(sortKeysDeep);
  if (typeof value === "object") {
    const obj = value as Record<string, unknown>;
    const sorted: Record<string, unknown> = {};
    for (const k of Object.keys(obj).sort()) {
      sorted[k] = sortKeysDeep(obj[k]);
    }
    return sorted;
  }
  return value;
}

function stripReserved(values: Record<string, unknown>): Record<string, unknown> {
  const copy = { ...values };
  for (const k of RESERVED_BASICS_KEYS) delete copy[k];
  return copy;
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

  const { data: rgd, isLoading: rgdLoading } = useRGD(rgdName);
  const { data: schemaResponse, isLoading: schemaLoading } = useRGDSchema(rgdName);
  const { data: projectsData, isLoading: projectsLoading } = useProjects();

  const schema = schemaResponse?.schema ?? null;

  if (rgdLoading || schemaLoading || projectsLoading) {
    return <PageSkeleton />;
  }

  if (!schema) {
    return (
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
    setVisitedIds((prev) => {
      if (prev.has(activeTabId)) return prev;
      const next = new Set(prev);
      next.add(activeTabId);
      return next;
    });
  }, [activeTabId]);

  // Compliance + preflight state.
  const [complianceResult, setComplianceResult] =
    useState<"pass" | "warning" | "block">("pass");
  const [complianceViolations, setComplianceViolations] = useState<
    ComplianceValidateViolation[]
  >([]);
  const [warningsAcknowledged, setWarningsAcknowledged] = useState(false);
  const [preflightValid, setPreflightValid] = useState(true);
  const [preflightMessage, setPreflightMessage] = useState<string | undefined>();
  const [isValidating, setIsValidating] = useState(false);
  const [isPreflighting, setIsPreflighting] = useState(false);
  const lastFetchedHashRef = useRef<string>("");

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [justSubmitted, setJustSubmitted] = useState(false);

  // Re-run compliance + preflight when entering Review tab or when form values change.
  // Using form.watch subscription (non-render) avoids re-rendering DeployPageContent on
  // every keystroke, which would propagate to GeneralTab and cause Radix Select's SlotClone
  // to create a new composeRefs function every render (triggering an infinite setState loop).
  useEffect(() => {
    if (activeTabId !== "review") return;

    const ac = new AbortController();
    let debounceTimer: ReturnType<typeof setTimeout> | undefined;

    const runChecks = () => {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        const allValues = form.getValues() as Record<string, unknown>;
        const runHash = JSON.stringify(sortKeysDeep(allValues));
        if (runHash === lastFetchedHashRef.current) return;
        lastFetchedHashRef.current = runHash;

        const namespace =
          (form.getValues("namespace") as string | undefined) ?? "";
        const project = (form.getValues("project") as string | undefined) ?? "";
        const spec = stripReserved(allValues);

        setIsValidating(true);
        setIsPreflighting(true);
        setWarningsAcknowledged(false);

        void validateCompliance({
          rgdName,
          project,
          namespace: isClusterScoped ? undefined : namespace || undefined,
          values: allValues,
        })
          .then((res) => {
            if (ac.signal.aborted) return;
            setComplianceResult(res.result);
            setComplianceViolations(res.violations);
          })
          .catch(() => {
            if (ac.signal.aborted) return;
            setComplianceResult("pass");
            setComplianceViolations([]);
          })
          .finally(() => {
            if (ac.signal.aborted) return;
            setIsValidating(false);
          });

        void preflightInstance(schema.group, schema.kind, {
          name: "preflight-check",
          namespace: isClusterScoped ? undefined : namespace || undefined,
          projectId: project,
          rgdName,
          spec,
        })
          .then((res) => {
            if (ac.signal.aborted) return;
            setPreflightValid(res.valid);
            setPreflightMessage(res.message);
          })
          .catch(() => {
            if (ac.signal.aborted) return;
            setPreflightValid(true);
            setPreflightMessage(undefined);
          })
          .finally(() => {
            if (ac.signal.aborted) return;
            setIsPreflighting(false);
          });
      }, 250);
    };

    // Run immediately when entering Review tab.
    runChecks();

    // Subscribe to form value changes while on Review tab (non-render subscription).
    // eslint-disable-next-line react-hooks/incompatible-library -- intentional: subscribing without re-rendering, see comment above
    const subscription = form.watch(() => {
      if (!ac.signal.aborted) runChecks();
    });

    return () => {
      clearTimeout(debounceTimer);
      ac.abort();
      subscription.unsubscribe();
    };
  }, [activeTabId, form, schema, isClusterScoped, rgdName]);

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

  const { trigger } = form;
  const handleNext = useCallback(async () => {
    if (activeIndex >= tabs.length - 1) return;

    const currentTab = tabs[activeIndex];
    let fieldsToValidate: string[] = [];

    switch (currentTab.kind) {
      case "general": {
        // General owns Knodex plumbing fields PLUS top-level RGD scalars
        // (and the folded-in top-level `externalRef` object, if present).
        const knodex = ["instanceName", "project"];
        if (!isClusterScoped) knodex.push("namespace");
        fieldsToValidate = [
          ...knodex,
          ...Object.keys(currentTab.properties ?? {}),
        ];
        break;
      }
      case "schema":
        fieldsToValidate = Object.keys(currentTab.properties ?? {}).map(
          (k) => `${currentTab.id}.${k}`
        );
        break;
    }

    if (fieldsToValidate.length > 0) {
      const isValid = await trigger(fieldsToValidate as Parameters<typeof trigger>[0]);
      if (!isValid) return;
    }

    setActiveTab(tabs[activeIndex + 1].id);
  }, [activeIndex, tabs, setActiveTab, trigger, isClusterScoped]);

  const handleCancel = useCallback(() => {
    navigate(`/catalog/${encodeURIComponent(rgdName)}`);
  }, [navigate, rgdName]);

  const breadcrumbs = [
    { label: "Catalog", href: "/catalog" },
    {
      label: rgd?.title ?? rgdName,
      href: `/catalog/${encodeURIComponent(rgdName)}`,
    },
    { label: "Deploy" },
  ];

  const activeTab = tabs[activeIndex];

  return (
    <FormProvider {...form}>
      <PageHeader
        title={schema.title ?? rgd?.title ?? rgdName}
        breadcrumbs={breadcrumbs}
        leftActions={
          <h2 className="text-lg font-semibold leading-none">
            Deploy{" "}
            <span className="text-muted-foreground font-normal">
              {rgd?.title ?? rgdName}
            </span>
          </h2>
        }
        actions={
          <div className="flex items-center gap-2">
            <DocsButton
              docsUrl={rgd?.docsUrl}
              rgdLabel={rgd?.title ?? rgdName}
            />
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={handleCancel}
              disabled={isSubmitting}
              aria-label="Close"
              data-testid="deploy-header-close"
            >
              <X className="h-5 w-5" />
            </Button>
          </div>
        }
      />
      <div className="container mx-auto px-4 sm:px-6 lg:px-8 py-4 pb-24">
        <div className="space-y-4">
          <DeployTabs
            tabs={tabs}
            activeId={activeTabId}
            onSelect={setActiveTab}
            visitedIds={visitedIds}
          />

          <div
            className="rounded-md border p-5"
            style={{
              backgroundColor: "var(--surface-primary)",
              borderColor: "rgba(255,255,255,0.06)",
            }}
          >
            {activeTab?.kind === "general" && (
              <GeneralTab
                schema={schema}
                tab={activeTab}
                allowedDeploymentModes={rgd?.allowedDeploymentModes}
              />
            )}
            {activeTab?.kind === "schema" && (
              <SchemaTab tab={activeTab} />
            )}
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
          </div>
        </div>
      </div>

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

      <DiscardDialog
        hasUnsavedChanges={form.formState.isDirty && !justSubmitted}
      />
    </FormProvider>
  );
}
