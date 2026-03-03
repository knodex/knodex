#!/bin/bash
set -e

# Compliance E2E Test Setup Script
# This script prepares the cluster with OPA Gatekeeper and test fixtures
# for comprehensive compliance E2E testing.
#
# Usage:
#   ./scripts/compliance-test-setup.sh [--install-gatekeeper] [--fixtures-only] [--cleanup]
#
# Options:
#   --install-gatekeeper  Install OPA Gatekeeper if not present
#   --fixtures-only       Only apply test fixtures (skip Gatekeeper check)
#   --cleanup             Remove test fixtures and cleanup

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/fixtures/compliance"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Gatekeeper version to install
GATEKEEPER_VERSION="${GATEKEEPER_VERSION:-v3.14.0}"

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

# Check prerequisites
check_prerequisites() {
    log_step "Checking prerequisites..."

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is required but not installed."
        exit 1
    fi

    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Is it running?"
        log_info "Run: make qa-deploy"
        exit 1
    fi

    log_info "Prerequisites satisfied."
}

# Check if Gatekeeper is installed
check_gatekeeper_installed() {
    if kubectl get namespace gatekeeper-system &> /dev/null 2>&1; then
        if kubectl get pods -n gatekeeper-system -l control-plane=controller-manager --no-headers 2>/dev/null | grep -q Running; then
            return 0
        fi
    fi
    return 1
}

# Install OPA Gatekeeper
install_gatekeeper() {
    log_step "Installing OPA Gatekeeper ${GATEKEEPER_VERSION}..."

    # Apply Gatekeeper manifests from official release
    kubectl apply -f "https://raw.githubusercontent.com/open-policy-agent/gatekeeper/${GATEKEEPER_VERSION}/deploy/gatekeeper.yaml"

    log_info "Waiting for Gatekeeper to be ready..."

    # Wait for controller-manager to be ready
    kubectl wait --for=condition=ready pod \
        -l control-plane=controller-manager \
        -n gatekeeper-system \
        --timeout=120s

    # Wait for audit controller to be ready
    kubectl wait --for=condition=ready pod \
        -l control-plane=audit-controller \
        -n gatekeeper-system \
        --timeout=120s

    log_info "OPA Gatekeeper installed successfully."
}

# Apply test fixtures
apply_fixtures() {
    log_step "Applying compliance test fixtures..."

    if [ ! -f "$FIXTURES_DIR/gatekeeper-test-fixtures.yaml" ]; then
        log_error "Test fixtures not found at $FIXTURES_DIR/gatekeeper-test-fixtures.yaml"
        exit 1
    fi

    # Apply ConstraintTemplates first
    log_info "Applying ConstraintTemplates..."
    kubectl apply -f "$FIXTURES_DIR/gatekeeper-test-fixtures.yaml" \
        --selector='app.kubernetes.io/component=constraint-template' 2>/dev/null || true

    # Wait for CRDs to be established
    log_info "Waiting for constraint CRDs to be established..."
    sleep 5

    # Check if K8sRequiredLabels CRD exists
    for i in {1..10}; do
        if kubectl get crd k8srequiredlabels.constraints.gatekeeper.sh &> /dev/null; then
            log_info "Constraint CRDs are ready."
            break
        fi
        log_info "Waiting for constraint CRDs... (attempt $i/10)"
        sleep 3
    done

    # Apply Constraints
    log_info "Applying Constraints..."
    kubectl apply -f "$FIXTURES_DIR/gatekeeper-test-fixtures.yaml" \
        --selector='app.kubernetes.io/component=constraint' 2>/dev/null || true

    # Apply test namespace and pods
    log_info "Applying test namespace and violation-generating resources..."
    kubectl apply -f "$FIXTURES_DIR/gatekeeper-test-fixtures.yaml"

    log_info "Test fixtures applied successfully."
}

# Wait for violations to be generated
wait_for_violations() {
    log_step "Waiting for Gatekeeper audit to detect violations..."

    # Gatekeeper audit runs periodically (default 60s)
    # Force an audit sync by restarting the audit controller
    log_info "Triggering audit sync..."
    kubectl rollout restart deployment/gatekeeper-audit -n gatekeeper-system 2>/dev/null || true

    # Wait for audit pod to be ready again
    sleep 10
    kubectl wait --for=condition=ready pod \
        -l control-plane=audit-controller \
        -n gatekeeper-system \
        --timeout=60s 2>/dev/null || true

    # Wait for violations to appear in constraint status
    for i in {1..12}; do
        VIOLATION_COUNT=$(kubectl get k8srequiredlabels pod-must-have-labels -o jsonpath='{.status.totalViolations}' 2>/dev/null || echo "0")
        if [ "$VIOLATION_COUNT" != "" ] && [ "$VIOLATION_COUNT" != "0" ]; then
            log_info "Violations detected: $VIOLATION_COUNT"
            return 0
        fi
        log_info "Waiting for violations to be detected... (attempt $i/12)"
        sleep 10
    done

    log_warn "Violations may not be fully populated yet. The audit controller runs periodically."
    log_info "You can check manually with: kubectl get k8srequiredlabels -o yaml"
}

