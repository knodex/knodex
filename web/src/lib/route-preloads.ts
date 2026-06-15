// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { lazyWithPreload } from "./lazy-preload";
import { queryClient, STALE_TIME } from "./query-client";
import { listRGDs, listInstances } from "@/api/rgd";

// Route components with preload support — used by App.tsx for routing and
// by Sidebar/DashboardLayout for hover/idle prefetching.
export const CatalogRoute = lazyWithPreload(() => import("@/routes/CatalogRoute"));
export const RGDDetailRoute = lazyWithPreload(() => import("@/routes/RGDDetailRoute"));
export const InstancesRoute = lazyWithPreload(() => import("@/routes/InstancesRoute"));
export const InstanceDetailRoute = lazyWithPreload(() => import("@/routes/InstanceDetailRoute"));
export const Settings = lazyWithPreload(() => import("@/routes/Settings").then(m => ({ default: m.Settings })));
export const RepositoriesSettings = lazyWithPreload(() => import("@/routes/RepositoriesSettings").then(m => ({ default: m.RepositoriesSettings })));
export const ProjectsSettings = lazyWithPreload(() => import("@/routes/ProjectsSettings").then(m => ({ default: m.ProjectsSettings })));
export const ProjectDetail = lazyWithPreload(() => import("@/routes/ProjectDetail").then(m => ({ default: m.ProjectDetail })));
export const SSOSettings = lazyWithPreload(() => import("@/routes/SSOSettings").then(m => ({ default: m.SSOSettings })));
export const TeamsSettings = lazyWithPreload(() => import("@/routes/TeamsSettings").then(m => ({ default: m.TeamsSettings })));
export const UsersSettings = lazyWithPreload(() => import("@/routes/UsersSettings").then(m => ({ default: m.UsersSettings })));
export const RoleTemplatesSettings = lazyWithPreload(() => import("@/routes/RoleTemplatesSettings").then(m => ({ default: m.RoleTemplatesSettings })));
export const AuditSettings = lazyWithPreload(() => import("@/routes/AuditSettings").then(m => ({ default: m.AuditSettings })));
export const LicenseSettings = lazyWithPreload(() => import("@/routes/LicenseSettings").then(m => ({ default: m.LicenseSettings })));
export const AuditPage = lazyWithPreload(() => import("@/routes/AuditPage"));
export const ComplianceDashboard = lazyWithPreload(() => import("@/routes/ComplianceRoutes").then(m => ({ default: m.ComplianceDashboard })));
export const ConstraintTemplatesPage = lazyWithPreload(() => import("@/components/compliance/ConstraintTemplatesPage"));
export const ConstraintTemplateDetailPage = lazyWithPreload(() => import("@/components/compliance/ConstraintTemplateDetailPage"));
export const ConstraintsPage = lazyWithPreload(() => import("@/components/compliance/ConstraintsPage"));
export const ConstraintDetailPage = lazyWithPreload(() => import("@/components/compliance/ConstraintDetailPage"));
export const ViolationsPage = lazyWithPreload(() => import("@/components/compliance/ViolationsPage"));
export const SecretsRoute = lazyWithPreload(() => import("@/routes/SecretsRoutes").then(m => ({ default: m.SecretsRoute })));
export const SecretDetailRoute = lazyWithPreload(() => import("@/routes/SecretsRoutes").then(m => ({ default: m.SecretDetailRoute })));
export const CategoryPage = lazyWithPreload(() => import("@/components/categories/CategoryPage").then(m => ({ default: m.CategoryPage })));
export const UserInfoPage = lazyWithPreload(() => import("@/components/account/UserInfoPage").then(m => ({ default: m.UserInfoPage })));
export const DeployRoute = lazyWithPreload(() => import("@/routes/DeployRoute"));
export const DeployRGDRoute = lazyWithPreload(() => import("@/routes/DeployRGDRoute"));
export const DeployDisabledRoute = lazyWithPreload(() => import("@/routes/DeployDisabled"));
export const AgentsRoute = lazyWithPreload(() => import("@/routes/AgentsRoute"));
export const AgentsListRoute = lazyWithPreload(() => import("@/components/agents/AgentsListPage").then(m => ({ default: m.AgentsListPage })));
export const AgentsModelsRoute = lazyWithPreload(() => import("@/components/agents/AgentsModelsPage").then(m => ({ default: m.AgentsModelsPage })));
export const AgentsTemplatesRoute = lazyWithPreload(() => import("@/components/agents/AgentsTemplatesPage").then(m => ({ default: m.AgentsTemplatesPage })));
export const AgentChatLayoutRoute = lazyWithPreload(() => import("@/components/agents/AgentChatLayout").then(m => ({ default: m.AgentChatLayout })));
export const AgentChatRoute = lazyWithPreload(() => import("@/components/agents/AgentChatPage").then(m => ({ default: m.AgentChatPage })));
export const SessionsListRoute = lazyWithPreload(() => import("@/components/agents/SessionsListPage").then(m => ({ default: m.SessionsListPage })));

/** Route preload map — used by Sidebar to trigger chunk downloads on hover.
 *  Prefetches both the JS chunk AND the initial data query to eliminate
 *  the chunk-load → mount → fetch waterfall on navigation. */
export const routePreloads: Record<string, () => Promise<unknown>> = {
  "/catalog": () => {
    CatalogRoute.preload();
    return queryClient.prefetchQuery({ queryKey: ["rgds", undefined], queryFn: () => listRGDs(), staleTime: STALE_TIME.FREQUENT });
  },
  "/instances": () => {
    InstancesRoute.preload();
    return queryClient.prefetchQuery({ queryKey: ["instances", undefined], queryFn: () => listInstances(), staleTime: STALE_TIME.FREQUENT });
  },
  "/agents": AgentsRoute.preload,
  "/secrets": SecretsRoute.preload,
  "/compliance": ComplianceDashboard.preload,
  "/audit": AuditPage.preload,
  "/settings": Settings.preload,
  "/projects": ProjectsSettings.preload,
  "/repositories": RepositoriesSettings.preload,
};
