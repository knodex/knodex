// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route, Navigate, useParams, useLocation, type Location } from "react-router-dom";
import { queryClient } from "@/lib/query-client";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { LoginPage, AuthCallback } from "@/components/auth";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "sonner";
import {
  CatalogRoute,
  RGDDetailRoute,
  InstancesRoute,
  InstanceDetailRoute,
  Settings,
  RepositoriesSettings,
  ProjectsSettings,
  ProjectDetail,
  SSOSettings,
  TeamsSettings,
  UsersSettings,
  RoleTemplatesSettings,
  AuditSettings,
  LicenseSettings,
  AuditPage,
  ComplianceDashboard,
  ConstraintTemplatesPage,
  ConstraintTemplateDetailPage,
  ConstraintsPage,
  ConstraintDetailPage,
  ViolationsPage,
  SecretsRoute,
  SecretDetailRoute,
  CategoryPage,
  UserInfoPage,
  DeployRoute,
  DeployRGDRoute,
  AgentsRoute,
  AgentsListRoute,
  AgentsModelsRoute,
  AgentsTemplatesRoute,
  AgentChatLayoutRoute,
  AgentChatRoute,
  SessionsListRoute,
} from "@/lib/route-preloads";
import { MobileDeployGuard } from "@/components/layout/MobileDeployGuard";
import { IndexRedirect } from "@/components/layout/IndexRedirect";
import { LazyRouteElement } from "@/components/layout/LazyRouteElement";
import { SettingsLayout } from "@/components/settings/SettingsLayout";
import { AgentsLayout } from "@/components/agents/AgentsLayout";

/** Index route for /agents/list/:namespace/:name — immediately navigates to a new session. */
function AgentChatNewConversation() {
  const { namespace, name } = useParams<{ namespace: string; name: string }>();
  const [newId] = useState(() => crypto.randomUUID());
  return (
    <Navigate
      to={`/agents/list/${encodeURIComponent(namespace!)}/${encodeURIComponent(name!)}/chat/${newId}`}
      replace
    />
  );
}

/**
 * Inner component (must be inside BrowserRouter to use useLocation).
 * Implements the modal-routes pattern: when `location.state.backgroundLocation`
 * is set (e.g. navigating to /deploy-rgd from the RGD builder), the main
 * Routes tree renders the background page while an overlay Routes renders the
 * drawer on top. The Sheet uses a Radix portal so it renders over the
 * background regardless of component tree position.
 */
