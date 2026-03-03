#!/bin/bash
set -e

# E2E Test Data Setup Script for  Comprehensive RBAC E2E Tests
# This script provisions test users, projects, and generates JWT tokens
# for automated E2E testing with Chrome DevTools MCP

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
# Auto-detect namespace based on branch (matches qa-deploy.sh logic)
if [ -n "${NAMESPACE:-}" ]; then
    # Use explicit NAMESPACE if set
    :
elif git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9-]/-/g' | head -c 20)
    NAMESPACE="knodex-${BRANCH}"
else
    NAMESPACE="knodex-main"
fi
JWT_SECRET=${JWT_SECRET:-"test-jwt-secret-key-for-local-dev-only"}
JWT_EXPIRY="3600" # 1 hour in seconds

# Output directory for test artifacts - unified at project root test-results/
OUTPUT_DIR="${PROJECT_ROOT}/test-results/e2e"
mkdir -p "$OUTPUT_DIR"

echo "=========================================="
echo "E2E Test Data Setup"
echo "=========================================="
echo ""

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Generate JWT token using base64 and openssl
# This creates valid JWT tokens for E2E testing without needing the server
# Note: Uses casbin_roles instead of is_global_admin
generate_jwt_token() {
    local user_id=$1
    local email=$2
    local display_name=$3
    local casbin_roles=$4  # JSON array string like '["role:serveradmin"]' or '[]'
    local projects=$5  # JSON array string like '["proj-1","proj-2"]'
    local default_project=$6
    local groups=${7:-'[]'}  # JSON array string like '["oidc-azuread-admins"]' or '[]'

    local now=$(date +%s)
    local exp=$((now + JWT_EXPIRY))

    # Create header (HS256 algorithm)
    local header='{"alg":"HS256","typ":"JWT"}'
    local header_b64=$(echo -n "$header" | base64 | tr -d '=' | tr '/+' '_-' | tr -d '\n')

    # Create payload - Note: casbin_roles replaces is_global_admin
    # groups is the OIDC groups claim from the IdP
    # iss/aud are required by backend ValidateToken (STORY-235)
    local payload="{\"sub\":\"$user_id\",\"email\":\"$email\",\"name\":\"$display_name\",\"projects\":$projects,\"default_project\":\"$default_project\",\"casbin_roles\":$casbin_roles,\"groups\":$groups,\"iss\":\"knodex\",\"aud\":\"knodex-api\",\"exp\":$exp,\"iat\":$now}"
    local payload_b64=$(echo -n "$payload" | base64 | tr -d '=' | tr '/+' '_-' | tr -d '\n')

    # Create signature
    local signature=$(echo -n "${header_b64}.${payload_b64}" | openssl dgst -sha256 -hmac "$JWT_SECRET" -binary | base64 | tr -d '=' | tr '/+' '_-' | tr -d '\n')

    # Return complete JWT
    echo "${header_b64}.${payload_b64}.${signature}"
}

# Check prerequisites
check_prerequisites() {
    log_step "Checking prerequisites..."

    command -v kubectl >/dev/null 2>&1 || { log_error "kubectl is required but not installed."; exit 1; }
    command -v jq >/dev/null 2>&1 || { log_error "jq is required but not installed."; exit 1; }
    command -v openssl >/dev/null 2>&1 || { log_error "openssl is required but not installed."; exit 1; }

    # Check if Kind cluster is available
    if ! kubectl cluster-info >/dev/null 2>&1; then
        log_error "Cannot connect to Kubernetes cluster. Is Kind running?"
        log_info "Run: make qa-deploy"
        exit 1
    fi

    log_info "All prerequisites met"
}

# Get JWT secret from deployed server
get_jwt_secret_from_cluster() {
    log_step "Retrieving JWT secret from cluster..."

    # Try knodex-jwt secret first (used in QA/Kind deployments)
    local secret=$(kubectl get secret -n "$NAMESPACE" knodex-jwt -o jsonpath='{.data.JWT_SECRET}' 2>/dev/null || echo "")
    if [ -n "$secret" ]; then
        JWT_SECRET=$(echo "$secret" | base64 -d)
        log_info "JWT secret retrieved from knodex-jwt secret"
        return
    fi

    # Fallback to knodex-secrets (used in production)
    secret=$(kubectl get secret -n "$NAMESPACE" knodex-secrets -o jsonpath='{.data.jwt-secret}' 2>/dev/null || echo "")
    if [ -n "$secret" ]; then
        JWT_SECRET=$(echo "$secret" | base64 -d)
        log_info "JWT secret retrieved from knodex-secrets"
        return
    fi

    log_warn "Could not retrieve JWT secret from cluster, using default test secret"
}

