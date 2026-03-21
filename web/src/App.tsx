// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { lazy, Suspense } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { LoginPage, AuthCallback } from "@/components/auth";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Loader2 } from "lucide-react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "sonner";
import { isEnterprise } from "@/hooks/useCompliance";

// Lazy load route components for code splitting
const CatalogRoute = lazy(() => import("@/routes/CatalogRoutes").then(m => ({ default: m.CatalogRoute })));
const RGDDetailRoute = lazy(() => import("@/routes/CatalogRoutes").then(m => ({ default: m.RGDDetailRoute })));
const DeployRoute = lazy(() => import("@/routes/CatalogRoutes").then(m => ({ default: m.DeployRoute })));
const InstancesRoute = lazy(() => import("@/routes/InstanceRoutes").then(m => ({ default: m.InstancesRoute })));
const InstanceDetailRoute = lazy(() => import("@/routes/InstanceRoutes").then(m => ({ default: m.InstanceDetailRoute })));
const Settings = lazy(() => import("@/routes/Settings").then(m => ({ default: m.Settings })));
const RepositoriesSettings = lazy(() => import("@/routes/RepositoriesSettings").then(m => ({ default: m.RepositoriesSettings })));
const ProjectsSettings = lazy(() => import("@/routes/ProjectsSettings").then(m => ({ default: m.ProjectsSettings })));
const ProjectDetail = lazy(() => import("@/routes/ProjectDetail").then(m => ({ default: m.ProjectDetail })));
const SSOSettings = lazy(() => import("@/routes/SSOSettings").then(m => ({ default: m.SSOSettings })));
const AuditSettings = lazy(() => import("@/routes/AuditSettings").then(m => ({ default: m.AuditSettings })));
const AuditPage = lazy(() => import("@/routes/AuditPage"));
const ComplianceDashboard = lazy(() => import("@/routes/ComplianceRoutes").then(m => ({ default: m.ComplianceDashboard })));
const ConstraintTemplatesPage = lazy(() => import("@/components/compliance/ConstraintTemplatesPage"));
const ConstraintTemplateDetailPage = lazy(() => import("@/components/compliance/ConstraintTemplateDetailPage"));
const ConstraintsPage = lazy(() => import("@/components/compliance/ConstraintsPage"));
const ConstraintDetailPage = lazy(() => import("@/components/compliance/ConstraintDetailPage"));
const ViolationsPage = lazy(() => import("@/components/compliance/ViolationsPage"));
// SecretsRoutes are only declared (and their chunk generated) in enterprise builds.
// isEnterprise() is a build-time constant: in OSS builds this resolves to null,
// eliminating both the dynamic import expression and the secrets chunk from the OSS bundle.
const SecretsRoute = isEnterprise()
  ? lazy(() => import("@/routes/SecretsRoutes").then(m => ({ default: m.SecretsRoute })))
  : null;
const SecretDetailRoute = isEnterprise()
  ? lazy(() => import("@/routes/SecretsRoutes").then(m => ({ default: m.SecretDetailRoute })))
  : null;
const ViewPage = lazy(() => import("@/components/views/ViewPage").then(m => ({ default: m.ViewPage })));
const UserInfoPage = lazy(() => import("@/components/account/UserInfoPage").then(m => ({ default: m.UserInfoPage })));

// Loading fallback for lazy-loaded routes
function RouteLoader() {
  return (
    <div className="flex items-center justify-center min-h-[400px]">
      <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
    </div>
  );
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5,
      retry: 1,
    },
  },
});

