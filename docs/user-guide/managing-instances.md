---
title: "Managing Instances"
linkTitle: "Managing Instances"
description: "Monitor deployed instances and view their status in knodex"
weight: 3
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Managing Instances

Monitor deployed instances and view their status in knodex.

## Viewing Instance Status

After deploying an instance from the catalog, you can monitor its status from the Instances page.

### Accessing Your Instances

1. Navigate to **Instances** in the left sidebar
2. Select your project from the project selector (if you have access to multiple projects)
3. View all instances deployed in your project

### Instance Status

Each instance displays its current status:

| Status          | Description                            |
| --------------- | -------------------------------------- |
| **Ready**       | All resources are healthy and running  |
| **Progressing** | Resources are being created or updated |
| **Degraded**    | Some resources are not healthy         |
| **Failed**      | Instance deployment failed             |

### Instance Details

Click on an instance to view:

- **Name**: The instance identifier
- **RGD**: The ResourceGraphDefinition used to create this instance
- **Namespace**: Where the instance is deployed
- **Created**: When the instance was deployed
- **Status**: Current health status

### Deleting an Instance

To delete an instance:

1. Navigate to the instance you want to delete
2. Click the **Delete** button
3. Confirm the deletion in the dialog

{{< alert title="Warning" color="warning" >}}
Deleting an instance removes all associated Kubernetes resources. This action cannot be undone.
{{< /alert >}}

## Coming Soon

The following features are planned for future releases:

- **Update Instance**: Modify instance parameters after deployment
- **View Deployment Logs**: Access logs from deployed resources directly in the UI

---

**Next:** [Browsing the Catalog](../browsing-catalog/) | **Previous:** [Deploying Instances](../deploying-instances/)
