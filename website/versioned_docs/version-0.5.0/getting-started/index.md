---
title: Getting Started
description: Install Knodex, deploy your first RGD instance, and learn the basics in minutes.
sidebar_position: 1
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Getting Started

This guide walks you through installing Knodex, verifying the deployment, and deploying your first instance.

## Requirements

- Kubernetes cluster (1.32+)
- [KRO](https://kro.run) — or let the chart install it (see compatibility matrix below)
- kubectl configured
- Helm 3.x

## Version Compatibility

Each Knodex release is tested against a specific KRO version. Using a different KRO version may work but is not guaranteed.

| Knodex | KRO | Kubernetes | Helm |
|--------|-----|------------|------|
| 0.5.0  | 0.9.1 | 1.32+ | 3.x |

The Helm chart bundles the matching KRO version as an optional dependency. Setting `kro.enabled=true` installs the correct version automatically.

## Install with Helm

```bash
helm install knodex oci://ghcr.io/knodex/charts/knodex \
  --namespace knodex \
  --create-namespace \
  --set kro.enabled=true
```

Omit `--set kro.enabled=true` if KRO is already installed on your cluster.

## Verify Installation

Check that all pods are running:

```bash
kubectl get pods -n knodex
```

Expected output:

```
NAME                              READY   STATUS    RESTARTS   AGE
knodex-server-6f8b9c7d4-x2k9m    1/1     Running   0          2m
knodex-redis-59ddd568d9-xxxxx     1/1     Running   0          2m
```

Verify the server is healthy:

```bash
kubectl port-forward svc/knodex-server 8080:8080 -n knodex
```

```bash
curl http://localhost:8080/healthz
# {"status":"healthy"}
```

## Log In

Get the auto-generated admin password:

```bash
kubectl get secret knodex-initial-admin-password -n knodex \
  -o jsonpath='{.data.password}' | base64 -d && echo
```

Open [http://localhost:8080](http://localhost:8080) and log in with username `admin` and the retrieved password.

## Deploy a Sample RGD

Apply a sample ResourceGraphDefinition:

```bash
kubectl apply -f https://raw.githubusercontent.com/knodex/knodex/main/deploy/examples/rgds/simple-app.yaml
```

Verify it's active:

```bash
kubectl get rgd simple-app
```

```
NAME         STATE    AGE
simple-app   Active   30s
```

The `simple-app` RGD has the `knodex.io/catalog: "true"` annotation, so it appears in the catalog automatically.

## Deploy Your First Instance

1. Navigate to the **Catalog** in the sidebar
2. Select the **Simple Application** RGD — it accepts `appName`, `image` (defaults to `nginx:latest`), and `port` (defaults to `80`)
3. Click **Deploy**, fill in `appName` (e.g., `my-first-app`), select a namespace
4. Review the YAML preview and click **Submit**

Monitor the instance in the **Instances** view. Verify with kubectl:

```bash
kubectl get simpleapps -A
```

## Upgrading

```bash
helm upgrade knodex oci://ghcr.io/knodex/charts/knodex \
  --namespace knodex \
  -f values.yaml
```

## Uninstalling

```bash
helm uninstall knodex --namespace knodex
```

To also remove the namespace and any persistent data:

```bash
kubectl delete namespace knodex
```

:::warning[Data Loss]
Deleting the namespace removes all data, including Redis state. Ensure you have backups if needed.
:::

## Next Steps

| Topic | Description |
|-------|-------------|
| [User Guide](../user-guide/) | Browse the catalog, deploy and manage instances |
| [Administration](../administration/) | Configure OIDC, RBAC, and production settings |
| [RGD Authoring](../rgd-authoring/) | Write and configure ResourceGraphDefinitions |
