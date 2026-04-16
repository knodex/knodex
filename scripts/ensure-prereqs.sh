#!/bin/bash
# Copyright 2026 Knodex Authors
# SPDX-License-Identifier: AGPL-3.0-only
# ensure-prereqs.sh - Auto-detect and install prerequisites for testing
# This script checks cluster connectivity and installs KRO/CRDs if missing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

KRO_VERSION="${KRO_VERSION:-0.9.1}"
KRO_NAMESPACE="kro-system"
GATEKEEPER_VERSION="3.17.1"
GATEKEEPER_NAMESPACE="gatekeeper-system"

# Enterprise build configuration (set via environment variable)
ENTERPRISE_BUILD="${ENTERPRISE_BUILD:-false}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

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

# Check kubectl connectivity
check_cluster_connection() {
    log_info "Checking cluster connection..."

    if ! kubectl cluster-info &>/dev/null; then
        log_error "No cluster connection. Run 'make cluster-up' or configure kubeconfig."
        echo ""
        echo "  To create a local Kind cluster:"
        echo "    make cluster-up"
        echo ""
        echo "  To use an external cluster (AKS/GKE/EKS):"
        echo "    kubectl config use-context <your-cluster-context>"
        echo ""
        exit 1
    fi

    local context=$(kubectl config current-context 2>/dev/null || echo "unknown")
    log_success "Connected to cluster: ${context}"
}

# Check/Install KRO
install_kro() {
    log_info "Checking KRO installation..."

    if kubectl get crd resourcegraphdefinitions.kro.run &>/dev/null; then
        # KRO is installed, check version
        if helm list -n "${KRO_NAMESPACE}" 2>/dev/null | grep -q "kro"; then
            local installed_version=$(helm list -n "${KRO_NAMESPACE}" -o json 2>/dev/null | grep -o '"chart":"kro-[^"]*"' | head -1 | sed 's/.*kro-v\{0,1\}\([^"]*\)".*/\1/')
            log_success "KRO v${installed_version:-unknown} already installed"
        else
            log_success "KRO CRD found (installed outside Helm)"
        fi
        return 0
    fi

    # KRO not installed, install it
    log_info "Installing KRO v${KRO_VERSION}..."

    if ! command -v helm &>/dev/null; then
        log_error "Helm is required to install KRO. Install from: https://helm.sh/docs/intro/install/"
        exit 1
    fi

    helm install kro oci://registry.k8s.io/kro/charts/kro \
        --namespace "${KRO_NAMESPACE}" \
        --create-namespace \
        --version "${KRO_VERSION}" \
        --set config.featureGates.InstanceConditionEvents=true \
        --wait --timeout 300s

    # Verify installation
    if kubectl get crd resourcegraphdefinitions.kro.run &>/dev/null; then
        log_success "KRO v${KRO_VERSION} installed successfully"
    else
        log_error "KRO installation failed - CRD not found"
        exit 1
    fi
}

# Check/Install knodex CRDs
install_crds() {
    log_info "Checking knodex CRDs..."

    if kubectl get crd projects.knodex.io &>/dev/null; then
        log_success "Knodex CRDs already installed"
        return 0
    fi

    # CRDs not installed, install them
    log_info "Installing knodex CRDs..."

    if [ ! -d "${PROJECT_DIR}/deploy/crds" ]; then
        log_error "CRDs directory not found at ${PROJECT_DIR}/deploy/crds"
        exit 1
    fi

    kubectl apply -k "${PROJECT_DIR}/deploy/crds/"

    # Wait for CRDs to be established
    log_info "Waiting for CRDs to be established..."
    kubectl wait --for=condition=established --timeout=30s crd/projects.knodex.io 2>/dev/null || {
        log_warn "Timeout waiting for Project CRD, but continuing..."
    }

    log_success "Knodex CRDs installed"
}

