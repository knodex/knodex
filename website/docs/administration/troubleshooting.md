---
title: Troubleshooting
description: Diagnostic commands, common issues, and resolution steps for Knodex deployments.
sidebar_position: 10
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["oss", "enterprise"]} />

# Troubleshooting

This guide covers common issues, diagnostic commands, and resolution steps for Knodex deployments.

## Checking Server Health

### Health Endpoint

The liveness probe checks that the server is running:

```bash
kubectl port-forward svc/knodex-server 8080:8080 -n knodex
curl http://localhost:8080/healthz
```

Expected response:

```json
{"status":"healthy"}
```

### Readiness Endpoint

The readiness probe checks that all dependencies (Redis, Kubernetes API) are reachable:

```bash
curl http://localhost:8080/readyz
```

If readiness fails, the response includes details about which dependency is unhealthy.

## Server Logs

View the Knodex server logs:

```bash
kubectl logs -l app.kubernetes.io/name=knodex -n knodex --tail=100
```

Follow logs in real-time:

```bash
kubectl logs -l app.kubernetes.io/name=knodex -n knodex -f
```

Check previous container logs (after a crash):

```bash
kubectl logs -l app.kubernetes.io/name=knodex -n knodex --previous
```

## Common Issues

### Server Won't Start

**Symptoms**: Pod is in `CrashLoopBackOff` or `Error` state.

**Redis connection failure**:
```
ERROR  failed to connect to Redis  address=knodex-redis:6379
```

Resolution:
1. Check Redis is running: `kubectl get pods -n knodex -l app.kubernetes.io/name=redis`
2. Verify Redis address: check `REDIS_ADDRESS` in the server ConfigMap
3. If `redis.auth.enabled`, verify `REDIS_PASSWORD` is set correctly
4. Check the `wait-for-redis` init container logs: `kubectl logs <pod> -n knodex -c wait-for-redis`

**PostgreSQL connection failure (Enterprise)**:
```
ERROR  failed to connect to database  error="dial tcp: connection refused"
```
or
```
ERROR  migrations failed  error="pq: password authentication failed for user"
```

Resolution:
1. Check PostgreSQL is running:
   ```bash
   # Embedded subchart
   kubectl get pods -n knodex -l app.kubernetes.io/name=postgresql
   # CloudNativePG
   kubectl get pods -n knodex -l cnpg.io/cluster=knodex-cluster
   ```
2. Verify `DATABASE_URL` is set in the server Secret or ConfigMap:
   ```bash
   kubectl get secret knodex-secret -n knodex -o jsonpath='{.data.DATABASE_URL}' | base64 -d
   ```
3. Confirm the database is reachable from within the cluster:
   ```bash
   kubectl run pg-test --rm -it --image=postgres:16-alpine --restart=Never -- \
     psql "$DATABASE_URL" -c "SELECT version();"
   ```
4. For CloudNativePG, verify the application secret exists:
   ```bash
   kubectl get secret knodex-cluster-app -n knodex -o jsonpath='{.data.uri}' | base64 -d
   ```

**Kubernetes permissions**:
```
ERROR  failed to list resourcegraphdefinitions  error="forbidden"
```

Resolution:
1. Verify the ClusterRole and ClusterRoleBinding exist (see [Kubernetes RBAC](kubernetes-rbac))
2. Test permissions: `kubectl auth can-i list resourcegraphdefinitions.kro.run --as=system:serviceaccount:knodex:knodex`

**Invalid configuration**:
```
ERROR  invalid configuration  field=OIDC_ISSUER_URL error="required when OIDC is enabled"
```

Resolution: Check all required environment variables are set. See [Configuration](configuration) for the complete reference.

### RGDs Not Appearing in Catalog

**Missing catalog annotation**:

RGDs must have the `knodex.io/catalog: "true"` annotation to appear in the catalog:

```yaml
annotations:
  knodex.io/catalog: "true"
```

**Package filter**:

If `CATALOG_PACKAGE_FILTER` is set, only RGDs with a matching `knodex.io/package` label are visible. See [Catalog Filter](catalog-filter).

Check the current filter:

```bash
kubectl get configmap knodex-config -n knodex -o jsonpath='{.data.CATALOG_PACKAGE_FILTER}'
```

**Organization/project scoping**:

RGDs may be scoped to a specific organization or project. Verify the user's project membership includes the project that owns the RGD.

### OIDC Login Fails

**Issuer URL unreachable**:
```
ERROR  failed to discover OIDC provider  issuer=https://login.microsoftonline.com/...
```

Resolution:
1. Verify the issuer URL is correct and accessible from the cluster
2. Check DNS resolution: `kubectl run -it --rm debug --image=curlimages/curl -- curl -s <issuer-url>/.well-known/openid-configuration`

**Client ID or secret mismatch**:
```
ERROR  OIDC token validation failed  error="invalid_client"
```

Resolution:
1. Verify `OIDC_CLIENT_ID` matches the application registration
2. Verify `OIDC_CLIENT_SECRET` is correct and not expired
3. For Azure AD, check that admin consent has been granted

**Redirect URL mismatch**:

The identity provider rejects the callback because the redirect URL does not match:

Resolution:
1. The redirect URL must be exactly `https://<your-domain>/api/v1/auth/callback`
2. Check the registered redirect URIs in your identity provider
3. Ensure the scheme (`https` vs `http`) matches

