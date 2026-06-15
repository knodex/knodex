-- Copyright 2026 Knodex Authors
-- SPDX-License-Identifier: AGPL-3.0-only
--
-- 0005 down: drop the identity tables (children first), then the schema. The
-- RLS policies, indexes, and FORCE/ENABLE toggles are owned by the tables and
-- drop along with them. The knodex_app / knodex_migrate roles are intentionally
-- NOT dropped — they may be shared with future schemas and own grants elsewhere.

DROP TABLE IF EXISTS identity.federated_identities;
DROP TABLE IF EXISTS identity.users;
DROP SCHEMA IF EXISTS identity;
