# target

Resolves a resource name into the URNs `pulumi --target` takes, and refuses a
name the stack does not hold.

```bash
target -dir <project-dir> -stack <stack> [-group-package <pkg>] <selector[,selector…]>
target -dir <project-dir> -stack <stack> -list
```

| Selector | Means |
|----------|-------|
| `traefik` | the resource with that name, if exactly one has it |
| `Release:traefik` | that name with that type leaf, when one name has two types |
| `group:Ingress` | a component resource and its whole subtree |
| `group:Network:net-a` | one of two components of the same type |
| `traefik,cert-manager` | both, as one run with two `--target` flags |

## `-list`

Prints what the stack holds, one selector per line:

```
Cluster:platform-dev
Firewall:platform-dev-firewall
Network:platform-dev-network
Release:traefik
```

It exists because that list was previously reachable only through a **failure**:
the refusal below carries it, so seeing a stack's contents meant mistyping a
selector on purpose.

It is the same list, from the same function, and a test asserts that — two
copies would drift, and the drift would be invisible because each would look
right on its own.

**Selectors, not resources.** The lines are deduplicated, so a stack of 24
resources can print 22 lines, and one selector may match more than one
resource: a component and a provider resource under it can share a type leaf
and a name, as `Network:platform-dev-network` does. Targeting it takes both,
which is what the list says it will.

`-list` takes no selector and refuses one rather than ignoring it — ignoring it
would leave a caller believing the list had been filtered. The stack name and
the project directory are still checked first.

## Why it exists

A `--target` that matches nothing **succeeds**:

```
pulumi preview --target '**::Release::does-not-exist'
Resources:
    + 1 to create
    24 unchanged
```

Exit code zero. So a mistyped name is an apply that claims to have worked and
changed nothing. A refusal here lists what the stack does hold instead.

## What it refuses

- **A name that matches nothing**, with the stack's contents as `Type:name`.
- **A name that matches two types**, naming every qualified form rather than
  guessing which was meant.
- **The same selector twice**, and a list where any one selector fails — the
  operator asked for all of them, and a partial apply that reports success is
  the failure this prevents.
- **`group:` with no `-group-package`.** A component named `Ingress` and a
  Kubernetes `Ingress` share a type leaf, so matching on the leaf alone would
  take every Ingress in the cluster.
- **`group:Type` when two components share that type**, naming both as
  `group:Type:name` rather than picking one.
- **A listing with no `parent` field at all.** The subtree is walked by
  following `parent`; without it a group would resolve to its own node and
  quietly leave the children behind.
- **A stack name that is not one**, and a directory with no `Pulumi.yaml`.

A comma-separated list is one `pulumi` run rather than one per component, which
is worth more than the typing: two runs are two chances for the second to act
on state the first one changed.

## How a group is resolved

By following `parent` from the component's URN, not by looking for its type in
other URNs. A URN's type path carries the types of a resource's ancestors and
not their names, so the substring form could not tell two instances of one
component apart — measured, with two `Network` components it returned the first
node, its child and the **other** node's child, and left the other node out.
