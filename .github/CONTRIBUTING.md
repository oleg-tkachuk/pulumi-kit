# Contributing

Pull requests are welcome. This is a small kit with one maintainer, so two
things keep it reviewable:

**Conventional Commits.** The subject decides the next version — see
[docs/releases.md](../docs/releases.md). A non-conforming subject silently
produces no release.

**Tests with the change.** Every decision in here is reachable from a test that
needs no Pulumi backend: the functions that parse are separate from the ones
that call the CLI, precisely so that is possible. A bug fix starts with a test
that reproduces it.

`task verify-all` runs what CI runs — gofmt, tidy, vet, tests, golangci-lint,
govulncheck and actionlint; running it before opening a request saves a round
trip. With the `pulumi` CLI installed, `task test:integration` runs the same
packages against it — no account and no network, the backend is a temporary
directory. `task deps:outdated` and `task deps:update` check and bump the Go
dependencies; `task --list` shows the rest.

**Security reports are the exception to the public process.** See
[SECURITY.md](SECURITY.md).

Licence is [MIT](../LICENSE).
