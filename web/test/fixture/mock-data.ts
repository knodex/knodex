// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * E2E Test Fixtures - Mock data for testing
 */

import type { Project, ProjectListResponse } from "../../src/types/project";
import type {
  CatalogRGD,
  Instance,
  InstanceListResponse,
  RGDListResponse,
} from "../../src/types/rgd";
import type { Category, CategoryList } from "../../src/types/category";

/**
 * Mock RGD catalog data
 */
export const mockRGDs: CatalogRGD[] = [
  {
    name: "postgres-database",
    namespace: "databases",
    description: "PostgreSQL database with automated backups and monitoring",
    version: "v1.0.0",
    tags: ["database", "sql", "production-ready"],
    category: "database",
    labels: { tier: "production", managed: "true" },
    instances: 5,
    apiVersion: "kro.run/v1alpha1",
    kind: "ResourceGraphDefinition",
    status: "Active",
    createdAt: "2025-01-15T10:30:00Z",
    updatedAt: "2025-01-20T14:45:00Z",
  },
  {
    name: "redis-cache",
    namespace: "caching",
    description: "Redis cache cluster for high-performance caching",
    version: "v2.1.0",
    tags: ["cache", "nosql", "high-availability"],
    category: "storage",
    labels: { tier: "production", managed: "true" },
    instances: 3,
    apiVersion: "kro.run/v1alpha1",
    kind: "ResourceGraphDefinition",
    status: "Active",
    createdAt: "2025-01-10T08:00:00Z",
    updatedAt: "2025-01-18T09:30:00Z",
  },
  {
    name: "nginx-ingress",
    namespace: "networking",
    description: "NGINX Ingress Controller for Kubernetes",
    version: "v1.5.0",
    tags: ["ingress", "loadbalancer", "networking"],
    category: "networking",
    labels: { tier: "infrastructure", managed: "true" },
    instances: 2,
    apiVersion: "kro.run/v1alpha1",
    kind: "ResourceGraphDefinition",
    status: "Active",
    createdAt: "2025-01-05T12:00:00Z",
    updatedAt: "2025-01-15T16:00:00Z",
  },
];

/**
 * Mock RGD list response
 */
export const mockRGDListResponse: RGDListResponse = {
  items: mockRGDs,
  totalCount: mockRGDs.length,
  page: 1,
  pageSize: 10,
};

/**
 * Mock instances data
 */
export const mockInstances: Instance[] = [
  {
    name: "prod-db-1",
    namespace: "production",
    rgdName: "postgres-database",
    rgdNamespace: "databases",
    apiVersion: "databases.kro.run/v1alpha1",
    kind: "PostgresDatabase",
    health: "Healthy",
    conditions: [
      {
        type: "Ready",
        status: "True",
        reason: "ResourcesReady",
        message: "All resources are ready",
        lastTransitionTime: "2025-01-20T10:00:00Z",
      },
    ],
    spec: { replicas: 3, storage: "100Gi" },
    status: {
      state: "ACTIVE",
      readyReplicas: 3,
      serviceIP: "10.96.0.15",
      connectionString: "postgresql://prod-db-1.production:5432/app",
      endpoints: {
        primary: "https://db-primary.example.com",
        readonly: "https://db-readonly.example.com",
      },
      readyNodes: ["node-1", "node-2", "node-3"],
      autoBackup: true,
    },
    createdAt: "2025-01-15T10:30:00Z",
    updatedAt: "2025-01-20T14:45:00Z",
  },
  {
    name: "staging-cache",
    namespace: "staging",
    rgdName: "redis-cache",
    rgdNamespace: "caching",
    apiVersion: "caching.kro.run/v1alpha1",
    kind: "RedisCache",
    health: "Progressing",
    conditions: [
      {
        type: "Ready",
        status: "False",
        reason: "Provisioning",
        message: "Resources are being provisioned",
        lastTransitionTime: "2025-01-20T14:00:00Z",
      },
    ],
    spec: { replicas: 1, memory: "2Gi" },
    status: { readyReplicas: 0 },
    createdAt: "2025-01-20T14:00:00Z",
    updatedAt: "2025-01-20T14:30:00Z",
  },
  {
    name: "dev-ingress",
    namespace: "development",
    rgdName: "nginx-ingress",
    rgdNamespace: "networking",
    apiVersion: "networking.kro.run/v1alpha1",
    kind: "NginxIngress",
    health: "Unhealthy",
    conditions: [
      {
        type: "Ready",
        status: "False",
        reason: "ConfigError",
        message: "Invalid configuration detected",
        lastTransitionTime: "2025-01-19T12:00:00Z",
      },
    ],
    spec: { class: "nginx", replicas: 1 },
    status: { readyReplicas: 0 },
    createdAt: "2025-01-18T09:00:00Z",
    updatedAt: "2025-01-19T12:00:00Z",
  },
];

