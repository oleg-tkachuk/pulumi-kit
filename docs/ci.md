# CI

Five jobs on every pull request, in [ci.yaml](../.github/workflows/ci.yaml).

| Job | Checks |
|-----|--------|
| Build, vet and test | `gofmt`, `go vet`, `go test -race`, and prints total coverage |
| Lint | `golangci-lint` with the set in [.golangci.yaml](../.golangci.yaml), which includes `gosec` |
| Reachable vulnerabilities | `govulncheck`: an advisory only when a vulnerable symbol is actually called |
| Workflows | `actionlint` for syntax, then `zizmor` for unpinned actions, dangerous triggers and over-broad tokens |
| Dispatch release | on a push to `main` only, after the four above — see [releases.md](releases.md) |

`actionlint` runs before `zizmor` on purpose: `zizmor` will read a file
`actionlint` would have rejected, so a broken workflow should fail as a syntax
error rather than as a confusing audit result.

## Pins

Actions are pinned by commit SHA with the tag in a comment. Dependabot keeps
the module, the actions and npm current — see
[dependabot.yml](../.github/dependabot.yml).

The three tool versions in `ci.yaml`'s `env` are **bumped by hand**: they carry
`# renovate:` annotations in the style `hetzner-iac` uses, but nothing in this
repository reads them yet. Dependabot does not look inside a workflow's `env`.