**Missing scopes**:

Users authenticate but have no project access:

Resolution: Ensure the `groups` scope (or equivalent) is configured. See [OIDC Integration](oidc-integration).

### Instances Stuck in Pending

**KRO controller not running**:

```bash
kubectl get pods -n kro-system
```

If no KRO pods are running, install KRO or enable it in the Helm chart (`kro.enabled: true`).

**Resource quotas exceeded**:

```bash
kubectl describe resourcequota -n <namespace>
```

Check if the namespace has quotas that prevent resource creation.

**Image pull errors**:

```bash
kubectl describe pod <instance-pod> -n <namespace>
```

Look for `ImagePullBackOff` events and verify image references and pull secrets.

### Permission Denied Errors

**Casbin policy issue**:

Users see "permission denied" when accessing resources through the Knodex UI or API:

1. Verify the user's OIDC groups match the groups in the Project CRD roles
2. Check the role policies include the required resource and action
3. If using destination-scoped roles, verify the target namespace is in the role's `destinations`
4. Check server logs for Casbin enforcement details

**Kubernetes RBAC issue**:

The server itself cannot access Kubernetes resources:

1. Check server logs for `403 Forbidden` errors from the Kubernetes API
2. Verify RBAC configuration per [Kubernetes RBAC](kubernetes-rbac)

### WebSocket Disconnections

**Symptoms**: Real-time updates stop, the UI shows a connection warning.

**Proxy timeout**:

Reverse proxies and load balancers may close idle WebSocket connections. Configure your ingress to allow long-lived connections:

For nginx ingress:

```yaml
metadata:
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
```

**Load balancer configuration**:

Cloud load balancers (ALB, Azure Application Gateway) have default idle timeouts. Increase the timeout to at least 300 seconds.

### Redis Connection Issues

**Symptoms**: Server starts but performance degrades, or sessions are lost.

**Check Redis connectivity**:

```bash
kubectl exec -it knodex-redis-* -n knodex -- redis-cli ping
```

Expected response: `PONG`

**Check Redis memory**:

```bash
kubectl exec -it knodex-redis-* -n knodex -- redis-cli info memory
```

**Authentication failure**:

If Redis auth is enabled, verify the password matches between Redis and the Knodex server:

```bash
kubectl get secret knodex-redis -n knodex -o jsonpath='{.data.redis-password}' | base64 -d
```

### PostgreSQL Connection Issues (Enterprise)

**Symptoms**: Server crashes at startup or audit/compliance data is not persisting.

**Check PostgreSQL connectivity**:

```bash
# Embedded subchart
kubectl exec -it knodex-postgresql-0 -n knodex -- psql -U knodex -c "SELECT version();"

# CloudNativePG
kubectl exec -it knodex-cluster-1 -n knodex -- psql -U knodex -c "SELECT version();"
```

**Verify migration state**:

```bash
kubectl exec -it knodex-postgresql-0 -n knodex -- \
  psql -U knodex -c "SELECT version, dirty FROM schema_migrations;"
```

If `dirty=true`, a previous migration run was interrupted. This requires manual intervention — contact support.

**Check data is persisting**:

```bash
# Connect to verify enterprise data
kubectl exec -it knodex-postgresql-0 -n knodex -- psql -U knodex -c "\dn"
# Expected: audit, compliance, public schemas listed

kubectl exec -it knodex-postgresql-0 -n knodex -- \
  psql -U knodex -c "SELECT count(*) FROM compliance.violations;"

kubectl exec -it knodex-postgresql-0 -n knodex -- \
  psql -U knodex -c "SELECT count(*) FROM audit.events;"
```

**Verify DATABASE_URL is injected**:

```bash
kubectl get configmap knodex-config -n knodex -o jsonpath='{.data.DATABASE_URL}'
# or from secret (if using external credential):
kubectl get secret knodex-secret -n knodex -o jsonpath='{.data.DATABASE_URL}' | base64 -d
```

If `DATABASE_URL` is empty, check the Helm values. With the embedded subchart (`postgresql.enabled: true`), the chart assembles the DSN automatically from `postgresql.auth.*`. With an external database, `postgres.connectionStringSecret` must reference a valid Secret.

## Diagnostic Commands

### Full Pod Status

```bash
kubectl get pods -n knodex -o wide
kubectl describe pod -l app.kubernetes.io/name=knodex -n knodex
```

### Check Events

```bash
kubectl get events -n knodex --sort-by='.lastTimestamp'
```

### Verify CRDs

```bash
kubectl get crd | grep -E "knodex|kro"
```

### Test API Connectivity

```bash
kubectl port-forward svc/knodex-server 8080:8080 -n knodex
curl -v http://localhost:8080/healthz
curl -v http://localhost:8080/readyz
curl -v http://localhost:8080/api/v1/rgds
```

### Check Helm Release

```bash
helm list -n knodex
helm get values knodex -n knodex
helm history knodex -n knodex
```

## Getting Help

- **GitHub Issues**: Report bugs and request features on the [Knodex GitHub repository](https://github.com/knodex/knodex)
- **KRO Documentation**: For RGD authoring and KRO-specific issues, see the [KRO project](https://github.com/kro-run/kro)
- **Kubernetes Documentation**: For cluster-level issues, refer to the [Kubernetes docs](https://kubernetes.io/docs/)