/**
 * Mock instance list response
 */
export const mockInstanceListResponse: InstanceListResponse = {
  items: mockInstances,
  totalCount: mockInstances.length,
  page: 1,
  pageSize: 10,
};

/**
 * Mock microservices-platform RGD with conditional fields
 */
export const mockMicroservicesPlatformRGD: CatalogRGD = {
  name: "microservices-platform",
  namespace: "default",
  description:
    "Microservices platform with API gateway, auth service, and optional service mesh",
  version: "v1.0.0",
  tags: ["microservices", "platform", "istio", "external-ref", "ha"],
  category: "examples",
  labels: { "knodex.io/catalog": "true" },
  instances: 0,
  apiVersion: "kro.run/v1alpha1",
  kind: "ResourceGraphDefinition",
  status: "Active",
  createdAt: "2025-01-22T10:00:00Z",
  updatedAt: "2025-01-22T10:00:00Z",
};

/**
 * Mock schema response for microservices-platform with conditional sections
 */
export const mockMicroservicesPlatformSchema = {
  crdFound: true,
  schema: {
    group: "kro.run",
    version: "v1alpha1",
    kind: "MicroservicesPlatform",
    description:
      "Microservices platform with API gateway, auth service, and optional service mesh",
    properties: {
      platformName: {
        type: "string",
        description: "Name of the platform",
      },
      environment: {
        type: "string",
        description: "Environment (dev, staging, prod)",
        default: "dev",
      },
      useExistingDatabase: {
        type: "boolean",
        description:
          "Use an existing external database instead of creating a new one",
        default: false,
      },
      externalRef: {
        type: "object",
        properties: {
          externaldb: {
            type: "object",
            properties: {
              name: {
                type: "string",
                description: "Name of the external database service",
                default: "",
              },
              namespace: {
                type: "string",
                description: "Namespace of the external database service",
                default: "",
              },
            },
            externalRefSelector: {
              apiVersion: "v1",
              kind: "Service",
              useInstanceNamespace: false,
              autoFillFields: { name: "name", namespace: "namespace" },
            },
          },
        },
      },
      highAvailability: {
        type: "boolean",
        description: "Enable high availability mode with multiple replicas",
        default: false,
      },
    },
    propertyOrder: ["platformName", "environment", "useExistingDatabase", "externalRef", "highAvailability"],
    required: ["platformName"],
    conditionalSections: [
      {
        controllingField: "spec.useExistingDatabase",
        condition: "${schema.spec.useExistingDatabase == true}",
        expectedValue: true,
        clientEvaluable: true,
        rules: [
          { field: "spec.useExistingDatabase", op: "==", value: true },
        ],
        affectedProperties: ["externalRef"],
      },
    ],
  },
};

/**
 * Mock composite RGD with nested externalRef selectors (cross-RGD resolved)
 * Simulates AKSApplicationExternalSecretOperator pattern where template resources
 * like AKVESOBinding have spec.externalRef fields resolved via cross-RGD lookup.
 */
