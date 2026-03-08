# Developer Guide

This guide covers setting up a development environment for knodex and contributing to the project.

---

## Architecture Overview

knodex consists of three main components:

```
┌─────────────────────────────────────────────────────────────┐
│                      knodex                           │
├─────────────────┬─────────────────────┬─────────────────────┤
│    Web          │      Server         │       Redis         │
│  (React + Vite) │  (Go stdlib router) │   (Session Store)   │
│    Port 3000    │     Port 8080       │     Port 6379       │
└────────┬────────┴──────────┬──────────┴──────────┬──────────┘
         │                   │                     │
         │                   ▼                     │
         │         ┌─────────────────┐             │
         │         │  Kubernetes API │             │
         │         │  (KRO + CRDs)   │             │
         │         └─────────────────┘             │
         │                   │                     │
         └───────────────────┼─────────────────────┘
                             ▼
                    ┌─────────────────┐
                    │    Casbin       │
                    │  (RBAC Engine)  │
                    └─────────────────┘
```

### Server (Go)

- **Framework:** Standard library + custom middleware
- **Router:** Go 1.22+ ServeMux
- **Auth:** JWT + OIDC
- **RBAC:** Casbin
- **K8s Client:** client-go

### Web (React)

- **Framework:** React 19
- **Build Tool:** Vite
- **Styling:** TailwindCSS v4
- **State:** Zustand + React Query
- **Types:** TypeScript

---

## Development Setup

### Prerequisites

- Go 1.24+
- Node.js 20+
- Docker
- kubectl
- Kind (for local cluster): `brew install kind`
- Tilt (for local development with hot-reload): `brew install tilt`
- golangci-lint (for linting): `brew install golangci-lint`

### Clone Repository

```bash
git clone https://github.com/knodex/knodex.git
cd knodex
```

### Start Development Environment

```bash
# One-time: Create Kind cluster with KRO
make cluster-up

# Start Tilt for live Kubernetes development (recommended)
make tilt-up

# Or start natively (requires external Redis at localhost:6379):
make dev            # Server (:8080) + Web (:3000)
make dev-server     # Server only on :8080
make dev-web        # Web only on :3000 (Vite)
```

### Verify Setup

```bash
# Check server health
curl http://localhost:8080/healthz

# Check web UI (Vite dev server)
open http://localhost:3000
```

---

## Project Structure

```
knodex/
├── server/
│   ├── main.go                   # Entry point
│   ├── internal/
│   │   ├── api/
│   │   │   ├── router.go         # HTTP routing (Go 1.22+ ServeMux)
│   │   │   ├── handlers/         # Request handlers
│   │   │   ├── middleware/       # Auth, logging, security headers
│   │   │   └── response/        # Standardized error responses
│   │   ├── auth/                 # JWT + OIDC authentication
│   │   ├── bootstrap/            # App initialization
│   │   ├── clients/              # K8s and Redis clients
│   │   ├── config/               # Environment-based config
│   │   ├── deployment/           # Instance deployment (Direct/GitOps/Hybrid)
│   │   ├── health/               # Health check endpoints
│   │   ├── k8s/parser/           # Type-safe K8s object parsing
│   │   ├── models/               # Domain models
│   │   ├── rbac/                 # Casbin RBAC engine
│   │   ├── services/             # Business logic layer
│   │   └── watcher/              # K8s informers for live updates
│   ├── ee/                       # Enterprise-only (build tag gated)
│   │   ├── compliance/           # Deployment compliance auditing
│   │   ├── gatekeeper/           # OPA Gatekeeper integration
│   │   └── views/                # Custom category views
│   └── test/
│       └── e2e/                  # Server E2E tests
│
├── web/
│   ├── src/
│   │   ├── api/                  # API client
│   │   ├── components/           # React components (ui/, catalog/, etc.)
│   │   ├── hooks/                # Custom React hooks
│   │   ├── routes/               # Page routing
│   │   ├── stores/               # Zustand stores
│   │   ├── types/                # TypeScript types
│   │   └── App.tsx               # Main app
│   ├── test/e2e/                 # Playwright E2E tests
│   └── vite.config.ts
│
├── deploy/                       # Kubernetes deployment
│   ├── charts/knodex/            # Helm chart
│   ├── crds/                     # Custom Resource Definitions
│   ├── examples/                 # Example manifests (RGDs, Projects)
│   ├── server/                   # Server deployment manifests
│   ├── redis/                    # Redis deployment manifests
│   └── test/                     # Test infrastructure (mock OIDC, etc.)
│
├── docs/                         # Documentation
└── scripts/                      # Build/deploy scripts
```

