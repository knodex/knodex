# Changelog

## [0.8.0](https://github.com/knodex/knodex/compare/v0.7.0...v0.8.0) (2026-05-31)


### Features

* **catalog:** surface knodex.io/docs-url annotation in UI ([339fce5](https://github.com/knodex/knodex/commit/339fce5b4d01881d31b8fe4a203ece37ac2e0782))
* **projects:** add wrapper-RGD routing for project creation ([#79](https://github.com/knodex/knodex/issues/79)) ([22a81a3](https://github.com/knodex/knodex/commit/22a81a3e2b0fdf0f2b66598cbec365c17026b3d2))
* table-default /instances + shared table primitive ([3624603](https://github.com/knodex/knodex/commit/3624603c3a07ca8a24d9f424d9a31b7722ae6807))
* **web:** collapsible sidebar with user menu dropdown ([26f7a53](https://github.com/knodex/knodex/commit/26f7a539514a219824c18bad09cf6cb08330efa5))
* **web:** redesign top bar with breadcrumb and search trigger ([dee213e](https://github.com/knodex/knodex/commit/dee213e5caf6361677b7a1824c89b41a8b1a0786))


### Bug Fixes

* **ci:** add least-privilege permissions to release-summary job ([#87](https://github.com/knodex/knodex/issues/87)) ([554bb07](https://github.com/knodex/knodex/commit/554bb07005bed677755a6f7c5c02e955e3e1abf2))
* **deploy:** prevent review tab overflow and remove duplicate breadcrumb ([4e10c6e](https://github.com/knodex/knodex/commit/4e10c6e8406a1f80da16deea866facb98cdf4562))
* **deps:** apply Dependabot security updates ([6be17ef](https://github.com/knodex/knodex/commit/6be17ef6ddabe4603260b80c9a06c8fec1fb4675))
* **deps:** bump mermaid ≥11.15.0 to fix XSS and Gantt DoS ([197e9aa](https://github.com/knodex/knodex/commit/197e9aa4d76febb3f7231e31bb771ff3021e64aa))
* **deps:** bump vulnerable npm dependencies across root/web/website ([41840c2](https://github.com/knodex/knodex/commit/41840c23efd2c58b567e5f923ec70981f309562c))
* **deps:** pin lodash and lodash-es to &gt;=4.18.0 to fix code injection ([93c0351](https://github.com/knodex/knodex/commit/93c03513bf21f8e2ec3b6bcd08f7a1159e083798))
* **deps:** pin picomatch ≥4.0.4 to patch two GHSA advisories ([8a28ba7](https://github.com/knodex/knodex/commit/8a28ba73c22158bcaf46dee5cf148977d1e381bb))
* **server/static:** return 404 for missing assets to prevent MIME errors ([#81](https://github.com/knodex/knodex/issues/81)) ([e0b3985](https://github.com/knodex/knodex/commit/e0b39858ac7de46c7caeaa84774ecadf1929e604))
* **test/e2e:** tighten Deploy button selectors to exact accessible name ([eccaa5c](https://github.com/knodex/knodex/commit/eccaa5cf76da6523d747c229b5d1717389089fb1))

## [0.7.0](https://github.com/knodex/knodex/compare/v0.6.0...v0.7.0) (2026-05-21)


### ⚠ BREAKING CHANGES

* **api:** Instance API route structure now includes GVK components; clients must update URL patterns for instance CRUD operations.

### Features

* **api:** GVK-aware instance routes and multi-package catalog ([88d5a73](https://github.com/knodex/knodex/commit/88d5a73ab81e1f86672f57162430bdf016d27061))

## [0.6.0](https://github.com/knodex/knodex/compare/v0.5.0...v0.6.0) (2026-05-11)


### Features

* add PostgreSQL data store support and compliance page gating ([#72](https://github.com/knodex/knodex/issues/72)) ([48ef638](https://github.com/knodex/knodex/commit/48ef63833c38dd4192e61e15f1f92655460e4315))
* **deploy:** add PostgreSQL local dev and Helm chart support ([274df70](https://github.com/knodex/knodex/commit/274df7070b20a7ec23ac4cbb7af083528ddfdb6c))
* PostgreSQL support (Helm chart) and Enterprise docs ([#73](https://github.com/knodex/knodex/issues/73)) ([cbe8bd3](https://github.com/knodex/knodex/commit/cbe8bd3f9875dc71fa801fae2f50528840080de4))
* SSO/OIDC improvements and Helm/RBAC fixes ([#69](https://github.com/knodex/knodex/issues/69)) ([8254e27](https://github.com/knodex/knodex/commit/8254e27b99baef4530a42c752185e7dce82ea482))


### Bug Fixes

* **deploy:** show inline name validation error and collapse fields by default ([#61](https://github.com/knodex/knodex/issues/61)) ([261ec7c](https://github.com/knodex/knodex/commit/261ec7ca51c759d9a956e1f6aec1785356c36513))
* **website:** remove stale redirect to /docs after route base path change ([7d9484a](https://github.com/knodex/knodex/commit/7d9484aa0710a8b849947bd32a51f6590ec87c6e))

## [0.5.0](https://github.com/knodex/knodex/compare/v0.4.0...v0.5.0) (2026-04-16)


### Features

* add categories system, RBAC destinations, and catalog improvements ([#55](https://github.com/knodex/knodex/issues/55)) ([7616442](https://github.com/knodex/knodex/commit/76164423a8774f2fb9373f718294561fabb1f93b))

## [0.4.0](https://github.com/knodex/knodex/compare/v0.3.1...v0.4.0) (2026-03-22)


### Features

* **secrets:** add secrets management and catalog improvements ([9ea567a](https://github.com/knodex/knodex/commit/9ea567a899ef4ca70cff08df7502885ab837a309))


### Bug Fixes

* **secrets:** remove enterprise guards from OSS secrets feature ([698cbaf](https://github.com/knodex/knodex/commit/698cbaf1a43c6aa96cec28d4d69c95ee3db6177e))

## [0.3.1](https://github.com/knodex/knodex/compare/v0.3.0...v0.3.1) (2026-03-19)


### Bug Fixes

* **security:** resolve CodeQL code scanning findings ([#41](https://github.com/knodex/knodex/issues/41)) ([260803c](https://github.com/knodex/knodex/commit/260803c04d8c097dc74e5f81b0e9362af54040ea))

## [0.3.0](https://github.com/knodex/knodex/compare/v0.2.0...v0.3.0) (2026-03-18)


### Features

* **catalog:** add dependency tracking and add-ons UI ([#39](https://github.com/knodex/knodex/issues/39)) ([56b6463](https://github.com/knodex/knodex/commit/56b6463307c78fa515504f28a8d2513d770881d5))

## [0.2.0](https://github.com/knodex/knodex/compare/v0.1.0...v0.2.0) (2026-03-15)


### Features

* **catalog:** show inactive RGDs and fix Redis password persistence ([#32](https://github.com/knodex/knodex/issues/32)) ([0fbab3c](https://github.com/knodex/knodex/commit/0fbab3cbf4cbb65a5a786c88190a7ed1c50d8b5a))
* initial open-source release of Knodex ([1c80d7a](https://github.com/knodex/knodex/commit/1c80d7a1eaa72814104a72af5a74efac5f037a97))

## [0.1.0](https://github.com/knodex/knodex/releases/tag/v0.1.0) (2026-03-10)

Initial open-source release of Knodex.

### Features

- Web UI for viewing and managing KRO ResourceGraphDefinitions (RGDs)
- Real-time updates via WebSocket
- OIDC authentication with group-based RBAC (Casbin)
- Multi-tenant Project CRD with ArgoCD-aligned authorization
- Helm chart for Kubernetes deployment
- Instance deployment and lifecycle management
- RGD catalog with organization-scoped visibility
