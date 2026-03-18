# Changelog

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