export const mockCompositeRGD: CatalogRGD = {
  name: "aks-app-eso",
  namespace: "default",
  description:
    "AKS Application with External Secret Operator binding and ArgoCD cluster ref",
  version: "v1.0.0",
  tags: ["composite", "eso", "external-ref", "nested"],
  category: "examples",
  labels: { "knodex.io/catalog": "true" },
  instances: 0,
  apiVersion: "kro.run/v1alpha1",
  kind: "ResourceGraphDefinition",
  status: "Active",
  createdAt: "2025-01-22T10:00:00Z",
  updatedAt: "2025-01-22T10:00:00Z",
};

/**
 * Mock schema for composite RGD with both resource-level and nested externalRef selectors.
 * - argocdClusterRef: resource-level externalRef (direct)
 * - keyVaultRef: nested externalRef (resolved via cross-RGD lookup from AKVESOBinding)
 */
export const mockCompositeRGDSchema = {
  crdFound: true,
  schema: {
    group: "kro.run",
    version: "v1alpha1",
    kind: "AKSAppESO",
    description:
      "AKS Application with ESO binding and ArgoCD cluster reference",
    properties: {
      appName: {
        type: "string",
        description: "Name of the application",
      },
      externalRef: {
        type: "object",
        properties: {
          argocdClusterRef: {
            type: "object",
            description: "ArgoCD cluster reference (resource-level externalRef)",
            properties: {
              name: {
                type: "string",
                description: "Name of the ArgoCD cluster",
                default: "",
              },
              namespace: {
                type: "string",
                description: "Namespace of the ArgoCD cluster",
                default: "",
              },
            },
            externalRefSelector: {
              apiVersion: "kro.run/v1alpha1",
              kind: "ArgoCDAKSCluster",
              useInstanceNamespace: false,
              autoFillFields: { name: "name", namespace: "namespace" },
            },
          },
          keyVaultRef: {
            type: "object",
            description:
              "Key Vault reference (nested externalRef, resolved via cross-RGD lookup)",
            properties: {
              name: {
                type: "string",
                description: "Name of the Azure Key Vault instance",
                default: "",
              },
              namespace: {
                type: "string",
                description: "Namespace of the Azure Key Vault instance",
                default: "",
              },
            },
            externalRefSelector: {
              apiVersion: "kro.run/v1alpha1",
              kind: "AzureKeyVault",
              useInstanceNamespace: false,
              autoFillFields: { name: "name", namespace: "namespace" },
            },
          },
        },
      },
    },
    required: ["appName"],
  },
};

/**
 * Mock K8s resources for nested externalRef selectors (ArgoCDAKSCluster instances)
 */
export const mockArgoCDClusters = {
  items: [
    {
      name: "aks-prod-cluster",
      namespace: "argocd",
      labels: { env: "prod" },
      createdAt: "2025-01-15T10:00:00Z",
    },
    {
      name: "aks-staging-cluster",
      namespace: "argocd",
      labels: { env: "staging" },
      createdAt: "2025-01-16T11:00:00Z",
    },
  ],
  count: 2,
};

/**
 * Mock K8s resources for nested externalRef selectors (AzureKeyVault instances)
 */
export const mockAzureKeyVaults = {
  items: [
    {
      name: "prod-keyvault",
      namespace: "secrets",
      labels: { env: "prod" },
      createdAt: "2025-01-15T10:00:00Z",
    },
    {
      name: "staging-keyvault",
      namespace: "secrets",
      labels: { env: "staging" },
      createdAt: "2025-01-16T11:00:00Z",
    },
  ],
  count: 2,
};

/**
 * Mock K8s resources for ExternalRef selectors
 */
export const mockK8sServices = {
  items: [
    {
      name: "postgres-service",
      namespace: "default",
      labels: { app: "postgres" },
      createdAt: "2025-01-15T10:00:00Z",
    },
    {
      name: "mysql-service",
      namespace: "default",
      labels: { app: "mysql" },
      createdAt: "2025-01-16T11:00:00Z",
    },
    {
      name: "mongodb-service",
      namespace: "default",
      labels: { app: "mongodb" },
      createdAt: "2025-01-17T12:00:00Z",
    },
  ],
  count: 3,
};

