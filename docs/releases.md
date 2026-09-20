# Releases

The tag is the release. A Go module needs no upload — the proxy serves whatever
the tag points at.

## What a commit type releases

`semantic-release` reads the history with the plain `conventionalcommits`
preset, configured in [release.config.cjs](../release.config.cjs):

| Commit | Release |
|--------|---------|
| `fix:`, `perf:`, `revert:` | patch |
| `feat:` | minor |
| `!` or a `BREAKING CHANGE:` footer | major |
| `docs:`, `style:`, `refactor:`, `test:`, `build:`, `ci:`, `chore:` | nothing |

So a CI-only change releases nothing, which is the intent.

## The chain

1. A push to `main` runs [ci.yaml](../.github/workflows/ci.yaml).
2. Its `dispatch-release` job sends a `repository_dispatch`.
3. [release.yml](../.github/workflows/release.yml) computes the version, pushes
   the tag, creates the GitHub release, proves the tag is consumable, and tells
   the module proxy the version exists.

`repository_dispatch` rather than the obvious alternatives, for two reasons
that are not style. `workflow_run` is refused by `zizmor`'s dangerous-triggers
audit, whose own docs say no guard satisfies it. And GitHub starts no workflow
run from an event `GITHUB_TOKEN` caused — which is also why the tag pushed in
step 3 cannot chain a workflow of its own. `repository_dispatch` is one of the
two documented exceptions that always create a run, so this needs no extra
credential.

## Two steps that exist because they were once wrong

**Proving the tag is consumable** runs `go run …/cmd/<name>@<tag> -h` and
requires a usage line. Its first version could not fail: `2>&1 | head -3`
consumed three lines of `go: downloading` on stderr and truncated before the
program said anything, while `|| true` would have swallowed a non-zero exit.
It reported success having proved nothing.

**Warming the module proxy** requests the new version's `.info`. The check
above uses `GOPROXY=direct`, so nothing in the workflow asked
`proxy.golang.org` for the version — and a consumer reading the proxy did not
see it. Measured on `v0.1.2`: Renovate found the version in the proxy's `list`
and still proposed no upgrade, because `@latest` was cached at the previous
tag.
