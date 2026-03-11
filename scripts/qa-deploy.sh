#!/bin/bash
# Copyright 2026 Knodex Authors
# SPDX-License-Identifier: AGPL-3.0-only
# qa-deploy.sh - Deploy application to Kubernetes cluster
# Cluster-agnostic: works with Kind, AKS, GKE, EKS
# Prerequisites (KRO/CRDs) are handled by ensure-prereqs.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Fixed namespace for simplified deployment
NAMESPACE="knodex"

# Enterprise build configuration (set via environment variable)
ENTERPRISE_BUILD="${ENTERPRISE_BUILD:-false}"
BUILD_TAGS=""
if [ "${ENTERPRISE_BUILD}" = "true" ]; then
    BUILD_TAGS="enterprise"
fi

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

# Check cluster connection
check_cluster() {
    log_info "Checking cluster connection..."

    if ! kubectl cluster-info &>/dev/null; then
        log_error "No cluster connection."
        echo ""
        echo "  For local development:"
        echo "    make cluster-up"
        echo ""
        echo "  For external clusters (AKS/GKE/EKS):"
        echo "    kubectl config use-context <your-cluster-context>"
        echo ""
        exit 1
    fi

    local context=$(kubectl config current-context 2>/dev/null || echo "unknown")
    log_success "Connected to: ${context}"
}

# Detect if running in Kind cluster
is_kind_cluster() {
    local context=$(kubectl config current-context 2>/dev/null || echo "")
    [[ "${context}" == kind-* ]]
}

# Get Kind cluster name from context
get_kind_cluster_name() {
    local context=$(kubectl config current-context 2>/dev/null || echo "")
    echo "${context#kind-}"
}

# Build Docker images
build_images() {
    log_info "Building Docker images..."

    cd "${PROJECT_DIR}"

    # Determine build type and web mode
    WEB_BUILD_MODE=""
    if [ "${ENTERPRISE_BUILD}" = "true" ]; then
        log_info "Building ENTERPRISE edition..."
        WEB_BUILD_MODE="enterprise"
    else
        log_info "Building OSS edition..."
    fi

    # Detect host platform for local builds (e.g., arm64 on Apple Silicon)
    local host_platform="linux/$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')"
    log_info "Building for platform: ${host_platform}"

    log_info "Building unified image (web embedded in Go binary)..."
    docker build \
        --platform "${host_platform}" \
        --build-arg BUILD_MODE="${WEB_BUILD_MODE}" \
        --build-arg BUILD_TAGS="${BUILD_TAGS}" \
        -t knodex-server:local \
        -f Dockerfile .
    log_success "Unified image built"

    # Build mock OIDC server for E2E testing
    log_info "Building mock OIDC server image..."
    docker build --platform "${host_platform}" -t mock-oidc:local -f server/test/mocks/oidc/Dockerfile server/
    log_success "Mock OIDC server image built"

    # Load into Kind if applicable
    if is_kind_cluster; then
        local cluster_name=$(get_kind_cluster_name)
        log_info "Loading images into Kind cluster: ${cluster_name}..."
        kind load docker-image knodex-server:local --name "${cluster_name}"
        kind load docker-image mock-oidc:local --name "${cluster_name}"
        log_success "Images loaded into Kind"
    else
        log_info "External cluster detected - images must be pushed to registry"
    fi
}

# Create secrets from .env file
create_secrets() {
    log_info "Creating secrets from .env file..."

    # Check if .env file exists
    if [ ! -f "${PROJECT_DIR}/.env" ]; then
        log_warn ".env file not found. Skipping secret creation."
        log_info "To create secrets, copy .env.example to .env and add your values"
        return 0
    fi

    # Source .env file to get variables
    set -a
    source "${PROJECT_DIR}/.env"
    set +a

    # Create github-token secret if GITHUB_TOKEN is set
    if [ -n "${GITHUB_TOKEN}" ]; then
        log_info "Creating github-token secret..."
        kubectl create secret generic github-token \
            --from-literal=token="${GITHUB_TOKEN}" \
            --namespace="${NAMESPACE}" \
            --dry-run=client -o yaml | kubectl apply -f - > /dev/null
        log_success "github-token secret created"
    fi

}

