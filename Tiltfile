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
        port_forward(8080, 8080, name='Application'),
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
    cmd='echo "Opening app at http://localhost:8080 (or http://localhost:3000 for Vite HMR)"',
    trigger_mode=TRIGGER_MODE_MANUAL,
    auto_init=False,
    labels=['tools'],
)

# Print startup message
print("""
╔══════════════════════════════════════════════════════════════════╗
║                    Knodex Tilt Development                      ║
╠══════════════════════════════════════════════════════════════════╣
║                                                                  ║
║  Services:                                                       ║
║    Application:    http://localhost:8080  (API + embedded UI)    ║
║    Web Dev:        http://localhost:3000  (Vite HMR)            ║
║    Redis:          localhost:6379                                ║
║    Tilt UI:        http://localhost:10350                        ║
║                                                                  ║
║  Tips:                                                           ║
║    - Edit Go files → Air auto-rebuilds (10-15s)                  ║
║    - Edit React files → Vite HMR updates (<2s)                   ║
║    - Use localhost:5173 during web development                   ║
║    - Use localhost:8080 for production-like testing              ║
║    - Press 's' in Tilt UI to see logs                            ║
║    - Press 'r' to manually trigger resource rebuild              ║
║                                                                  ║
╚══════════════════════════════════════════════════════════════════╝
""")
