# Design

## What each package owns

| Package | Owns |
|---------|------|
| [`internal/pkg/pulumi`](../internal/pkg/pulumi) | running the CLI, and the two checks every caller makes first: the stack name and the project directory |
| [`internal/pkg/stack`](../internal/pkg/stack) | whether a stack exists, making sure it does, and its fully qualified reference |
| [`internal/pkg/target`](../internal/pkg/target) | turning a selector into URNs, or refusing with the list of what the stack holds |
| [`cmd/*`](../cmd) | argument parsing and printing. No decisions |

## One runner, not one per command

Both commands started with their own way of invoking the CLI — one setting
`cmd.Dir`, the other passing `--cwd` — and they disagreed about more than that.
Only one validated the stack name it handed to `exec`, and only one said
anything useful when the directory held no `Pulumi.yaml`. Neither difference
was a decision; both were accidents of having been written separately.

`internal/pkg/pulumi` is the one place all three now live. The asymmetry that
unifying them exposed became its own fix: `stack` validates the name.

## Parsing is separated from calling

Every function that reads the CLI's output is separate from the one that runs
it. That is what lets the decisions be tested against output written by hand,
with no Pulumi backend and no network — which is most of what the test suite
does.

## The group package is required, not guessed

A component named `Ingress` has the type token `<pkg>:platform:Ingress`. A
Kubernetes Ingress has `kubernetes:networking.k8s.io/v1:Ingress`. The same
type **leaf**.

Matching on the leaf alone made `group:Ingress` take every Ingress object in
the cluster, so `target` needs the package a component's token begins with and
refuses a `group:` selector without it. Targeting more than was asked for is
the one failure a tool about targeting must not have.

## Why these two tools exist at all

Both are wrappers around a silence.

**`--target` that matches nothing succeeds.** Measured:

```
pulumi preview --target '**::Release::does-not-exist'
Resources:
    + 1 to create
    24 unchanged
```

Exit code zero. A caller that passes a mistyped name reports success and
applies nothing.

**`stack init || stack select` reports the wrong failure.** The `||` form
discards init's stderr to keep the "already exists" case quiet, so a rejected
stack tag, a bad token or no network all surface as select complaining the
stack does not exist. That cost a real diagnosis once: init was refusing a
description over 256 characters, and the operator read `no stack named 'dev'
found`.
