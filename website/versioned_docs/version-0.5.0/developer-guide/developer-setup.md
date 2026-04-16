---
title: Developer Setup
description: Development environment setup, architecture overview, coding patterns, and contribution guidelines for Knodex
sidebar_position: 1
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Developer Setup

This guide covers everything you need to start contributing to Knodex, from environment setup through coding patterns and debugging techniques.

## Architecture Overview

Knodex is a Kubernetes Resource Orchestrator (KRO) visualization and management tool with a Go server and React web UI.

```
┌─────────────────┐     ┌─────────────────────────┐     ┌──────────────┐
│   Web (React)   │────▶│    Server (Go)           │────▶│  Redis       │
│   Vite :3000    │     │    stdlib router :8080    │     │  :6379       │
│                 │◀────│                           │     └──────────────┘
└─────────────────┘     │  ┌─────────┐ ┌─────────┐ │
                        │  │ JWT/OIDC│ │ Casbin  │ │
                        │  │  Auth   │ │  RBAC   │ │
                        │  └─────────┘ └─────────┘ │
                        └────────────┬──────────────┘
                                     │
                                     ▼
                        ┌─────────────────────────┐
                        │   Kubernetes API Server  │
                        │   (KRO CRDs, Projects)   │
                        └─────────────────────────┘
```

### Server Technology

| Component | Technology |
|-----------|-----------|
| HTTP Router | Go stdlib `net/http` with Go 1.22+ ServeMux patterns |
| Authentication | JWT tokens with OIDC provider integration |
| Authorization | Casbin policy engine with namespace-scoped RBAC |
| Kubernetes Client | `client-go` with dynamic informers and watchers |
| Real-time | WebSocket hub for push updates |
| Language | Go 1.25 |

### Web Technology

| Component | Technology |
|-----------|-----------|
| Framework | React 19 with TypeScript |
| Build Tool | Vite |
| Styling | TailwindCSS v4 |
| State Management | Zustand (client state) + React Query (server state) |
| UI Components | Radix UI primitives (shadcn/ui style) |
| Graph Visualization | XY Flow |
| Forms | React Hook Form + Zod validation |

## Prerequisites

