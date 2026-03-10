# Changelog

## [0.0.22](https://github.com/knodex/knodex/compare/v0.0.21...v0.0.22) (2026-03-10)


### Features

* add session restore and default Project CRD ([3951d65](https://github.com/knodex/knodex/commit/3951d6578bc8c175a113501a2f0254b43875d3c3))


### Bug Fixes

* **ci:** add reference RGDs for canary tests ([6d32641](https://github.com/knodex/knodex/commit/6d32641526beb057d1a3f57b7ff72e81ee7a2625))
* **ci:** build KRO image from source for Kind cluster ([744e9c1](https://github.com/knodex/knodex/commit/744e9c15a40eb2babed8d75570383cd9e9f7e1f4))
* **ci:** build KRO with go build + distroless (no Dockerfile in KRO repo) ([8cb5517](https://github.com/knodex/knodex/commit/8cb551712c54c8dd5179c4e46abba047c7ae8499))
* **ci:** use GitHub-hosted runner for KRO canary and fix clone logic ([f624492](https://github.com/knodex/knodex/commit/f624492ff620fc51c9a718a1e4d433d149801890))

## [0.0.21](https://github.com/knodex/knodex/compare/v0.0.20...v0.0.21) (2026-03-09)


### Features

* **chart:** add production readiness values and validation guards ([618d749](https://github.com/knodex/knodex/commit/618d74906f47cf5b99f410bd85f22304896b80cb))

## [0.0.20](https://github.com/knodex/knodex/compare/v0.0.19...v0.0.20) (2026-03-08)


### Features

* add KRO centralization, security hardening, and util packages ([59d7494](https://github.com/knodex/knodex/commit/59d7494abeb833df000dfd529a44c83ecd97c5aa))


### Bug Fixes

* **rbac:** add authorization to views endpoints and fix ClusterRole rules ([8e97af7](https://github.com/knodex/knodex/commit/8e97af77e269bb0c0ca53b586682782325c20239))

## [0.0.19](https://github.com/knodex/knodex/compare/v0.0.18...v0.0.19) (2026-03-05)


### Features

* add instance spec editing and audit change details ([ad7746d](https://github.com/knodex/knodex/commit/ad7746dcbfc551f2b54572d9ef37f010ef1113c7))
* **schema:** add nested externalRef dropdown resolution for composite RGDs ([0c51052](https://github.com/knodex/knodex/commit/0c510525ce0711fc17b71d99af0cdd1dd1646911))
* **ui:** add inline role builder and improve layout consistency ([72f2fdb](https://github.com/knodex/knodex/commit/72f2fdb65a865cd439b09b96f98fde6231fceb56))


### Bug Fixes

* **repo:** remove repository enabled field ([561dc7a](https://github.com/knodex/knodex/commit/561dc7aa23eea030445a9a122416778a8c6e0a1d))
* **tests:** register ProjectList kind in watcher manager tests ([8921947](https://github.com/knodex/knodex/commit/89219474627d455b18ada86ce9500174081f064a))

## [0.0.18](https://github.com/knodex/knodex/compare/v0.0.17...v0.0.18) (2026-03-03)


### Bug Fixes

* **build:** resolve relative path breakage in sync script ([3877388](https://github.com/knodex/knodex/commit/3877388a752124d211598ffb1bf6e418779c944f))


### Miscellaneous Chores

* re-release 0.0.18 ([37f2220](https://github.com/knodex/knodex/commit/37f22209b13cd3a393fda7f7b435e9cbd809f8a4))
* re-release 0.0.18 ([4173471](https://github.com/knodex/knodex/commit/41734710658295a3e6e0c01b1f531b010eb4432a))
* re-release 0.0.18 ([304cc62](https://github.com/knodex/knodex/commit/304cc6296716408f0e3bc1648a1d4881353b2096))

## [0.0.18](https://github.com/knodex/knodex/compare/v0.0.17...v0.0.18) (2026-03-03)


### Bug Fixes

* **build:** resolve relative path breakage in sync script ([3877388](https://github.com/knodex/knodex/commit/3877388a752124d211598ffb1bf6e418779c944f))


### Miscellaneous Chores

* re-release 0.0.18 ([4173471](https://github.com/knodex/knodex/commit/41734710658295a3e6e0c01b1f531b010eb4432a))
* re-release 0.0.18 ([304cc62](https://github.com/knodex/knodex/commit/304cc6296716408f0e3bc1648a1d4881353b2096))

## [0.0.18](https://github.com/knodex/knodex/compare/v0.0.17...v0.0.18) (2026-03-03)


### Bug Fixes

* **build:** resolve relative path breakage in sync script ([3877388](https://github.com/knodex/knodex/commit/3877388a752124d211598ffb1bf6e418779c944f))


### Miscellaneous Chores

* re-release 0.0.18 ([304cc62](https://github.com/knodex/knodex/commit/304cc6296716408f0e3bc1648a1d4881353b2096))

## [0.0.18](https://github.com/knodex/knodex/compare/v0.0.17...v0.0.18) (2026-03-03)


### Bug Fixes

* **build:** resolve relative path breakage in sync script ([3877388](https://github.com/knodex/knodex/commit/3877388a752124d211598ffb1bf6e418779c944f))

## [0.0.17](https://github.com/knodex/knodex/compare/v0.0.16...v0.0.17) (2026-03-03)


### Features

* initial open-source release of Knodex ([5cebd50](https://github.com/knodex/knodex/commit/5cebd5063c7a9f78a3ddfe273861b93b0180fbcd))


### Bug Fixes

* **chart:** pin Redis image by digest after Bitnami removed semver tags ([3366ee6](https://github.com/knodex/knodex/commit/3366ee64b2a75aeb1eea36f354491a3a28f90282))
* **chart:** update image registry to knodex org and use appVersion tag ([f3e5322](https://github.com/knodex/knodex/commit/f3e532275b3c98c975dfd567c254c498ddc85761))
* **ci:** expand server test scope to all packages ([23ab4d0](https://github.com/knodex/knodex/commit/23ab4d04bef2ef9492e83f1f133a762620d67960))
* **ci:** upgrade golangci-lint-action to v7 for golangci-lint v2 support ([ff20e7c](https://github.com/knodex/knodex/commit/ff20e7c351bd86d1ade62971edd062abcb265095))
* **deploy:** align E2E Redis image with Helm chart ([c18b6f6](https://github.com/knodex/knodex/commit/c18b6f6f28623c3ed9f8ae3089a8e8d3a2acd693))
* **docs:** update README badges to point to OSS repo ([15aad51](https://github.com/knodex/knodex/commit/15aad51738081aba0adc3c476ddd413f47579550))
* **e2e:** add project and instance test fixtures ([0a61ae0](https://github.com/knodex/knodex/commit/0a61ae03b7ec17790692eb7339f140cfa59edb8a))
* **lint:** use 0600 file permissions to satisfy gosec G306 ([14a0c68](https://github.com/knodex/knodex/commit/14a0c689f94af78a8a050ad6a6649129c175f253))
