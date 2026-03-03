#!/bin/bash
# Mirror CI pipeline execution locally for debugging
# Usage: ./scripts/ci-local.sh [--quick]
#
# Options:
#   --quick     Run reduced burn-in (3 iterations instead of 10)

set -euo pipefail

QUICK_MODE=false
if [ "${1:-}" = "--quick" ]; then
    QUICK_MODE=true
fi

echo ""
echo "============================================"
echo "  Local CI Pipeline Mirror"
echo "============================================"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

passed() {
    echo -e "${GREEN}✓${NC} $1"
}

failed() {
    echo -e "${RED}✗${NC} $1"
    exit 1
}

info() {
    echo -e "${YELLOW}→${NC} $1"
}

# Stage 1: Lint
echo ""
echo "============================================"
echo "  Stage 1: Lint"
echo "============================================"

info "Running server linter..."
cd server
if golangci-lint run --timeout=5m; then
    passed "Backend lint passed"
else
    failed "Backend lint failed"
fi
cd ..

info "Running web linter..."
cd web
if npm run lint; then
    passed "Frontend lint passed"
else
    failed "Frontend lint failed"
fi

info "Running web typecheck..."
if npm run typecheck; then
    passed "Frontend typecheck passed"
else
    failed "Frontend typecheck failed"
fi
cd ..

# Stage 2: Unit Tests
echo ""
echo "============================================"
echo "  Stage 2: Unit Tests"
echo "============================================"

info "Running server tests..."
cd server
if go test -v -race ./internal/...; then
    passed "Backend tests passed"
else
    failed "Backend tests failed"
fi
cd ..

info "Running web tests..."
cd web
if npm test; then
    passed "Frontend tests passed"
else
    failed "Frontend tests failed"
fi
cd ..

# Stage 3: Build
echo ""
echo "============================================"
echo "  Stage 3: Build"
echo "============================================"

info "Building server (OSS)..."
cd server
if CGO_ENABLED=0 go build -ldflags="-w -s" -o ../bin/knodex-oss .; then
    passed "Backend OSS build passed"
else
    failed "Backend OSS build failed"
fi

info "Building server (Enterprise)..."
if CGO_ENABLED=0 go build -tags=enterprise -ldflags="-w -s" -o ../bin/knodex-enterprise .; then
    passed "Backend Enterprise build passed"
else
    failed "Backend Enterprise build failed"
fi
cd ..

info "Building web (OSS)..."
cd web
if npm run build; then
    passed "Frontend OSS build passed"
else
    failed "Frontend OSS build failed"
fi

info "Building web (Enterprise)..."
if npm run build:enterprise; then
    passed "Frontend Enterprise build passed"
else
    failed "Frontend Enterprise build failed"
fi
cd ..

# Stage 4: Docker Build (optional)
echo ""
echo "============================================"
echo "  Stage 4: Docker Build"
echo "============================================"

if command -v docker &> /dev/null; then
    info "Building unified Docker image (web embedded in Go binary)..."

    if docker build -t knodex-server:local -f Dockerfile .; then
        passed "Unified Docker image built"
    else
        failed "Unified Docker image build failed"
    fi
else
    info "Docker not available, skipping image builds"
fi

# Stage 5: Burn-In (if cluster available)
echo ""
echo "============================================"
echo "  Stage 5: Burn-In (E2E)"
echo "============================================"

if kubectl cluster-info &> /dev/null; then
    ITERATIONS=3
    if [ "$QUICK_MODE" = true ]; then
        ITERATIONS=1
    fi

    info "Running burn-in ($ITERATIONS iterations)..."

    for i in $(seq 1 $ITERATIONS); do
        echo ""
        echo "→ Burn-in iteration $i/$ITERATIONS"
        cd web
        if npx playwright test; then
            passed "Burn-in $i passed"
        else
            failed "Burn-in $i failed - flaky test detected"
        fi
        cd ..
    done
else
    info "No Kubernetes cluster available, skipping E2E burn-in"
    info "Run 'make cluster-up' and 'make qa' for full E2E testing"
fi

# Summary
echo ""
echo "============================================"
echo "  Local CI Pipeline Summary"
echo "============================================"
echo ""
passed "All local CI checks passed!"
echo ""
echo "Ready to push. The remote CI will additionally:"
echo "  - Build and push Docker images to ghcr.io"
echo "  - Run parallelized E2E tests (4 shards)"
echo "  - Run full burn-in on PRs to main"
echo ""
