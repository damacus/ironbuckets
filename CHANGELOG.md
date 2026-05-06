# Changelog

## [1.4.0](https://github.com/damacus/ironbuckets/compare/v1.3.0...v1.4.0) (2026-05-06)


### Features

* add request timeout middleware for handlers ([#24](https://github.com/damacus/ironbuckets/issues/24)) ([2ee13be](https://github.com/damacus/ironbuckets/commit/2ee13bec5afd7d140cf81852a6d9334dcdac59c4))
* gate OIDC routes behind explicit configuration ([#21](https://github.com/damacus/ironbuckets/issues/21)) ([d603113](https://github.com/damacus/ironbuckets/commit/d603113756021043756dc4fbd9568aa19301b38b))


### Bug Fixes

* correct object pagination truncation detection ([#19](https://github.com/damacus/ironbuckets/issues/19)) ([702ab63](https://github.com/damacus/ironbuckets/commit/702ab6379c386feabd57223b18f21a1f23a664f6))
* **deps:** update module github.com/minio/minio-go/v7 to v7.1.0 ([#37](https://github.com/damacus/ironbuckets/issues/37)) ([694e032](https://github.com/damacus/ironbuckets/commit/694e032d10b6432f75a9804771153ec0793234da))
* enforce S3-compatible bucket naming rules ([#18](https://github.com/damacus/ironbuckets/issues/18)) ([1c28daa](https://github.com/damacus/ironbuckets/commit/1c28daa25a261f775cbf8aa117dd80d6b0bef9c8))
* harden invalid cookie clearing and upload filename handling ([#16](https://github.com/damacus/ironbuckets/issues/16)) ([20531dd](https://github.com/damacus/ironbuckets/commit/20531dd26596b44f3b3849b8118589daf18d7381))
* use localhost as default MinIO endpoint ([#22](https://github.com/damacus/ironbuckets/issues/22)) ([adeb4fa](https://github.com/damacus/ironbuckets/commit/adeb4fae7a268f2c32e01d7d0f42714ebf4aec0c))
* validate object key input for object operations ([#17](https://github.com/damacus/ironbuckets/issues/17)) ([d99cb55](https://github.com/damacus/ironbuckets/commit/d99cb555e9f09c5a347bf462f153316a0dbf1b9a))

## [1.3.0](https://github.com/damacus/ironbuckets/compare/v1.2.1...v1.3.0) (2026-02-10)


### Features

* add HTMX boosting for SPA-like navigation ([#9](https://github.com/damacus/ironbuckets/issues/9)) ([7ab376d](https://github.com/damacus/ironbuckets/commit/7ab376db29af0496a6d548d1060219cba3c80039))
* Security workflows ([#14](https://github.com/damacus/ironbuckets/issues/14)) ([2ee555a](https://github.com/damacus/ironbuckets/commit/2ee555a2deb84bdd40707ea18613b0c83c0601aa))

## [1.2.1](https://github.com/damacus/ironbuckets/compare/v1.2.0...v1.2.1) (2026-01-14)


### Bug Fixes

* Fix tests ([#7](https://github.com/damacus/ironbuckets/issues/7)) ([70613f1](https://github.com/damacus/ironbuckets/commit/70613f104c1ec02638b897341e826c0121ab0c7c))

## [1.2.0](https://github.com/damacus/ironbuckets/compare/v1.1.0...v1.2.0) (2026-01-10)


### Features

* Bucket Stats ([#5](https://github.com/damacus/ironbuckets/issues/5)) ([e0812d2](https://github.com/damacus/ironbuckets/commit/e0812d2b8beec2d8b3fcf2d0cb10c5ab07e11826))

## [1.1.0](https://github.com/damacus/ironbuckets/compare/v1.0.0...v1.1.0) (2025-12-15)


### Features

* Initial release of IronBuckets - MinIO Web UI ([db544e6](https://github.com/damacus/ironbuckets/commit/db544e659f3c83f210809ee122dd1a33437593a4))


### Bug Fixes

* **ci:** Fix e2e test runner ([1516d3d](https://github.com/damacus/ironbuckets/commit/1516d3d9311d6a3770d6ffba3a72e7b2149fc226))
* **ci:** Update release workflow ([c1a40f2](https://github.com/damacus/ironbuckets/commit/c1a40f2ffb1a98d53807c799df8750005d76f584))
* **deps:** Update deps ([d898b44](https://github.com/damacus/ironbuckets/commit/d898b4483e42967af65804b3661cf718fe01263a))
* **gitignore:** restore cmd/server and fix server pattern ([86ca65b](https://github.com/damacus/ironbuckets/commit/86ca65b4a1c9e037ef686641ea7441eb25ae9512))
* **gitignore:** restore cmd/server and fix server pattern ([54ef5af](https://github.com/damacus/ironbuckets/commit/54ef5afd2680c480c48695cd90b05de865508ac9))
* **lint:** fix all errcheck errors ([1f15fe8](https://github.com/damacus/ironbuckets/commit/1f15fe895da18bfe77ba618aeebe042551f02d66))
* **tests:** fix shouldUseSSL ([d6f73dc](https://github.com/damacus/ironbuckets/commit/d6f73dcb7f32e88cdf729c45cc439d3bba02b67a))

## 1.0.0 (2025-11-25)


### Features

* Initial release of IronBuckets - MinIO Web UI ([db544e6](https://github.com/damacus/ironbuckets/commit/db544e659f3c83f210809ee122dd1a33437593a4))


### Bug Fixes

* **ci:** Fix e2e test runner ([1516d3d](https://github.com/damacus/ironbuckets/commit/1516d3d9311d6a3770d6ffba3a72e7b2149fc226))
* **ci:** Update release workflow ([c1a40f2](https://github.com/damacus/ironbuckets/commit/c1a40f2ffb1a98d53807c799df8750005d76f584))
* **deps:** Update deps ([d898b44](https://github.com/damacus/ironbuckets/commit/d898b4483e42967af65804b3661cf718fe01263a))
* **gitignore:** restore cmd/server and fix server pattern ([86ca65b](https://github.com/damacus/ironbuckets/commit/86ca65b4a1c9e037ef686641ea7441eb25ae9512))
* **gitignore:** restore cmd/server and fix server pattern ([54ef5af](https://github.com/damacus/ironbuckets/commit/54ef5afd2680c480c48695cd90b05de865508ac9))
* **lint:** fix all errcheck errors ([1f15fe8](https://github.com/damacus/ironbuckets/commit/1f15fe895da18bfe77ba618aeebe042551f02d66))
* **tests:** fix shouldUseSSL ([d6f73dc](https://github.com/damacus/ironbuckets/commit/d6f73dcb7f32e88cdf729c45cc439d3bba02b67a))
