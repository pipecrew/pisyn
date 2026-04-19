# Changelog

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