# Create test projects
create_test_projects() {
    log_step "Creating test projects..."

    # Project 1: Alpha Team - ArgoCD-compatible schema
    cat <<EOF | kubectl apply -f -
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: proj-alpha-team
  labels:
    team: alpha
spec:
  description: "Alpha Team project - E2E testing"
  destinations:
    - server: "*"
      namespace: ns-alpha-team
  namespaceResourceWhitelist:
    - group: ""
      kind: "*"
    - group: apps
      kind: "*"
    - group: batch
      kind: "*"
  clusterResourceWhitelist: []
  roles:
    - name: admin
      description: "Full project access"
      policies:
        - "p, proj:proj-alpha-team:admin, project, *, proj-alpha-team, allow"
        - "p, proj:proj-alpha-team:admin, instance, *, proj-alpha-team/*, allow"
        - "p, proj:proj-alpha-team:admin, rgd, view, *, allow"
      groups:
        - alpha-admins
        - "user:user-alpha-admin"
    - name: developer
      description: "Can deploy and manage instances"
      policies:
        - "p, proj:proj-alpha-team:developer, project, view, proj-alpha-team, allow"
        - "p, proj:proj-alpha-team:developer, instance, deploy, proj-alpha-team/*, allow"
        - "p, proj:proj-alpha-team:developer, instance, view, proj-alpha-team/*, allow"
        - "p, proj:proj-alpha-team:developer, instance, delete, proj-alpha-team/*, allow"
        - "p, proj:proj-alpha-team:developer, rgd, view, *, allow"
      groups:
        - alpha-developers
        - "user:user-alpha-developer"
    - name: viewer
      description: "Read-only access"
      policies:
        - "p, proj:proj-alpha-team:viewer, project, view, proj-alpha-team, allow"
        - "p, proj:proj-alpha-team:viewer, instance, view, proj-alpha-team/*, allow"
        - "p, proj:proj-alpha-team:viewer, rgd, view, *, allow"
      groups:
        - alpha-viewers
        - "user:user-alpha-viewer"
EOF

    # Project 2: Beta Team - ArgoCD-compatible schema
    cat <<EOF | kubectl apply -f -
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: proj-beta-team
  labels:
    team: beta
spec:
  description: "Beta Team project - E2E testing"
  destinations:
    - server: "*"
      namespace: ns-beta-team
  namespaceResourceWhitelist:
    - group: ""
      kind: "*"
    - group: apps
      kind: "*"
  clusterResourceWhitelist: []
  roles:
    - name: admin
      description: "Full project access"
      policies:
        - "p, proj:proj-beta-team:admin, project, *, proj-beta-team, allow"
        - "p, proj:proj-beta-team:admin, instance, *, proj-beta-team/*, allow"
        - "p, proj:proj-beta-team:admin, rgd, view, *, allow"
      groups:
        - beta-admins
        - "user:user-beta-admin"
    - name: developer
      description: "Can deploy and manage instances"
      policies:
        - "p, proj:proj-beta-team:developer, project, view, proj-beta-team, allow"
        - "p, proj:proj-beta-team:developer, instance, deploy, proj-beta-team/*, allow"
        - "p, proj:proj-beta-team:developer, instance, view, proj-beta-team/*, allow"
        - "p, proj:proj-beta-team:developer, rgd, view, *, allow"
      groups:
        - beta-developers
        - "user:user-beta-developer"
EOF

    # Project 3: Shared (for testing shared RGDs) - ArgoCD-compatible schema
    cat <<EOF | kubectl apply -f -
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: proj-shared
  labels:
    visibility: public
spec:
  description: "Shared Resources project - E2E testing"
  destinations:
    - server: "*"
      namespace: ns-shared
  namespaceResourceWhitelist:
    - group: ""
      kind: "*"
  clusterResourceWhitelist: []
  roles:
    - name: admin
      description: "Shared project admin"
      policies:
        - "p, proj:proj-shared:admin, project, *, proj-shared, allow"
        - "p, proj:proj-shared:admin, instance, *, proj-shared/*, allow"
        - "p, proj:proj-shared:admin, rgd, view, *, allow"
      groups:
        - shared-admins
EOF

    # Project 4: Azure AD Staging - For OIDC group testing with wildcard namespaces
    cat <<EOF | kubectl apply -f -
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: proj-azuread-staging
  labels:
    team: azure-ad
    environment: staging
spec:
  description: "Azure AD Staging project - OIDC E2E testing with wildcard namespaces"
  destinations:
    - server: "*"
      namespace: "staging*"
    - server: "*"
      namespace: "knodex*"
  namespaceResourceWhitelist:
    - group: ""
      kind: "*"
    - group: apps
      kind: "*"
  clusterResourceWhitelist: []
  roles:
    - name: admin
      description: "Full project access via OIDC group"
      policies:
        - "p, proj:proj-azuread-staging:admin, project, *, proj-azuread-staging, allow"
        - "p, proj:proj-azuread-staging:admin, instance, *, proj-azuread-staging/*, allow"
        - "p, proj:proj-azuread-staging:admin, rgd, view, *, allow"
        - "p, proj:proj-azuread-staging:admin, compliance, get, *, allow"
      groups:
        - "7e24cb11-e404-4b4d-9e2c-96d6e7b4733c"
        - "oidc-azuread-admins"
EOF

    # Project 5: Platform - For namespace dropdown tests
    # Note: namespace pattern must match CRD regex - use "platform*" not "platform-*"
    cat <<EOF | kubectl apply -f -
apiVersion: knodex.io/v1alpha1
kind: Project
metadata:
  name: proj-platform
  labels:
    team: platform
spec:
  description: "Platform project - E2E testing"
  destinations:
    - server: "*"
      namespace: "platform-services"
  namespaceResourceWhitelist:
    - group: ""
      kind: "*"
    - group: apps
      kind: "*"
  clusterResourceWhitelist: []
  roles:
    - name: admin
      description: "Full platform access"
      policies:
        - "p, proj:proj-platform:admin, project, *, proj-platform, allow"
        - "p, proj:proj-platform:admin, instance, *, proj-platform/*, allow"
        - "p, proj:proj-platform:admin, rgd, view, *, allow"
      groups:
        - platform-admins
EOF

    log_info "Test projects created with ArgoCD-compatible schema"

    # Create namespaces for projects
    log_step "Creating project namespaces..."

    for ns in ns-alpha-team ns-beta-team ns-shared staging-app staging-db knodex-test platform-services; do
        kubectl create namespace "$ns" --dry-run=client -o yaml | kubectl apply -f -
        kubectl label namespace "$ns" knodex.io/managed-by=e2e-test --overwrite
        # Label namespaces with their associated project
        case "$ns" in
            ns-alpha-team)
                kubectl label namespace "$ns" knodex.io/project=proj-alpha-team --overwrite
                ;;
            ns-beta-team)
                kubectl label namespace "$ns" knodex.io/project=proj-beta-team --overwrite
                ;;
            ns-shared)
                kubectl label namespace "$ns" knodex.io/project=proj-shared --overwrite
                ;;
            staging-*)
                kubectl label namespace "$ns" knodex.io/project=proj-azuread-staging --overwrite
                ;;
            knodex-test)
                kubectl label namespace "$ns" knodex.io/project=proj-azuread-staging --overwrite
                ;;
            platform-*)
                kubectl label namespace "$ns" knodex.io/project=proj-platform --overwrite
                ;;
        esac
    done

    log_info "Project namespaces created and labeled"
}

