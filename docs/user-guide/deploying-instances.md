---
title: "Deploying Instances"
linkTitle: "Deploying Instances"
description: "Step-by-step guide to deploying applications using knodex"
weight: 2
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Deploying Instances

Step-by-step guide to deploying applications using knodex.

## Overview

Deploying an instance creates Kubernetes resources based on ResourceGraphDefinition (RGD) templates. The deployment process includes:

1. **Select RGD**: Choose application template
2. **Configure**: Fill deployment form
3. **Preview**: Review generated YAML
4. **Deploy**: Submit to Kubernetes
5. **Monitor**: Watch deployment progress

## Deployment Methods

### Method 1: Via Catalog (Recommended)

1. Navigate to **Catalog**
2. Search/filter for desired RGD
3. Click **Deploy** button on RGD card
4. Fill deployment form
5. Click **Deploy Now**

### Method 2: Via RGD Details Page

1. Navigate to **Catalog**
2. Click on RGD card to open details
3. Review schema and examples
4. Click **Deploy** button (top right)
5. Fill deployment form
6. Click **Deploy Now**

### Method 3: From Instance Detail (Add-on Deployment)

Deploy an add-on directly from a parent instance's detail page:

1. Navigate to a running instance
2. Find the **Deploy on this instance** section (above deployment history)
3. Browse available add-ons for this instance's Kind
4. Click **Deploy** on an add-on
5. The deploy form opens with the external reference fields pre-filled (instance name and namespace)
6. Fill remaining parameters
7. Click **Deploy Now**

**Use Case:** Deploy monitoring, logging, or other add-ons that reference an existing parent instance. The external reference is pre-populated so the add-on knows which parent instance to attach to.

### Method 4: From Example Template

1. Open RGD details page
2. Click **Examples** tab
3. Find example matching your use case
4. Click **Deploy with this config**
5. Pre-filled form opens
6. Modify as needed
7. Click **Deploy Now**

## Deployment Form

### Basic Information

Required fields for all deployments:

| Field             | Description            | Example                         |
| ----------------- | ---------------------- | ------------------------------- |
| **Instance Name** | Unique identifier      | `my-webapp-dev`                 |
| **Namespace**     | Organization namespace | `kro-engineering` (auto-filled) |
| **Labels**        | Metadata tags          | `app=webapp, env=dev`           |

**Instance Name Requirements:**

- Lowercase letters, numbers, hyphens only
- Must start with letter
- Must end with letter or number
- 3-63 characters
- Unique within namespace

### RGD-Specific Parameters

Parameters defined by the RGD schema:

#### Example: Nginx Web Application

```yaml
┌─────────────────────────────────────────────────────────┐
│ Deploy Nginx Web Application                           │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Instance Name: *                                       │
│  ┌─────────────────────────────────────────────────┐   │
│  │ my-webapp-dev                                   │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  Replicas: *                                            │
│  ┌───┐                                                  │
│  │ 2 │  (Min: 1, Max: 10)                              │
│  └───┘                                                  │
│                                                         │
│  Image: *                                               │
│  ┌─────────────────────────────────────────────────┐   │
│  │ nginx:1.25                                      │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  Port:                                                  │
│  ┌────┐                                                 │
│  │ 80 │  (Default: 80)                                 │
│  └────┘                                                 │
│                                                         │
│  [Advanced Options ▼]                                   │
│                                                         │
│  [◀ Back]  [Preview YAML]  [Deploy Now]               │
└─────────────────────────────────────────────────────────┘
```

### Advanced Options

Expandable section with optional parameters:

#### Resource Requests/Limits

```yaml
Resources:
  Requests:
    CPU: ┌──────────┐ (e.g., 100m, 500m, 1)
      │ 250m     │
      └──────────┘
    Memory: ┌──────────┐ (e.g., 128Mi, 512Mi, 1Gi)
      │ 512Mi    │
      └──────────┘

  Limits:
    CPU: ┌──────────┐
      │ 1000m    │
      └──────────┘
    Memory: ┌──────────┐
      │ 2Gi      │
      └──────────┘
```

#### Ingress Configuration

```yaml
Ingress:
  Enabled: [✓] Yes  [ ] No

  Host: ┌───────────────────────────────┐
        │ my-webapp.example.com         │
        └───────────────────────────────┘

  TLS: [✓] Enabled

  Certificate: ┌──────────────────────┐
               │ letsencrypt-prod     │
               └──────────────────────┘
```

#### Environment Variables

```yaml
Environment Variables:
┌───────────────┬─────────────────────────┐
│ Key           │ Value                   │
├───────────────┼─────────────────────────┤
│ DATABASE_URL  │ postgres://db:5432/app  │
│ REDIS_URL     │ redis://redis:6379      │
│ LOG_LEVEL     │ info                    │
└───────────────┴─────────────────────────┘

[+ Add Variable]
```

### Form Validation

**Real-time validation indicators:**

✅ **Valid field** - Green checkmark

