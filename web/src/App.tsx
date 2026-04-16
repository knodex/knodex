// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Suspense } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { queryClient } from "@/lib/query-client";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { RouteErrorBoundary } from "@/components/ui/route-error-boundary";
import { LoginPage, AuthCallback } from "@/components/auth";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { PageSkeleton } from "@/components/ui/page-skeleton";
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
  DeployWizardRoute,
} from "@/lib/route-preloads";
import { MobileDeployGuard } from "@/components/layout/MobileDeployGuard";

// Loading fallback for lazy-loaded routes
function RouteLoader() {
  return <PageSkeleton />;
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
            <Routes>
              {/* Public routes */}
              <Route path="/login" element={<LoginPage />} />
              <Route path="/auth/callback" element={<AuthCallback />} />

              {/* Protected routes - wrapped in DashboardLayout */}
              <Route path="/" element={<DashboardLayout />}>
                {/* Default redirect */}
                <Route index element={<Navigate to="/instances" replace />} />

                {/* Catalog routes - lazy loaded */}
                <Route
                  path="catalog"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <CatalogRoute />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="catalog/:rgdName"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <RGDDetailRoute />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="deploy/:rgdName"
                  element={
                    <RouteErrorBoundary>
                      <MobileDeployGuard>
                        <Suspense fallback={<RouteLoader />}>
                          <DeployWizardRoute />
                        </Suspense>
                      </MobileDeployGuard>
                    </RouteErrorBoundary>
                  }
                />

                {/* Instance routes - lazy loaded */}
                <Route
                  path="instances"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <InstancesRoute />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="instances/:namespace/:kind/:name"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <InstanceDetailRoute />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />

                {/* Secrets routes - lazy loaded */}
                <Route
                  path="secrets"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <SecretsRoute />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="secrets/:namespace/:name"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <SecretDetailRoute />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />

                {/* Category routes - nested under catalog (OSS) */}
                <Route
                  path="catalog/categories/:slug"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <CategoryPage />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />

                {/* Compliance routes - lazy loaded (Enterprise only - protected in component) */}
                <Route
                  path="compliance"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <ComplianceDashboard />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="compliance/templates"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <ConstraintTemplatesPage />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="compliance/templates/:name"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <ConstraintTemplateDetailPage />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="compliance/constraints"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <ConstraintsPage />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="compliance/constraints/:kind/:name"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <ConstraintDetailPage />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="compliance/violations"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <ViolationsPage />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />

                {/* Audit route - lazy loaded (Enterprise only - protected in component) */}
                <Route
                  path="audit"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <AuditPage />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />

                {/* Account route - lazy loaded (all authenticated users) */}
                <Route
                  path="user-info"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <UserInfoPage />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />

                {/* Projects & Repositories - top-level routes (authz handled in components via Casbin) */}
                <Route
                  path="repositories"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <RepositoriesSettings />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="projects"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <ProjectsSettings />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="projects/:name"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <ProjectDetail />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />

                {/* Settings routes - lazy loaded (Global Admin only - protected in component) */}
                <Route
                  path="settings"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <Settings />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="settings/sso"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <SSOSettings />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="settings/audit"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <AuditSettings />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />
                <Route
                  path="settings/license"
                  element={
                    <RouteErrorBoundary>
                      <Suspense fallback={<RouteLoader />}>
                        <LicenseSettings />
                      </Suspense>
                    </RouteErrorBoundary>
                  }
                />

                {/* 404 fallback */}
                <Route path="*" element={<Navigate to="/instances" replace />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </QueryClientProvider>
      </TooltipProvider>
    </ErrorBoundary>
  );
}

export default App;
