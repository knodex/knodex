-- Copyright 2026 Knodex Authors
-- SPDX-License-Identifier: AGPL-3.0-only
--
-- 0005 create identity schema: the canonical, edition-neutral user roster
-- materialised on every successful OIDC login (Story 15.2 / FR-U1, FR-U2).
-- This migration ships in the SHARED (internal/database) source so it runs on
-- every edition — OSS, Enterprise, and Cloud (R5-5). There is NO outbox table
-- (R5-7).
--
-- Two tables:
--   * identity.users — one row per canonical user, ULID-keyed, org-scoped,
--     state-tracked (active|removed; soft-delete + resurrect-on-login — R5-3/R5-4).
--   * identity.federated_identities — one row per (org_id, issuer, sub) OIDC
--     triple, with SCIM-ready nullable sub + external_id + source_connector_id
--     (R5-6) so SCIM lands later as a thin source-kind adapter, not a parallel store.
--
-- Org isolation mirrors audit.events (0002): NULLIF(current_setting(...)) RLS with
-- USING + WITH CHECK, FORCE ROW LEVEL SECURITY. The non-BYPASSRLS knodex_app role
-- (provisioned below) is subject to these policies (NFR-U13, R5-9).

CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE identity.users (
    id            TEXT PRIMARY KEY,                              -- ULID, 26-char crockford base32
    org_id        TEXT NOT NULL,
    email         TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    state         TEXT NOT NULL DEFAULT 'active'
                      CHECK (state IN ('active','removed')),     -- R5: keep extensible; SCIM adds non-resurrecting removal later
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX users_org_state_idx     ON identity.users (org_id, state);            -- BilledSeatCount (FR-U7)
CREATE INDEX users_org_last_seen_idx ON identity.users (org_id, last_seen_at DESC);-- List ordering (15.2a) + inactive badge

CREATE TABLE identity.federated_identities (
    org_id             TEXT NOT NULL,
    issuer             TEXT NOT NULL,
    sub                TEXT,                                      -- NULLABLE (R5-6): SCIM-pushed rows have no sub until first login
    external_id        TEXT,                                      -- NULLABLE: IdP-side opaque id (SCIM externalID); NULL on OIDC
    source_connector_id TEXT,                                     -- NULLABLE: SCIM connector id; NULL on OIDC
    internal_user_id   TEXT NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    provider_kind      TEXT NOT NULL DEFAULT 'oidc',
    source_kind        TEXT NOT NULL DEFAULT 'oidc_jit'
                           CHECK (source_kind IN ('oidc_jit','keycloak_projection','scim_push')),  -- R5-6
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- OIDC natural key (sub present); SCIM rows (sub NULL) are keyed by the partial unique below.
CREATE UNIQUE INDEX fed_identities_issuer_sub_uq
    ON identity.federated_identities (org_id, issuer, sub) WHERE sub IS NOT NULL;
CREATE UNIQUE INDEX fed_identities_connector_external_uq
    ON identity.federated_identities (org_id, source_connector_id, external_id)
    WHERE source_connector_id IS NOT NULL AND external_id IS NOT NULL;
CREATE INDEX fed_identities_internal_user_idx ON identity.federated_identities (internal_user_id);

ALTER TABLE identity.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.users FORCE  ROW LEVEL SECURITY;
CREATE POLICY users_isolation ON identity.users
    USING      (org_id = NULLIF(current_setting('app.org_id', true), ''))
    WITH CHECK (org_id = NULLIF(current_setting('app.org_id', true), ''));

ALTER TABLE identity.federated_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.federated_identities FORCE  ROW LEVEL SECURITY;
CREATE POLICY fed_identities_isolation ON identity.federated_identities
    USING      (org_id = NULLIF(current_setting('app.org_id', true), ''))
    WITH CHECK (org_id = NULLIF(current_setting('app.org_id', true), ''));

-- Postgres roles (NFR-U13, R5-9). Idempotent: re-running this migration (or a
-- deployment that already provisioned the roles via Helm — Story 15.3) is safe.
-- knodex_app is the app's pgxpool role — NOT BYPASSRLS, so the RLS policies above
-- actually constrain it. knodex_migrate owns migration-time DDL and carries
-- BYPASSRLS for the migration window only. Both are created NOLOGIN here; the
-- deployment grants LOGIN + a password (or maps the connecting login role into
-- knodex_app via GRANT knodex_app TO <login_user>).
--
-- Role creation is BEST-EFFORT and privilege-tolerant. `CREATE ROLE` needs
-- CREATEROLE, and granting BYPASSRLS specifically needs SUPERUSER — privileges
-- the connecting migration role lacks on managed Postgres (RDS/Cloud SQL/Azure),
-- where BYPASSRLS is often outright disallowed. Since R5-5 makes this migration
-- run at boot on EVERY edition, an unguarded CREATE ROLE would hard-fail startup
-- on managed Postgres. We therefore swallow insufficient_privilege (and downgrade
-- BYPASSRLS to a plain role if that attribute is refused): the operator/Helm path
-- (Story 15.3) is expected to provision these roles out-of-band, and local-dev/CI
-- run as a superuser, so the CREATE succeeds there. The schema + RLS + GRANTs below
-- do NOT require superuser and are what this migration actually depends on.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'knodex_app') THEN
        CREATE ROLE knodex_app NOLOGIN;
    END IF;
EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'skipping CREATE ROLE knodex_app (insufficient privilege); provision it out-of-band (Story 15.3)';
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'knodex_migrate') THEN
        BEGIN
            CREATE ROLE knodex_migrate NOLOGIN BYPASSRLS;
        EXCEPTION WHEN insufficient_privilege THEN
            -- BYPASSRLS refused (non-superuser / managed Postgres): fall back to a
            -- plain role so dependent GRANTs still succeed. The migration window's
            -- BYPASSRLS need is satisfied by whatever superuser/owner actually runs
            -- migrations here.
            RAISE NOTICE 'creating knodex_migrate without BYPASSRLS (insufficient privilege)';
            CREATE ROLE knodex_migrate NOLOGIN;
        END;
    END IF;
EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'skipping CREATE ROLE knodex_migrate (insufficient privilege); provision it out-of-band (Story 15.3)';
END
$$;

-- GRANTs are guarded too: they no-op (with a NOTICE) if the target role was not
-- created above, so the migration never fails on a managed Postgres that owns its
-- own role lifecycle.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'knodex_app') THEN
        GRANT USAGE ON SCHEMA identity TO knodex_app;
        GRANT SELECT, INSERT, UPDATE, DELETE ON identity.users, identity.federated_identities TO knodex_app;
    ELSE
        RAISE NOTICE 'skipping GRANTs to knodex_app (role absent)';
    END IF;
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'knodex_migrate') THEN
        GRANT USAGE, CREATE ON SCHEMA identity TO knodex_migrate;
        GRANT SELECT, INSERT, UPDATE, DELETE ON identity.users, identity.federated_identities TO knodex_migrate;
    ELSE
        RAISE NOTICE 'skipping GRANTs to knodex_migrate (role absent)';
    END IF;
END
$$;
