# Security policy

## Reporting a vulnerability

Use GitHub's private vulnerability reporting on this repository
(**Security → Report a vulnerability**). Please do not open a public issue for
something exploitable.

## What is in scope

Both commands shell out to the `pulumi` CLI and parse its output, so the
interesting surface is what reaches `exec` and what is trusted on the way back:

- **Argument handling.** Everything is passed as an argument vector, never
  through a shell. The stack name is matched against a pattern before it
  reaches `exec`, and the project directory has to contain a `Pulumi.yaml`.
  A path or name that escapes either check is in scope.
- **What the tools print.** `target` writes URNs a caller interpolates into a
  `pulumi` command line. A selector that makes it emit something other than a
  URN from the stack's own state is in scope.
- **Over-matching.** `group:` taking a resource outside the named component is
  in scope: a target list wider than asked for is how a destroy removes
  something nobody named.
- **The release pipeline.** An unpinned action, a token with more than it
  needs, or a trigger that runs untrusted code with repository write access.

## What is not

- The `pulumi` CLI itself, and the backend it talks to. Report those upstream.
- Anything requiring the reporter to already be able to run arbitrary commands
  on the machine. A caller who can pass `-dir` can already run `pulumi`.
- Stack names or URNs appearing in CI logs. They are not secrets, and the
  tools deliberately read `stack --show-urns` rather than `stack export`,
  which would put every resource's inputs and any encrypted secret on a pipe.

## Supported versions

The latest tag. This is a single-module repository with no release branches, so
a fix ships as a new patch version rather than as a backport.