```
Instance Name: ┌─────────────────┐ ✓
               │ my-webapp-dev   │
               └─────────────────┘
```

❌ **Invalid field** - Red error message

```
Instance Name: ┌─────────────────┐ ✗
               │ MyWebApp        │
               └─────────────────┘
               Must be lowercase
```

⚠️ **Warning** - Yellow warning (not blocking)

```
Replicas: ┌───┐ ⚠
          │ 10│
          └───┘
          High replica count may exceed quota
```

## YAML Preview

### Preview Before Deploy

Click **Preview YAML** to see generated manifest:

```yaml
apiVersion: kro.run/v1alpha1
kind: WebApplication
metadata:
  name: my-webapp-dev
  namespace: kro-engineering
  labels:
    app: webapp
    environment: dev
  annotations:
    knodex.io/deployed-by: "alice@example.com"
    knodex.io/deployed-at: "2024-01-20T17:30:00Z"
spec:
  replicas: 2
  image: nginx:1.25
  port: 80
  resources:
    requests:
      cpu: 250m
      memory: 512Mi
    limits:
      cpu: 1000m
      memory: 2Gi
  ingress:
    enabled: true
    host: my-webapp.example.com
    tls:
      enabled: true
      secretName: letsencrypt-prod
  env:
    - name: DATABASE_URL
      value: "postgres://db:5432/app"
    - name: REDIS_URL
      value: "redis://redis:6379"
    - name: LOG_LEVEL
      value: "info"
```

**YAML Editor Features:**

- **Syntax Highlighting**: Keywords, values, errors
- **Line Numbers**: Easy reference
- **Copy Button**: Copy YAML to clipboard
- **Edit Inline**: Modify YAML directly (advanced users)
- **Validation**: Real-time YAML syntax checking

### Edit YAML Directly

**For advanced users:**

1. Click **Edit YAML** button
2. Modify manifest directly
3. Changes reflected in form
4. Click **Apply Changes**

{{< alert color="warning" title="Advanced Feature" >}}
Direct YAML editing bypasses form validation. Ensure valid Kubernetes YAML.
{{< /alert >}}

## Deploying the Instance

### Final Checks

Before clicking **Deploy Now**, verify:

- ✅ Instance name is unique
- ✅ All required fields filled
- ✅ Resource requests are reasonable
- ✅ Image name and tag are correct
- ✅ Environment variables are set

### Deploy Button

Click **Deploy Now** to create instance.

**Deployment Progress:**

```
Deploying my-webapp-dev...

✓ Validating manifest
✓ Creating instance resource
✓ Waiting for pods to start
⏳ Pulling container image...
```

**Expected Duration:**

| Stage             | Time               |
| ----------------- | ------------------ |
| Validation        | < 1 second         |
| Resource creation | 1-2 seconds        |
| Image pull        | 10-60 seconds      |
| Pod startup       | 5-30 seconds       |
| **Total**         | **~30-90 seconds** |

### Success Confirmation

```
✓ Deployment Successful!

Instance: my-webapp-dev
Status: Running
Pods: 2/2 ready
URL: https://my-webapp.example.com

[View Instance] [Deploy Another]
```

### Deployment Failures

**Common Errors:**

#### Error: "Instance name already exists"

**Solution:** Choose a different name or delete existing instance

---

#### Error: "Insufficient resources"

```
Pod cannot be scheduled: Insufficient cpu (requested 500m, available 200m)
```

**Solution:**

- Reduce CPU/memory requests
- Contact Platform Admin to increase quota

---

#### Error: "Image pull failed"

```
Failed to pull image "nginx:invalid-tag": not found
```

**Solution:**

- Verify image name and tag
- Ensure registry is accessible
- Check image pull secrets (if private registry)

## Advanced Deployment Options

### Dry Run

Preview deployment without creating resources:

1. Fill deployment form
2. Click **Dry Run** button
3. Review what would be created
4. No resources are actually deployed

**Use Case:** Testing manifests, validating configurations

### Deploy with Approval

For production deployments with approval workflow:

1. Fill deployment form
2. Click **Request Approval**
3. Deployment request sent to approvers
4. Wait for approval
5. Instance deployed after approval

**Status:** "Pending Approval" until approved/rejected

### Deploy from Git

Deploy instance directly from repository:

1. Navigate to **Repositories** tab
2. Select repository
3. Click **Deploy from Git**
4. Choose file (e.g., `/deploy/prod.yaml`)
5. Preview manifest
6. Click **Deploy**

**Use Case:** GitOps workflows, version-controlled deployments

## Deployment Templates

### Save as Template

Save configuration for reuse:

1. Fill deployment form
2. Click **Save as Template**
3. Enter template name: `Nginx Dev Template`
4. Template saved to your profile

### Use Saved Template

Quick deploy with saved configuration:

1. Click **My Templates** in toolbar
2. Select template
3. Pre-filled form opens
4. Modify if needed
5. Deploy

**Use Case:** Deploying similar instances repeatedly

**Next:** [Managing Instances](../managing-instances/) →
