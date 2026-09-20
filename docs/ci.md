# CI

Five jobs on every pull request, in [ci.yaml](../.github/workflows/ci.yaml).

| Job | Checks |
|-----|--------|
| Build, vet and test | `gofmt`, `go vet`, `go test -race`, and prints total coverage |
| Lint | `golangci-lint` with the set in [.golangci.yaml](../.golangci.yaml), which includes `gosec` |
| Reachable vulnerabilities | `govulncheck`: an advisory only when a vulnerable symbol is actually called |
| Workflow syntax | `actionlint`: schema, expression syntax, `needs:` naming a job that exists, and shellcheck over every `run:` block |
| Workflow permissions | `zizmor`: unpinned actions, dangerous triggers, over-broad tokens — exceptions in [zizmor.yml](../.github/zizmor.yml) |
| Dispatch release | on a push to `main` only, after the five above — see [releases.md](releases.md) |

The two workflow jobs are separate rather than two steps in one, because a job
name is a required check: an audit buried inside another job's name leaves
nothing on the pull request saying it ran.

`zizmor` runs with `GH_TOKEN`, because without one it runs offline and skips
the audits that need the API. `--persona=regular` rather than `auditor`: the
auditor persona is documented as tolerating false positives, which is the wrong
contract for a blocking gate.

## Runners

Pinned to `ubuntu-24.04` rather than `ubuntu-latest`, which warned on every job
that it migrates to Ubuntu 26 on 19 October 2026. A floating label changes the
OS under these jobs on a date nobody here chose; pinned, that migration is a
commit. Spelled per job because `runs-on` reads neither `env` nor a
workflow-level default.

## Pins

Actions are pinned by commit SHA with the tag in a comment. Dependabot keeps
the module, the actions and npm current — see
[dependabot.yml](../.github/dependabot.yml).

The three tool versions in `ci.yaml`'s `env` are **bumped by hand**: they carry
`# renovate:` annotations in the style `hetzner-iac` uses, but nothing in this
repository reads them yet. Dependabot does not look inside a workflow's `env`.