---

## Backend Packages

Each package in `server/internal/` has a specific responsibility:

| Package | Purpose |
|---------|---------|
| `api/` | HTTP routing, request handlers, and middleware (auth, logging, security headers) |
| `auth/` | JWT token handling, OIDC provider integration, session management |
| `bootstrap/` | Application initialization and startup sequence |
| `clients/` | Kubernetes and Redis client initialization and configuration |
| `compliance/` | Deployment compliance checking against policy rules |
| `config/` | Environment-based configuration loading |
| `deployment/` | Instance deployment orchestration (Direct, GitOps, Hybrid modes) |
| `health/` | Health check endpoints with component status reporting |
| `history/` | Deployment history tracking and retrieval |
| `k8s/` | Kubernetes utilities including the object parser library |
| `logger/` | Structured logging with configurable output formats |
| `manifest/` | Kubernetes manifest generation and templating |
| `metrics/` | Prometheus metrics collection and exposure |
| `models/` | Shared domain models (Project, User, RGD, Instance) |
| `parser/` | RGD spec parsing and resource extraction |
| `rbac/` | Casbin-based RBAC policy enforcement and management |
| `repository/` | Git repository connection and validation |
| `resilience/` | Circuit breaker and retry patterns for external calls |
| `schema/` | JSON schema validation for RGD parameters |
| `services/` | Business logic layer coordinating multiple packages |
| `sso/` | SSO integration and group-to-role mapping |
| `watcher/` | Kubernetes informers for real-time resource updates |
| `websocket/` | WebSocket connection management for live updates |

### Enterprise Packages (`server/ee/`)

These packages are only compiled with `-tags=enterprise`:

| Package | Purpose |
|---------|---------|
| `ee/compliance/` | Advanced compliance auditing with policy rules |
| `ee/views/` | Custom category view configurations |
| `ee/gatekeeper/` | OPA Gatekeeper ConstraintTemplate/Constraint viewing |
| `ee/watcher/` | Enterprise-specific resource watchers |

---

## Server Development

### Running Server

```bash
cd server

# Run directly
go run .

# Or use make (from project root)
make dev-server
```

### Configuration

Environment variables:

```bash
export SERVER_ADDRESS=":8080"
export REDIS_ADDRESS="localhost:6379"
export LOG_LEVEL="debug"
export LOG_FORMAT="text"
export KUBERNETES_IN_CLUSTER="false"
```

### Adding a New Handler

1. Create handler file in `internal/api/handlers/`:

```go
// internal/api/handlers/example.go
package handlers

import (
    "encoding/json"
    "net/http"
)

type ExampleHandler struct {
    // dependencies
}

func NewExampleHandler() *ExampleHandler {
    return &ExampleHandler{}
}

func (h *ExampleHandler) List(w http.ResponseWriter, r *http.Request) {
    // Implementation
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *ExampleHandler) Get(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name") // Go 1.22+ path params
    // Implementation
}
```

2. Register in router (`internal/api/router.go`):

```go
func SetupRoutes(mux *http.ServeMux) {
    exampleHandler := handlers.NewExampleHandler()

    mux.HandleFunc("GET /api/v1/examples", exampleHandler.List)
    mux.HandleFunc("GET /api/v1/examples/{name}", exampleHandler.Get)
}
```

