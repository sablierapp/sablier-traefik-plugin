# Changelog

## [1.3.1](https://github.com/sablierapp/sablier-traefik-plugin/compare/v1.3.0...v1.3.1) (2026-08-18)


### Chores

* **deps:** bump actions/checkout from 6.0.2 to 7.0.1 ([#56](https://github.com/sablierapp/sablier-traefik-plugin/issues/56)) ([7885d1b](https://github.com/sablierapp/sablier-traefik-plugin/commit/7885d1bc9905a0a5a92b722c783e44e1ef002d62))
* **deps:** bump actions/create-github-app-token from 3.1.1 to 3.2.0 ([#45](https://github.com/sablierapp/sablier-traefik-plugin/issues/45)) ([c6c6322](https://github.com/sablierapp/sablier-traefik-plugin/commit/c6c63228454ef7ab30fb0061ece5473520d18e89))
* **deps:** bump actions/setup-go from 6.4.0 to 7.0.0 ([#55](https://github.com/sablierapp/sablier-traefik-plugin/issues/55)) ([8c70a43](https://github.com/sablierapp/sablier-traefik-plugin/commit/8c70a4326e51337c4fb6c3447caa5ff8022982dd))
* **deps:** bump golangci/golangci-lint-action from 9.2.0 to 9.2.1 ([#46](https://github.com/sablierapp/sablier-traefik-plugin/issues/46)) ([741b778](https://github.com/sablierapp/sablier-traefik-plugin/commit/741b778e110fe78a5fea98a74a64235582631e4f))
* **deps:** bump golangci/golangci-lint-action from 9.2.1 to 9.3.0 ([#53](https://github.com/sablierapp/sablier-traefik-plugin/issues/53)) ([4a52bc9](https://github.com/sablierapp/sablier-traefik-plugin/commit/4a52bc9294540de09a31e4c8e4cba0359ca3cd02))

## [1.3.0](https://github.com/sablierapp/sablier-traefik-plugin/compare/v1.2.0...v1.3.0) (2026-05-16)


### Features

* add fail open ([#40](https://github.com/sablierapp/sablier-traefik-plugin/issues/40)) ([1f3721b](https://github.com/sablierapp/sablier-traefik-plugin/commit/1f3721b636cfa2a07c9afc7bf3a1071ffd147d71))

## [1.2.0](https://github.com/sablierapp/sablier-traefik-plugin/compare/v1.1.0...v1.2.0) (2026-05-16)


### Features

* add keep alive ([#35](https://github.com/sablierapp/sablier-traefik-plugin/issues/35)) ([1ed46cb](https://github.com/sablierapp/sablier-traefik-plugin/commit/1ed46cb8f0be1759b90bad898ff0d28c89430d96))
* add plugin User-Agent ([#34](https://github.com/sablierapp/sablier-traefik-plugin/issues/34)) ([a32235c](https://github.com/sablierapp/sablier-traefik-plugin/commit/a32235c691e0d690c6b13a4415666a622f2a74ea))
* ignoreUserAgent is a list of regexp ([#38](https://github.com/sablierapp/sablier-traefik-plugin/issues/38)) ([b432f44](https://github.com/sablierapp/sablier-traefik-plugin/commit/b432f4435f4e8787f00b49236015ccf11f2c4bcb))


### Bug Fixes

* add Cache-Control: no-store when returning Sablier response ([#37](https://github.com/sablierapp/sablier-traefik-plugin/issues/37)) ([804630d](https://github.com/sablierapp/sablier-traefik-plugin/commit/804630d9a3884b65ab8c6c719ae628f3ac174974))
* **config:** empty name and empty group silently built a request with… ([#31](https://github.com/sablierapp/sablier-traefik-plugin/issues/31)) ([bffd1c6](https://github.com/sablierapp/sablier-traefik-plugin/commit/bffd1c6ef543d47880975f70418896de6ed60c72))
* forward sablier status code ([#33](https://github.com/sablierapp/sablier-traefik-plugin/issues/33)) ([dc832c6](https://github.com/sablierapp/sablier-traefik-plugin/commit/dc832c62a62556f7c9ae902dc53f4597958e8eba))
* mark as ready on non 503 ([#36](https://github.com/sablierapp/sablier-traefik-plugin/issues/36)) ([c4cb0f3](https://github.com/sablierapp/sablier-traefik-plugin/commit/c4cb0f372d7d7419402ad4ab6034d52bda27c883))
* propagate request context ([#32](https://github.com/sablierapp/sablier-traefik-plugin/issues/32)) ([f9b18d7](https://github.com/sablierapp/sablier-traefik-plugin/commit/f9b18d768fc7f74c072fe1693a0780273c8db7a1))


### Documentation

* fix release-please tags ([e755e71](https://github.com/sablierapp/sablier-traefik-plugin/commit/e755e71e4db2ee2f4097a4db4a47bec4ada948e4))
* update plugin version ([e0f7d9c](https://github.com/sablierapp/sablier-traefik-plugin/commit/e0f7d9cc97cd04fb10b6c8a7a3d97b2fa400c92b))


### Chores

* **deps:** bump actions/checkout from 5.0.0 to 6.0.0 ([#7](https://github.com/sablierapp/sablier-traefik-plugin/issues/7)) ([58b5fbf](https://github.com/sablierapp/sablier-traefik-plugin/commit/58b5fbfc064a4819bd3c9cd8e1fa0a7efe63548a))
* **deps:** bump actions/checkout from 6.0.0 to 6.0.1 ([#13](https://github.com/sablierapp/sablier-traefik-plugin/issues/13)) ([57a5647](https://github.com/sablierapp/sablier-traefik-plugin/commit/57a5647b2f1d92a7e6404df247be62aff7e87bf5))
* **deps:** bump actions/checkout from 6.0.1 to 6.0.2 ([#16](https://github.com/sablierapp/sablier-traefik-plugin/issues/16)) ([7e5ba59](https://github.com/sablierapp/sablier-traefik-plugin/commit/7e5ba593b3e13651afa427e8bdf49e9126678e4b))
* **deps:** bump actions/create-github-app-token from 2.1.4 to 2.2.0 ([#6](https://github.com/sablierapp/sablier-traefik-plugin/issues/6)) ([245d723](https://github.com/sablierapp/sablier-traefik-plugin/commit/245d723d3c26f967abfd4ca127477947a7913715))
* **deps:** bump actions/create-github-app-token from 2.2.0 to 2.2.1 ([#14](https://github.com/sablierapp/sablier-traefik-plugin/issues/14)) ([c9b9c6d](https://github.com/sablierapp/sablier-traefik-plugin/commit/c9b9c6d283ed08bd4dbb0d564312c454936f4875))
* **deps:** bump actions/create-github-app-token from 2.2.1 to 3.1.1 ([#22](https://github.com/sablierapp/sablier-traefik-plugin/issues/22)) ([6b99aa1](https://github.com/sablierapp/sablier-traefik-plugin/commit/6b99aa1508bc86944e03057a17acbb8730231b05))
* **deps:** bump actions/setup-go from 6.0.0 to 6.1.0 ([#5](https://github.com/sablierapp/sablier-traefik-plugin/issues/5)) ([0b33e15](https://github.com/sablierapp/sablier-traefik-plugin/commit/0b33e15fa64f73bf0381d894c45e185773ebdf94))
* **deps:** bump actions/setup-go from 6.1.0 to 6.2.0 ([#15](https://github.com/sablierapp/sablier-traefik-plugin/issues/15)) ([09f4200](https://github.com/sablierapp/sablier-traefik-plugin/commit/09f420084bfd66fa8bd3315345e83f1a5151e378))
* **deps:** bump actions/setup-go from 6.2.0 to 6.3.0 ([#17](https://github.com/sablierapp/sablier-traefik-plugin/issues/17)) ([1f14779](https://github.com/sablierapp/sablier-traefik-plugin/commit/1f1477999551f578af3522944925db1b971b7843))
* **deps:** bump actions/setup-go from 6.3.0 to 6.4.0 ([#20](https://github.com/sablierapp/sablier-traefik-plugin/issues/20)) ([d0421ed](https://github.com/sablierapp/sablier-traefik-plugin/commit/d0421ed5fd39408237ca8b09b7f186d0f0f8ba38))
* **deps:** bump golangci/golangci-lint-action from 9.0.0 to 9.1.0 ([#8](https://github.com/sablierapp/sablier-traefik-plugin/issues/8)) ([d304dad](https://github.com/sablierapp/sablier-traefik-plugin/commit/d304dade61e0d14a41fa4ad1732a7676aa14bc9c))
* **deps:** bump golangci/golangci-lint-action from 9.1.0 to 9.2.0 ([#12](https://github.com/sablierapp/sablier-traefik-plugin/issues/12)) ([8824985](https://github.com/sablierapp/sablier-traefik-plugin/commit/88249854b47c40818c8f105f69ca1539eef5da5e))
* **deps:** bump googleapis/release-please-action from 4.4.0 to 5.0.0 ([#25](https://github.com/sablierapp/sablier-traefik-plugin/issues/25)) ([c56a4b4](https://github.com/sablierapp/sablier-traefik-plugin/commit/c56a4b4613f6ec4cd2f87ae9b5a8f0fe4c11ad3b))

## [1.1.0](https://github.com/sablierapp/sablier-traefik-plugin/compare/v1.0.1...v1.1.0) (2025-11-22)


### Features

* add config option to ignore user agent ([#3](https://github.com/sablierapp/sablier-traefik-plugin/issues/3)) ([bbfdd21](https://github.com/sablierapp/sablier-traefik-plugin/commit/bbfdd21faa6c44097359cf82da7a916b15c36a12))


### Bug Fixes

* lint issues ([#4](https://github.com/sablierapp/sablier-traefik-plugin/issues/4)) ([d08b1a3](https://github.com/sablierapp/sablier-traefik-plugin/commit/d08b1a379e83e3ae739227f42132a6ea347fe060))


### Documentation

* add docker usage example ([95eb328](https://github.com/sablierapp/sablier-traefik-plugin/commit/95eb32881ee45263a0f7b023602c4eae34b45a64))
* add plugin configuration documentation ([4a142fe](https://github.com/sablierapp/sablier-traefik-plugin/commit/4a142fe527703ae82e6c824d0b560f48e084de45))
* add sponsor section ([3ff4989](https://github.com/sablierapp/sablier-traefik-plugin/commit/3ff498919aeb9e289bd70dfbddc00221ac14d5d4))
* add support section ([b432224](https://github.com/sablierapp/sablier-traefik-plugin/commit/b432224aa1daa67863ad5718fdd9f14d28a5f568))
* usage section ([a0d3c82](https://github.com/sablierapp/sablier-traefik-plugin/commit/a0d3c8267a2b52c4aa33626d4062747cef7788b3))

## [1.0.1](https://github.com/sablierapp/sablier-traefik-plugin/compare/v1.0.0...v1.0.1) (2025-11-09)


### Bug Fixes

* add docker example ([129a776](https://github.com/sablierapp/sablier-traefik-plugin/commit/129a776c22e855e96770c5dadb0e04382ee3f154))
* add golangci-lint ([aac5654](https://github.com/sablierapp/sablier-traefik-plugin/commit/aac5654177350fa2ac4cec2b2ee9660fa1d850e8))
* use github.com/sablierapp/sablier-traefik-plugin ([3e638bd](https://github.com/sablierapp/sablier-traefik-plugin/commit/3e638bd39bc8fcd168118a989d0fb35d7b5b12ec))


### Documentation

* add CONTRIBUTING and SUPPORT files ([ae1132c](https://github.com/sablierapp/sablier-traefik-plugin/commit/ae1132c8311d234effed0510002730b9c7630c6a))


### Chores

* add asset path ([a515b66](https://github.com/sablierapp/sablier-traefik-plugin/commit/a515b66202b603fa9ee2d708fbefcb102251fc6f))
