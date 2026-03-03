.PHONY: help dev dev-web dev-server build build-web build-server \
        build-oss build-enterprise build-all \
        build-server-oss build-server-enterprise \
        build-web-oss build-web-enterprise \
        clean lint lint-fix lint-server lint-web \
        cluster-up cluster-down \
        test test-server test-web e2e qa qa-stop \
        tilt-up tilt-down tilt-status \
        docs-serve docs-build docs-clean docs-install \
        api-docs \
        helm-lint helm-template helm-package \
        _ensure-prereqs _deploy-app

# Default target
help:
	@echo "Knodex - Development Commands"
	@echo ""
	@echo "Development (Local with Tilt):"
	@echo "  make tilt-up        - Start Tilt for live Kubernetes development (recommended)"
	@echo "  make tilt-down      - Stop Tilt and cleanup"
	@echo "  make tilt-status    - Show Tilt resource status"
	@echo ""
	@echo "Development (Native - requires external Redis):"
	@echo "  make dev            - Start server + web (needs Redis running)"
	@echo "  make dev-server     - Start server only"
	@echo "  make dev-web        - Start web only"
	@echo ""
	@echo "Build:"
	@echo "  make build            - Build all services"
	@echo "  make build-server     - Build server binary"
	@echo "  make build-web        - Build web static files"
	@echo "  make build-oss        - Build OSS edition (server + web)"
	@echo "  make build-enterprise - Build Enterprise edition (server + web)"
	@echo "  make build-all        - Build all editions (OSS + Enterprise)"
	@echo ""
	@echo "Cluster Management:"
	@echo "  make cluster-up     - Create Kind cluster (one-time setup)"
	@echo "  make cluster-down   - Delete Kind cluster"
	@echo ""
	@echo "Testing (cluster-agnostic - works with Kind, AKS, GKE, EKS):"
	@echo "  make test           - Unit tests (fast, no cluster needed)"
	@echo "  make e2e            - E2E tests (requires cluster)"
	@echo "  make qa             - Full QA: deploy app + run all tests"
	@echo "  make qa-stop        - Cleanup app deployment"
	@echo ""
	@echo "Quality:"
	@echo "  make lint           - Run linters"
	@echo "  make clean          - Clean build artifacts"
	@echo ""
	@echo "Documentation:"
	@echo "  make docs-install   - Install documentation dependencies"
	@echo "  make docs-serve     - Serve documentation locally (http://localhost:8000)"
	@echo "  make docs-build     - Build documentation static site"
	@echo "  make docs-clean     - Clean documentation build artifacts"
	@echo ""
	@echo "Helm Chart:"
	@echo "  make helm-lint      - Lint Helm chart"
	@echo "  make helm-template  - Render chart templates locally"
	@echo "  make helm-package   - Package chart into .tgz"
	@echo ""
	@echo "API Documentation:"
	@echo "  make api-docs       - Validate OpenAPI spec and sync to server embed"

# ===== Development =====
# NOTE: For local development, use 'make tilt-up' which manages all services in Kubernetes.
# The targets below are for native development and require Redis to be running externally.

dev:
	@echo ""
	@echo "============================================"
	@echo "  Native Development Mode"
	@echo "  Requires Redis running externally."
	@echo ""
	@echo "  Recommended: Use 'make tilt-up' instead"
	@echo "  for fully managed local development."
	@echo "============================================"
	@echo ""
	@echo "Starting development servers..."
	@(cd server && go run .) & \
	(cd web && npm run dev) & \
	wait

dev-server:
	@echo "Starting server (requires Redis at localhost:6379)..."
	cd server && go run .

dev-web:
	@echo "Starting web..."
	cd web && npm run dev

# ===== Build =====
# Build web first, embed into Go binary
build: build-web _embed-web build-server

build-server:
	@echo "Building server (with embedded web)..."
	cd server && CGO_ENABLED=0 go build -ldflags="-w -s" -o ../bin/knodex-server .

build-web:
	@echo "Building web..."
	cd web && npm run build

# Copy web dist into Go embed path
_embed-web:
	@echo "Embedding web dist into Go binary..."
	@rm -rf server/internal/static/dist
	@cp -r web/dist server/internal/static/dist

# ===== Build - OSS Edition =====
build-oss: build-web-oss _embed-web build-server-oss
	@echo ""
	@echo "============================================"
	@echo "  OSS build completed!"
	@echo "============================================"

build-server-oss:
	@echo "Building server (OSS, embedded web)..."
	cd server && CGO_ENABLED=0 go build -ldflags="-w -s" -o ../bin/knodex-oss .

