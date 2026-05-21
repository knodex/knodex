---
title: Tilt Local Development
description: Fast iterative development with Tilt, Kind, and live reloading for both Go server and React frontend
sidebar_position: 2
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Tilt Local Development

Tilt provides fast, iterative local development by watching source files and automatically syncing changes into your Kind cluster. This is the recommended development workflow for Knodex.

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Tilt | latest | `brew install tilt` |
| Kind | 0.20+ | `brew install kind` |
| kubectl | 1.28+ | `brew install kubectl` |
| Docker | 24+ | [docker.com](https://www.docker.com/) |

## Quick Start

```bash
# 1. Create a Kind cluster with KRO and CRDs
make cluster-up

# 2. Start Tilt
make tilt-up

# 3. Open the Tilt UI to monitor services
open http://localhost:10350
```

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                   Your Machine                        │
│                                                       │
│  ┌────────────┐    watches     ┌───────────────────┐ │
│  │ Source      │───────────────▶│     Tilt          │ │
│  │ .go / .tsx  │               │  (orchestrator)    │ │
│  └────────────┘               └────────┬──────────┘ │
│                                        │             │
│  ┌────────────────────┐    ┌──────────▼──────────┐ │
│  │  Vite Dev Server   │    │    Kind Cluster      │ │
│  │  :3000 (HMR)       │    │                      │ │
│  │                     │    │  ┌────────────────┐  │ │
│  │  Proxies API ──────────▶│  │ knodex-server  │  │ │
│  │  to :8080           │    │  │ :8080          │  │ │
│  │                     │    │  └───────┬────────┘  │ │
│  └────────────────────┘    │          │            │ │
│                             │  ┌───────▼────────┐  │ │
│                             │  │   Redis         │  │ │
│                             │  │   :6379         │  │ │
│                             │  └────────────────┘  │ │
│                             └──────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

## Live Update Flow

### Server (Go)

1. Edit a `.go` file
2. Tilt detects the change
3. Source files are synced into the container
4. Air (hot-reload tool) rebuilds the binary
5. Server restarts automatically

**Typical turnaround: ~10-15 seconds**

### Web (React/TypeScript)

1. Edit a `.tsx` or `.ts` file
2. Vite HMR picks up the change instantly
3. Browser updates without a full page reload

**Typical turnaround: ~1-2 seconds**

## Usage

### Starting Tilt

```bash
# Foreground (logs stream to terminal)
make tilt-up

# Background (detached)
tilt up -d
```

### Stopping Tilt

```bash
# If running in foreground
# Press Ctrl+C

# Cleanup resources
make tilt-down
```

### Tilt UI

The Tilt UI at `http://localhost:10350` provides:

- **Resource status**: See which services are running, building, or errored
- **Log streaming**: View logs from each service in real time
- **Trigger buttons**: Manually trigger rebuilds
- **Keyboard shortcuts**: Press `?` in the UI to see all shortcuts

## Port Forwarding

Tilt automatically sets up port forwarding for all services:

| Service | Port | URL |
|---------|------|-----|
| Knodex Server | 8080 | `http://localhost:8080` |
| Vite Dev Server | 3000 | `http://localhost:3000` |
| Redis | 6379 | `localhost:6379` |
| Tilt UI | 10350 | `http://localhost:10350` |

:::tip[Use Port 3000 for Development]
Access the app through `http://localhost:3000` during development. The Vite dev server proxies API requests to the server on `:8080` and provides HMR for instant frontend updates.
:::

## Development Workflow

### Typical Session

1. Start your cluster and Tilt:
   ```bash
   make cluster-up    # Only needed once
   make tilt-up
   ```
2. Open `http://localhost:3000` in your browser
3. Open the Tilt UI at `http://localhost:10350` in another tab
4. Edit code -- changes appear automatically
5. Check the Tilt UI if something looks wrong
6. When done, press `Ctrl+C` and optionally run `make tilt-down`

### Forcing a Rebuild

If Tilt does not pick up a change, trigger a rebuild manually:

```bash
# From the CLI
tilt trigger knodex-server

# Or click the trigger button in the Tilt UI
```

### Viewing Logs

```bash
# Stream all logs
tilt logs

# Stream logs for a specific resource
tilt logs knodex-server

# Or use the Tilt UI log panel
```

## Configuration

### Tiltfile Options

Pass flags through `make tilt-up` or directly to `tilt up`:

```bash
# Use a custom namespace
tilt up -- --namespace=my-namespace

# Enable enterprise features
tilt up -- --enterprise
```

### Environment Variables

Environment variables for the server can be set in the Tiltfile or via Kubernetes ConfigMaps. Common overrides:

| Variable | Purpose |
|----------|---------|
| `LOG_LEVEL` | Set to `debug` for verbose output |
| `SWAGGER_UI_ENABLED` | Automatically set to `true` in Tilt |
| `REDIS_ADDRESS` | Override Redis connection |

## Troubleshooting

### Cluster Not Found

```
Error: no kind cluster found
```

Run `make cluster-up` to create the Kind cluster before starting Tilt.

### Port Already in Use

```
Error: listen tcp :3000: bind: address already in use
```

Kill the process using the port:

```bash
lsof -ti :3000 | xargs kill -9
```

### Pod in CrashLoopBackOff

Check the logs in the Tilt UI or via CLI:

```bash
tilt logs knodex-server
kubectl logs -f deployment/knodex-server -n knodex
```

Common causes:
- Redis not ready yet (wait a few seconds for the init container)
- Missing environment variables
- CRDs not applied (`make cluster-up` handles this)

### Files Not Syncing

If live update seems stuck:

1. Check the Tilt UI for sync errors
2. Try `tilt trigger <resource>` to force a sync
3. Restart Tilt: `Ctrl+C`, then `make tilt-up`

### Slow Rebuilds

- Ensure Docker has adequate resources (at least 4 GB RAM, 2 CPUs)
- Close other Docker-heavy workloads
- Check that your Go module cache is warm (`go mod download`)

### Full Reset

If things are in a bad state, tear everything down and start fresh:

```bash
make tilt-down
kind delete cluster --name knodex
make cluster-up
make tilt-up
```

## Comparison: Development Modes

| | `make dev` | `make qa` | `make tilt-up` |
|---|---|---|---|
| **First deploy** | ~5s (native) | ~2-3 min (full deploy) | ~1-2 min (build + deploy) |
| **Code change** | Instant (native) | Redeploy required | ~1-2s (web), ~10-15s (server) |
| **Kubernetes** | No (runs locally) | Yes (full cluster) | Yes (Kind cluster) |
| **Redis** | External required | Deployed automatically | Deployed automatically |
| **Best for** | Quick iteration without K8s | Full integration testing | Realistic K8s development |
