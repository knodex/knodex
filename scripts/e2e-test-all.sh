#!/bin/bash
# Copyright 2026 Knodex Authors
# SPDX-License-Identifier: AGPL-3.0-only
# Unified E2E Test Script
# Runs both Backend (Go) and Frontend (Playwright) E2E tests against the same environment
#
# Usage:
#   ./scripts/e2e-test-all.sh              # Run all E2E tests (server + web)
#   ./scripts/e2e-test-all.sh server       # Run only server E2E tests
#   ./scripts/e2e-test-all.sh web          # Run only web E2E tests
#   ./scripts/e2e-test-all.sh oidc         # Run only OIDC-related tests (server + web)
#   ./scripts/e2e-test-all.sh --no-setup   # Skip environment setup (assumes already deployed)
#   ./scripts/e2e-test-all.sh --no-cleanup # Skip cleanup after tests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test results tracking
SERVER_TESTS_PASSED=0
SERVER_TESTS_FAILED=0
WEB_TESTS_PASSED=0
WEB_TESTS_FAILED=0

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_section() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
    echo ""
}

# Parse command line arguments
SKIP_SETUP=false
SKIP_CLEANUP=false
RUN_SERVER=true
RUN_WEB=true
RUN_OIDC_ONLY=false

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            server)
                RUN_SERVER=true
                RUN_WEB=false
                shift
                ;;
            web)
                RUN_SERVER=false
                RUN_WEB=true
                shift
                ;;
            oidc)
                RUN_OIDC_ONLY=true
                shift
                ;;
            --no-setup)
                SKIP_SETUP=true
                shift
                ;;
            --no-cleanup)
                SKIP_CLEANUP=true
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    echo "Unified E2E Test Script"
    echo ""
    echo "Usage: $0 [options] [test-type]"
    echo ""
    echo "Test Types:"
    echo "  server      Run only server (Go) E2E tests"
    echo "  web         Run only web (Playwright) E2E tests"
    echo "  oidc        Run only OIDC-related tests (both server and web)"
    echo "  (none)      Run all E2E tests (default)"
    echo ""
    echo "Options:"
    echo "  --no-setup    Skip environment setup (assumes already deployed)"
    echo "  --no-cleanup  Skip cleanup after tests"
    echo "  --help, -h    Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                      # Run all E2E tests with setup and cleanup"
    echo "  $0 server              # Run only server tests"
    echo "  $0 web                # Run only web tests"
    echo "  $0 oidc                 # Run OIDC tests only"
    echo "  $0 --no-cleanup         # Run all tests, keep environment for debugging"
    echo "  $0 web --no-setup     # Run web tests against existing deployment"
}

# Wait for server health
wait_for_server() {
    local url=$1
    local max_attempts=${2:-60}
    local attempt=1

    log_info "Waiting for server health check at $url..."

    while [ $attempt -le $max_attempts ]; do
        if curl -sf "$url/healthz" > /dev/null 2>&1; then
            log_info "Backend is healthy after $attempt attempts."
            return 0
        fi

        if [ $((attempt % 10)) -eq 0 ]; then
            log_info "Still waiting for server... attempt $attempt/$max_attempts"
        fi

        sleep 2
        attempt=$((attempt + 1))
    done

    log_error "Backend failed to become healthy after $max_attempts attempts"
    return 1
}


