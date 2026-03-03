#!/bin/bash
# test-cleanup.sh - Clean up test data and environment
#
# Usage:
#   ./scripts/test-cleanup.sh              # Clean up test fixtures only
#   ./scripts/test-cleanup.sh full         # Clean up fixtures and deployment namespace
#   ./scripts/test-cleanup.sh cluster      # Delete entire Kind cluster
#   ./scripts/test-cleanup.sh integration  # Clean up integration test cluster

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Clean up E2E test fixtures
cleanup_fixtures() {
    log_info "Cleaning up E2E test fixtures..."

    # Get namespace from branch config
    eval "$("$SCRIPT_DIR/qa-branch-config.sh" export 2>/dev/null)" || true
    local namespace="${NAMESPACE:-knodex-main}"

    # Delete test Projects created by E2E tests (labeled with e2e-test=true)
    log_info "Deleting test Projects..."
    kubectl delete projects.knodex.io -l e2e-test=true --all-namespaces 2>/dev/null || true

    # Delete test ProjectRoleBindings created by E2E tests
    log_info "Deleting test ProjectRoleBindings..."
    kubectl delete projectrolebindings.knodex.io -l e2e-test=true --all-namespaces 2>/dev/null || true

    # Delete test namespaces created by E2E tests
    log_info "Deleting test namespaces..."
    kubectl delete namespace -l e2e-test=true 2>/dev/null || true

    log_info "E2E test fixtures cleaned up."
}

# Full cleanup including cluster namespace
cleanup_full() {
    log_info "Performing full E2E cleanup..."

    cleanup_fixtures

    # Clean up the namespace
    log_info "Cleaning up deployment namespace..."
    "$SCRIPT_DIR/qa-deploy.sh" cleanup-namespace 2>/dev/null || true

    log_info "Full E2E cleanup complete."
}

# Cleanup cluster
cleanup_cluster() {
    log_info "Deleting entire Kind cluster..."
    "$SCRIPT_DIR/qa-deploy.sh" cleanup-cluster 2>/dev/null || true
    log_info "Kind cluster deleted."
}

# Cleanup integration test cluster
cleanup_integration() {
    local cluster_name="${INTEGRATION_TEST_CLUSTER:-kro-integration-test}"

    # Security: Validate cluster name
    if [[ ! "$cluster_name" =~ ^[a-zA-Z0-9-]+$ ]]; then
        log_error "Invalid cluster name. Only alphanumeric characters and hyphens allowed."
        exit 1
    fi

    if [ ${#cluster_name} -gt 63 ]; then
        log_error "Cluster name too long (max 63 characters)"
        exit 1
    fi

    log_info "Cleaning up integration test cluster: $cluster_name"

    if ! command -v kind &> /dev/null; then
        log_error "Kind is not installed."
        exit 1
    fi

    if kind get clusters 2>/dev/null | grep -q "^${cluster_name}$"; then
        log_info "Deleting Kind cluster '$cluster_name'..."
        kind delete cluster --name "$cluster_name"
        log_info "Kind cluster deleted successfully"
    else
        log_info "Kind cluster '$cluster_name' does not exist (already cleaned up)"
    fi
}

# Show usage
show_usage() {
    echo "Usage: $0 [fixtures|full|cluster|integration]"
    echo ""
    echo "Modes:"
    echo "  fixtures    - Clean up only E2E test fixtures (default)"
    echo "  full        - Clean up fixtures and deployment namespace"
    echo "  cluster     - Delete entire Kind cluster"
    echo "  integration - Delete integration test Kind cluster"
    echo ""
    echo "Examples:"
    echo "  $0                    # Clean up test fixtures"
    echo "  $0 full               # Clean up everything but keep cluster"
    echo "  $0 cluster            # Delete Kind cluster entirely"
    echo "  $0 integration        # Clean up integration test cluster"
}

# Main function
main() {
    local mode="${1:-fixtures}"

    case "$mode" in
        fixtures)
            cleanup_fixtures
            ;;
        full)
            cleanup_full
            ;;
        cluster)
            cleanup_cluster
            ;;
        integration)
            cleanup_integration
            ;;
        -h|--help|help)
            show_usage
            ;;
        *)
            log_error "Unknown mode: $mode"
            show_usage
            exit 1
            ;;
    esac
}

main "$@"
