---
title: License Activation
description: Configure and verify your Knodex Enterprise license using environment variables or Helm values.
sidebar_position: 1
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["enterprise"]} />

# License Activation

Knodex Enterprise features require a valid license key. The license can be provided as a file path or as inline text.

## Configuration Methods

### Option 1: File Path

Set the `KNODEX_LICENSE_PATH` environment variable to point to a file containing the license key:

```bash
export KNODEX_LICENSE_PATH=/etc/knodex/license.jwt
```

### Option 2: Inline Text

Set the `KNODEX_LICENSE_TEXT` environment variable with the license key content directly:

```bash
export KNODEX_LICENSE_TEXT="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
```

If both `KNODEX_LICENSE_PATH` and `KNODEX_LICENSE_TEXT` are set, `KNODEX_LICENSE_PATH` takes precedence.

## Helm Values

The recommended approach for production is to store the license in a Kubernetes Secret and reference it in Helm values:

```yaml
# Create the secret
kubectl create secret generic knodex-license \
  --from-file=license.jwt=/path/to/license.jwt \
  -n knodex
```

```yaml
# values.yaml
enterprise:
  license:
    # Option A: Mount from a Secret
    existingSecret: knodex-license
    secretKey: license.jwt

  # Option B: Inline (not recommended for production)
  # text: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
```

The Helm chart mounts the secret at `/etc/knodex/license.jwt` and sets `KNODEX_LICENSE_PATH` automatically.

## Verify License Status

### Via API

```bash
curl -s http://localhost:8080/api/v1/license \
  -H "Authorization: Bearer $TOKEN" | jq .
```

A valid license returns:

```json
{
  "valid": true,
  "expiresAt": "2027-01-01T00:00:00Z",
  "organization": "acme-corp",
  "features": ["compliance", "audit", "organizations"]
}
```

### Via UI

Navigate to **Settings** in the Knodex UI. The license status is displayed in the **License** section showing validity, expiration date, and enabled features.

## Seat counting

License-seat usage is computed from the canonical user roster. The authoritative source is the `identity.users` table: the billed count is `COUNT(*) FROM identity.users WHERE state='active'` — an entitlement-based, per-seat count, identical on every edition.

Seat **enforcement is unchanged**: the `SeatReconciler`, the threshold warnings, the License Settings UI, and the `AllowedSeats` check all behave exactly as before. Only the storage source the count is computed from changed (previously `license.user_seats`). Idle users **still count** toward the billed total — there is no idle auto-decay, so a provisioned user counts until explicitly removed.

### Reclaiming a seat

To remove a user from the billed seat count, soft-delete the user via the Users API:

```bash
curl -X DELETE http://localhost:8080/api/v1/users/{id} \
  -H "Authorization: Bearer $TOKEN"
```

This sets the user's `state` to `removed`, decrements the seat count, and returns a reclaim note: *"Seat reclaimed. Permanent exclusion requires IdP-side revocation."* Reclaiming a seat is not the same as permanent exclusion — if the same federated identity signs in again via SSO, the user is resurrected and counts once more. **Permanent exclusion requires IdP-side revocation** (remove the user's access at your identity provider).

The Users API (`GET /api/v1/users`, `GET /api/v1/users/{id}`, `DELETE /api/v1/users/{id}`) is operator-gated through the existing `settings` permission — list/get require `settings` read, delete requires `settings` update.

:::note[Seat usage drops to zero after upgrading across the storage rebuild]
When upgrading across the release that re-pointed seat counting at `identity.users`, seat usage drops to zero and re-populates as users log back in via SSO. See the [Upgrade Notes](upgrade-notes#license-seat-counter--storage-rebuild) for the full upgrade procedure.
:::

## License Expiration

When a license expires:
- Enterprise features continue to function in read-only mode for a 7-day grace period
- After the grace period, Enterprise API endpoints return `402 Payment Required`
- OSS features (catalog, instances, projects, RBAC) are unaffected

:::note[Contact Sales]
To obtain or renew a license key, contact the Knodex sales team at sales@knodex.io.
:::

## Troubleshooting

| Issue | Resolution |
|-------|-----------|
| `license file not found` | Verify `KNODEX_LICENSE_PATH` points to an existing file |
| `invalid license signature` | Ensure the license key is not truncated or corrupted |
| `license expired` | Renew your license; OSS features remain available |
| `feature not available` | Check that the license includes the required feature entitlement |
| Enterprise endpoints return 404 | Server was built without the `enterprise` build tag |
