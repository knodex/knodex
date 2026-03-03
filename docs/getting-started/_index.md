---
title: "Getting Started"
linkTitle: "Getting Started"
description: "Get knodex running and deploy your first instance in minutes"
weight: 10
product_tags:
  - oss
  - enterprise
---

# Getting Started

This guide walks you through installing knodex and deploying your first RGD instance.

## Requirements

- Kubernetes cluster (1.32+)
- [KRO](https://kro.run) installed ( version 0.7.1 )
- kubectl configured
- Helm 4.x

## 1. Install knodex

```bash
helm install knodex oci://ghcr.io/knodex/charts/knodex \
  --namespace knodex \
  --create-namespace
```

## 2. Access the UI

Port-forward to access the UI:

```bash
kubectl port-forward -n knodex svc/knodex-server 8080:8080
```

Open [http://localhost:8080](http://localhost:8080)

## 3. Login

Get the auto-generated admin password:

```bash
kubectl get secret knodex-initial-admin-password \
  -n knodex \
  -o jsonpath='{.data.password}' | base64 -d && echo
```

Login with:

- **Username:** `admin`
- **Password:** (output from above)

## 4. Deploy a Sample RGD

If your cluster doesn't have any RGDs yet, deploy a sample:

```bash
kubectl apply -f https://raw.githubusercontent.com/awslabs/kro/main/examples/simple-webapp/rgd.yaml
```

Refresh the catalog in the UI.

## 5. Deploy Your First Instance

1. Click on an RGD card in the catalog
2. Click **Deploy**
3. Fill in the required fields:
   - **Name:** `my-first-instance`
   - **Namespace:** `default`
   - Configure any RGD-specific fields
4. Click **Deploy Instance**

## 6. Verify Deployment

Check instance status in the UI or via kubectl:

```bash
kubectl get all -l knodex.io/instance=my-first-instance -n default
```

## Next Steps

| Goal                  | Guide                                                    |
| --------------------- | -------------------------------------------------------- |
| Configure OIDC/SSO    | [OIDC Integration](../operator-manual/oidc-integration/) |
| Set up organizations  | [Organizations](../platform-guide/organizations/)        |
| Configure RBAC        | [RBAC Setup](../operator-manual/rbac-setup/)             |
| Enable GitOps         | [Repositories](../platform-guide/repositories/)          |
| Production deployment | [Installation](../operator-manual/installation/)         |