# Note: User CRD has been removed
# Users are now ephemeral (OIDC) or stored in ConfigMap/Secret (local)
# For E2E tests, we use JWT tokens to authenticate - no user CRs needed
create_test_users() {
    log_step "Skipping User CRD creation (Note: removed)..."
    log_info "Test users will be authenticated via JWT tokens"
    log_info "User permissions come from JWT claims and Casbin policies in Projects"
}

# Generate JWT tokens for all test users
# Note: Uses casbin_roles instead of is_global_admin
generate_all_tokens() {
    log_step "Generating JWT tokens for test users..."

    local tokens_file="$OUTPUT_DIR/test-tokens.json"

    # Generate tokens - Note: casbin_roles replaces is_global_admin
    # Global admin has role:serveradmin, others have empty array
    local global_admin_token=$(generate_jwt_token \
        "user-global-admin" \
        "admin@e2e-test.local" \
        "Global Administrator" \
        '["role:serveradmin"]' \
        '["proj-alpha-team","proj-beta-team","proj-shared"]' \
        "proj-alpha-team")

    local alpha_admin_token=$(generate_jwt_token \
        "user-alpha-admin" \
        "alpha-admin@e2e-test.local" \
        "Alpha Team Admin" \
        '[]' \
        '["proj-alpha-team"]' \
        "proj-alpha-team")

    local alpha_developer_token=$(generate_jwt_token \
        "user-alpha-developer" \
        "alpha-dev@e2e-test.local" \
        "Alpha Developer" \
        '[]' \
        '["proj-alpha-team"]' \
        "proj-alpha-team")

    local alpha_viewer_token=$(generate_jwt_token \
        "user-alpha-viewer" \
        "alpha-viewer@e2e-test.local" \
        "Alpha Viewer" \
        '[]' \
        '["proj-alpha-team"]' \
        "proj-alpha-team")

    local beta_admin_token=$(generate_jwt_token \
        "user-beta-admin" \
        "beta-admin@e2e-test.local" \
        "Beta Team Admin" \
        '[]' \
        '["proj-beta-team"]' \
        "proj-beta-team")

    local beta_developer_token=$(generate_jwt_token \
        "user-beta-developer" \
        "beta-dev@e2e-test.local" \
        "Beta Developer" \
        '[]' \
        '["proj-beta-team"]' \
        "proj-beta-team")

    local no_orgs_token=$(generate_jwt_token \
        "user-no-orgs" \
        "no-orgs@e2e-test.local" \
        "No Organizations User" \
        '[]' \
        '[]' \
        "")

    # OIDC Azure AD User - has group-based access to proj-azuread-staging
    # The oidc-azuread-admins group maps to proj:proj-azuread-staging:admin role
    local oidc_azuread_token=$(generate_jwt_token \
        "user-oidc-azuread" \
        "oidc-user@azuread.example.com" \
        "OIDC Azure AD User" \
        '[]' \
        '["proj-azuread-staging"]' \
        "proj-azuread-staging" \
        '["oidc-azuread-admins"]')

    # Write tokens to JSON file - Note: casbin_roles replaces is_global_admin
    # Format: roles is a Record<string, string> mapping project -> role
    cat > "$tokens_file" <<EOF
{
  "generated_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "expires_in_seconds": $JWT_EXPIRY,
  "users": {
    "global_admin": {
      "user_id": "user-global-admin",
      "email": "admin@e2e-test.local",
      "display_name": "Global Administrator",
      "casbin_roles": ["role:serveradmin"],
      "roles": {"proj-alpha-team": "admin", "proj-beta-team": "admin", "proj-shared": "admin"},
      "projects": ["proj-alpha-team", "proj-beta-team", "proj-shared"],
      "token": "$global_admin_token"
    },
    "alpha_admin": {
      "user_id": "user-alpha-admin",
      "email": "alpha-admin@e2e-test.local",
      "display_name": "Alpha Team Admin",
      "casbin_roles": [],
      "roles": {"proj-alpha-team": "admin"},
      "projects": ["proj-alpha-team"],
      "token": "$alpha_admin_token"
    },
    "alpha_developer": {
      "user_id": "user-alpha-developer",
      "email": "alpha-dev@e2e-test.local",
      "display_name": "Alpha Developer",
      "casbin_roles": [],
      "roles": {"proj-alpha-team": "developer"},
      "projects": ["proj-alpha-team"],
      "token": "$alpha_developer_token"
    },
    "alpha_viewer": {
      "user_id": "user-alpha-viewer",
      "email": "alpha-viewer@e2e-test.local",
      "display_name": "Alpha Viewer",
      "casbin_roles": [],
      "roles": {"proj-alpha-team": "viewer"},
      "projects": ["proj-alpha-team"],
      "token": "$alpha_viewer_token"
    },
    "beta_admin": {
      "user_id": "user-beta-admin",
      "email": "beta-admin@e2e-test.local",
      "display_name": "Beta Team Admin",
      "casbin_roles": [],
      "roles": {"proj-beta-team": "admin"},
      "projects": ["proj-beta-team"],
      "token": "$beta_admin_token"
    },
    "beta_developer": {
      "user_id": "user-beta-developer",
      "email": "beta-dev@e2e-test.local",
      "display_name": "Beta Developer",
      "casbin_roles": [],
      "roles": {"proj-beta-team": "developer"},
      "projects": ["proj-beta-team"],
      "token": "$beta_developer_token"
    },
    "no_orgs": {
      "user_id": "user-no-orgs",
      "email": "no-orgs@e2e-test.local",
      "display_name": "No Organizations User",
      "casbin_roles": [],
      "roles": {},
      "projects": [],
      "token": "$no_orgs_token"
    },
    "oidc_azuread": {
      "user_id": "user-oidc-azuread",
      "email": "oidc-user@azuread.example.com",
      "display_name": "OIDC Azure AD User",
      "casbin_roles": [],
      "groups": ["oidc-azuread-admins"],
      "roles": {"proj-azuread-staging": "admin"},
      "projects": ["proj-azuread-staging"],
      "token": "$oidc_azuread_token"
    }
  }
}
EOF

    log_info "JWT tokens written to: $tokens_file"

    # Also write a shell script to export tokens as environment variables
    local env_file="$OUTPUT_DIR/test-tokens.env"
    cat > "$env_file" <<EOF
# E2E Test Tokens - Generated $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Source this file: source $env_file

export E2E_TOKEN_GLOBAL_ADMIN="$global_admin_token"
export E2E_TOKEN_ALPHA_ADMIN="$alpha_admin_token"
export E2E_TOKEN_ALPHA_DEVELOPER="$alpha_developer_token"
export E2E_TOKEN_ALPHA_VIEWER="$alpha_viewer_token"
export E2E_TOKEN_BETA_ADMIN="$beta_admin_token"
export E2E_TOKEN_BETA_DEVELOPER="$beta_developer_token"
export E2E_TOKEN_NO_ORGS="$no_orgs_token"
export E2E_TOKEN_OIDC_AZUREAD="$oidc_azuread_token"
EOF

    log_info "Environment variables written to: $env_file"

    # Tokens are now in unified test-results/e2e directory
    log_info "Tokens available at: $tokens_file"
}

