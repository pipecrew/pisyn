# Changelog

## [0.3.1](https://github.com/pipecrew/pisyn/compare/v0.3.0...v0.3.1) (2026-04-23)


### Bug Fixes

* update dependencies and actions ([#45](https://github.com/pipecrew/pisyn/issues/45)) ([fec872a](https://github.com/pipecrew/pisyn/commit/fec872a290d25c3bcd53334220b5c91ebeaa00fb))

## [0.3.0](https://github.com/pipecrew/pisyn/compare/v0.2.2...v0.3.0) (2026-04-22)


### Features

* **graph:** group jobs by stage using Mermaid subgraph ([24e54a9](https://github.com/pipecrew/pisyn/commit/24e54a9637e8b0420e3627868d4b1c58beb82156)), closes [#10](https://github.com/pipecrew/pisyn/issues/10)


### Bug Fixes

* cycle in graph namings ([20cf6ea](https://github.com/pipecrew/pisyn/commit/20cf6eaccd85221530ca1049cce1fc9ef4cd61f9))
* **deps:** update module charm.land/bubbletea/v2 to v2.0.6 ([2334f36](https://github.com/pipecrew/pisyn/commit/2334f361e9dcf64467cc00fa12fb344504ff4ca2))
* **deps:** update module charm.land/bubbletea/v2 to v2.0.6 ([f228e59](https://github.com/pipecrew/pisyn/commit/f228e593eecf30e9109bcbd16f3e8675696feaaa))
* **pisyn:** deep-copy RetryCfg, AllowFailureCfg, Interruptible in Job.Clone ([f7f6e2b](https://github.com/pipecrew/pisyn/commit/f7f6e2b59da8aace2d6c2e60b66bb586dab88b5d))
* **pisyn:** deep-copy RetryCfg, AllowFailureCfg, Interruptible in Job.Clone ([b091254](https://github.com/pipecrew/pisyn/commit/b0912541c8e5ed4b93f837afad9ab1f5fb07fee3))

## [0.2.2](https://github.com/pipecrew/pisyn/compare/v0.2.1...v0.2.2) (2026-04-18)


### Bug Fixes

* **importer:** handle GitLab CI cache key object form ([311fc1f](https://github.com/pipecrew/pisyn/commit/311fc1f038def0767c309f0b5c64072685f3d1e8))

## [0.2.1](https://github.com/pipecrew/pisyn/compare/v0.2.0...v0.2.1) (2026-04-18)


### Bug Fixes

* map PISYN_PROJECT_URL to CI_PROJECT_URL instead of CI_REPOSITORY_URL ([ab0305a](https://github.com/pipecrew/pisyn/commit/ab0305abfd85d9ad29976b734d72cb66bc00b6f1))
* map PISYN_PROJECT_URL to CI_PROJECT_URL instead of CI_REPOSITORY_URL ([8ea00d1](https://github.com/pipecrew/pisyn/commit/8ea00d13c080a943e51e22f45fff15e12cd542c3)), closes [#11](https://github.com/pipecrew/pisyn/issues/11)

## [0.2.0](https://github.com/pipecrew/pisyn/compare/v0.1.1...v0.2.0) (2026-04-14)


### Features

* add init command for reverse import of gitlab-ci files ([890752e](https://github.com/pipecrew/pisyn/commit/890752e5fb5fab7875edd470c79bacc03554b18c))
* add init command for reverse import of gitlab-ci files ([21064c1](https://github.com/pipecrew/pisyn/commit/21064c17b6c5f20a9706ecfbbc5f344c11d8082f))

## [0.1.1](https://github.com/pipecrew/pisyn/compare/v0.1.0...v0.1.1) (2026-04-14)


### Bug Fixes

* replace panic with error in duplicate job name check ([fc192d5](https://github.com/pipecrew/pisyn/commit/fc192d5cce42f2db7577e6c60c2276429fdf4701))
* replace panic with error in duplicate job name check ([ef3c63d](https://github.com/pipecrew/pisyn/commit/ef3c63d399fd6e4f14a5be6036c76fdcf56922d5))
* run duplicate-name validation in Build() and use Pipeline.Name ([5932596](https://github.com/pipecrew/pisyn/commit/59325962e020077eb9165c483706fff175ca1101))

## [0.1.0](https://github.com/pipecrew/pisyn/compare/v0.0.0...v0.1.0) (2026-04-09)


### Features

* add gifs to docs ([c440247](https://github.com/pipecrew/pisyn/commit/c44024759d62df5293f71bb86c3df944e834b235))
* add new variable - VarProjectURL ([cf8f509](https://github.com/pipecrew/pisyn/commit/cf8f509c464976a91acc1839f6b7fa472c9487bc))
* add OnPushTag() - a new pushtrigger for tags ([ce942f4](https://github.com/pipecrew/pisyn/commit/ce942f477a895a1a37474f25b4258a2f97641721))
* add SetFetchDepth() and properly set safe.directory on github if depth &gt; -1 ([f7b6041](https://github.com/pipecrew/pisyn/commit/f7b60413339e255be3e1ea99d7f69d3eacd52523))
* eat your own dogfood - use pisyn to create workflows ([e3778c0](https://github.com/pipecrew/pisyn/commit/e3778c0c6d31ac463c09954ad63514334dabcde0))
* implement GitLab multi-pipeline merge 🎉 ([2f840df](https://github.com/pipecrew/pisyn/commit/2f840df29a9a581dfcf0e35555d8121524bc081c))