# Deploy mock OIDC server for E2E testing
deploy_mock_oidc() {
    log_info "Deploying mock OIDC server..."

    cd "${PROJECT_DIR}"

    # Check if manifests exist
    if [ ! -d "deploy/test/mock-oidc" ]; then
        log_warn "Mock OIDC manifests not found, skipping..."
        return 0
    fi

    # Apply mock OIDC manifests
    kubectl apply -k deploy/test/mock-oidc/ -n "${NAMESPACE}"

    # Wait for deployment
    log_info "Waiting for mock OIDC server..."
    kubectl wait --for=condition=available --timeout=60s deployment/mock-oidc -n "${NAMESPACE}" || {
        log_warn "Mock OIDC server not ready, E2E auth tests may fail"
        return 0
    }

    log_success "Mock OIDC server deployed"
}

# Deploy application
deploy_app() {
    log_info "Deploying application to namespace ${NAMESPACE}..."

    cd "${PROJECT_DIR}"

    # Create namespace if it doesn't exist
    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

    # Create secrets
    create_secrets

    # Apply base manifests with dev overlay
    log_info "Applying Kustomize manifests..."
    kubectl apply -k scripts/overlays/dev/

    # Wait for Redis first (server depends on it)
    log_info "Waiting for Redis..."
    kubectl wait --for=condition=available --timeout=60s deployment/knodex-redis -n "${NAMESPACE}" || {
        log_error "Redis deployment failed"
        kubectl describe deployment knodex-redis -n "${NAMESPACE}"
        exit 1
    }
    kubectl wait --for=condition=ready --timeout=60s pod -l app.kubernetes.io/component=redis -n "${NAMESPACE}"
    log_success "Redis ready"

    # Force restart deployment to pick up new image
    log_info "Restarting deployment..."
    kubectl rollout restart deployment/knodex-server -n "${NAMESPACE}"

    # Wait for server (serves both API and web)
    log_info "Waiting for server..."
    kubectl wait --for=condition=available --timeout=120s deployment/knodex-server -n "${NAMESPACE}" || {
        log_error "Backend deployment failed"
        kubectl describe deployment knodex-server -n "${NAMESPACE}"
        kubectl logs -n "${NAMESPACE}" -l app.kubernetes.io/component=server --tail=50
        exit 1
    }
    log_success "Server ready (serves API + embedded web)"
}

# Deploy example RGDs
deploy_example_rgds() {
    log_info "Deploying example RGDs..."

    cd "${PROJECT_DIR}"

    if [ ! -d "deploy/examples/rgds" ]; then
        log_warn "Example RGD directory not found, skipping..."
        return 0
    fi

    kubectl apply -f deploy/examples/rgds/
    sleep 2

    if [ -d "deploy/examples/instances" ]; then
        log_info "Applying example instances..."
        kubectl apply -f deploy/examples/instances/ || log_warn "Failed to apply some instances"
    fi

    log_success "Example RGDs deployed"
}

# Deploy example Projects (RBAC)
deploy_example_projects() {
    log_info "Deploying example Projects..."

    cd "${PROJECT_DIR}"

    if [ ! -d "deploy/examples/projects" ]; then
        log_warn "Example projects directory not found, skipping..."
        return 0
    fi

    kubectl apply -f deploy/examples/projects/
    log_success "Example Projects deployed"
}