function AppRoutes() {
  const location = useLocation();
  const state = location.state as { backgroundLocation?: Location } | null;

  const mainRoutes = (
    <Routes location={state?.backgroundLocation ?? location}>
      {/* Public routes */}
      <Route path="/login" element={<LoginPage />} />
      <Route path="/auth/callback" element={<AuthCallback />} />

      {/* Protected routes - wrapped in DashboardLayout */}
      <Route path="/" element={<DashboardLayout />}>
        {/* Default landing (story 17.1): a bare member with no project
            bindings lands on the self-scoped "My Access" view instead of
            /instances (which 403-walls them); everyone else → /instances. */}
        <Route index element={<IndexRedirect />} />

        {/* Catalog routes - lazy loaded */}
        <Route path="catalog" element={<LazyRouteElement component={CatalogRoute} />} />
        <Route
          path="catalog/:rgdName"
          element={<LazyRouteElement component={RGDDetailRoute} />}
        />
        <Route
          path="deploy/:rgdName"
          element={<LazyRouteElement component={DeployRoute} wrapper={MobileDeployGuard} />}
        />
        {/* Deploy a generated RGD (Story 50.2) — spec arrives via router state.
            Rendered here for direct navigation; when navigated with
            backgroundLocation in state, the overlay Routes below takes over. */}
        <Route
          path="deploy-rgd"
          element={<LazyRouteElement component={DeployRGDRoute} wrapper={MobileDeployGuard} />}
        />

        {/* Agents workspace — master-detail shell (AgentsLayout) wraps each
            tab, mirroring the Settings routes (wrapper, not nested Outlet). */}
        <Route path="agents" element={<LazyRouteElement component={AgentsRoute} wrapper={AgentsLayout} />} />
        <Route path="agents/list" element={<LazyRouteElement component={AgentsListRoute} wrapper={AgentsLayout} />} />
        <Route path="agents/templates" element={<LazyRouteElement component={AgentsTemplatesRoute} wrapper={AgentsLayout} />} />
        <Route path="agents/models" element={<LazyRouteElement component={AgentsModelsRoute} wrapper={AgentsLayout} />} />
        {/* Kagent-style agent chat with sessions sidebar (namespaced identity) */}
        <Route
          path="agents/list/:namespace/:name"
          element={<LazyRouteElement component={AgentChatLayoutRoute} />}
        >
          <Route index element={<AgentChatNewConversation />} />
          <Route
            path="chat/:sessionId"
            element={<LazyRouteElement component={AgentChatRoute} />}
          />
        </Route>
        {/* Chat session list */}
        <Route
          path="agents/sessions"
          element={<LazyRouteElement component={SessionsListRoute} />}
        />

        {/* Instance routes - lazy loaded */}
        <Route path="instances" element={<LazyRouteElement component={InstancesRoute} />} />
        {/*
         * GVK-aware instance detail routes, mirroring kube-apiserver URL ordering:
         * group and version first, then optional namespace. Cluster-scoped resources
         * omit the namespace segment (segment count is the discriminator), matching
         * how the real K8s API drops the namespaces/{ns} path component rather than
         * using a sentinel like "cluster".
         *
         *   Namespaced:     /instances/:group/:version/:namespace/:kind/:name
         *   Cluster-scoped: /instances/:group/:version/:kind/:name
         */}
        <Route
          path="instances/:group/:version/:namespace/:kind/:name"
          element={<LazyRouteElement component={InstanceDetailRoute} />}
        />
        <Route
          path="instances/:group/:version/:kind/:name"
          element={<LazyRouteElement component={InstanceDetailRoute} />}
        />

        {/* Secrets routes - lazy loaded */}
        <Route path="secrets" element={<LazyRouteElement component={SecretsRoute} />} />
        <Route
          path="secrets/:namespace/:name"
          element={<LazyRouteElement component={SecretDetailRoute} />}
        />

        {/* Category routes - nested under catalog (OSS) */}
        <Route
          path="catalog/categories/:slug"
          element={<LazyRouteElement component={CategoryPage} />}
        />

        {/* Compliance routes - lazy loaded (Enterprise only - protected in component) */}
        <Route
          path="compliance"
          element={<LazyRouteElement component={ComplianceDashboard} />}
        />
        <Route
          path="compliance/templates"
          element={<LazyRouteElement component={ConstraintTemplatesPage} />}
        />
        <Route
          path="compliance/templates/:name"
          element={<LazyRouteElement component={ConstraintTemplateDetailPage} />}
        />
        <Route
          path="compliance/constraints"
          element={<LazyRouteElement component={ConstraintsPage} />}
        />
        <Route
          path="compliance/constraints/:kind/:name"
          element={<LazyRouteElement component={ConstraintDetailPage} />}
        />
        <Route
          path="compliance/violations"
          element={<LazyRouteElement component={ViolationsPage} />}
        />

        {/* Audit route - lazy loaded (Enterprise only - protected in component) */}
        <Route path="audit" element={<LazyRouteElement component={AuditPage} />} />

        {/* Account route - lazy loaded (all authenticated users) */}
        <Route path="user-info" element={<LazyRouteElement component={UserInfoPage} />} />

        {/* Projects & Repositories - top-level routes (authz handled in components via Casbin) */}
        <Route
          path="repositories"
          element={<LazyRouteElement component={RepositoriesSettings} />}
        />
        <Route
          path="projects"
          element={<LazyRouteElement component={ProjectsSettings} />}
        />
        <Route
          path="projects/:name"
          element={<LazyRouteElement component={ProjectDetail} />}
        />

        {/* Settings routes — master-detail shell (SettingsLayout) wraps
            each sub-page; the persistent menu replaces the old card grid.
            Global Admin only — protected in component. */}
        <Route
          path="settings"
          element={<LazyRouteElement component={Settings} wrapper={SettingsLayout} />}
        />
        <Route
          path="settings/sso"
          element={<LazyRouteElement component={SSOSettings} wrapper={SettingsLayout} />}
        />
        {/* settings/teams + settings/users: the federated TeamsSettings and
            the local identity.users roster. */}
        <Route
          path="settings/teams"
          element={<LazyRouteElement component={TeamsSettings} wrapper={SettingsLayout} />}
        />
        <Route
          path="settings/users"
          element={<LazyRouteElement component={UsersSettings} wrapper={SettingsLayout} />}
        />
        <Route
          path="settings/role-templates"
          element={
            <LazyRouteElement component={RoleTemplatesSettings} wrapper={SettingsLayout} />
          }
        />
        <Route
          path="settings/audit"
          element={<LazyRouteElement component={AuditSettings} wrapper={SettingsLayout} />}
        />
        <Route
          path="settings/license"
          element={<LazyRouteElement component={LicenseSettings} wrapper={SettingsLayout} />}
        />

        {/* 404 fallback. */}
        <Route path="*" element={<Navigate to="/instances" replace />} />
      </Route>
    </Routes>
  );

  return (
    <>
      {mainRoutes}
      {/* Overlay: when /deploy-rgd is navigated with a backgroundLocation in
          state, render the drawer on top of the background page. The Sheet uses
          a Radix portal so it layers correctly regardless of DOM position. */}
      {state?.backgroundLocation && (
        <Routes>
          <Route
            path="deploy-rgd"
            element={<LazyRouteElement component={DeployRGDRoute} wrapper={MobileDeployGuard} />}
          />
        </Routes>
      )}
    </>
  );
}

function App() {
  return (
    <ErrorBoundary>
      <TooltipProvider>
        <QueryClientProvider client={queryClient}>
          <Toaster
            position="bottom-right"
            visibleToasts={3}
            gap={8}
            closeButton
          />
          <BrowserRouter>
            <AppRoutes />
          </BrowserRouter>
        </QueryClientProvider>
      </TooltipProvider>
    </ErrorBoundary>
  );
}

export default App;
