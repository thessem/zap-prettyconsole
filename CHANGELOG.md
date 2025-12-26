# Changelog

## [0.6.0](https://github.com/thessem/zap-prettyconsole/compare/v0.5.2...v0.6.0) (2025-12-26)


### Features

* Add df.WithRichBytes() for improved byte slice formatting ([#27](https://github.com/thessem/zap-prettyconsole/issues/27)) ([6b1b241](https://github.com/thessem/zap-prettyconsole/commit/6b1b2412ee05725fbdd5d2824d3080da0b4c7e81))
* Fix unnecessarily long time logs with dd ([#25](https://github.com/thessem/zap-prettyconsole/issues/25)) ([019fff7](https://github.com/thessem/zap-prettyconsole/commit/019fff7a92de8c50d109050b5aadcfb4578c96f2))
* Print common fixed sized by arrays as hex through dd ([#28](https://github.com/thessem/zap-prettyconsole/issues/28)) ([2092019](https://github.com/thessem/zap-prettyconsole/commit/2092019f58852ba6d07ba96f3a70dcae932dae21))


### Documentation

* Fix example code in Readme to match generated image ([#22](https://github.com/thessem/zap-prettyconsole/issues/22)) ([06d0bbd](https://github.com/thessem/zap-prettyconsole/commit/06d0bbd46f755e9bdfc7461d120940db5f48a159))
* Fix object example in readme ([#20](https://github.com/thessem/zap-prettyconsole/issues/20)) ([44b8f84](https://github.com/thessem/zap-prettyconsole/commit/44b8f84736276086a799161b337f36283170d420))


### Miscellaneous Chores

* Improve Makefile targets and test coverage  ([#26](https://github.com/thessem/zap-prettyconsole/issues/26)) ([1ef67a8](https://github.com/thessem/zap-prettyconsole/commit/1ef67a8dae4abf0d1e2286f79e3c1ee83b2e11bb))

## [0.5.2](https://github.com/thessem/zap-prettyconsole/compare/v0.5.1...v0.5.2) (2024-08-23)


### Bug Fixes

* Fix issues identified by golangci-lint ([#16](https://github.com/thessem/zap-prettyconsole/issues/16)) ([9d655f8](https://github.com/thessem/zap-prettyconsole/commit/9d655f861333e9683a2a0bd054eef974f8636d0e))


### Documentation

* Add changelog for previous versions ([#18](https://github.com/thessem/zap-prettyconsole/issues/18)) ([e5019c1](https://github.com/thessem/zap-prettyconsole/commit/e5019c11671ae5c94cafdbf0209444e18da57f94))


### Miscellaneous Chores

* Delete IntelliJ project config ([#15](https://github.com/thessem/zap-prettyconsole/issues/15)) ([c30df76](https://github.com/thessem/zap-prettyconsole/commit/c30df76e2713b7472c3ed98138421a1fb0c89213))


### Continuous Integration

* Add basic GitHub actions configuration for release ([#12](https://github.com/thessem/zap-prettyconsole/issues/12)) ([e571c45](https://github.com/thessem/zap-prettyconsole/commit/e571c45dd3baa0b13ce04285f46a023d2db04655))
* Add Golangci-lint GitHub action ([#19](https://github.com/thessem/zap-prettyconsole/issues/19)) ([592ac55](https://github.com/thessem/zap-prettyconsole/commit/592ac554c5f618b05b326b84276d589c85c85a95))
* Add golangci-lint, flake.nix to run it and VSCode config for it ([#17](https://github.com/thessem/zap-prettyconsole/issues/17)) ([921f5b6](https://github.com/thessem/zap-prettyconsole/commit/921f5b63a3ef14e4b70d776cc2232d74eede5b0e))
* Add tests and benchmarks to CI ([#14](https://github.com/thessem/zap-prettyconsole/issues/14)) ([00985c8](https://github.com/thessem/zap-prettyconsole/commit/00985c88d19ba66a3eee21a199efb8cebfb34b7b))

## [0.5.1](https://github.com/thessem/zap-prettyconsole/compare/v0.5.0...v0.5.1) (2024-06-14)

- **fix:** Avoid panics when Unwrap/Cause return nil errors ([1455806](https://github.com/thessem/zap-prettyconsole/commit/1455806e09aae5319ce8072477d4d2a4e5865730))

## [0.5.0](https://github.com/thessem/zap-prettyconsole/compare/v0.4.0...v0.5.0) (2024-03-23)

- **feat:** Support Go 1.20 error interfaces ([cc9b274](https://github.com/thessem/zap-prettyconsole/commit/cc9b27481c4242d3ad8ee69c65004e0faebeb6f8))

- **docs:** Don't repeat error causes in error message ([cc9b274](https://github.com/thessem/zap-prettyconsole/commit/cc9b27481c4242d3ad8ee69c65004e0faebeb6f8))

## [0.4.0](https://github.com/thessem/zap-prettyconsole/compare/v0.3.1...v0.4.0) (2023-01-23)

- **feat:** Add ability to print pre-formatted strings ([09220456](https://github.com/thessem/zap-prettyconsole/commit/09220456fee8abe59f9d2661f8377cc9de7bdfaa))

## [0.3.1](https://github.com/thessem/zap-prettyconsole/compare/v0.3.0...v0.3.1) (2022-12-02)

- **fix:** Fix stacktrace outputs when -trimpath is set ([426ebc3](https://github.com/thessem/zap-prettyconsole/commit/426ebc3aeb56808cd50bb8071a18181f3703daee))

- **docs:** Update docs ([b9232f6](https://github.com/thessem/zap-prettyconsole/commit/b9232f6964e8879286de4c007b938e723dec96ff))

- **chore:** Bump dependencies ([19d55120](https://github.com/thessem/zap-prettyconsole/commit/19d55120562450a36fc314806a2b78705cc90d31))

- **chore:** Update benchmarking ([19d55120](https://github.com/thessem/zap-prettyconsole/commit/19d55120562450a36fc314806a2b78705cc90d31))

## [0.3.0](https://github.com/thessem/zap-prettyconsole/compare/v0.2.0...v0.3.0) (2022-08-29)

- **fix:** Account for `.With` functionality ([d3371ba](https://github.com/thessem/zap-prettyconsole/commit/d3371baaacd6dbb31828ec944ffa2f4959f63579))

- **docs:** Fix broken link in Readme ([ca86b8b](https://github.com/thessem/zap-prettyconsole/commit/ca86b8bd80529f6eca3ed17b99eba7e9ba030276))

- **chore:** Delete zap-prettyconsole.test ([a98269d](https://github.com/thessem/zap-prettyconsole/commit/a98269d63d17b4b09e96d22225a61d38825c1600))

- **chore:** Delete out.png ([62e33e9](https://github.com/thessem/zap-prettyconsole/commit/62e33e942ff12e53cdd7ab777258f97263ffc8ff))

## [0.2.0](https://github.com/thessem/zap-prettyconsole/compare/v0.1.0...v0.2.0) (2022-08-28)

- **feat:** Add more configuration options ([56698f7](https://github.com/thessem/zap-prettyconsole/commit/56698f7db466a69e1f75982baf44ea9c923eaa15))

- **feat:** Automate readme ([56698f7](https://github.com/thessem/zap-prettyconsole/commit/56698f7db466a69e1f75982baf44ea9c923eaa15))

## [0.1.0](https://github.com/thessem/zap-prettyconsole/commits/v0.1.0) (2022-08-26)

- **feat:** Initial implementation of zap-prettyconsole ([b0d2061](https://github.com/thessem/zap-prettyconsole/commit/b0d2061bc87e1180d2a7e6811bfabf6028793c07))

- **fix:** Do a better job of not sharing the encoder ([1eaaae5](https://github.com/thessem/zap-prettyconsole/commit/1eaaae5355ae731f8d57f0422fca941a1f5bb69f))

- **fix:** Add line ending to logger output ([c134005](https://github.com/thessem/zap-prettyconsole/commit/c13400595bfb17f102d73288a733c5529e7f04a7))

- **docs:** Add Readme and License ([9cdd17d](https://github.com/thessem/zap-prettyconsole/commit/9cdd17d80e7fc327396fd9f36013164bb2f3577f))
