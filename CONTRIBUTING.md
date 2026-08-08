# Contributing

Thanks for your interest in improving zap-prettyconsole!

## Pull request titles

Releases and the changelog are automated with
[release-please](https://github.com/googleapis/release-please), which reads
[conventional commit](https://www.conventionalcommits.org/) messages. Pull
requests are squash-merged, so **the PR title becomes the commit message** and
must follow the convention, e.g.:

- `feat: Add support for X` (minor version bump)
- `fix: Handle nil errors in Y` (patch version bump)
- `docs:`, `chore:`, `ci:`, `test:`, `refactor:` (no release)

A CI check validates PR titles automatically.

## Development

Everyday tasks are driven through the Makefile:

```console
make test       # run tests with the race detector
make coverage   # run tests and print a coverage summary
make lint       # run golangci-lint (see .golangci.toml)
make fmt        # format code with gofumpt
make bench      # run benchmarks
```

A [devenv](https://devenv.sh)-based Nix flake is provided (`nix develop`) with
the Go toolchain, golangci-lint and gofumpt used by CI.

## The README is generated

Do not edit `README.md` directly. It is rendered from
`internal/readme/readme.tmpl` by `internal/readme/readme.go`, which also runs
the benchmarks to fill in the performance tables:

```console
make README.md
```

The screenshots are generated from the examples in
`internal/readme/example_test.go` using
[termshot](https://github.com/homeport/termshot), so those examples are real,
tested code. CI checks that the committed README matches the template (ignoring
the benchmark timings, which vary run to run).

## Supported Go versions and dependencies

This library aims to stay as widely consumable as possible, so the version
floors in `go.mod` are kept deliberately low:

- The minimum Go version is declared in `go.mod` and the full test suite
  runs against exactly that version in CI, alongside the two most recent
  Go releases. Only raise the declared minimum when the code actually
  needs a newer language feature, and update the matrix in
  `.github/workflows/test.yml` to match.
- Dependency requirements are only raised when there is a concrete reason:
  a needed fix or feature, or a security advisory (Dependabot is
  configured to propose security updates but no routine version bumps).
  In a Go library, `go.mod` declares *minimums* that propagate to every
  consumer via minimal version selection, so routine dependency bumps
  restrict consumers for no benefit — consumers can always select newer
  versions themselves. Note that `go test` in this repository resolves to
  exactly the declared minimums, so they are what CI exercises.