/**
 * Mock projects data (ArgoCD-compatible project structure)
 */
export const mockProjects: Project[] = [
  {
    name: "acme-corp",
    description: "Acme Corp production deployments",
    destinations: [
      { namespace: "acme-*" },
    ],
    roles: [
      {
        name: "platform-admin",
        description: "Full access to project",
        policies: [
          "p, proj:acme-corp:platform-admin, *, *, acme-corp/*, allow",
        ],
        groups: ["acme-admins"],
      },
      {
        name: "developer",
        description: "Deploy and view access",
        policies: [
          "p, proj:acme-corp:developer, applications, *, acme-corp/*, allow",
        ],
        groups: ["acme-developers"],
      },
      {
        name: "viewer",
        description: "Read-only access",
        policies: [
          "p, proj:acme-corp:viewer, applications, get, acme-corp/*, allow",
        ],
        groups: ["acme-viewers"],
      },
    ],
    resourceVersion: "1",
    createdAt: "2025-01-01T00:00:00Z",
    createdBy: "admin@acme.com",
  },
  {
    name: "tech-startup",
    description: "Tech Startup development environment",
    destinations: [
      { namespace: "tech-startup-*" },
    ],
    roles: [
      {
        name: "developer",
        description: "Deploy and view access",
        policies: [
          "p, proj:tech-startup:developer, applications, *, tech-startup/*, allow",
        ],
        groups: ["tech-startup-devs"],
      },
    ],
    resourceVersion: "1",
    createdAt: "2025-01-10T00:00:00Z",
    createdBy: "founder@tech-startup.com",
  },
  {
    name: "enterprise-solutions",
    description: "Enterprise Solutions platform",
    destinations: [
      { namespace: "enterprise-*" },
    ],
    roles: [
      {
        name: "platform-admin",
        description: "Full access to project",
        policies: [
          "p, proj:enterprise-solutions:platform-admin, *, *, enterprise-solutions/*, allow",
        ],
        groups: ["enterprise-admins"],
      },
      {
        name: "viewer",
        description: "Read-only access",
        policies: [
          "p, proj:enterprise-solutions:viewer, applications, get, enterprise-solutions/*, allow",
        ],
        groups: ["enterprise-viewers"],
      },
    ],
    resourceVersion: "1",
    createdAt: "2025-01-15T00:00:00Z",
    createdBy: "owner@enterprise.com",
  },
];

/**
 * Mock project list response
 */
export const mockProjectListResponse: ProjectListResponse = {
  items: mockProjects,
  totalCount: mockProjects.length,
};

/**
 * Mock current user project memberships
 */
export const mockUserProjects: Project[] = mockProjects;

/**
 * Mock category data (OSS feature)
 */
export const mockCategories: Category[] = [
  {
    name: "Testing Resources",
    slug: "testing",
    icon: "TestTube2",
    iconType: "lucide",
    count: 3,
  },
  {
    name: "Databases",
    slug: "databases",
    icon: "Database",
    iconType: "lucide",
    count: 5,
  },
  {
    name: "Networking",
    slug: "networking",
    icon: "Network",
    iconType: "lucide",
    count: 2,
  },
];

/**
 * Mock category list response
 */
export const mockCategoryListResponse: CategoryList = {
  categories: mockCategories,
};

/**
 * API endpoint paths
 */
export const API_PATHS = {
  rgds: "/api/v1/rgds",
  rgdCount: "/api/v1/rgds/count",
  instances: "/api/v1/instances",
  instanceCount: "/api/v1/instances/count",
  // K8s-aligned namespaced instance path: /api/v1/namespaces/{ns}/instances/{kind}/{name}
  // Tests that mock instance detail pages must also intercept this pattern
  namespacedInstances: "/api/v1/namespaces/*/instances",
  projects: "/api/v1/projects",
  categories: "/api/v1/categories",
  canI: "/api/v1/account/can-i",
  health: "/healthz",
  ready: "/readyz",
} as const;
