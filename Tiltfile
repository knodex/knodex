# Tiltfile for knodex local development
# See: https://docs.tilt.dev/

# Configuration
config.define_string('namespace')
config.define_bool('enterprise')
cfg = config.parse()

# Default namespace
namespace = cfg.get('namespace', 'knodex-tilt')

# Enterprise build flag
enterprise_build = cfg.get('enterprise', False)

# ============================================================================
# Cluster Setup
# ============================================================================

# Ensure we're using the correct Kind cluster
allow_k8s_contexts('kind-knodex-qa')

# Check if cluster is accessible
local_resource(
    'cluster-check',
    cmd='kubectl cluster-info --context kind-knodex-qa > /dev/null 2>&1 || (echo "Kind cluster not found. Run: make cluster-up" && exit 1)',
    auto_init=True,
    trigger_mode=TRIGGER_MODE_MANUAL,
)

# ============================================================================
# Kubernetes Resources
# ============================================================================

# Apply Tilt-specific Kubernetes manifests
k8s_yaml(kustomize('./scripts/overlays/tilt'))

# ============================================================================
# Server (Go) - serves both API and embedded web
# ============================================================================

# Build server dev image with live update
docker_build(
    'knodex-server-dev',
    context='./server',
    dockerfile='./server/Dockerfile.dev',
    live_update=[
        # Sync Go source files
        sync('./server', '/app'),
        # Air will automatically rebuild when files change
    ],
    # Ignore test files and tmp directory
    ignore=[
        '*_test.go',
        'tmp/',
        '.air.toml.log',
    ],
)

# Configure server Kubernetes resource
k8s_resource(
    'knodex-server',
    port_forwards=[
        port_forward(8088, 8080, name='Application'),
    ],
    labels=['server'],
    resource_deps=['cluster-check'],
)

# ============================================================================
# Redis
# ============================================================================

# Redis uses the standard image, no build needed
k8s_resource(
    'knodex-redis',
    port_forwards=[
        port_forward(6379, 6379, name='Redis'),
    ],
    labels=['infrastructure'],
)

# ============================================================================
# Enterprise Data (Compliance + Views)
# ============================================================================