### Adding RBAC Protection

```go
import "github.com/knodex/knodex/server/internal/api/response"

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
    user := auth.GetUserFromContext(r.Context())

    // Check permission using Casbin
    allowed, err := h.enforcer.Enforce(user.Email, "projects", "create", "*")
    if err != nil || !allowed {
        response.Forbidden(w, "permission denied")
        return
    }

    // Continue with creation...
}
```

### Writing Tests

```go
// internal/api/handlers/example_test.go
package handlers_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/knodex/knodex/server/internal/api/handlers"
)

func TestExampleHandler_List(t *testing.T) {
    handler := handlers.NewExampleHandler()

    req := httptest.NewRequest("GET", "/api/v1/examples", nil)
    w := httptest.NewRecorder()

    handler.List(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", w.Code)
    }
}
```

Run tests:

```bash
# All tests
make test

# Specific package
go test -v ./internal/api/handlers/...

# With coverage
go test -cover ./...
```

---

## Web Development

### Running Web

```bash
cd web

# Install dependencies
npm install

# Start dev server
npm run dev
```

### Adding a New Component

1. Create component in `src/components/`:

```tsx
// src/components/projects/ProjectCard.tsx
import { Card, CardHeader, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';

interface ProjectCardProps {
  project: {
    name: string;
    displayName: string;
    description?: string;
    roleCount: number;
  };
  onClick?: () => void;
}

export function ProjectCard({ project, onClick }: ProjectCardProps) {
  return (
    <Card className="cursor-pointer hover:shadow-md" onClick={onClick}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">{project.displayName}</h3>
          <Badge variant="secondary">{project.roleCount} roles</Badge>
        </div>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-muted-foreground">
          {project.description || 'No description'}
        </p>
      </CardContent>
    </Card>
  );
}
```

2. Use in a page:

```tsx
// src/pages/ProjectsPage.tsx
import { useQuery } from '@tanstack/react-query';
import { ProjectCard } from '@/components/projects/ProjectCard';
import { api } from '@/api/client';

export function ProjectsPage() {
  const { data: projects, isLoading } = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.projects.list(),
  });

  if (isLoading) return <div>Loading...</div>;

  return (
    <div className="grid grid-cols-3 gap-4">
      {projects?.map((project) => (
        <ProjectCard key={project.name} project={project} />
      ))}
    </div>
  );
}
```

### API Client

Add API methods in `src/api/`:

```typescript
// src/api/projects.ts
import { apiClient } from './client';
import type { Project, CreateProjectRequest } from '@/types';

export const projectsApi = {
  list: async (): Promise<Project[]> => {
    const response = await apiClient.get('/api/v1/projects');
    return response.data.items;
  },

  get: async (name: string): Promise<Project> => {
    const response = await apiClient.get(`/api/v1/projects/${name}`);
    return response.data;
  },

  create: async (data: CreateProjectRequest): Promise<Project> => {
    const response = await apiClient.post('/api/v1/projects', data);
    return response.data;
  },

  delete: async (name: string): Promise<void> => {
    await apiClient.delete(`/api/v1/projects/${name}`);
  },
};
```

### State Management (Zustand)

```typescript
// src/stores/auth.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface AuthState {
  user: User | null;
  token: string | null;
  setAuth: (user: User, token: string) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      setAuth: (user, token) => set({ user, token }),
      logout: () => set({ user: null, token: null }),
    }),
    { name: 'auth-storage' }
  )
);
```

### Writing Tests