# Deploy example Gatekeeper resources (Enterprise only)
deploy_example_gatekeeper() {
    if [ "${ENTERPRISE_BUILD}" != "true" ]; then
        log_info "Skipping Gatekeeper examples (not enterprise build)"
        return 0
    fi

    log_info "Deploying example Gatekeeper resources..."

    cd "${PROJECT_DIR}"

    if [ ! -d "deploy/examples/gatekeeper" ]; then
        log_warn "Example Gatekeeper directory not found, skipping..."
        return 0
    fi

    # Check if Gatekeeper CRDs are installed
    if ! kubectl get crd constrainttemplates.templates.gatekeeper.sh &>/dev/null; then
        log_warn "Gatekeeper CRDs not installed, skipping example deployment..."
        log_info "To install Gatekeeper, run: helm install gatekeeper gatekeeper/gatekeeper -n gatekeeper-system --create-namespace"
        return 0
    fi

    # First apply ConstraintTemplates and wait for them
    kubectl apply -f deploy/examples/gatekeeper/constraint-templates.yaml

    # Wait for ConstraintTemplate CRDs to be created
    log_info "Waiting for Gatekeeper CRDs to be ready..."
    sleep 5

    # Now apply Constraints
    kubectl apply -f deploy/examples/gatekeeper/constraints.yaml || {
        log_warn "Some constraints may have failed - CRDs might not be ready yet"
        sleep 5
        kubectl apply -f deploy/examples/gatekeeper/constraints.yaml || log_warn "Constraint deployment had errors"
    }

    log_success "Example Gatekeeper resources deployed"
}

