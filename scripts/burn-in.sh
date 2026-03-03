#!/bin/bash
# Standalone burn-in test runner for flaky test detection
# Usage: ./scripts/burn-in.sh [ITERATIONS] [--grep PATTERN]
#
# Examples:
#   ./scripts/burn-in.sh               # 10 iterations, all tests
#   ./scripts/burn-in.sh 5             # 5 iterations, all tests
#   ./scripts/burn-in.sh 3 --grep auth # 3 iterations, only auth tests
#   ./scripts/burn-in.sh --grep rbac   # 10 iterations, only rbac tests

set -euo pipefail

# Parse arguments
ITERATIONS=10
GREP_PATTERN=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --grep)
            GREP_PATTERN="$2"
            shift 2
            ;;
        [0-9]*)
            ITERATIONS=$1
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [ITERATIONS] [--grep PATTERN]"
            exit 1
            ;;
    esac
done

echo ""
echo "============================================"
echo "  Burn-In Test Runner"
echo "============================================"
echo ""
echo "Iterations: $ITERATIONS"
if [ -n "$GREP_PATTERN" ]; then
    echo "Filter: $GREP_PATTERN"
fi
echo ""

# Check prerequisites
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: No Kubernetes cluster available."
    echo "Run 'make cluster-up' first."
    exit 1
fi

# Ensure app is deployed
if ! kubectl get deployment knodex-server -n knodex &> /dev/null; then
    echo "Application not deployed. Running 'make qa' to deploy..."
    make qa
fi

# Results tracking
PASSED=0
FAILED=0
FAILED_ITERATIONS=""
RESULTS_FILE="burn-in-results-$(date +%Y%m%d-%H%M%S).md"

echo "## Burn-In Results" > "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"
echo "**Date:** $(date -u +"%Y-%m-%d %H:%M:%S UTC")" >> "$RESULTS_FILE"
echo "**Iterations:** $ITERATIONS" >> "$RESULTS_FILE"
if [ -n "$GREP_PATTERN" ]; then
    echo "**Filter:** \`$GREP_PATTERN\`" >> "$RESULTS_FILE"
fi
echo "" >> "$RESULTS_FILE"
echo "| Iteration | Status | Duration |" >> "$RESULTS_FILE"
echo "|-----------|--------|----------|" >> "$RESULTS_FILE"

cd web

for i in $(seq 1 $ITERATIONS); do
    echo ""
    echo "============================================"
    echo "  Burn-in iteration $i/$ITERATIONS"
    echo "============================================"
    echo ""

    START_TIME=$(date +%s)

    # Run tests
    set +e
    if [ -n "$GREP_PATTERN" ]; then
        npx playwright test --grep="$GREP_PATTERN" 2>&1 | tee "../burn-in-iteration-$i.log"
    else
        npx playwright test 2>&1 | tee "../burn-in-iteration-$i.log"
    fi
    RESULT=$?
    set -e

    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))

    if [ $RESULT -eq 0 ]; then
        echo "" >> "../$RESULTS_FILE"
        echo "| $i | ✅ Passed | ${DURATION}s |" >> "../$RESULTS_FILE"
        PASSED=$((PASSED + 1))
        echo ""
        echo "✓ Iteration $i passed in ${DURATION}s"
    else
        echo "| $i | ❌ Failed | ${DURATION}s |" >> "../$RESULTS_FILE"
        FAILED=$((FAILED + 1))
        FAILED_ITERATIONS="$FAILED_ITERATIONS $i"

        # Capture failure artifacts
        mkdir -p "../burn-in-failures/iteration-$i"
        cp -r ../test-results/* "../burn-in-failures/iteration-$i/" 2>/dev/null || true
        cp test-results/* "../burn-in-failures/iteration-$i/" 2>/dev/null || true

        echo ""
        echo "✗ Iteration $i FAILED after ${DURATION}s"
        echo "  Artifacts saved to burn-in-failures/iteration-$i/"
    fi
done

cd ..

# Summary
echo "" >> "$RESULTS_FILE"
echo "## Summary" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"
echo "- **Passed:** $PASSED/$ITERATIONS" >> "$RESULTS_FILE"
echo "- **Failed:** $FAILED/$ITERATIONS" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"

echo ""
echo "============================================"
echo "  Burn-In Summary"
echo "============================================"
echo ""
echo "Passed: $PASSED/$ITERATIONS"
echo "Failed: $FAILED/$ITERATIONS"
echo ""
echo "Results saved to: $RESULTS_FILE"

if [ $FAILED -gt 0 ]; then
    echo "" >> "$RESULTS_FILE"
    echo "### ⚠️ Flaky Tests Detected!" >> "$RESULTS_FILE"
    echo "" >> "$RESULTS_FILE"
    echo "Failed iterations:$FAILED_ITERATIONS" >> "$RESULTS_FILE"
    echo "" >> "$RESULTS_FILE"
    echo "Check \`burn-in-failures/\` for detailed artifacts." >> "$RESULTS_FILE"

    echo ""
    echo "⚠️  FLAKY TESTS DETECTED!"
    echo ""
    echo "Failed iterations:$FAILED_ITERATIONS"
    echo ""
    echo "Even ONE failure indicates tests that need investigation."
    echo "Review burn-in-failures/ directory for traces and screenshots."
    echo ""
    exit 1
else
    echo "" >> "$RESULTS_FILE"
    echo "### ✅ No Flaky Tests Detected!" >> "$RESULTS_FILE"
    echo "All $ITERATIONS iterations passed successfully." >> "$RESULTS_FILE"

    echo ""
    echo "✅ No flaky tests detected!"
    echo "All $ITERATIONS iterations passed successfully."
    echo ""
fi