# Setup test environment
setup_environment() {
    if [ "$SKIP_SETUP" = true ]; then
        log_info "Skipping environment setup (--no-setup flag)"
        return 0
    fi

    log_section "Setting Up E2E Test Environment"

    # Deploy using the QA deploy script
    log_info "Deploying application to Kind cluster..."
    "$SCRIPT_DIR/qa-deploy.sh" deploy

    # Fixed ports for simplified deployment (no multi-branch support)
    # Single URL: server serves both API and embedded web
    export E2E_TESTS=true
    export E2E_API_URL="http://localhost:8080"
    export E2E_BASE_URL="http://localhost:8080"

    # JWT secret must match backend's JWT_SECRET (from kustomize overlay)
    # so that test auth helper generates tokens the backend will accept
    export E2E_JWT_SECRET="test-jwt-secret-key-for-qa-testing-only"

    # Mock OIDC configuration for E2E testing
    export MOCK_OIDC_ENABLED=true
    export MOCK_OIDC_ISSUER_URL="http://mock-oidc:8081"
    export MOCK_OIDC_CLIENT_ID="test-client-id"
    export MOCK_OIDC_CLIENT_SECRET="test-client-secret"

    log_info "Environment configuration:"
    log_info "  Application URL: $E2E_BASE_URL"
    log_info "  Mock OIDC:       $MOCK_OIDC_ISSUER_URL"

    # Wait for service to be healthy
    if ! wait_for_server "$E2E_API_URL" 60; then
        log_error "Backend failed health check"
        return 1
    fi

    log_info "E2E test environment is ready!"
}

# Run server E2E tests
run_server_tests() {
    log_section "Running Backend E2E Tests (Go)"

    # Ensure environment variables are set
    if [ -z "$E2E_API_URL" ]; then
        export E2E_API_URL="http://localhost:8080"
    fi

    log_info "Backend API URL: $E2E_API_URL"

    local test_pattern=""
    if [ "$RUN_OIDC_ONLY" = true ]; then
        test_pattern="-run TestOIDC"
        log_info "Running OIDC tests only..."
    fi

    cd "$PROJECT_ROOT/server"

    # Run Go E2E tests
    set +e
    E2E_TESTS=true E2E_API_URL="$E2E_API_URL" \
        go test -tags=e2e -v -timeout 10m ./test/e2e/... $test_pattern 2>&1 | tee /tmp/server-e2e-results.txt
    local exit_code=$?
    set -e

    # Parse results
    SERVER_TESTS_PASSED=$(grep -c "^--- PASS:" /tmp/server-e2e-results.txt 2>/dev/null) || SERVER_TESTS_PASSED=0
    SERVER_TESTS_FAILED=$(grep -c "^--- FAIL:" /tmp/server-e2e-results.txt 2>/dev/null) || SERVER_TESTS_FAILED=0

    cd "$PROJECT_ROOT"

    if [ $exit_code -ne 0 ]; then
        log_error "Backend E2E tests failed!"
        return 1
    fi

    log_info "Backend E2E tests completed: $SERVER_TESTS_PASSED passed, $SERVER_TESTS_FAILED failed"
    return 0
}

# Run web E2E tests
run_web_tests() {
    log_section "Running Frontend E2E Tests (Playwright)"

    # Ensure environment variables are set
    if [ -z "$E2E_BASE_URL" ]; then
        export E2E_BASE_URL="http://localhost:8080"
    fi

    log_info "Application URL: $E2E_BASE_URL"

    cd "$PROJECT_ROOT/web"

    # Create/update .env.e2e with current configuration
    cat > .env.e2e << EOF
E2E_BASE_URL=$E2E_BASE_URL
E2E_JWT_SECRET=$E2E_JWT_SECRET
MOCK_OIDC_ENABLED=$MOCK_OIDC_ENABLED
MOCK_OIDC_ISSUER_URL=$MOCK_OIDC_ISSUER_URL
MOCK_OIDC_CLIENT_ID=$MOCK_OIDC_CLIENT_ID
MOCK_OIDC_CLIENT_SECRET=$MOCK_OIDC_CLIENT_SECRET
EOF

    local test_filter=""
    if [ "$RUN_OIDC_ONLY" = true ]; then
        test_filter="--grep 'OIDC|oidc|Authentication'"
        log_info "Running OIDC-related tests only..."
    fi

    # Run Playwright tests
    set +e
    npm run test:e2e -- $test_filter 2>&1 | tee /tmp/web-e2e-results.txt
    local exit_code=$?
    set -e

    # Parse results (Playwright output format)
    WEB_TESTS_PASSED=$(grep -oE '[0-9]+ passed' /tmp/web-e2e-results.txt 2>/dev/null | head -1 | grep -oE '[0-9]+') || WEB_TESTS_PASSED=0
    WEB_TESTS_FAILED=$(grep -oE '[0-9]+ failed' /tmp/web-e2e-results.txt 2>/dev/null | head -1 | grep -oE '[0-9]+') || WEB_TESTS_FAILED=0

    cd "$PROJECT_ROOT"

    if [ $exit_code -ne 0 ]; then
        log_warn "Some web E2E tests failed"
        return 1
    fi

    log_info "Frontend E2E tests completed: $WEB_TESTS_PASSED passed, $WEB_TESTS_FAILED failed"
    return 0
}