function App() {
  return (
    <ErrorBoundary>
      <TooltipProvider>
        <QueryClientProvider client={queryClient}>
          <Toaster position="top-right" richColors closeButton />
          <BrowserRouter>
            <Routes>
              {/* Public routes */}
              <Route path="/login" element={<LoginPage />} />
              <Route path="/auth/callback" element={<AuthCallback />} />

              {/* Protected routes - wrapped in DashboardLayout */}
              <Route path="/" element={<DashboardLayout />}>
                {/* Default redirect */}
                <Route index element={<Navigate to="/catalog" replace />} />

                {/* Catalog routes - lazy loaded */}
                <Route path="catalog" element={<Suspense fallback={<RouteLoader />}><CatalogRoute /></Suspense>} />
                <Route path="catalog/:rgdName" element={<Suspense fallback={<RouteLoader />}><RGDDetailRoute /></Suspense>} />
                <Route path="catalog/:rgdName/deploy" element={<Suspense fallback={<RouteLoader />}><DeployRoute /></Suspense>} />

                {/* Instance routes - lazy loaded */}
                <Route path="instances" element={<Suspense fallback={<RouteLoader />}><InstancesRoute /></Suspense>} />
                <Route path="instances/:namespace/:kind/:name" element={<Suspense fallback={<RouteLoader />}><InstanceDetailRoute /></Suspense>} />

                {/* Secrets routes - lazy loaded (Enterprise only) */}
                {SecretsRoute && SecretDetailRoute && (
                  <>
                    <Route path="secrets" element={<Suspense fallback={<RouteLoader />}><SecretsRoute /></Suspense>} />
                    <Route path="secrets/:namespace/:name" element={<Suspense fallback={<RouteLoader />}><SecretDetailRoute /></Suspense>} />
                  </>
                )}

                {/* Custom View routes - lazy loaded (Enterprise only) */}
                <Route path="views/:slug" element={<Suspense fallback={<RouteLoader />}><ViewPage /></Suspense>} />

                {/* Compliance routes - lazy loaded (Enterprise only - protected in component) */}
                <Route path="compliance" element={<Suspense fallback={<RouteLoader />}><ComplianceDashboard /></Suspense>} />
                <Route path="compliance/templates" element={<Suspense fallback={<RouteLoader />}><ConstraintTemplatesPage /></Suspense>} />
                <Route path="compliance/templates/:name" element={<Suspense fallback={<RouteLoader />}><ConstraintTemplateDetailPage /></Suspense>} />
                <Route path="compliance/constraints" element={<Suspense fallback={<RouteLoader />}><ConstraintsPage /></Suspense>} />
                <Route path="compliance/constraints/:kind/:name" element={<Suspense fallback={<RouteLoader />}><ConstraintDetailPage /></Suspense>} />
                <Route path="compliance/violations" element={<Suspense fallback={<RouteLoader />}><ViolationsPage /></Suspense>} />

                {/* Audit route - lazy loaded (Enterprise only - protected in component) */}
                <Route path="audit" element={<Suspense fallback={<RouteLoader />}><AuditPage /></Suspense>} />

                {/* Account route - lazy loaded (all authenticated users) */}
                <Route path="user-info" element={<Suspense fallback={<RouteLoader />}><UserInfoPage /></Suspense>} />

                {/* Settings routes - lazy loaded (Global Admin only - protected in component) */}
                <Route path="settings" element={<Suspense fallback={<RouteLoader />}><Settings /></Suspense>} />
                <Route path="settings/repositories" element={<Suspense fallback={<RouteLoader />}><RepositoriesSettings /></Suspense>} />
                <Route path="settings/projects" element={<Suspense fallback={<RouteLoader />}><ProjectsSettings /></Suspense>} />
                <Route path="settings/projects/:name" element={<Suspense fallback={<RouteLoader />}><ProjectDetail /></Suspense>} />
                <Route path="settings/sso" element={<Suspense fallback={<RouteLoader />}><SSOSettings /></Suspense>} />
                <Route path="settings/audit" element={<Suspense fallback={<RouteLoader />}><AuditSettings /></Suspense>} />

                {/* 404 fallback */}
                <Route path="*" element={<Navigate to="/catalog" replace />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </QueryClientProvider>
      </TooltipProvider>
    </ErrorBoundary>
  );
}

export default App;
