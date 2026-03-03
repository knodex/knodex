#!/bin/bash
# Run tests only for changed files to speed up local development
# Usage: ./scripts/test-changed.sh [--all]
#
# Options:
#   --all       Force run all tests regardless of changes

set -euo pipefail

RUN_ALL=false
if [ "${1:-}" = "--all" ]; then
    RUN_ALL=true
fi

echo ""
echo "============================================"
echo "  Selective Test Runner"
echo "============================================"
echo ""

# Get changed files
CHANGED_FILES=$(git diff --name-only HEAD~1 2>/dev/null || git diff --name-only --cached)

if [ -z "$CHANGED_FILES" ]; then
    echo "No changed files detected. Running all tests..."
    RUN_ALL=true
fi

RUN_SERVER=false
RUN_WEB=false
RUN_E2E=false

if [ "$RUN_ALL" = true ]; then
    RUN_SERVER=true
    RUN_WEB=true
    RUN_E2E=true
else
    echo "Changed files:"
    echo "$CHANGED_FILES" | sed 's/^/  - /'
    echo ""

    # Determine what to test based on changes
    if echo "$CHANGED_FILES" | grep -qE '^server/'; then
        RUN_SERVER=true
        echo "→ Server changes detected"
    fi

    if echo "$CHANGED_FILES" | grep -qE '^web/src/'; then
        RUN_WEB=true
        echo "→ Web source changes detected"
    fi

    if echo "$CHANGED_FILES" | grep -qE '^web/test/e2e/'; then
        RUN_E2E=true
        echo "→ E2E test changes detected"
    fi

    # Always run E2E if server or web changes
    if [ "$RUN_SERVER" = true ] || [ "$RUN_WEB" = true ]; then
        RUN_E2E=true
    fi

    echo ""
fi

# Track results
RESULTS=""

# Run server tests
if [ "$RUN_SERVER" = true ]; then
    echo "============================================"
    echo "  Running Server Tests"
    echo "============================================"

    cd server
    if go test -v ./internal/...; then
        RESULTS="${RESULTS}✓ Server tests passed\n"
    else
        RESULTS="${RESULTS}✗ Server tests FAILED\n"
        echo "Server tests failed!"
    fi
    cd ..
else
    RESULTS="${RESULTS}○ Server tests skipped (no changes)\n"
fi

# Run web unit tests
if [ "$RUN_WEB" = true ]; then
    echo ""
    echo "============================================"
    echo "  Running Web Unit Tests"
    echo "============================================"

    cd web
    if npm test; then
        RESULTS="${RESULTS}✓ Web unit tests passed\n"
    else
        RESULTS="${RESULTS}✗ Web unit tests FAILED\n"
        echo "Web unit tests failed!"
    fi
    cd ..
else
    RESULTS="${RESULTS}○ Web unit tests skipped (no changes)\n"
fi

# Run E2E tests (only if cluster is available)
if [ "$RUN_E2E" = true ]; then
    if kubectl cluster-info &> /dev/null; then
        echo ""
        echo "============================================"
        echo "  Running E2E Tests"
        echo "============================================"

        cd web

        # Build grep pattern from changed E2E test files
        E2E_FILTER=""
        E2E_CHANGES=$(echo "$CHANGED_FILES" | grep -E '^web/test/e2e/.*\.spec\.ts$' || true)

        if [ -n "$E2E_CHANGES" ] && [ "$RUN_ALL" = false ]; then
            # Extract test names from changed files
            E2E_FILTER=$(echo "$E2E_CHANGES" | xargs -I{} basename {} .spec.ts | tr '\n' '|' | sed 's/|$//')
            echo "Running E2E tests matching: $E2E_FILTER"
            if npx playwright test --grep="$E2E_FILTER"; then
                RESULTS="${RESULTS}✓ E2E tests (selective) passed\n"
            else
                RESULTS="${RESULTS}✗ E2E tests (selective) FAILED\n"
            fi
        else
            if npx playwright test; then
                RESULTS="${RESULTS}✓ E2E tests (full) passed\n"
            else
                RESULTS="${RESULTS}✗ E2E tests (full) FAILED\n"
            fi
        fi
        cd ..
    else
        RESULTS="${RESULTS}○ E2E tests skipped (no cluster)\n"
        echo ""
        echo "Note: E2E tests skipped because no Kubernetes cluster is available."
        echo "Run 'make cluster-up' and 'make qa' for full E2E testing."
    fi
else
    RESULTS="${RESULTS}○ E2E tests skipped (no affecting changes)\n"
fi

# Print summary
echo ""
echo "============================================"
echo "  Test Results Summary"
echo "============================================"
echo ""
echo -e "$RESULTS"

# Exit with error if any tests failed
if echo -e "$RESULTS" | grep -q "✗"; then
    exit 1
fi

echo "All selected tests passed!"
