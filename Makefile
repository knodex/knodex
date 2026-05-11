.PHONY: help dev dev-web dev-server dev-up dev-down db-reset \
        build build-web build-server \
        clean lint lint-fix lint-server lint-web \
        cluster-up cluster-down \
        test test-server test-web e2e qa qa-stop \
        tilt-up tilt-down tilt-status \
        docs-serve docs-build docs-clean docs-install docs-version \
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
	@echo "Development (Native - requires external Redis + Postgres):"
	@echo "  make dev-up         - Start Postgres + Redis via docker-compose (for EE builds)"
	@echo "  make dev-down       - Stop docker-compose dependencies"
	@echo "  make dev            - Start server + web (needs Redis; EE also needs Postgres)"
	@echo "  make dev-server     - Start server only"
	@echo "  make dev-web        - Start web only"
	@echo "  make db-reset       - Wipe local Postgres data (Tilt or docker-compose)"
	@echo ""
	@echo "Build:"
	@echo "  make build            - Build all services"
	@echo "  make build-server     - Build server binary"
	@echo "  make build-web        - Build web static files"
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
	@echo "  For EE builds with Postgres: run 'make dev-up' first"
	@echo "  to start docker-compose dependencies."
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
	@echo "For EE builds with Postgres: run 'make dev-up' first to start docker-compose dependencies."
	cd server && go run .

dev-web:
	@echo "Starting web..."
	cd web && npm run dev

# Bring up local Postgres + Redis via docker-compose. Used by the native dev path
# (`make dev`). The Tilt path (`make tilt-up`) provisions both inside Kind via the
# kustomize overlay, so this target is not needed when using Tilt.
dev-up:
	@echo "Starting dev dependencies (Postgres + Redis) via docker-compose..."
	docker compose up -d postgres redis
	@echo ""
	@echo "  Postgres: localhost:5432  (user=knodex db=knodex)"
	@echo "  Redis:    localhost:6379"
	@echo ""
	@echo "  Run 'make dev' to start the server."

dev-down:
	@echo "Stopping dev dependencies..."
	docker compose down
	@echo "Dev dependencies stopped."

# Reset local Postgres state. Detects whether the active environment is Tilt
# (in-cluster Pod backed by emptyDir) or docker-compose (named volume) and
# resets it idempotently. Migrations re-apply on next server connection.
db-reset:
	@echo "Detecting active Postgres environment..."
	@if kubectl get deployment knodex-postgres -n knodex-tilt >/dev/null 2>&1; then \
		echo "  Tilt-managed Postgres detected — restarting Pod (emptyDir wipe)..."; \
		kubectl rollout restart deployment/knodex-postgres -n knodex-tilt; \
		kubectl rollout status deployment/knodex-postgres -n knodex-tilt --timeout=60s; \
		echo "  Postgres restarted. Migrations re-apply on next server connection."; \
	elif docker ps --filter "name=knodex-dev-postgres" --filter "status=running" -q 2>/dev/null | grep -q .; then \
		echo "  docker-compose Postgres detected — recreating service + volume..."; \
		docker compose rm -fsv postgres; \
		docker volume rm "$$(docker compose config 2>/dev/null | sed -n 's/^name: //p')_knodex_pgdata" 2>/dev/null || true; \
		docker compose up -d postgres; \
		echo "  Postgres recreated. Migrations re-apply on next server connection."; \
	else \
		echo "  No local Postgres detected — start one with 'make tilt-up' or 'make dev-up'."; \
	fi

# ===== Build =====
# Build web first, embed into Go binary
build: build-web _embed-web build-server

build-server:
	@echo "Building server (with embedded web)..."
	cd server && CGO_ENABLED=0 go build -ldflags="-w -s" -o ../bin/knodex-server .

build-web:
	@echo "Building web..."
	cd web && npm run build

# Copy web dist into Go embed path (exclude pre-compressed .gz/.br and stats.html)
_embed-web:
	@echo "Embedding web dist into Go binary..."
	@rm -rf server/internal/static/dist
	@rsync -a --exclude='*.gz' --exclude='*.br' --exclude='stats.html' web/dist/ server/internal/static/dist/

# ===== Build - OSS Edition =====
	@echo ""
	@echo "============================================"
	@echo "  OSS build completed!"
	@echo "============================================"

	@echo "Building server (OSS, embedded web)..."
	cd server && CGO_ENABLED=0 go build -ldflags="-w -s" -o ../bin/knodex-oss .

	@echo "Building web (OSS)..."
	cd web && npm run build

# ===== Build - Enterprise Edition =====
	@echo ""
	@echo "============================================"
	@echo "============================================"


	@echo "Building web (Enterprise)..."

# ===== Build - All Editions =====
	@echo ""
	@echo "============================================"
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
tilt-up: _ensure-prereqs
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

# ===== Documentation - Docusaurus =====
docs-install:
	@echo "Installing documentation dependencies..."
	cd website && npm install
	@echo "Documentation dependencies installed successfully!"
	@echo "Run 'make docs-serve' to preview documentation locally."

docs-serve:
	@echo "Starting documentation server..."
	@echo ""
	@echo "Documentation server starting at http://localhost:4000"
	@echo "Press Ctrl+C to stop the server."
	@echo ""
	cd website && npx docusaurus start --port 4000

docs-build:
	@echo "Building documentation..."
	cd website && npm run build
	@echo ""
	@echo "Documentation built successfully!"
	@echo "Output: website/build/"

docs-version:
ifndef VERSION
	@echo "Usage: make docs-version VERSION=x.y.z"
	@echo ""
	@echo "This freezes the current docs as a versioned snapshot."
	@echo "Example: make docs-version VERSION=0.1.0"
	@exit 1
endif
	@echo "Creating documentation version $(VERSION)..."
	cd website && npx docusaurus docs:version $(VERSION)
	@echo ""
	@echo "Version $(VERSION) created!"
	@echo "Update 'lastVersion' in website/docusaurus.config.ts to '$(VERSION)'"
	@echo "Then commit: versioned_docs/, versioned_sidebars/, versions.json, docusaurus.config.ts"

docs-clean:
	@echo "Cleaning documentation build artifacts..."
	rm -rf website/build/ website/.docusaurus/
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
