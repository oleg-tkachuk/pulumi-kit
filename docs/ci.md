# CI

In [ci.yaml](../.github/workflows/ci.yaml).

| Job | Checks |
|-----|--------|
| Changed paths | whether anything the jobs below read has changed |
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

## Which of them a merge waits for

Six are required by `main`'s protection: `Changed paths`, `Build, vet and test`,
`Lint`, `Reachable vulnerabilities`, `Workflow syntax` and `Workflow
permissions`. `Dispatch release` is not — it never runs on a pull request.

A **skipped** required check does not block a merge, which is what makes the
gate below safe.

Renaming a job renames its check, and a required check that never reports again
blocks every open pull request — so a rename is a protection change in the same
breath.

## A documentation change runs nothing

`Changed paths` diffs the pull request and every other job is gated on its
answer, so editing a README reports five skipped checks rather than building
and scanning the tree.

The filter is an **exclusion** list — documentation, the licence, `.gitignore`
and images — and the direction is the point. An inclusion list has to be
extended for each new kind of input, and forgetting is silent and green: a
change to `.golangci.yaml` that skipped the linter, or to a workflow that
skipped the workflow audits. Excluding documentation cannot fail that way,
because a file type nobody has thought about yet is relevant by default.

Both ways of getting the pattern wrong are quiet, and one of them is
dangerous: too broad, and a change to code is called documentation, every check
reports skipped, and the pull request is green having run nothing.
`internal/ci` reads the pattern out of the workflow and holds it to a table of
real paths — checked by adding `\.go$` to it and watching the gate fail.

A push is diffed too, against the branch's previous head. It used to be
exempt — "a push to main is always relevant" — and that exemption was the whole
gap: merging a README ran the full build on `main` anyway, which is what the
gate was asked to stop.

Three cases have no usable base and mean everything: a push that created the
branch reports all zeros, a force-push can name a commit the clone does not
have, and a pull request event carries no previous head.

`Dispatch release` runs under `always()`, because a documentation-only push
skips the five checks and a plain `needs` would skip the dispatch with them —
no release would ever be cut for a push that only edits prose. It still refuses
to dispatch after a `failure` or a `cancelled`. Releasing from a push whose
checks were skipped is not a hole: the code is byte-identical to the commit
before it, which was checked. `internal/ci` pins both halves of that condition,
because each has its own way of being quietly wrong.

## Timeouts

Every job allows five minutes. They finish in twenty to forty-five seconds, so
five is generous and the point is to fail fast: a `setup-go` that hung held a
job for the whole ten minutes the workflow used to allow while its three
neighbours had finished in under twenty-five seconds.

`setup-go` also has a two-minute timeout of its own, so that failure names the
step rather than reporting the job as slow.

The release job keeps ten minutes. It installs npm dependencies, computes a
version from the whole history and fetches the module twice; cutting it to five
would risk aborting a release that was working.

## Runners

Pinned to `ubuntu-24.04` rather than `ubuntu-latest`, which warned on every job
that it migrates to Ubuntu 26 on 19 October 2026. A floating label changes the
OS under these jobs on a date nobody here chose; pinned, that migration is a
commit. Spelled per job because `runs-on` reads neither `env` nor a
workflow-level default.

## Pins

Actions are pinned by commit SHA with the tag in a comment, and everything —
the module, the actions, npm and the three tool versions in `ci.yaml`'s `env` —
is Renovate's, configured in [renovate.json](../.github/renovate.json).

It replaced Dependabot rather than joining it. Dependabot handles gomod,
github-actions and npm natively but cannot see a version pinned inside a
workflow's `env`, which is the whole reason for the custom manager; and two
bots on the same ecosystems means two pull requests for one bump.

Renovate runs from [renovate.yaml](../.github/workflows/renovate.yaml) rather
than the hosted app, because installing the app needs account rights that were
not available. That costs a `RENOVATE_TOKEN` secret — `GITHUB_TOKEN` cannot
serve, because GitHub starts no workflow run from an event it caused, so the
checks on a Renovate pull request would never run and it could never merge. The
workflow refuses with the exact scopes to grant when the secret is missing.

The cron decides how often Renovate runs; `renovate.json`'s schedule decides
what it may do when it does. Regular updates wait for the whole of Monday so
they batch into one review; vulnerability alerts are exempt. A manual
`workflow_dispatch` ignores the schedule, which is how a first pass happens
without waiting.

A bump releases nothing: tool, action and npm updates are typed `ci` and the
module graph `chore`, neither of which semantic-release acts on. That is right
while the module graph is test-only — a dependency reaching the commands
themselves would need a type it does act on, and `renovate.json` says so.