# Cleanup test fixtures
cleanup() {
    log_step "Cleaning up compliance test fixtures..."

    if [ -f "$FIXTURES_DIR/gatekeeper-test-fixtures.yaml" ]; then
        kubectl delete -f "$FIXTURES_DIR/gatekeeper-test-fixtures.yaml" --ignore-not-found=true 2>/dev/null || true
    fi

    # Delete test namespace
    kubectl delete namespace compliance-test --ignore-not-found=true 2>/dev/null || true

    log_info "Cleanup complete."
}

# Print status summary
print_status() {
    log_step "Compliance Test Environment Status"
    echo ""

    echo "=== Gatekeeper Status ==="
    kubectl get pods -n gatekeeper-system 2>/dev/null || echo "Gatekeeper not installed"
    echo ""

    echo "=== ConstraintTemplates ==="
    kubectl get constrainttemplates -l app.kubernetes.io/part-of=e2e-tests 2>/dev/null || echo "No templates found"
    echo ""

    echo "=== Constraints ==="
    echo "K8sRequiredLabels:"
    kubectl get k8srequiredlabels -l app.kubernetes.io/part-of=e2e-tests 2>/dev/null || echo "None"
    echo ""
    echo "K8sPSPPrivilegedContainer:"
    kubectl get k8spsprivilegedcontainer -l app.kubernetes.io/part-of=e2e-tests 2>/dev/null || echo "None"
    echo ""
    echo "K8sAllowedRepos:"
    kubectl get k8sallowedrepos -l app.kubernetes.io/part-of=e2e-tests 2>/dev/null || echo "None"
    echo ""

    echo "=== Test Resources ==="
    kubectl get pods -n compliance-test -l app.kubernetes.io/part-of=e2e-tests 2>/dev/null || echo "No test pods"
    kubectl get deployments -n compliance-test -l app.kubernetes.io/part-of=e2e-tests 2>/dev/null || echo "No test deployments"
    echo ""

    echo "=== Violations Summary ==="
    echo "pod-must-have-labels violations:"
    kubectl get k8srequiredlabels pod-must-have-labels -o jsonpath='{.status.totalViolations}' 2>/dev/null || echo "0"
    echo ""
    echo "deployment-must-have-app-label violations:"
    kubectl get k8srequiredlabels deployment-must-have-app-label -o jsonpath='{.status.totalViolations}' 2>/dev/null || echo "0"
    echo ""
}

# Main function
main() {
    local INSTALL_GATEKEEPER=false
    local FIXTURES_ONLY=false
    local CLEANUP=false
    local STATUS_ONLY=false

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --install-gatekeeper)
                INSTALL_GATEKEEPER=true
                shift
                ;;
            --fixtures-only)
                FIXTURES_ONLY=true
                shift
                ;;
            --cleanup)
                CLEANUP=true
                shift
                ;;
            --status)
                STATUS_ONLY=true
                shift
                ;;
            --help|-h)
                echo "Usage: $0 [options]"
                echo ""
                echo "Options:"
                echo "  --install-gatekeeper  Install OPA Gatekeeper if not present"
                echo "  --fixtures-only       Only apply test fixtures (skip Gatekeeper check)"
                echo "  --cleanup             Remove test fixtures"
                echo "  --status              Show current status only"
                echo "  --help, -h            Show this help message"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done

    echo "=========================================="
    echo "Compliance E2E Test Setup"
    echo "=========================================="
    echo ""

    check_prerequisites

    if [ "$STATUS_ONLY" = true ]; then
        print_status
        exit 0
    fi

    if [ "$CLEANUP" = true ]; then
        cleanup
        exit 0
    fi

    if [ "$FIXTURES_ONLY" = false ]; then
        if check_gatekeeper_installed; then
            log_info "OPA Gatekeeper is already installed."
        else
            if [ "$INSTALL_GATEKEEPER" = true ]; then
                install_gatekeeper
            else
                log_error "OPA Gatekeeper is not installed."
                log_info "Run with --install-gatekeeper to install it, or install manually."
                exit 1
            fi
        fi
    fi

    apply_fixtures
    wait_for_violations

    echo ""
    print_status

    echo ""
    log_info "Compliance E2E test environment is ready!"
    log_info "Run E2E tests with: npm run test:e2e -- compliance"
}

main "$@"
