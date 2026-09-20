# target

Resolves a resource name into the URNs `pulumi --target` takes, and refuses a
name the stack does not hold.

```bash
target -dir <project-dir> -stack <stack> [-group-package <pkg>] <selector[,selector…]>
```

| Selector | Means |
|----------|-------|
| `traefik` | the resource with that name, if exactly one has it |
| `Release:traefik` | that name with that type leaf, when one name has two types |
| `group:Ingress` | a component resource and everything under it |
| `traefik,cert-manager` | both, as one run with two `--target` flags |

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
- **A stack name that is not one**, and a directory with no `Pulumi.yaml`.

A comma-separated list is one `pulumi` run rather than one per component, which
is worth more than the typing: two runs are two chances for the second to act
on state the first one changed.
