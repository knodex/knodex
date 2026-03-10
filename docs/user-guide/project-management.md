---
title: "Project Management"
linkTitle: "Project Management"
description: "Manage your project team members and repositories in knodex"
weight: 6
product_tags:
  - oss
  - enterprise
---

{{< product-tag oss cloud enterprise >}}

# Project Management

Manage your project team members and repositories in knodex.

## Overview

As a **Platform Admin** of a project, you can:

- Add and remove team members from your project
- Assign roles to team members
- Configure repositories for GitOps deployments

{{< alert title="Note" >}}
**Creating projects** is restricted to Global Admins. If you need a new project, contact your platform administrator.
{{< /alert >}}

## Managing Team Members

### Viewing Current Members

1. Navigate to **Settings** in the left sidebar
2. Select your project from the project selector
3. Click on the **Members** tab

You will see a list of all users who have access to your project, along with their assigned roles.

### Adding a Team Member

1. Navigate to **Settings** → **Members**
2. Click **Add Member**
3. Enter the user's email address
4. Select a role for the user:

| Role               | Permissions                                               |
| ------------------ | --------------------------------------------------------- |
| **Platform Admin** | Full project management, can add members and repositories |
| **Developer**      | Deploy and manage instances, view catalog                 |
| **Viewer**         | Read-only access to catalog and instances                 |

5. Click **Add**

The user will have access to your project on their next login.

### Changing a Member's Role

1. Navigate to **Settings** → **Members**
2. Find the user in the list
3. Click the role dropdown next to their name
4. Select the new role
5. Confirm the change

### Removing a Team Member

1. Navigate to **Settings** → **Members**
2. Find the user you want to remove
3. Click the **Remove** button (trash icon)
4. Confirm the removal

{{< alert title="Warning" color="warning" >}}
Removing a member immediately revokes their access to the project. They will no longer see the project in their project selector.
{{< /alert >}}

## Managing Repositories

Repositories are required for **GitOps** and **Hybrid** deployment modes. Configure a repository to enable your team to deploy instances via Git.

### Viewing Configured Repositories

1. Navigate to **Settings** in the left sidebar
2. Select your project
3. Click on the **Repositories** tab

### Adding a Repository

1. Navigate to **Settings** → **Repositories**
2. Click **Add Repository**
3. Fill in the repository details:

| Field              | Description                  | Example                              |
| ------------------ | ---------------------------- | ------------------------------------ |
| **Repository URL** | GitHub repository URL        | `https://github.com/myorg/manifests` |
| **Branch**         | Target branch for commits    | `main`                               |
| **Path**           | Directory path for manifests | `apps/my-project`                    |

4. Configure authentication:
   - **GitHub App** (recommended): Select a configured GitHub App
   - **Personal Access Token**: Enter a PAT with repo access

5. Click **Save**

### Testing Repository Connection

After adding a repository:

1. Click **Test Connection** next to the repository
2. Verify the connection status shows **Connected**

If the test fails, check:

- Repository URL is correct
- Authentication credentials are valid
- The branch exists
- You have write access to the repository

### Editing a Repository

1. Navigate to **Settings** → **Repositories**
2. Click **Edit** next to the repository
3. Update the configuration
4. Click **Save**

### Removing a Repository

1. Navigate to **Settings** → **Repositories**
2. Click **Remove** next to the repository
3. Confirm the removal

{{< alert title="Note" >}}
Removing a repository disables GitOps and Hybrid deployment modes for your project. Only Direct deployment will be available.
{{< /alert >}}

## Role Permissions Summary

| Action              | Viewer | Developer | Platform Admin |
| ------------------- | ------ | --------- | -------------- |
| View catalog        | ✓      | ✓         | ✓              |
| View instances      | ✓      | ✓         | ✓              |
| Deploy instances    |        | ✓         | ✓              |
| Delete instances    |        | ✓         | ✓              |
| Add/remove members  |        |           | ✓              |
| Manage repositories |        |           | ✓              |

## Frequently Asked Questions

### How do I create a new project?

Project creation is restricted to Global Admins. Contact your platform administrator to request a new project.

### Why can't I add members?

Only **Platform Admins** can add members to a project. Check your role in the Members list. If you need this permission, ask a Platform Admin of your project to upgrade your role.

### Why is GitOps mode not available?

GitOps and Hybrid modes require a configured repository. Add a repository in **Settings** → **Repositories** to enable these deployment modes.

### How do I change my own role?

You cannot change your own role. Ask another Platform Admin in your project to update your permissions.

### What happens when I remove a member?

The user loses all access to the project immediately. They will:

- No longer see the project in their project selector
- Lose access to all instances in the project
- Be unable to deploy to the project

---

**Next:** [Deployment Modes](../deployment-modes/) | **Previous:** [Managing Instances](../managing-instances/)
