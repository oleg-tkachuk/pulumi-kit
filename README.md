# pulumi-kit

Small command-line tools for driving Pulumi from a Taskfile or CI job. Each one
exists because Pulumi's own answer to a mistake is silence.

Run them without installing anything:

```sh
go run github.com/oleg-tkachuk/pulumi-kit/cmd/target@latest -h
go run github.com/oleg-tkachuk/pulumi-kit/cmd/stack@latest
```

## `target`

Resolves a resource name into the URNs `pulumi --target` takes, and refuses a
name the stack does not hold.

A `--target` that matches nothing **succeeds** — `pulumi preview --target
'**::Release::does-not-exist'` reports `24 unchanged` and exits zero — so a
mistyped name is an apply that claims to have worked and changed nothing.

```sh
target -dir layers/ingress -stack dev traefik
target -dir layers/ingress -stack dev Release:argocd          # when a name has two types
target -dir layers/ingress -stack dev traefik,cert-manager    # one apply, two targets
target -dir . -stack dev -group-package acme group:Ingress    # a component and its children
```

`-group-package` is the package a component's type token begins with, and it is
required for `group:` selectors: a component named `Ingress` and a Kubernetes
`Ingress` share a type leaf, so matching on the leaf alone would target every
Ingress in the cluster.

## `stack`

```sh
stack exists <project-dir> <stack>   # zero if it is there, else the list of stacks that are
stack ensure <project-dir> <stack>   # select it, creating it first if needed; prints created|existing
stack ref    <project-dir> <stack>   # the <org>/<project>/<stack> a StackReference needs
```

`ensure` replaces `pulumi stack init || pulumi stack select`, which hides the
real failure: init's stderr is discarded to keep the "already exists" case
quiet, so a rejected tag or a bad token surfaces only as select complaining the
stack does not exist.

## Using them from a Taskfile

```yaml
vars:
  TARGET: 'go run github.com/oleg-tkachuk/pulumi-kit/cmd/target@v0.1.0'
tasks:
  apply:
    cmds:
      - |
        targets="$({{.TARGET}} -dir "{{.dir}}" -stack "{{.stack}}" "{{.target}}" \
          | awk '{printf " --target %s", $0}')"
        pulumi --non-interactive --cwd "{{.dir}}" --stack "{{.stack}}" up --yes ${targets}
```

Pin a tag rather than `@latest` anywhere a build has to be reproducible.