# Create test RGDs with project labels for RBAC testing
# RGDs are cluster-scoped, so we use labels for project-based access control
create_test_rgds() {
    log_step "Creating test RGDs with project labels..."

    # Delete conflicting RGDs first (spec.schema.kind is immutable in KRO)
    # These may have been created by qa-deploy with different schema kinds
    log_info "Removing conflicting RGDs that may have different schema.kind..."
    for rgd in simple-app webapp-with-features microservices-platform; do
        if kubectl get rgd "$rgd" &>/dev/null; then
            kubectl delete rgd "$rgd" --ignore-not-found=true 2>/dev/null || true
            # Wait for deletion to complete
            kubectl wait --for=delete rgd/"$rgd" --timeout=30s 2>/dev/null || true
        fi
    done

    # RGD for Alpha Team (only alpha team members should see)
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: alpha-database
  labels:
    app.kubernetes.io/name: alpha-database
    knodex.io/project: proj-alpha-team
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: database
    knodex.io/description: "Alpha Team database for testing"
spec:
  schema:
    apiVersion: v1alpha1
    kind: AlphaDatabase
    spec:
      size: 'string | default="small"'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          size: '\${schema.spec.size}'
EOF

    # RGD for Beta Team (only beta team members should see)
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: beta-cache
  labels:
    app.kubernetes.io/name: beta-cache
    knodex.io/project: proj-beta-team
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: cache
    knodex.io/description: "Beta Team cache for testing"
spec:
  schema:
    apiVersion: v1alpha1
    kind: BetaCache
    spec:
      replicas: 'integer | default=1'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          replicas: '\${schema.spec.replicas}'
EOF

    # Shared/Public RGD (all authenticated users should see)
    # Note: Public visibility = catalog: true with NO project label
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: shared-logging
  labels:
    app.kubernetes.io/name: shared-logging
    # No knodex.io/project label = PUBLIC (visible to all authenticated users)
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: monitoring
    knodex.io/description: "Shared logging for all teams"
spec:
  schema:
    apiVersion: v1alpha1
    kind: SharedLogging
    spec:
      retention: 'string | default="7d"'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          retention: '\${schema.spec.retention}'
EOF

    # Platform-wide RGD (no org label = global admin only)
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: platform-networking
  labels:
    app.kubernetes.io/name: platform-networking
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: networking
    knodex.io/description: "Platform-wide networking configuration"
spec:
  schema:
    apiVersion: v1alpha1
    kind: PlatformNetworking
    spec:
      cidr: 'string | default="10.0.0.0/16"'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          cidr: '\${schema.spec.cidr}'
EOF

    # Simple App - Public RGD used by many E2E tests
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: simple-app
  labels:
    app.kubernetes.io/name: simple-app
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: application
    knodex.io/description: "Simple application template for testing"
spec:
  schema:
    apiVersion: v1alpha1
    kind: SimpleApp
    spec:
      appName: 'string | default="my-app"'
      replicas: 'integer | default=1'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          appName: '\${schema.spec.appName}'
          replicas: '\${schema.spec.replicas}'
EOF

    # Webapp with Features - Public RGD for advanced tests
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp-with-features
  labels:
    app.kubernetes.io/name: webapp-with-features
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: application
    knodex.io/description: "Web application with advanced features"
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebappWithFeatures
    spec:
      name: 'string | default="webapp"'
      enableCache: 'boolean | default=false'
      enableMetrics: 'boolean | default=true'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          name: '\${schema.spec.name}'
          enableCache: '\${schema.spec.enableCache}'
          enableMetrics: '\${schema.spec.enableMetrics}'
EOF

    # Microservices Platform - Used in deploy form tests
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: microservices-platform
  labels:
    app.kubernetes.io/name: microservices-platform
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: platform
    knodex.io/description: "Microservices platform deployment template"
spec:
  schema:
    apiVersion: v1alpha1
    kind: MicroservicesPlatform
    spec:
      serviceName: 'string | default="service"'
      port: 'integer | default=8080'
      environment: 'string | default="development"'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          serviceName: '\${schema.spec.serviceName}'
          port: '\${schema.spec.port}'
          environment: '\${schema.spec.environment}'
EOF

    # Postgres Database - Used in catalog tests
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: postgres-database
  labels:
    app.kubernetes.io/name: postgres-database
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: database
    knodex.io/description: "PostgreSQL database instance"
spec:
  schema:
    apiVersion: v1alpha1
    kind: PostgresDatabase
    spec:
      version: 'string | default="15"'
      storage: 'string | default="10Gi"'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          version: '\${schema.spec.version}'
          storage: '\${schema.spec.storage}'
EOF

    # Redis Cache - Used in catalog tests
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: redis-cache
  labels:
    app.kubernetes.io/name: redis-cache
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: cache
    knodex.io/description: "Redis cache instance"
spec:
  schema:
    apiVersion: v1alpha1
    kind: RedisCache
    spec:
      maxMemory: 'string | default="256mb"'
      evictionPolicy: 'string | default="allkeys-lru"'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          maxMemory: '\${schema.spec.maxMemory}'
          evictionPolicy: '\${schema.spec.evictionPolicy}'
EOF

    # Nginx Ingress - Used in catalog tests
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: nginx-ingress
  labels:
    app.kubernetes.io/name: nginx-ingress
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: networking
    knodex.io/description: "Nginx ingress controller"
spec:
  schema:
    apiVersion: v1alpha1
    kind: NginxIngress
    spec:
      host: 'string | default="localhost"'
      tlsEnabled: 'boolean | default=false'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          host: '\${schema.spec.host}'
          tlsEnabled: '\${schema.spec.tlsEnabled}'
EOF

    # Azure AD Staging App - For OIDC namespace dropdown tests
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: azuread-staging-app
  labels:
    app.kubernetes.io/name: azuread-staging-app
    knodex.io/project: proj-azuread-staging
  annotations:
    knodex.io/catalog: "true"
    knodex.io/category: application
    knodex.io/description: "Azure AD staging application"
spec:
  schema:
    apiVersion: v1alpha1
    kind: AzureadStagingApp
    spec:
      appName: 'string | default="staging-app"'
      environment: 'string | default="staging"'
  resources:
    - id: configmap
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: '\${schema.metadata.name}-config'
        data:
          appName: '\${schema.spec.appName}'
          environment: '\${schema.spec.environment}'
EOF

    log_info "Test RGDs created with project labels"
}

