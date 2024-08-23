# Changelog

## [0.5.2](https://github.com/thessem/zap-prettyconsole/compare/v0.5.1...v0.5.2) (2024-08-23)


### Bug Fixes

* Fix issues identified by golangci-lint ([e2b37a4](https://github.com/thessem/zap-prettyconsole/commit/e2b37a416ca2d5e17a637cf8be7e08f0a1961998))


### Documentation

* Add changelog for previous versions ([3012859](https://github.com/thessem/zap-prettyconsole/commit/3012859f443e3dd4930b31da2ee22a3dcb442e20))


### Miscellaneous Chores

* Delete IntelliJ project config ([97991e4](https://github.com/thessem/zap-prettyconsole/commit/97991e4118aaf0c7f089e8edc13004ab4f03bd2b))


### Continuous Integration

* Add basic GitHub actions configuration for release ([389598b](https://github.com/thessem/zap-prettyconsole/commit/389598b85aa53353bed76d7764034f53d532a8b8))
* Add golangci-lint configuration to project ([0d53925](https://github.com/thessem/zap-prettyconsole/commit/0d53925644cedf99f44314c548bd0407eb09672f))
* Add golangci-lint, flake.nix to run it and VSCode config for it ([62f6c50](https://github.com/thessem/zap-prettyconsole/commit/62f6c5010adab601d85b4e41b3b094ca36d4bda9))
* Add tests and benchmarks to CI ([b3b6894](https://github.com/thessem/zap-prettyconsole/commit/b3b6894128a96fbbeb832ecca70006518be3568a))

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
