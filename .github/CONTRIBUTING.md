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

`go test ./...` and `golangci-lint run` are what CI runs first; running them
before opening a request saves a round trip.

**Security reports are the exception to the public process.** See
[SECURITY.md](SECURITY.md).

Licence is [MIT](../LICENSE).