if enterprise_build:
    # Deploy Gatekeeper constraint templates and constraints for compliance UI
    local_resource(
        'enterprise-compliance-data',
        cmd="""
set -e
# Check if Gatekeeper CRDs are installed
if ! kubectl get crd constrainttemplates.templates.gatekeeper.sh --context kind-knodex-qa >/dev/null 2>&1; then
    echo "Gatekeeper CRDs not found. Installing OPA Gatekeeper..."
    helm repo add gatekeeper https://open-policy-agent.github.io/gatekeeper/charts 2>/dev/null || true
    helm repo update >/dev/null 2>&1
    helm install gatekeeper gatekeeper/gatekeeper \
        --namespace gatekeeper-system --create-namespace \
        --version v3.17.1 \
        --set replicas=1 \
        --set audit.replicas=1 \
        --kube-context kind-knodex-qa \
        --wait --timeout 180s
    echo "Gatekeeper installed"
fi

# Apply constraint templates first
kubectl apply -f deploy/examples/gatekeeper/constraint-templates.yaml --context kind-knodex-qa
echo "Waiting for Gatekeeper CRDs to be ready..."
sleep 5

# Apply constraints (retry once if CRDs aren't ready yet)
kubectl apply -f deploy/examples/gatekeeper/constraints.yaml --context kind-knodex-qa || {
    echo "Retrying constraint deployment..."
    sleep 5
    kubectl apply -f deploy/examples/gatekeeper/constraints.yaml --context kind-knodex-qa
}
echo "Compliance data deployed"
""",
        resource_deps=['cluster-check'],
        labels=['enterprise'],
    )

    # Install CAPI + CAPZ (Cluster API Provider Azure) with ASO CRDs
    # Required for aks-cilium ClusterClass templates and Azure RGD reconciliation
    local_resource(
        'enterprise-capi-capz',
        cmd="""
set -e
CONTEXT="kind-knodex-qa"

# Check if CAPI is already installed
if kubectl get crd clusters.cluster.x-k8s.io --context "$CONTEXT" >/dev/null 2>&1; then
    echo "CAPI + CAPZ already installed, skipping"
    exit 0
fi

echo "==> Installing Cluster API + CAPZ with ASO CRDs..."

# ASO CRDs required by knodex-azure-catalog RGDs (beyond CAPZ defaults)
# Format: group.azure.com/* (semicolon-separated, glob patterns matching group/Kind)
export ADDITIONAL_ASO_CRDS="\\
authorization.azure.com/*;\\
containerregistry.azure.com/*;\\
containerservice.azure.com/*;\\
dbformysql.azure.com/*;\\
dbforpostgresql.azure.com/*;\\
keyvault.azure.com/*;\\
managedidentity.azure.com/*;\\
network.azure.com/*;\\
resources.azure.com/*;\\
storage.azure.com/*"

# Dummy Azure credentials (CRDs only — controllers won't reconcile without real creds)
export AZURE_SUBSCRIPTION_ID_B64="$(echo -n '00000000-0000-0000-0000-000000000000' | base64)"
export AZURE_TENANT_ID_B64="$(echo -n '00000000-0000-0000-0000-000000000000' | base64)"
export AZURE_CLIENT_ID_B64="$(echo -n '00000000-0000-0000-0000-000000000000' | base64)"
export AZURE_CLIENT_SECRET_B64="$(echo -n 'dummy' | base64)"
export EXP_MACHINE_POOL=true
export CLUSTER_TOPOLOGY=true

# Install CAPI + CAPZ (ASO may timeout on first start with dummy creds — that's OK)
clusterctl init --infrastructure azure --kubeconfig-context "$CONTEXT" --wait-providers || true

# Wait for ASO to install CRDs (it restarts a few times before settling)
echo "==> Waiting for ASO CRDs to be installed..."
for i in $(seq 1 30); do
    if kubectl get crd resourcegroups.resources.azure.com flexibleservers.dbforpostgresql.azure.com \
        --context "$CONTEXT" >/dev/null 2>&1; then
        echo "    ASO CRDs ready"
        break
    fi
    echo "    Waiting for ASO CRDs... ($i/30)"
    sleep 10
done

# Install Flux CRDs (source-controller + helm-controller)
echo "==> Installing Flux CRDs..."
kubectl apply --context "$CONTEXT" \
    -f https://github.com/fluxcd/source-controller/releases/latest/download/source-controller.crds.yaml 2>&1 | tail -1
kubectl apply --context "$CONTEXT" \
    -f https://github.com/fluxcd/helm-controller/releases/latest/download/helm-controller.crds.yaml 2>&1 | tail -1

# Install CNPG CRDs (use main branch for latest schema with probes.readiness.maximumLag)
echo "==> Installing CNPG CRDs..."
kubectl apply --context "$CONTEXT" --server-side --force-conflicts \
    -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/config/crd/bases/postgresql.cnpg.io_clusters.yaml 2>&1 | tail -1
kubectl apply --context "$CONTEXT" --server-side --force-conflicts \
    -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/config/crd/bases/postgresql.cnpg.io_scheduledbackups.yaml 2>&1 | tail -1
kubectl apply --context "$CONTEXT" --server-side --force-conflicts \
    -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/config/crd/bases/postgresql.cnpg.io_backups.yaml 2>&1 | tail -1
kubectl apply --context "$CONTEXT" --server-side --force-conflicts \
    -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/config/crd/bases/postgresql.cnpg.io_poolers.yaml 2>&1 | tail -1

# Install External Secrets CRDs (server-side for large CRDs)
echo "==> Installing External Secrets CRDs..."
kubectl apply --context "$CONTEXT" --server-side --force-conflicts \
    -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml 2>&1 | tail -1

# Install SAP Valkey operator CRD
echo "==> Installing Valkey operator CRD..."
kubectl apply --context "$CONTEXT" \
    -f https://raw.githubusercontent.com/SAP/valkey-operator/main/crds/cache.cs.sap.com_valkeys.yaml 2>&1 | tail -1

# Restart KRO to pick up new CRDs
echo "==> Restarting KRO to discover new CRDs..."
kubectl rollout restart deployment/kro -n kro-system --context "$CONTEXT"
kubectl rollout status deployment/kro -n kro-system --context "$CONTEXT" --timeout=60s

echo "==> CAPI + CAPZ + dependency CRDs installed"
kubectl get crd --context "$CONTEXT" -o name | grep -c "azure.com" | xargs -I{} echo "    {} ASO CRDs installed"
kubectl get crd clusterclasses.cluster.x-k8s.io --context "$CONTEXT" >/dev/null && echo "    ClusterClass CRD ready"
""",
        resource_deps=['cluster-check'],
        labels=['enterprise'],
    )

    # Deploy Azure and Controllers catalog RGDs from knodex-azure-package
    local_resource(
        'enterprise-azure-catalog',
        cmd="""
set -e
CONTEXT="kind-knodex-qa"
REPO_DIR="/tmp/knodex-azure-package"

# Clone or update the repo
if [ -d "$REPO_DIR/.git" ]; then
    echo "==> Updating knodex-azure-package..."
    git -C "$REPO_DIR" pull --ff-only 2>/dev/null || true
else
    echo "==> Cloning knodex-azure-package..."
    rm -rf "$REPO_DIR"
    git clone --depth 1 https://github.com/provops-org/knodex-azure-package.git "$REPO_DIR"
fi

# Render and apply knodex-controllers-catalog
echo "==> Deploying knodex-controllers-catalog RGDs..."
helm template knodex-controllers-catalog "$REPO_DIR/charts/knodex-controllers-catalog" \
    | kubectl apply --context "$CONTEXT" -f -

# Render and apply knodex-azure-catalog (management mode with all CRDs available)
echo "==> Deploying knodex-azure-catalog RGDs..."
helm template knodex-azure-catalog "$REPO_DIR/charts/knodex-azure-catalog" \
    --set mode=management \
    --set knodex.iconRegistry.enabled=false \
    | kubectl apply --context "$CONTEXT" --server-side --force-conflicts -f -

echo "==> Azure + Controllers catalog RGDs deployed"
""",
        resource_deps=['enterprise-capi-capz'],
        labels=['enterprise'],
    )

