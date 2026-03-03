# Mock OIDC Server

A lightweight, reusable mock OIDC provider for deterministic E2E testing. This package provides a fully-functional OpenID Connect server that can be used in both unit tests and Kubernetes-based E2E test environments.

## Features

- **Complete OIDC Implementation**: Discovery, Authorization, Token, JWKS, and UserInfo endpoints
- **Pre-configured Test Users**: Admin, Developer, Viewer, and edge-case users ready to use
- **Edge Case Simulation**: Expired tokens, invalid signatures, missing claims, token rejection
- **Thread-Safe**: Safe for concurrent use in parallel tests
- **Kubernetes Ready**: Includes manifests for deployment in Kind/K8s clusters

## Quick Start

### Unit Test Usage

```go
package mytest

import (
    "testing"
    "github.com/provops-org/knodex/server/test/mocks/oidc"
)

func TestWithMockOIDC(t *testing.T) {
    // Create server with default configuration
    server, err := oidc.NewServer()
    if err != nil {
        t.Fatal(err)
    }

    // Start server
    ctx := context.Background()
    if err := server.Start(ctx); err != nil {
        t.Fatal(err)
    }
    defer server.Stop(ctx)

    // Use the issuer URL in your tests
    issuerURL := server.IssuerURL()
    // Configure your application to use this issuer
}
```

### Custom Configuration

```go
server, err := oidc.NewServer(
    oidc.WithPort(9999),
    oidc.WithIssuerURL("http://my-issuer:9999"),
    oidc.WithClientCredentials("my-client", "my-secret"),
    oidc.WithTokenExpiry(30 * time.Minute),
)
```

## Pre-configured Test Users

The server comes with pre-configured test users for common RBAC scenarios:

| Email | Role | Groups | Description |
|-------|------|--------|-------------|
| `admin@test.local` | Admin | `knodex-admins` | Full administrative access |
| `developer@test.local` | Developer | `alpha-developers` | Development team access |
| `viewer@test.local` | Viewer | `alpha-viewers` | Read-only access |
| `platform-admin@test.local` | Platform Admin | `platform-admins` | Platform-level administration |
| `multi-group@test.local` | Multi-Group | `alpha-developers`, `alpha-viewers` | User in multiple groups |
| `no-groups@test.local` | No Groups | (none) | User without group membership |
| `expired@test.local` | Expired | - | Forces expired tokens |
| `unverified@test.local` | Unverified | - | Unverified email address |
| `invalid@test.local` | Invalid | - | Forces invalid claims |

### Using Email Constants

```go
import "github.com/provops-org/knodex/server/test/mocks/oidc"

// Use constants for type safety
email := oidc.AdminEmail        // "admin@test.local"
email := oidc.DeveloperEmail    // "developer@test.local"
email := oidc.ViewerEmail       // "viewer@test.local"

// Group constants
group := oidc.GroupKnodexAdmins    // "knodex-admins"
group := oidc.GroupAlphaDevelopers  // "alpha-developers"
```

## Edge Case Testing

### Expired Tokens

```go
server, _ := oidc.NewServer(
    oidc.EnableExpiredTokens(),
)
// All tokens will be issued with past expiration
```

### Invalid Signatures

```go
server, _ := oidc.NewServer(
    oidc.EnableInvalidSignature(),
)
// Tokens will be signed with wrong key
```

### Reject All Tokens

```go
server, _ := oidc.NewServer(
    oidc.EnableRejectAllTokens(),
)
// Token endpoint returns errors for all requests
```

### Per-User Scenarios

Certain test users have built-in behaviors:

```go
// This user always gets expired tokens
authCode, _ := server.GenerateAuthCode(
    oidc.ExpiredEmail,  // "expired@test.local"
    redirectURI,
    state,
    nonce,
)

// This user has unverified email
authCode, _ := server.GenerateAuthCode(
    oidc.UnverifiedEmail,  // "unverified@test.local"
    redirectURI,
    state,
    nonce,
)
```

## Adding Custom Users

```go
server, _ := oidc.NewServer()

server.AddUser(&oidc.TestUser{
    Email:         "custom@example.com",
    Subject:       "user-custom-123",
    Name:          "Custom User",
    Groups:        []string{"custom-group", "another-group"},
    EmailVerified: true,
})
```