# Verify test fixtures are deployed (RGDs and Projects)
verify_fixtures() {
    log_info "Verifying test fixtures..."

    # Check RGDs
    local rgd_count=$(kubectl get resourcegraphdefinitions --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [ "${rgd_count}" -lt 3 ]; then
        log_warn "Expected at least 3 RGDs, found ${rgd_count}."
        log_warn "Tests may fail. Run: kubectl apply -f deploy/examples/rgds/"
    else
        log_success "Found ${rgd_count} RGDs"
    fi

    # Check Projects
    local project_count=$(kubectl get projects.knodex.io --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [ "${project_count}" -lt 2 ]; then
        log_warn "Expected at least 2 Projects, found ${project_count}."
        log_warn "RBAC tests may fail. Run: kubectl apply -f deploy/examples/projects/"
    else
        log_success "Found ${project_count} Projects"
    fi

    # Check Gatekeeper (Enterprise only)
    if [ "${ENTERPRISE_BUILD}" = "true" ]; then
        local template_count=$(kubectl get constrainttemplates --no-headers 2>/dev/null | wc -l | tr -d ' ')
        if [ "${template_count}" -lt 1 ]; then
            log_warn "Expected at least 1 ConstraintTemplate, found ${template_count}."
            log_warn "Enterprise compliance tests may fail."
        else
            log_success "Found ${template_count} ConstraintTemplates"
        fi

        local constraint_count=$(kubectl get constraints --no-headers 2>/dev/null | wc -l | tr -d ' ')
        if [ "${constraint_count}" -lt 1 ]; then
            log_warn "Expected at least 1 Constraint, found ${constraint_count}."
        else
            log_success "Found ${constraint_count} Constraints"
        fi
    fi
}

# Verify health endpoints
verify_health() {
    log_info "Verifying health endpoints..."

    # Give services a moment to be accessible
    sleep 3

    # Get server service port
    local server_port=$(kubectl get svc knodex-server -n "${NAMESPACE}" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "8080")

    # For Kind, use NodePort; for others, use port-forward
    if is_kind_cluster; then
        local url="http://localhost:${server_port}"
    else
        # Start port-forward in background
        kubectl port-forward -n "${NAMESPACE}" svc/knodex-server 8080:8080 &
        local pf_pid=$!
        sleep 2
        local url="http://localhost:8080"
    fi

    # Test liveness
    log_info "Testing /healthz..."
    local healthz=$(curl -s "${url}/healthz" 2>/dev/null || echo '{"status":"error"}')
    if echo "$healthz" | grep -q '"status":"healthy"'; then
        log_success "/healthz returns healthy"
    else
        log_warn "/healthz check: $healthz"
    fi

    # Test readiness
    log_info "Testing /readyz..."
    local readyz=$(curl -s "${url}/readyz" 2>/dev/null || echo '{"status":"error"}')
    if echo "$readyz" | grep -q '"status":"healthy"'; then
        log_success "/readyz returns healthy"
    else
        log_warn "/readyz check: $readyz"
    fi

    # Cleanup port-forward if started
    [ -n "${pf_pid:-}" ] && kill "${pf_pid}" 2>/dev/null || true
}

# Start port-forward for server (serves both API and web)
start_port_forward() {
    log_info "Starting port-forward for server..."

    # Kill any existing port-forwards on these ports
    pkill -f "kubectl port-forward.*knodex-server" 2>/dev/null || true
    sleep 1

    # Start port-forward in background
    kubectl port-forward -n "${NAMESPACE}" svc/knodex-server 8080:8080 &
    local server_pid=$!

    sleep 2

    # Verify it's running
    if kill -0 "${server_pid}" 2>/dev/null; then
        log_success "Port-forward started"
        log_info "Application: http://localhost:8080"
        echo ""
        log_info "Port-forward PID: ${server_pid}"
        log_info "To stop: pkill -f 'kubectl port-forward.*knodex'"
    else
        log_error "Failed to start port-forward"
        exit 1
    fi
}

# Show deployment status
show_status() {
    log_info "Deployment Status:"
    echo ""
    log_info "Namespace: ${NAMESPACE}"
    echo ""
    kubectl get pods -n "${NAMESPACE}" -o wide
    echo ""
    kubectl get services -n "${NAMESPACE}"
    echo ""

    log_info "RGDs:"
    kubectl get resourcegraphdefinitions 2>/dev/null || echo "  No RGDs deployed"
    echo ""

    log_info "Projects (RBAC):"
    kubectl get projects.knodex.io 2>/dev/null || echo "  No projects deployed"
    echo ""

    # Show access URLs
    if is_kind_cluster; then
        local server_port=$(kubectl get svc knodex-server -n "${NAMESPACE}" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "8080")
        log_info "Access URL (Kind):"
        echo "  Application: http://localhost:${server_port}"
    else
        log_info "Access URL (use port-forward):"
        echo "  kubectl port-forward -n ${NAMESPACE} svc/knodex-server 8080:8080"
    fi
    echo ""
}

# Main
case "${1:-deploy}" in
    deploy)
        check_cluster
        build_images
        deploy_app
        deploy_mock_oidc
        deploy_example_rgds
        deploy_example_projects
        deploy_example_gatekeeper
        verify_fixtures
        show_status
        ;;
    deploy-only)
        # Skip image builds (used in CI where images are pre-built)
        check_cluster
        deploy_app
        deploy_mock_oidc
        deploy_example_rgds
        deploy_example_projects
        deploy_example_gatekeeper
        verify_fixtures
        show_status
        ;;
    verify)
        verify_health
        verify_fixtures
        ;;
    status)
        show_status
        ;;
    port-forward)
        start_port_forward
        ;;
    *)
        echo "Usage: $0 {deploy|deploy-only|verify|status|port-forward}"
        echo ""
        echo "Commands:"
        echo "  deploy       - Build and deploy application"
        echo "  deploy-only  - Deploy without building images (CI: images pre-built)"
        echo "  verify       - Verify health endpoints and fixtures"
        echo "  status       - Show deployment status"
        echo "  port-forward - Start kubectl port-forward for application (8080)"
        echo ""
        echo "Prerequisites:"
        echo "  make cluster-up    # Create Kind cluster (one-time)"
        echo ""
        echo "Examples:"
        echo "  ./scripts/qa-deploy.sh deploy"
        echo "  ./scripts/qa-deploy.sh status"
        echo "  ./scripts/qa-deploy.sh port-forward  # If NodePort doesn't work"
        exit 1
        ;;
esac
