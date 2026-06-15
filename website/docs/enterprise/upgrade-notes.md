---
title: Upgrade Notes
description: Operator-facing release and upgrade notes for Knodex Enterprise, including the user-identity seat-counter rebuild.
sidebar_position: 5
---

import ProductTag from "@site/src/components/ProductTag";

<ProductTag tags={["enterprise"]} />

# Upgrade Notes

This page collects operator-facing notes for upgrades that change runtime behavior, storage, or required infrastructure. Read the relevant section **before** upgrading a production deployment.

## License seat counter — storage rebuild

The persistent user-identity work re-points license-seat counting at the canonical user roster. This is a **storage-source change, not a billing-model change**.

License-seat enforcement continues to work as before — entitlement-based, per-seat. The `SeatReconciler`, threshold warnings, `LicenseSettings` UI, and `AllowedSeats` check are unchanged. Upgrading across this tag replaces the storage layer the count is computed from (`license.user_seats` → `identity.users`). Seat usage drops to zero immediately after upgrade and re-populates as users log back in via SSO. To remove a user from the billed seat count, use the new `DELETE /api/v1/users/{id}` endpoint. The dead `license.user_seats` table may be manually dropped post-upgrade if desired.

:::warning[Seat usage shows zero right after upgrade — this is expected]
The seat count is now computed as `COUNT(*) FROM identity.users WHERE state='active'`. The new roster is populated lazily, as each user completes an SSO login. Immediately after upgrade the roster is empty, so seat usage reads **zero** and climbs back to its steady-state value as your users sign in again. No action is required — do not interpret the zero as a license reset or a billing change.
:::

### Idle seats now persist on the bill until explicitly removed

The previous store auto-reclaimed seats for users idle beyond a 30-day `last_seen_at` window — idle users silently dropped off the billed count. The new entitlement count has **no idle decay**: a provisioned user keeps counting toward the billed total until explicitly removed via `DELETE /api/v1/users/{id}`.

In other words, seats that the old store would have auto-reclaimed now **persist on the bill** until an operator removes them. If you relied on idle auto-reclaim to hold your seat count down, you will now see those previously-decayed users counting again once they have logged in. Reclaim them explicitly with the Users API when they are no longer active.

`last_seen_at` still exists, but it is now **display-only** — it drives the inactive-user indicator in the UI and no longer participates in billing.

### Reclaiming a seat

To free a billed seat, soft-delete the user:

```bash
curl -X DELETE http://localhost:8080/api/v1/users/{id} \
  -H "Authorization: Bearer $TOKEN"
```

This flips the user's `state` to `removed` and decrements the billed count. The response carries a reclaim note: *"Seat reclaimed. Permanent exclusion requires IdP-side revocation."* If the same federated identity logs in again via SSO, the user is resurrected (`state='active'`) and counts once more — so **permanent exclusion requires revoking access at your identity provider**, not just calling `DELETE`.

See [License Activation](license-activation#seat-counting) for the authoritative seat-counting reference. The Users API exposes `GET /api/v1/users`, `GET /api/v1/users/{id}`, and `DELETE /api/v1/users/{id}` (all operator-gated via the `settings` permission).

### Cleaning up the old table (optional)

The `license.user_seats` table and its store were removed; nothing reads or writes it anymore. After a successful upgrade you may drop it at your convenience:

```sql
DROP TABLE IF EXISTS license.user_seats;
```

This is optional housekeeping — leaving the table in place has no functional effect.

## PostgreSQL is now required on every edition

PostgreSQL is required for **all editions** (OSS, Enterprise, and Cloud), not just Enterprise. Every edition now persists the canonical user roster and the server **fails fast at startup if `DATABASE_URL` is unset or PostgreSQL is unreachable**. See the [Getting Started](../getting-started/) requirements and [PostgreSQL Configuration](../administration/configuration#postgresql-configuration) for provisioning details, including the bundled subchart, the external opt-out, and backup responsibilities.