# Create test instances for RBAC testing
# Note: Instances are created using the CRDs generated by KRO from RGDs
# We need to wait for the CRDs to be created before creating instances
create_test_instances() {
    log_step "Waiting for RGD CRDs to be generated..."

    # Wait for AlphaDatabase CRD to be created
    local max_wait=60
    local wait_time=0
    while ! kubectl get crd alphadatabases.kro.run >/dev/null 2>&1; do
        if [ $wait_time -ge $max_wait ]; then
            log_warn "Timed out waiting for AlphaDatabase CRD. Skipping instance creation."
            log_info "Run 'kubectl get rgd -A' to check RGD status"
            return 0
        fi
        log_info "Waiting for CRDs... ($wait_time/$max_wait seconds)"
        sleep 5
        wait_time=$((wait_time + 5))
    done

    log_step "Creating test instances..."

    # Instance in Alpha Team namespace
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: AlphaDatabase
metadata:
  name: alpha-db-instance-1
  namespace: ns-alpha-team
spec:
  size: "medium"
EOF

    # Another instance in Alpha Team namespace
    cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: AlphaDatabase
metadata:
  name: alpha-db-instance-2
  namespace: ns-alpha-team
spec:
  size: "small"
EOF

    # Wait for BetaCache CRD
    wait_time=0
    while ! kubectl get crd betacaches.kro.run >/dev/null 2>&1; do
        if [ $wait_time -ge $max_wait ]; then
            log_warn "Timed out waiting for BetaCache CRD."
            break
        fi
        sleep 5
        wait_time=$((wait_time + 5))
    done

    if kubectl get crd betacaches.kro.run >/dev/null 2>&1; then
        # Instance in Beta Team namespace
        cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: BetaCache
metadata:
  name: beta-cache-instance-1
  namespace: ns-beta-team
spec:
  replicas: 3
EOF
    fi

    # Wait for SimpleApp CRD
    wait_time=0
    while ! kubectl get crd simpleapps.kro.run >/dev/null 2>&1; do
        if [ $wait_time -ge $max_wait ]; then
            log_warn "Timed out waiting for SimpleApp CRD."
            break
        fi
        sleep 5
        wait_time=$((wait_time + 5))
    done

    if kubectl get crd simpleapps.kro.run >/dev/null 2>&1; then
        # Simple app instance for global tests
        cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: SimpleApp
metadata:
  name: test-simple-app
  namespace: ns-shared
spec:
  appName: "test-app"
  replicas: 2
EOF
    fi

    # Wait for RedisCache CRD
    wait_time=0
    while ! kubectl get crd rediscaches.kro.run >/dev/null 2>&1; do
        if [ $wait_time -ge $max_wait ]; then
            log_warn "Timed out waiting for RedisCache CRD."
            break
        fi
        sleep 5
        wait_time=$((wait_time + 5))
    done

    if kubectl get crd rediscaches.kro.run >/dev/null 2>&1; then
        # staging-cache instance for E2E tests
        cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: RedisCache
metadata:
  name: staging-cache
  namespace: staging-app
spec:
  maxMemory: "512mb"
  evictionPolicy: "allkeys-lru"
EOF
    fi

    # Wait for NginxIngress CRD
    wait_time=0
    while ! kubectl get crd nginxingresses.kro.run >/dev/null 2>&1; do
        if [ $wait_time -ge $max_wait ]; then
            log_warn "Timed out waiting for NginxIngress CRD."
            break
        fi
        sleep 5
        wait_time=$((wait_time + 5))
    done

    if kubectl get crd nginxingresses.kro.run >/dev/null 2>&1; then
        # dev-ingress instance for E2E tests
        cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: NginxIngress
metadata:
  name: dev-ingress
  namespace: ns-shared
spec:
  host: "dev.example.com"
  tlsEnabled: false
EOF
    fi

    # Wait for PostgresDatabase CRD
    wait_time=0
    while ! kubectl get crd postgresdatabases.kro.run >/dev/null 2>&1; do
        if [ $wait_time -ge $max_wait ]; then
            log_warn "Timed out waiting for PostgresDatabase CRD."
            break
        fi
        sleep 5
        wait_time=$((wait_time + 5))
    done

    if kubectl get crd postgresdatabases.kro.run >/dev/null 2>&1; then
        # postgres-database instance for E2E tests
        cat <<EOF | kubectl apply -f -
apiVersion: kro.run/v1alpha1
kind: PostgresDatabase
metadata:
  name: postgres-database
  namespace: ns-shared
spec:
  version: "15"
  storage: "20Gi"
EOF
    fi

    log_info "Test instances created"
}