```typescript
// src/components/projects/ProjectCard.test.tsx
import { render, screen, fireEvent } from '@testing-library/react';
import { vi, describe, it, expect } from 'vitest';
import { ProjectCard } from './ProjectCard';

describe('ProjectCard', () => {
  const mockProject = {
    name: 'test-project',
    displayName: 'Test Project',
    description: 'A test project',
    roleCount: 3,
  };

  it('renders project information', () => {
    render(<ProjectCard project={mockProject} />);

    expect(screen.getByText('Test Project')).toBeInTheDocument();
    expect(screen.getByText('A test project')).toBeInTheDocument();
    expect(screen.getByText('3 roles')).toBeInTheDocument();
  });

  it('calls onClick when clicked', () => {
    const handleClick = vi.fn();
    render(<ProjectCard project={mockProject} onClick={handleClick} />);

    fireEvent.click(screen.getByText('Test Project'));
    expect(handleClick).toHaveBeenCalled();
  });
});
```

Run tests:

```bash
# Unit tests
npm test

# E2E tests
npm run test:e2e

# With coverage
npm run test:coverage
```

---

## Working with Kubernetes

### Local Development with Kind

```bash
# Create Kind cluster (one-time)
make cluster-up

# Start Tilt for live development
make tilt-up
```

`make cluster-up` creates the Kind cluster. KRO and CRDs are installed automatically when you run `make tilt-up`, `make e2e`, or `make qa`.

### Testing with Real Cluster

```bash
# Point to your cluster
export KUBECONFIG=~/.kube/config

# Run server with in-cluster=false (from server/ directory)
cd server && KUBERNETES_IN_CLUSTER=false go run .
```

---

## Adding New CRDs

### 1. Define the CRD

```yaml
# deploy/crds/example-crd.yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examples.knodex.io
spec:
  group: knodex.io
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                name:
                  type: string
  scope: Namespaced
  names:
    plural: examples
    singular: example
    kind: Example
```

### 2. Create Go Types

```go
// internal/models/example.go
package models

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Example struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              ExampleSpec `json:"spec,omitempty"`
}

type ExampleSpec struct {
    Name string `json:"name"`
}

type ExampleList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Example `json:"items"`
}
```

### 3. Create TypeScript Types

```typescript
// src/types/example.ts
export interface Example {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
  };
  spec: {
    name: string;
  };
}
```

---

## Code Style

### Go

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Run `go vet` before committing

```bash
# Format code
gofmt -w .

# Lint
go vet ./...
golangci-lint run
```

### TypeScript/React

- Follow ESLint configuration
- Use Prettier for formatting

```bash
# Lint
npm run lint

# Type check
npm run typecheck
```

---

## Git Workflow

### Branch Naming

```
feature/add-user-settings
fix/resolve-auth-timeout
docs/update-readme
```

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(auth): add OIDC provider support

- Implement OIDC authentication flow
- Add token validation middleware
- Support multiple OIDC providers

Implements OIDC authentication feature
```

### Pull Request Process

1. Create feature branch
2. Make changes with tests
3. Run linting and tests locally
4. Push and create PR
5. Wait for CI to pass
6. Request review
7. Squash merge to main

---

## Debugging

### Server Debugging

```bash
# Enable debug logging (from server/ directory)
cd server && LOG_LEVEL=debug go run .

# Use delve debugger
cd server && dlv debug .
```

### Web Debugging

1. Open browser DevTools (F12)
2. Use React DevTools extension
3. Check Network tab for API calls
4. Use `console.log` or debugger statements

### Kubernetes Debugging

```bash
# Check pod logs
kubectl logs -f deployment/knodex-server -n knodex

# Exec into pod
kubectl exec -it deployment/knodex-server -n knodex -- sh

# Check events
kubectl get events -n knodex --sort-by='.lastTimestamp'
```

---

## Building for Production

```bash
# Build everything
make build

# Build server only → bin/knodex-server
make build-server

# Build web only → web/dist/
make build-web
```

---

## Resources

- [Go Documentation](https://golang.org/doc/)
- [React Documentation](https://react.dev/)
- [Kubernetes Client-Go](https://github.com/kubernetes/client-go)
- [Casbin Documentation](https://casbin.org/docs/overview)
- [Vite Documentation](https://vitejs.dev/)
- [TailwindCSS Documentation](https://tailwindcss.com/docs)