# ============================================================================
# Dev Project & RBAC Seed Data
# ============================================================================

# Seed example RGDs into the catalog (infrastructure, applications, observability, examples categories).
local_resource(
    'seed-example-rgds',
    cmd='kubectl apply -f deploy/examples/rgds/ --context kind-knodex-qa',
    resource_deps=['knodex-server'],
    labels=['infrastructure'],
)

# Create the engineering project with 3 namespaces and 2 roles (operator, developer).
# Operator sees infrastructure + observability RGDs; developer sees applications + examples.
local_resource(
    'dev-project-seed',
    cmd='kubectl apply -f scripts/overlays/tilt/dev-project.yaml -n ' + namespace + ' --context kind-knodex-qa',
    resource_deps=['seed-example-rgds'],
    labels=['infrastructure'],
)

# Create local users (operator, developer) and assign project roles.
local_resource(
    'dev-users-seed',
    cmd='bash scripts/overlays/tilt/seed-users.sh',
    resource_deps=['dev-project-seed'],
    labels=['infrastructure'],
)

# Seed a cluster-provisioner instance with the current Kind kubeconfig.
# Creates a kubeconfig Secret in eng-shared for the flux-app RGD to consume.
local_resource(
    'seed-cluster',
    cmd='bash scripts/overlays/tilt/seed-cluster.sh',
    resource_deps=['dev-users-seed'],
    labels=['infrastructure'],
)

# ============================================================================
# Developer Experience
# ============================================================================

# Update trigger for forced rebuilds
local_resource(
    'force-rebuild-server',
    cmd='echo "Triggering server rebuild..."',
    trigger_mode=TRIGGER_MODE_MANUAL,
    auto_init=False,
    labels=['tools'],
)

# Web dev server (Vite HMR) - runs locally, proxies API to server
web_dev_cmd = 'cd web && npx vite --mode enterprise' if enterprise_build else 'cd web && npm run dev'

local_resource(
    'web-dev',
    serve_cmd=web_dev_cmd,
    labels=['web'],
    resource_deps=['knodex-server'],
)

# ============================================================================
# Tilt UI Configuration
# ============================================================================

# Group resources for better UI organization
update_settings(
    max_parallel_updates=3,
    k8s_upsert_timeout_secs=60,
)

# Custom buttons in Tilt UI
local_resource(
    'open-app',
    cmd='echo "Opening app at http://localhost:8088 (or http://localhost:3000 for Vite HMR)"',
    trigger_mode=TRIGGER_MODE_MANUAL,
    auto_init=False,
    labels=['tools'],
)

# Print startup message
edition_label = 'Enterprise' if enterprise_build else 'OSS'
enterprise_info = """║                                                                  ║
║  Enterprise Features:                                            ║
║    - Compliance: Gatekeeper templates, constraints, violations  ║
║    - Azure Catalog: AKS, PostgreSQL, MySQL, KeyVault RGDs       ║
║    - Controllers Catalog: cert-manager, CNPG, ArgoCD, ESO       ║
║    - CAPI + CAPZ v1.23.0 + ASO v2.13.0 (ClusterClass ready)    ║
║    - Audit trail, License management                              ║""" if enterprise_build else ''

print("""
╔══════════════════════════════════════════════════════════════════╗
║              Knodex Tilt Development ({edition})                ║
╠══════════════════════════════════════════════════════════════════╣
║                                                                  ║
║  Services:                                                       ║
║    Application:    http://localhost:8088  (API + embedded UI)    ║
║    Web Dev:        http://localhost:3000  (Vite HMR)            ║
║    Redis:          localhost:6379                                ║
║    Tilt UI:        http://localhost:10350                        ║
{enterprise}║                                                                  ║
║  Tips:                                                           ║
║    - Edit Go files → Air auto-rebuilds (10-15s)                  ║
║    - Edit React files → Vite HMR updates (<2s)                   ║
║    - Use localhost:5173 during web development                   ║
║    - Use localhost:8080 for production-like testing              ║
║    - Press 's' in Tilt UI to see logs                            ║
║    - Press 'r' to manually trigger resource rebuild              ║
║                                                                  ║
╚══════════════════════════════════════════════════════════════════╝
""".format(edition=edition_label, enterprise=enterprise_info))