## OIDC Endpoints

When running, the server exposes these endpoints:

| Endpoint | Path | Description |
|----------|------|-------------|
| Health | `GET /healthz` | Health check |
| Discovery | `GET /.well-known/openid-configuration` | OIDC discovery document |
| JWKS | `GET /.well-known/jwks.json` | JSON Web Key Set |
| Authorize | `GET /authorize` | Authorization endpoint |
| Token | `POST /token` | Token exchange endpoint |
| UserInfo | `GET /userinfo` | User information endpoint |

## Kubernetes Deployment

### Deploy to Kind Cluster

The mock OIDC server is automatically deployed when using `make qa-deploy`:

```bash
make qa-deploy
```

### Manual Deployment

```bash
kubectl apply -k deploy/test/mock-oidc/
```

### Service URL

When deployed in Kubernetes:
- **Internal**: `http://mock-oidc:8081`
- **External**: `http://localhost:<nodeport>`

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithPort(port)` | `8081` | Server listen port |
| `WithIssuerURL(url)` | `http://localhost:8081` | OIDC issuer URL |
| `WithClientCredentials(id, secret)` | `test-client-id`, `test-client-secret` | OAuth client credentials |
| `WithRedirectURL(url)` | `http://localhost:8080/api/v1/auth/oidc/callback` | Default redirect URI |
| `WithTokenExpiry(duration)` | `1 hour` | Token validity duration |
| `WithScenarios(config)` | `nil` | Scenario configuration |
| `EnableExpiredTokens()` | - | Enable expired token scenario |
| `EnableInvalidSignature()` | - | Enable invalid signature scenario |
| `EnableRejectAllTokens()` | - | Enable token rejection scenario |

## Integration with E2E Tests

### Environment Variables

When deployed via `make qa-deploy`, these environment variables are set:

```bash
MOCK_OIDC_ENABLED=true
MOCK_OIDC_ISSUER_URL=http://mock-oidc:8081
MOCK_OIDC_CLIENT_ID=test-client-id
MOCK_OIDC_CLIENT_SECRET=test-client-secret
```

### Frontend E2E Tests

The mock OIDC server works with Playwright tests:

```typescript
// Generate auth code directly via server API
const authCode = await fetch(`${MOCK_OIDC_URL}/authorize?` + new URLSearchParams({
    client_id: 'test-client-id',
    redirect_uri: 'http://localhost:8080/callback',
    response_type: 'code',
    scope: 'openid profile email groups',
    state: 'test-state',
    nonce: 'test-nonce',
    login_hint: 'admin@test.local',  // Specify user via login_hint
}));
```

### Backend E2E Tests

```go
// Use mock OIDC server for backend tests
os.Setenv("OIDC_ISSUER_URL", "http://mock-oidc:8081")
os.Setenv("OIDC_CLIENT_ID", "test-client-id")
os.Setenv("OIDC_CLIENT_SECRET", "test-client-secret")
```

## Architecture

```
server/test/mocks/oidc/
├── config.go       # Configuration types and options
├── users.go        # Test user definitions
├── server.go       # Main OIDC server implementation
├── cmd/
│   └── main.go     # Standalone server binary
├── Dockerfile      # Container image build
└── README.md       # This file

deploy/test/mock-oidc/
├── deployment.yaml     # Kubernetes Deployment
├── service.yaml        # Kubernetes Service
├── networkpolicy.yaml  # Network security
└── kustomization.yaml  # Kustomize configuration
```

## Testing

Run unit tests:

```bash
cd server
go test -v ./test/mocks/oidc/...
```

Check coverage:

```bash
go test -cover ./test/mocks/oidc/...
# Current coverage: 84.7%
```

## Troubleshooting

### Port Already in Use

If you get a "port already in use" error, use a different port:

```go
server, _ := oidc.NewServer(oidc.WithPort(0))  // Let OS assign port
```

### Token Validation Fails

Ensure your application is configured to:
1. Use the correct issuer URL from `server.IssuerURL()`
2. Fetch JWKS from `/.well-known/jwks.json`
3. Use the correct client ID

### User Not Found

The server only recognizes pre-configured users. Add custom users:

```go
server.AddUser(&oidc.TestUser{
    Email:   "myuser@example.com",
    Subject: "user-123",
    Name:    "My User",
})
```