# Verify test data setup
verify_setup() {
    log_step "Verifying test data setup..."

    local errors=0

    # Note: User CRD removed - users authenticated via JWT tokens
    log_info "User CRD check skipped (Note: removed)"

    # Check projects (now 5 projects)
    local proj_count=$(kubectl get projects.knodex.io --no-headers 2>/dev/null | wc -l)
    if [ "$proj_count" -ge 5 ]; then
        log_info "Projects created: $proj_count"
    else
        log_error "Expected at least 5 projects, found $proj_count"
        errors=$((errors + 1))
    fi

    # Check RGDs (now 11 RGDs)
    local rgd_count=$(kubectl get resourcegraphdefinitions.kro.run --no-headers 2>/dev/null | wc -l)
    if [ "$rgd_count" -ge 11 ]; then
        log_info "RGDs created: $rgd_count"
    else
        log_warn "Expected at least 11 RGDs, found $rgd_count (some may still be processing)"
    fi

    # Check namespaces (now 7 namespaces)
    for ns in ns-alpha-team ns-beta-team ns-shared staging-app staging-db knodex-test platform-services; do
        if kubectl get namespace "$ns" >/dev/null 2>&1; then
            log_info "Namespace exists: $ns"
        else
            log_error "Namespace missing: $ns"
            errors=$((errors + 1))
        fi
    done

    # Check tokens file
    if [ -f "$OUTPUT_DIR/test-tokens.json" ]; then
        log_info "Tokens file exists: $OUTPUT_DIR/test-tokens.json"
    else
        log_error "Tokens file missing"
        errors=$((errors + 1))
    fi

    if [ $errors -eq 0 ]; then
        log_info "All verification checks passed"
    else
        log_error "$errors verification check(s) failed"
        return 1
    fi
}