# Cleanup test environment
cleanup_environment() {
    if [ "$SKIP_CLEANUP" = true ]; then
        log_info "Skipping cleanup (--no-cleanup flag)"
        log_info "Environment is still running for debugging"
        return 0
    fi

    log_section "Cleaning Up E2E Test Environment"

    # Cleanup E2E test fixtures
    "$SCRIPT_DIR/test-cleanup.sh" fixtures 2>/dev/null || true

    log_info "E2E test cleanup completed"
}

# Print final summary
print_summary() {
    log_section "E2E Test Summary"

    local total_passed=$(( ${SERVER_TESTS_PASSED:-0} + ${WEB_TESTS_PASSED:-0} ))
    local total_failed=$(( ${SERVER_TESTS_FAILED:-0} + ${WEB_TESTS_FAILED:-0} ))
    local total_tests=$(( total_passed + total_failed ))

    echo ""
    echo "┌─────────────────────────────────────────────────────────────────┐"
    echo "│                       E2E Test Results                          │"
    echo "├─────────────────────────────────────────────────────────────────┤"

    if [ "$RUN_SERVER" = true ]; then
        echo "│  Backend (Go):                                                  │"
        echo "│    ✓ Passed: $SERVER_TESTS_PASSED                                                      │"
        echo "│    ✗ Failed: $SERVER_TESTS_FAILED                                                      │"
    fi

    if [ "$RUN_WEB" = true ]; then
        echo "│  Frontend (Playwright):                                         │"
        echo "│    ✓ Passed: $WEB_TESTS_PASSED                                                     │"
        echo "│    ✗ Failed: $WEB_TESTS_FAILED                                                     │"
    fi

    echo "├─────────────────────────────────────────────────────────────────┤"
    echo "│  Total: $total_passed passed, $total_failed failed                                     │"
    echo "└─────────────────────────────────────────────────────────────────┘"
    echo ""

    if [ $total_failed -gt 0 ]; then
        log_error "Some E2E tests failed!"
        echo ""
        echo "Test artifacts available at:"
        echo "  Backend:  /tmp/server-e2e-results.txt"
        echo "  Frontend: web/test-results/"
        echo "            web/playwright-report/"
        return 1
    else
        log_info "All E2E tests passed!"
        return 0
    fi
}

# Main function
main() {
    parse_args "$@"

    log_section "Unified E2E Test Runner"
    log_info "Starting E2E tests..."
    log_info "  Run Backend:  $RUN_SERVER"
    log_info "  Run Frontend: $RUN_WEB"
    log_info "  OIDC Only:    $RUN_OIDC_ONLY"
    log_info "  Skip Setup:   $SKIP_SETUP"
    log_info "  Skip Cleanup: $SKIP_CLEANUP"

    local has_failures=false

    # Setup environment
    if ! setup_environment; then
        log_error "Failed to setup test environment"
        exit 1
    fi

    # Run server tests
    if [ "$RUN_SERVER" = true ]; then
        if ! run_server_tests; then
            has_failures=true
        fi
    fi

    # Run web tests
    if [ "$RUN_WEB" = true ]; then
        if ! run_web_tests; then
            has_failures=true
        fi
    fi

    # Cleanup
    cleanup_environment

    # Print summary
    if ! print_summary; then
        exit 1
    fi

    if [ "$has_failures" = true ]; then
        exit 1
    fi
}

main "$@"