build-web-oss:
	@echo "Building web (OSS)..."
	cd web && npm run build

# ===== Build - Enterprise Edition =====
build-enterprise: build-web-enterprise _embed-web build-server-enterprise
	@echo ""
	@echo "============================================"
	@echo "  Enterprise build completed!"
	@echo "============================================"

build-server-enterprise:
	@echo "Building server (Enterprise, embedded web)..."
	cd server && CGO_ENABLED=0 go build -tags=enterprise -ldflags="-w -s" -o ../bin/knodex-enterprise .

build-web-enterprise:
	@echo "Building web (Enterprise)..."
	cd web && npm run build:enterprise

# ===== Build - All Editions =====
build-all: build-oss build-enterprise
	@echo ""
	@echo "============================================"
	@echo "  All builds completed (OSS + Enterprise)!"
	@echo "============================================"

# ===== Cluster Management =====
cluster-up:
	@echo "Creating Kind cluster..."
	@kind get clusters 2>/dev/null | grep -q "^knodex-qa$$" || \
		kind create cluster --name knodex-qa --config scripts/kind-config.yaml
	@kubectl config use-context kind-knodex-qa
	@echo ""
	@echo "============================================"
	@echo "  Cluster ready!"
	@echo "  Run 'make qa' to deploy and test."
	@echo "============================================"

cluster-down:
	@echo "Deleting Kind cluster..."
	kind delete cluster --name knodex-qa
	@echo "Cluster deleted."

# ===== Testing =====
# Unit tests (no cluster required)
# Dependencies ensure fail-fast: if server fails, web won't run
test: test-server test-web
	@echo ""
	@echo "============================================"
	@echo "  Unit tests completed!"
	@echo "============================================"

test-server:
	@echo ""
	@echo "============================================"
	@echo "  Running Server Unit Tests"
	@echo "============================================"
	cd server && go test -v ./internal/...

test-web:
	@echo ""
	@echo "============================================"
	@echo "  Running Web Unit Tests"
	@echo "============================================"
	cd web && npm test

# E2E tests (requires cluster)
e2e: _ensure-prereqs _ensure-app
	@echo ""
	@echo "============================================"
	@echo "  Running E2E Tests"
	@echo "============================================"
	./scripts/e2e-test-all.sh
	@echo ""
	@echo "============================================"
	@echo "  E2E tests completed!"
	@echo "============================================"

# Full QA cycle
qa: _ensure-prereqs _deploy-app
	@echo ""
	@echo "============================================"
	@echo "  Running Full QA Cycle"
	@echo "============================================"
	./scripts/e2e-test-all.sh
	@echo ""
	@echo "============================================"
	@echo "  QA complete!"
	@echo "  Use 'make qa-stop' to cleanup."
	@echo "============================================"

# Cleanup app deployment (not the cluster)
qa-stop:
	@echo "Cleaning up app deployment..."
	kubectl delete namespace knodex --ignore-not-found
	@echo "App namespace deleted."

# Internal: Check and install cluster prerequisites
# Pass KRO_VERSION to override the default (e.g., make qa KRO_VERSION=0.8.5)
_ensure-prereqs:
	@echo "Checking cluster prerequisites..."
	@KRO_VERSION=$(KRO_VERSION) ./scripts/ensure-prereqs.sh

# Internal: Ensure app is deployed
_ensure-app:
	@kubectl get deployment knodex-server -n knodex >/dev/null 2>&1 || \
		$(MAKE) _deploy-app

# Internal: Deploy app to cluster
_deploy-app:
	@echo "Deploying application..."
	./scripts/qa-deploy.sh deploy

# ===== Quality =====
# Dependencies ensure fail-fast: if server fails, web won't run
lint: lint-server lint-web
	@echo ""
	@echo "============================================"
	@echo "  Linting completed!"
	@echo "============================================"

lint-server:
	@echo "Linting server..."
	cd server && golangci-lint run

lint-web:
	@echo "Linting web..."
	cd web && npm run lint

lint-fix:
	@echo "Running linters with auto-fix..."
	cd server && golangci-lint run --fix && cd ../web && npm run lint --fix

clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf web/dist/
	rm -rf web/node_modules/.vite/
	@# Clean embedded web dist (keep placeholder)
	rm -rf server/internal/static/dist
	mkdir -p server/internal/static/dist
	echo '<!doctype html><html><head><title>Knodex</title></head><body><div id="root"></div><p>Placeholder. Run make build.</p></body></html>' > server/internal/static/dist/index.html
	touch server/internal/static/dist/.gitkeep