# Check/Install OPA Gatekeeper (for Enterprise builds)
install_gatekeeper() {
    if [ "${ENTERPRISE_BUILD}" != "true" ]; then
        log_info "Skipping Gatekeeper (not enterprise build)"
        return 0
    fi

    log_info "Checking OPA Gatekeeper installation..."

    if kubectl get crd constrainttemplates.templates.gatekeeper.sh &>/dev/null; then
        # Gatekeeper is installed
        if helm list -n "${GATEKEEPER_NAMESPACE}" 2>/dev/null | grep -q "gatekeeper"; then
            local installed_version=$(helm list -n "${GATEKEEPER_NAMESPACE}" -o json 2>/dev/null | grep -o '"app_version":"[^"]*"' | head -1 | sed 's/.*"\([^"]*\)"/\1/')
            log_success "OPA Gatekeeper v${installed_version:-unknown} already installed"
        else
            log_success "Gatekeeper CRDs found (installed outside Helm)"
        fi
        return 0
    fi

    # Gatekeeper not installed, install it
    log_info "Installing OPA Gatekeeper v${GATEKEEPER_VERSION}..."

    if ! command -v helm &>/dev/null; then
        log_error "Helm is required to install Gatekeeper. Install from: https://helm.sh/docs/intro/install/"
        exit 1
    fi

    # Add gatekeeper helm repo
    helm repo add gatekeeper https://open-policy-agent.github.io/gatekeeper/charts >/dev/null 2>&1 || true
    helm repo update >/dev/null 2>&1

    helm install gatekeeper gatekeeper/gatekeeper \
        --namespace "${GATEKEEPER_NAMESPACE}" \
        --create-namespace \
        --version "v${GATEKEEPER_VERSION}" \
        --set auditInterval=30 \
        --set constraintViolationsLimit=100 \
        --wait --timeout 180s

    # Wait for CRDs to be established
    log_info "Waiting for Gatekeeper CRDs..."
    kubectl wait --for=condition=established --timeout=60s crd/constrainttemplates.templates.gatekeeper.sh 2>/dev/null || {
        log_warn "Timeout waiting for ConstraintTemplate CRD"
    }

    # Verify installation
    if kubectl get crd constrainttemplates.templates.gatekeeper.sh &>/dev/null; then
        log_success "OPA Gatekeeper v${GATEKEEPER_VERSION} installed successfully"
    else
        log_error "Gatekeeper installation failed - CRD not found"
        exit 1
    fi
}

# Check/Install Mock OIDC (for E2E testing)
install_mock_oidc() {
    local namespace="${1:-knodex}"

    log_info "Checking Mock OIDC server..."

    # Create namespace if it doesn't exist
    kubectl create namespace "${namespace}" --dry-run=client -o yaml 2>/dev/null | kubectl apply -f - >/dev/null 2>&1 || true

    if kubectl get deployment mock-oidc -n "${namespace}" &>/dev/null; then
        log_success "Mock OIDC server already deployed in ${namespace}"
        return 0
    fi

    # Mock OIDC not deployed, deploy it
    log_info "Deploying Mock OIDC server to ${namespace}..."

    # Build and load mock OIDC image if running in Kind
    local context=$(kubectl config current-context 2>/dev/null || echo "")
    if [[ "${context}" == kind-* ]]; then
        local cluster_name="${context#kind-}"

        # Check if image exists in Kind
        if ! docker exec "${cluster_name}-control-plane" crictl images 2>/dev/null | grep -q "mock-oidc:local"; then
            log_info "Building Mock OIDC image..."
            local host_platform="linux/$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')"
            docker build --platform "${host_platform}" -t mock-oidc:local -f "${PROJECT_DIR}/server/test/mocks/oidc/Dockerfile" "${PROJECT_DIR}/server/" >/dev/null 2>&1 || {
                log_warn "Failed to build mock-oidc image, deployment may fail"
            }

            log_info "Loading Mock OIDC image into Kind..."
            kind load docker-image mock-oidc:local --name "${cluster_name}" >/dev/null 2>&1 || {
                log_warn "Failed to load mock-oidc image into Kind"
            }
        fi
    fi

    # Apply mock OIDC manifests
    if [ -d "${PROJECT_DIR}/deploy/test/mock-oidc" ]; then
        kubectl apply -k "${PROJECT_DIR}/deploy/test/mock-oidc/" -n "${namespace}" >/dev/null 2>&1 || {
            log_warn "Failed to deploy mock-oidc, E2E auth tests may fail"
            return 0
        }

        # Wait for deployment
        kubectl wait --for=condition=available --timeout=60s deployment/mock-oidc -n "${namespace}" 2>/dev/null || {
            log_warn "Mock OIDC deployment not ready yet"
        }

        log_success "Mock OIDC server deployed"
    else
        log_warn "Mock OIDC manifests not found, skipping"
    fi
}

# Main
main() {
    echo ""
    echo "============================================"
    echo "  Checking cluster prerequisites..."
    echo "============================================"
    echo ""

    check_cluster_connection
    install_kro
    install_crds
    install_gatekeeper

    # Only install mock OIDC if explicitly requested or namespace provided
    if [ -n "${1}" ]; then
        install_mock_oidc "${1}"
    fi

    echo ""
    log_success "All prerequisites ready!"
    echo ""
}

main "$@"