# Generate test config summary
generate_test_config() {
    log_step "Generating test configuration summary..."

    local config_file="$OUTPUT_DIR/test-config.md"

    cat > "$config_file" <<EOF
# E2E Test Configuration

Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

## Test Users

| User ID | Email | Role | Projects |
|---------|-------|------|----------|
| user-global-admin | admin@e2e-test.local | Global Admin | proj-alpha-team, proj-beta-team, proj-shared |
| user-alpha-admin | alpha-admin@e2e-test.local | Platform Admin | proj-alpha-team |
| user-alpha-developer | alpha-dev@e2e-test.local | Developer | proj-alpha-team |
| user-alpha-viewer | alpha-viewer@e2e-test.local | Viewer | proj-alpha-team |
| user-beta-admin | beta-admin@e2e-test.local | Platform Admin | proj-beta-team |
| user-beta-developer | beta-dev@e2e-test.local | Developer | proj-beta-team |
| user-no-projects | no-projects@e2e-test.local | No Role | (none) |

## Test Projects (ArgoCD-compatible schema)

| Project ID | Description | Allowed Namespaces | Roles |
|------------|-------------|-------------------|-------|
| proj-alpha-team | Alpha Team project | ns-alpha-team | admin, developer, viewer |
| proj-beta-team | Beta Team project | ns-beta-team | admin, developer |
| proj-shared | Shared Resources | ns-shared | admin |

### Project RBAC Policy Format

Policies use Casbin format: \`p, subject, resource, action, object, effect\`

- **subject**: \`proj:<project>:<role>\` (e.g., \`proj:proj-alpha-team:developer\`)
- **resource**: \`project\`, \`instance\`, \`rgd\`
- **action**: \`view\`, \`deploy\`, \`delete\`, \`*\` (all)
- **object**: resource scope (e.g., \`proj-alpha-team/*\` for all instances in project)
- **effect**: \`allow\` or \`deny\`

## Test RGDs

RGDs are cluster-scoped resources. Access control is managed via catalog annotation and project labels.
Note: Simplified visibility model:
- catalog: true (no project label) = PUBLIC (all authenticated users)
- catalog: true + project label = RESTRICTED (project members only)
- no catalog annotation = NOT in catalog (invisible to everyone)

| RGD Name | catalog Annotation | Project Label | Visibility |
|----------|-------------------|---------------|------------|
| alpha-database | true | proj-alpha-team | Alpha Team only |
| beta-cache | true | proj-beta-team | Beta Team only |
| shared-logging | true | (none) | PUBLIC - All authenticated users |
| platform-networking | true | (none) | PUBLIC - All authenticated users |

## Test Instances

| Instance Name | Namespace | RGD | Expected Visibility |
|---------------|-----------|-----|---------------------|
| alpha-db-instance-1 | ns-alpha-team | alpha-database | Alpha Team members |
| alpha-db-instance-2 | ns-alpha-team | alpha-database | Alpha Team members |
| beta-cache-instance-1 | ns-beta-team | beta-cache | Beta Team members |

## RBAC Test Matrix

### Catalog Visibility (AC-1 to AC-3)

Note: Based on catalog annotation and project labels.
- catalog: true (no project label) = PUBLIC (all authenticated users)
- catalog: true + project label = RESTRICTED (project members only)

| User | alpha-database | beta-cache | shared-logging | platform-networking |
|------|----------------|------------|----------------|---------------------|
| Global Admin | Yes (all) | Yes (all) | Yes (all) | Yes (all) |
| Alpha Admin | Yes (proj match) | No | Yes (public) | No |
| Alpha Viewer | Yes (proj match) | No | Yes (public) | No |
| Beta Admin | No | Yes (proj match) | Yes (public) | No |

### Deploy Button Visibility (AC-4, AC-5)

| User | Can Deploy |
|------|------------|
| Global Admin | Yes |
| Alpha Admin | Yes |
| Alpha Developer | Yes |
| Alpha Viewer | **No** |

### Instance Visibility (AC-6 to AC-9)

| User | alpha-db-* | beta-cache-* |
|------|------------|--------------|
| Global Admin | Yes | Yes |
| Alpha Admin | Yes | No |
| Alpha Developer | Yes | No |
| Beta Admin | No | Yes |

### Project Management (AC-10 to AC-19)

| Action | Global Admin | Project Admin | Developer | Viewer |
|--------|--------------|---------------|-----------|--------|
| Create Project | Yes | No | No | No |
| Delete Project | Yes | No | No | No |
| Edit Project | Yes | Yes (own project) | No | No |
| View Project | Yes | Yes | Yes | Yes |
| Manage Roles | Yes | Yes (own project) | No | No |
| Manage Destinations | Yes | Yes (own project) | No | No |

## Token Usage

Tokens are stored in:
- \`test-results/e2e/test-tokens.json\` - JSON format with user details
- \`test-results/e2e/test-tokens.env\` - Shell environment variables

To inject a token into localStorage for testing:
\`\`\`javascript
// In browser console or via Chrome DevTools MCP
localStorage.setItem('auth_token', '<JWT_TOKEN>');
localStorage.setItem('auth_user', JSON.stringify({
  id: '<USER_ID>',
  email: '<EMAIL>',
  displayName: '<DISPLAY_NAME>',
  isGlobalAdmin: <true/false>,
  projects: [<PROJECT_IDS>],
  defaultProject: '<DEFAULT_PROJECT>'
}));
window.location.reload();
\`\`\`
EOF

    log_info "Test configuration written to: $config_file"
}

# Cleanup function
cleanup_test_data() {
    log_step "Cleaning up test data..."

    # Note: User CRD removed - no user cleanup needed
    log_info "User CRD cleanup skipped (Note: removed)"

    # Delete test projects
    for proj in proj-alpha-team proj-beta-team proj-shared proj-azuread-staging proj-platform; do
        kubectl delete projects.knodex.io "$proj" --ignore-not-found
    done

    # Delete test RGDs
    for rgd in alpha-database beta-cache shared-logging platform-networking simple-app webapp-with-features microservices-platform postgres-database redis-cache nginx-ingress azuread-staging-app; do
        kubectl delete resourcegraphdefinitions.kro.run "$rgd" --ignore-not-found
    done

    # Delete test namespaces
    for ns in ns-alpha-team ns-beta-team ns-shared staging-app staging-db knodex-test platform-services; do
        kubectl delete namespace "$ns" --ignore-not-found
    done

    log_info "Test data cleaned up"
}

# Main function
main() {
    local action=${1:-setup}

    case "$action" in
        setup)
            check_prerequisites
            get_jwt_secret_from_cluster
            create_test_projects
            create_test_users
            create_test_rgds
            create_test_instances
            generate_all_tokens
            verify_setup
            generate_test_config

            echo ""
            echo "=========================================="
            echo "E2E Test Data Setup Complete"
            echo "=========================================="
            echo ""
            log_info "Tokens: $OUTPUT_DIR/test-tokens.json"
            log_info "Config: $OUTPUT_DIR/test-config.md"
            echo ""
            log_info "To run E2E tests with Chrome DevTools MCP:"
            log_info "1. Source tokens: source $OUTPUT_DIR/test-tokens.env"
            log_info "2. Open Chrome with debugging enabled"
            log_info "3. Use the e2e-test-writer agent to execute tests"
            ;;
        cleanup)
            check_prerequisites
            cleanup_test_data
            rm -f "$OUTPUT_DIR/test-tokens.json" "$OUTPUT_DIR/test-tokens.env"
            log_info "Cleanup complete"
            ;;
        tokens)
            check_prerequisites
            get_jwt_secret_from_cluster
            generate_all_tokens
            log_info "Tokens regenerated"
            ;;
        verify)
            check_prerequisites
            verify_setup
            ;;
        *)
            echo "Usage: $0 [setup|cleanup|tokens|verify]"
            echo ""
            echo "Commands:"
            echo "  setup   - Create test users, projects, and generate tokens (default)"
            echo "  cleanup - Remove all test data"
            echo "  tokens  - Regenerate JWT tokens only"
            echo "  verify  - Verify test data exists"
            exit 1
            ;;
    esac
}

main "$@"