Install the following tools before getting started:

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.25+ | [go.dev/dl](https://go.dev/dl/) |
| Node.js | 20+ | [nodejs.org](https://nodejs.org/) |
| Docker | 24+ | [docker.com](https://www.docker.com/) |
| kubectl | 1.28+ | `brew install kubectl` |
| Kind | 0.20+ | `brew install kind` |
| Tilt | latest | `brew install tilt` |
| golangci-lint | latest | `brew install golangci-lint` |

## Clone and Start

```bash
# Clone the repository
git clone git@github.com:knodex/knodex-ee.git
cd knodex-ee

# Create a Kind cluster with KRO and CRDs installed
make cluster-up

# Option A: Start with Tilt (recommended for iterative development)
make tilt-up

# Option B: Start natively (requires Redis running at localhost:6379)
make dev            # Server + web
make dev-server     # Server only
make dev-web        # Web only
```

## Verify

```bash
# Health check
curl http://localhost:8080/healthz

# Open the web UI
open http://localhost:3000
```

## Project Structure

```
knodex-ee/
├── server/
│   ├── main.go                 # Entry point
│   ├── app/                    # Application container, lifecycle
│   ├── internal/
│   │   ├── api/                # HTTP handlers, router, middleware
│   │   ├── auth/               # OIDC authentication
│   │   ├── rbac/               # Casbin RBAC engine
│   │   ├── kro/                # KRO-specific logic (watchers, parsers)
│   │   ├── deployment/         # Instance deployment controller
│   │   ├── services/           # Service layer interfaces
│   │   ├── models/             # Shared domain models
│   │   ├── config/             # Environment-based configuration
│   │   ├── clients/            # Redis and Kubernetes clients
│   │   ├── k8s/                # Kubernetes helpers (parser library)
│   │   ├── websocket/          # WebSocket hub
│   │   └── util/               # Utility packages
│   └── test/                   # Test fixtures and E2E tests
├── web/
│   ├── src/
│   │   ├── components/         # Feature and UI components
│   │   ├── routes/             # Route components
│   │   ├── api/                # API client layer
│   │   ├── hooks/              # Custom React hooks
│   │   ├── stores/             # Zustand stores
│   │   └── lib/                # Utility functions
│   └── test/
│       └── e2e/                # Playwright E2E tests
├── deploy/
│   ├── charts/knodex/          # Helm chart
│   ├── crds/                   # Custom Resource Definitions
│   └── examples/               # Example resources
├── website/                    # Documentation (Docusaurus)
└── scripts/                    # Build and deployment scripts
```

## Backend Packages

### Core Packages (`server/internal/`)

| Package | Purpose |
|---------|---------|
| `api/handlers` | HTTP handlers for all API endpoints |
| `api/middleware` | Auth, authz, logging, rate limiting, CORS, security headers |
| `api/router` | HTTP routing with Go 1.22+ ServeMux patterns |
| `api/response` | Standardized JSON error responses |
| `auth` | OIDC authentication and token management |
| `bootstrap` | Seed data and initial configuration |
| `clients` | Redis and Kubernetes client initialization |
| `compliance` | OSS compliance types |
| `config` | Environment-based configuration |
| `deployment` | Instance deployment controller, VCS integration |
| `drift` | GitOps drift detection |
| `health` | Health and readiness probes |
| `history` | History tracking service |
| `k8s/parser` | Type-safe Kubernetes object field access |
| `kro/watcher` | RGD and instance watchers |
| `kro/parser` | RGD resource parsing |
| `kro/schema` | Schema extraction |
| `kro/cel` | CEL expression handling |
| `kro/metadata` | KRO metadata helpers |
| `logger` | Structured logging |
| `manifest` | Manifest generation |
| `metrics` | Prometheus and GitOps metrics |
| `models` | Shared domain models |
| `rbac` | Casbin-based RBAC with PermissionService |
| `repository` | Git repository service (GitHub integration) |
| `resilience` | Circuit breaker patterns |
| `services` | Service layer interfaces |
| `sso` | SSO provider management |
| `websocket` | WebSocket hub for real-time updates |

## Server Development

### Running the Server

```bash
cd server
go run .
```

### Key Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SERVER_ADDRESS` | `:8080` | Bind address |
| `REDIS_ADDRESS` | `localhost:6379` | Redis connection |
| `LOG_LEVEL` | `info` | Log verbosity (`debug`, `info`, `warn`, `error`) |
| `KUBERNETES_IN_CLUSTER` | `false` | Use in-cluster Kubernetes config |
| `SWAGGER_UI_ENABLED` | `false` | Enable Swagger UI at `/swagger/` |

### Adding a New Handler

Create a handler in `server/internal/api/handlers/`:

```go
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
    "net/http"

    "github.com/knodex/knodex/server/internal/api/response"
)

type ExampleHandler struct {
    // inject dependencies here
}

func NewExampleHandler() *ExampleHandler {
    return &ExampleHandler{}
}

func (h *ExampleHandler) List(w http.ResponseWriter, r *http.Request) {
    items := []string{"one", "two", "three"}
    response.WriteJSON(w, http.StatusOK, items)
}
```

Register the handler in `server/internal/api/router.go`:

```go
exampleHandler := handlers.NewExampleHandler()
mux.HandleFunc("GET /api/v1/examples", exampleHandler.List)
```

### Adding RBAC Protection

Wrap the handler with the authorization middleware:

```go
mux.HandleFunc("GET /api/v1/examples",
    middleware.RequirePermission(enforcer, "examples", "get")(
        exampleHandler.List,
    ),
)
```

:::warning[Casbin Only]
All authorization checks must go through Casbin `Enforce()` calls. Never perform direct role checks. See the project's Casbin authorization model documentation for details.
:::

### Writing Server Tests

```go
// SPDX-License-Identifier: AGPL-3.0-only

package handlers_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/knodex/knodex/server/internal/api/handlers"
)

func TestExampleHandler_List(t *testing.T) {
    handler := handlers.NewExampleHandler()
    req := httptest.NewRequest(http.MethodGet, "/api/v1/examples", nil)
    rec := httptest.NewRecorder()

    handler.List(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", rec.Code)
    }
}
```

```bash
# Run all server tests
cd server && go test ./...

# Run a single test
cd server && go test -v -run TestExampleHandler_List ./internal/api/handlers/

# Run with race detection
cd server && go test -race ./...
```

## Web Development

### Running the Web UI

```bash
cd web
npm install
npm run dev    # Starts Vite dev server on :3000
```

### Adding a Component

```tsx
// SPDX-License-Identifier: AGPL-3.0-only

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

interface ProjectCardProps {
  name: string;
  namespace: string;
  status: "active" | "suspended";
}

export function ProjectCard({ name, namespace, status }: ProjectCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          {name}
          <Badge variant={status === "active" ? "default" : "secondary"}>
            {status}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-muted-foreground">
          Namespace: {namespace}
        </p>
      </CardContent>
    </Card>
  );
}
```

### API Client Pattern

API calls go through typed client functions in `web/src/api/`:

```typescript
// SPDX-License-Identifier: AGPL-3.0-only

import { apiClient } from "./client";

export interface Example {
  id: string;
  name: string;
}

export async function getExamples(): Promise<Example[]> {
  const { data } = await apiClient.get<Example[]>("/api/v1/examples");
  return data;
}
```

### State Management with Zustand

```typescript
// SPDX-License-Identifier: AGPL-3.0-only

import { create } from "zustand";

interface ExampleStore {
  selectedId: string | null;
  setSelectedId: (id: string | null) => void;
}

export const useExampleStore = create<ExampleStore>((set) => ({
  selectedId: null,
  setSelectedId: (id) => set({ selectedId: id }),
}));
```

### Writing Web Tests

Tests use Vitest with React Testing Library:

```typescript
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProjectCard } from "./ProjectCard";

describe("ProjectCard", () => {
  it("renders project name and status", () => {
    render(
      <ProjectCard name="alpha" namespace="alpha-ns" status="active" />
    );
    expect(screen.getByText("alpha")).toBeInTheDocument();
    expect(screen.getByText("active")).toBeInTheDocument();
  });
});
```

```bash
# Run all web tests
cd web && npm test

# Run tests in watch mode
cd web && npm test -- --watch

# Run a specific test file
cd web && npm test -- ProjectCard.test.tsx
```

## Working with Kubernetes

### Kind Local Development

The `make cluster-up` command creates a Kind cluster with:
- KRO controller installed
- Knodex CRDs applied
- Example resources loaded

```bash
# Create cluster
make cluster-up

# Verify KRO is running
kubectl get pods -n kro-system

# List example RGDs
kubectl get resourcegraphdefinitions
```

### Testing with a Real Cluster

Set your `KUBECONFIG` to point at a real cluster. The server will use whatever context is active:

```bash
export KUBECONFIG=~/.kube/config
kubectl config use-context my-cluster
make dev-server
```

## Adding New CRDs

1. Define the CRD YAML in `deploy/crds/`:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examples.knodex.io
spec:
  group: knodex.io
  names:
    kind: Example
    plural: examples
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
```

2. Add Go types in `server/internal/models/`
3. Add TypeScript types in `web/src/api/`
4. Apply to the cluster: `kubectl apply -f deploy/crds/`

## Code Style

### Go

- **Formatting**: `gofmt` (enforced automatically)
- **Vetting**: `go vet ./...`
- **Linting**: `golangci-lint run`
- **License headers**: Every file must have an SPDX header (`AGPL-3.0-only` for OSS, `LicenseRef-Knodex-Enterprise` for `ee/`)

```bash
make lint       # Run all linters
make lint-fix   # Auto-fix where possible
```

### TypeScript

- **Linting**: ESLint
- **Formatting**: Prettier
- **Strict mode**: TypeScript strict mode enabled

## Git Workflow

### Branch Naming

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feature/<description>` | `feature/add-audit-log` |
| Bug fix | `fix/<description>` | `fix/rbac-namespace-scope` |
| Documentation | `docs/<description>` | `docs/update-api-reference` |

### Conventional Commits

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
feat(catalog): add search by annotation
fix(rbac): enforce namespace scope on instance list
docs(api): add curl examples for deploy endpoint
refactor(kro): extract parser into separate package
test(e2e): add project creation workflow
```

### Pull Request Process

1. Create a feature branch from `main`
2. Make your changes with conventional commits
3. Ensure `make lint` and `make test` pass
4. Open a PR targeting `main`
5. Wait for CI checks to pass
6. Request review

## Debugging

### Server Debugging

```bash
# Enable debug logging
LOG_LEVEL=debug make dev-server

# Use Delve for interactive debugging
cd server
dlv debug . -- --address=:8080

# Attach to a running process
dlv attach <pid>
```

### Web Debugging

- **Browser DevTools**: Network tab for API calls, Console for errors
- **React DevTools**: Component tree inspection, state and props
- **Vite**: Built-in error overlay with source maps

### Kubernetes Debugging

```bash
# View server logs in the cluster
kubectl logs -f deployment/knodex-server -n knodex

# Exec into the server pod
kubectl exec -it deployment/knodex-server -n knodex -- sh

# Check events for a namespace
kubectl get events -n knodex --sort-by=.lastTimestamp
```

## Building for Production

```bash
# Build both server and web (embeds web assets into Go binary)
make build

# Build server only
make build-server

# Build web only
make build-web

# Build OSS edition
make build-oss

# Build Enterprise edition
make build-enterprise
```

The production binary is a single Go executable with the web UI embedded via `go:embed`.

## Resources

- [Go Documentation](https://go.dev/doc/)
- [React Documentation](https://react.dev/)
- [client-go](https://github.com/kubernetes/client-go)
- [Casbin](https://casbin.org/docs/overview)
- [Vite](https://vite.dev/)
- [TailwindCSS](https://tailwindcss.com/docs)
- [KRO](https://github.com/kubernetes-sigs/kro)