# ===== Tilt - Kubernetes Live Development =====
tilt-up:
	@kubectl cluster-info >/dev/null 2>&1 || { \
		echo ""; \
		echo "============================================"; \
		echo "  No Kubernetes cluster available"; \
		echo "  Run 'make cluster-up' first."; \
		echo "============================================"; \
		exit 1; \
	}
	@echo "Starting Tilt for live Kubernetes development..."
	@command -v tilt >/dev/null 2>&1 || { \
		echo ""; \
		echo "============================================"; \
		echo "  Tilt is not installed."; \
		echo ""; \
		echo "  Install with:"; \
		echo "    brew install tilt (macOS)"; \
		echo "    curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash"; \
		echo ""; \
		echo "  See: https://docs.tilt.dev/install.html"; \
		echo "============================================"; \
		exit 1; \
	}
	@echo ""
	@echo "Starting Tilt..."
	@echo "  Tilt UI: http://localhost:10350"
	@echo "  Server:  http://localhost:8080"
	@echo ""
	tilt up

tilt-down:
	@echo "Stopping Tilt..."
	tilt down
	@echo "Cleaning up Tilt namespace..."
	-kubectl delete namespace knodex-tilt --ignore-not-found

tilt-status:
	@command -v tilt >/dev/null 2>&1 || { echo "Tilt is not installed."; exit 1; }
	tilt get

# ===== Documentation - MkDocs Material =====
docs-install:
	@echo "Installing documentation dependencies..."
	@command -v pip3 >/dev/null 2>&1 || { echo "Error: pip3 is required but not installed. Install Python 3 first."; exit 1; }
	pip3 install -r requirements-docs.txt
	@echo "Documentation dependencies installed successfully!"
	@echo "Run 'make docs-serve' to preview documentation locally."

docs-serve:
	@echo "Starting documentation server..."
	@command -v mkdocs >/dev/null 2>&1 || { echo "Error: mkdocs not found. Run 'make docs-install' first."; exit 1; }
	@echo ""
	@echo "Documentation server starting at http://localhost:8000"
	@echo "Press Ctrl+C to stop the server."
	@echo ""
	mkdocs serve

docs-build:
	@echo "Building documentation..."
	@command -v mkdocs >/dev/null 2>&1 || { echo "Error: mkdocs not found. Run 'make docs-install' first."; exit 1; }
	mkdocs build
	@echo ""
	@echo "Documentation built successfully!"
	@echo "Output: site/"
	@echo ""
	@echo "To preview the built site, run:"
	@echo "  cd site && python3 -m http.server 8000"

docs-clean:
	@echo "Cleaning documentation build artifacts..."
	rm -rf site/
	@echo "Documentation cleaned successfully!"

# ===== API Documentation =====
api-docs:
	@echo "Validating OpenAPI specification..."
	@npx --yes @redocly/cli lint docs/api/openapi.yaml --skip-rule no-unused-components
	@echo ""
	@echo "Syncing OpenAPI spec to embedded location..."
	@cp docs/api/openapi.yaml server/internal/api/swagger/openapi.yaml
	@echo ""
	@echo "============================================"
	@echo "  OpenAPI spec valid and synced!"
	@echo "  Source: docs/api/openapi.yaml"
	@echo "  Embed:  server/internal/api/swagger/openapi.yaml"
	@echo "============================================"

# ===== Helm Chart =====
helm-lint:
	@echo "Linting Helm chart..."
	@command -v helm >/dev/null 2>&1 || { echo "Error: helm not found. Install from https://helm.sh"; exit 1; }
	helm lint deploy/charts/knodex
	@echo ""
	@echo "============================================"
	@echo "  Chart lint passed!"
	@echo "============================================"

helm-template:
	@echo "Rendering Helm chart templates..."
	@command -v helm >/dev/null 2>&1 || { echo "Error: helm not found. Install from https://helm.sh"; exit 1; }
	helm dependency build deploy/charts/knodex
	helm template knodex deploy/charts/knodex --values deploy/charts/knodex/values.yaml
	@echo ""
	@echo "============================================"
	@echo "  Template rendering passed!"
	@echo "============================================"

helm-package:
	@echo "Packaging Helm chart..."
	@command -v helm >/dev/null 2>&1 || { echo "Error: helm not found. Install from https://helm.sh"; exit 1; }
	helm dependency build deploy/charts/knodex
	helm package deploy/charts/knodex --destination bin/
	@echo ""
	@echo "============================================"
	@echo "  Chart packaged to bin/"
	@echo "============================================"
