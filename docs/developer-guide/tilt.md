# Tilt Local Development

Tilt provides a fast, iterative development experience for Kubernetes. Instead of rebuilding Docker images and redeploying for every change, Tilt watches your files and syncs changes directly into running containers.

## Prerequisites

### Required Tools

| Tool | Version | Installation |
|------|---------|--------------|
| Tilt | Latest | `brew install tilt` or [install.sh](https://docs.tilt.dev/install.html) |
| Kind | 0.20+ | `brew install kind` |
| kubectl | 1.28+ | `brew install kubectl` |
| Docker | 24+ | [Docker Desktop](https://www.docker.com/products/docker-desktop) |

### Verify Installation

```bash
# Check Tilt
tilt version

# Check Kind cluster exists
kind get clusters | grep knodex-qa
```

## Quick Start

```bash
# 1. Create Kind cluster (one-time setup)
make cluster-up

# 2. Start Tilt
make tilt-up

# 3. Open Tilt UI in browser
# http://localhost:10350
```

That's it! Tilt will:
- Build development Docker images
- Deploy to your Kind cluster
- Watch for file changes
- Automatically sync/rebuild as needed

## How It Works

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Your Machine                             │
├──────────────────────────┬──────────────────────────────────────┤
│     Source Files         │           Tilt                        │
│                          │                                       │
│  server/                │   ┌─────────────────────────────┐    │
│    ├── main.go      ────►│   │  Watch for changes          │    │
│    └── internal/    ────►│   │  Sync Go files to container │    │
│                          │   │  Run Vite locally            │    │
│  web/               │   │  Show logs in UI             │    │
│    └── src/         ────►│   └─────────────────────────────┘    │
│                          │              │                        │
│   ┌────────────────┐    │              │                        │
│   │ Vite Dev Server │    │              │                        │
│   │ localhost:3000  │    │              │                        │
│   │ (local process) │    │              │                        │
│   └────────────────┘    │              │                        │
├──────────────────────────┴──────────────┼────────────────────────┤
│                     Kind Cluster        │                        │
│                                         ▼                        │
│   ┌───────────────────────────────┐        ┌─────────────┐      │
│   │ knodex-server                 │        │    Redis    │      │
│   │  Go server + embedded web     │        │  Standard   │      │
│   │  App:  http://localhost:8080  │        │  Port: 6379 │      │
│   └───────────────────────────────┘        └─────────────┘      │
└─────────────────────────────────────────────────────────────────┘
```

### Live Update Flow

**Server (Go):**
1. You edit a `.go` file
2. Tilt detects the change
3. Files are synced to the container
4. Air (hot-reload tool) rebuilds the binary
5. New binary starts automatically
6. **Total time: ~10-15 seconds**

**Web (React):**
1. You edit a `.tsx` or `.css` file
2. Vite dev server (running locally via Tilt) detects the change
3. Vite's HMR pushes the update to the browser
4. Browser updates without refresh
5. **Total time: ~1-2 seconds**

## Usage

### Starting Tilt

```bash
# Standard start
make tilt-up

# Start in background (detached)
tilt up -d

# Start with specific namespace
tilt up -- --namespace=my-namespace
```

### Stopping Tilt

```bash
# Stop Tilt (keeps resources)
# Press Ctrl+C in terminal

# Stop and cleanup
make tilt-down
```

### Tilt UI

Open http://localhost:10350 to access the Tilt UI.

**Key Features:**
- **Resource Status**: See build/deploy status for each service
- **Logs**: Live streaming logs from all containers
- **Triggers**: Manually trigger rebuilds if needed
- **Errors**: Clear error messages with source locations

**Keyboard Shortcuts:**
| Key | Action |
|-----|--------|
| `s` | Open logs for selected resource |
| `r` | Trigger rebuild for selected resource |
| `j/k` | Navigate up/down |
| `Enter` | Select resource |
| `?` | Show help |

### Port Forwarding

Tilt automatically sets up port forwarding:

| Service | URL | Description |
|---------|-----|-------------|
| Application | http://localhost:8080 | Go server (API + embedded web) |
| Vite HMR | http://localhost:3000 | Vite dev server (hot module reload) |
| Redis | localhost:6379 | Redis cache |
| Tilt UI | http://localhost:10350 | Tilt dashboard |

Use `localhost:3000` during active web development for instant HMR updates. Use `localhost:8080` to test the production-like embedded build.

## Development Workflow

### Typical Session

```bash
# 1. Start your day
make tilt-up

# 2. Open Tilt UI to monitor
open http://localhost:10350

# 3. Open app in browser
open http://localhost:3000   # Vite HMR (or http://localhost:8080 for embedded)

# 4. Make code changes
# - Server: Edit Go files → Auto-rebuild in ~10s
# - Web: Edit React files → HMR in <2s

# 5. Check logs in Tilt UI if something breaks

# 6. End of day
make tilt-down
```

### Forcing Rebuilds

Sometimes you need to force a full rebuild:

```bash
# In Tilt UI: Click the refresh icon on a resource

# Or via CLI:
tilt trigger knodex-server
tilt trigger web-dev
```

### Viewing Logs

**In Tilt UI:**
1. Click on a resource
2. Press `s` to open logs
3. Use scroll to navigate

**Via CLI:**
```bash
# All logs
tilt logs

# Specific resource
tilt logs knodex-server

# Follow mode
tilt logs -f knodex-server
```

## Configuration

### Tiltfile Options

The Tiltfile supports these arguments:

```bash
# Use custom namespace
tilt up -- --namespace=my-dev

# Enable enterprise build
tilt up -- --enterprise=true
```

### Environment Variables

Set these in your shell or `.env` file:

```bash
# Use a different cluster context
export KUBECONFIG=~/.kube/config

# Tilt-specific settings
export TILT_PORT=10350  # UI port
```

## Troubleshooting

### Kind Cluster Not Found

**Error:** `No Kubernetes cluster available. Run 'make cluster-up' first.`

**Solution:**
```bash
# Check if cluster exists
kind get clusters

# If not, create it
make cluster-up
```

### Port Already in Use

**Error:** `bind: address already in use`

**Solution:**
```bash
# Find what's using the port
lsof -i :8080

# Kill the process or stop the other service
kill -9 <PID>

# Or use different ports
tilt up -- --namespace=alt-namespace
```

### Container Crash Loop

**Symptoms:** Pod keeps restarting, red status in Tilt UI

**Debugging:**
1. Click on the crashing resource in Tilt UI
2. Press `s` to view logs
3. Look for error messages at startup

**Common causes:**
- Go compilation error (check syntax)
- Missing environment variable
- Database/Redis connection failure

### Files Not Syncing

**Symptoms:** Changes not appearing in container

**Solutions:**
1. Check if file is in ignore list (Tiltfile `ignore` parameter)
2. Trigger manual sync: `tilt trigger <resource>`
3. Check Tilt logs for sync errors

### Slow Rebuilds

**Server taking too long?**
- Ensure `go mod download` cache is preserved
- Check if tests are accidentally running
- Look for expensive initialization in `main()`

**Web taking too long?**
- Check for large node_modules changes
- Ensure Vite cache is working
- Look for heavy imports

### Full Reset

When all else fails:

```bash
# Stop everything
make tilt-down

# Clean up namespace
kubectl delete namespace knodex-tilt --ignore-not-found

# Remove cached images
docker system prune -f

# Start fresh
make tilt-up
```

## Comparison with Other Methods

| Method | First Deploy | Code Change | Best For |
|--------|-------------|-------------|----------|
| `make dev` | Instant | Instant | Simple local dev |
| `make qa` | 2-5 min | 2-5 min | E2E testing |
| **Tilt** | 30-60s | 2-15s | K8s-integrated dev |

**When to use Tilt:**
- Testing Kubernetes-specific features (ConfigMaps, Secrets, RBAC)
- Working on features that need full cluster environment
- Debugging pod networking or service discovery
- Testing with real Redis in-cluster

**When to use `make dev`:**
- Simple web/server changes
- Quick iteration without K8s
- Lower resource usage

## Related Resources

- [Tilt Documentation](https://docs.tilt.dev/)
- [Testing](./testing.md) - Test guide and E2E testing
