# pulumi-kit

[![CI](https://github.com/oleg-tkachuk/pulumi-kit/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/oleg-tkachuk/pulumi-kit/actions/workflows/ci.yaml)
[![release](https://img.shields.io/github/v/release/oleg-tkachuk/pulumi-kit?sort=semver&label=release&cacheSeconds=3600)](https://github.com/oleg-tkachuk/pulumi-kit/releases/latest)
[![go](https://img.shields.io/github/go-mod/go-version/oleg-tkachuk/pulumi-kit?logo=go&logoColor=white&label=go&cacheSeconds=3600)](go.mod)
[![license: MIT](https://img.shields.io/github/license/oleg-tkachuk/pulumi-kit?label=license&cacheSeconds=3600)](LICENSE)

[![Pulumi](https://img.shields.io/badge/Pulumi-8A3391?logo=pulumi&logoColor=white)](https://www.pulumi.com)
[![Task](https://img.shields.io/badge/Task-29BEB0?logo=task&logoColor=white)](https://taskfile.dev)

Command-line tools for driving Pulumi from a Taskfile or CI job. Each one
exists because Pulumi's own answer to a mistake is silence.

## Contents

- [The commands](#the-commands)
- [Quick start](#quick-start)
- [Using them from a Taskfile](#using-them-from-a-taskfile)
- [Documentation](#documentation)
- [Licence](#licence)

## The commands

| Command | What it answers |
|---------|-----------------|
| [`target`](cmd/target) | which URNs does this name mean — and does it name anything at all |
| [`stack`](cmd/stack) | does this stack exist, make sure it does, what is its reference |

## Quick start

Nothing to install. `go run` fetches the pinned version when a task needs it,
so neither command enters your `go.mod`:

```bash
go run github.com/oleg-tkachuk/pulumi-kit/cmd/target@latest -h
go run github.com/oleg-tkachuk/pulumi-kit/cmd/stack@latest -h
```

Requires the `pulumi` CLI on `PATH` and Go 1.27 or newer.

## Using them from a Taskfile

Written for [Task](https://taskfile.dev) — the runner whose
[`vars`](https://taskfile.dev/reference/schema#variable) and
[`cmds`](https://taskfile.dev/reference/schema#task) this uses. Pin a tag
rather than `@latest` anywhere a build has to be reproducible:

```yaml
vars:
  KIT: 'go run github.com/oleg-tkachuk/pulumi-kit/cmd/target@v0.1.3'
tasks:
  apply:
    cmds:
      - |
        targets="$({{.KIT}} -dir "{{.dir}}" -stack "{{.stack}}" "{{.target}}" \
          | awk '{printf " --target %s", $0}')"
        pulumi --non-interactive --cwd "{{.dir}}" --stack "{{.stack}}" up --yes ${targets}
```

## Documentation

| Document | Open it when |
|----------|--------------|
| [cmd/target](cmd/target) | you need the selector syntax and what each refusal means |
| [cmd/stack](cmd/stack) | you need the three subcommands and what each prints |
| [docs/design.md](docs/design.md) | you want to know why it is shaped this way |
| [docs/ci.md](docs/ci.md) | you are changing what lands, or working out why a check ran |
| [docs/releases.md](docs/releases.md) | you want to know how a version gets cut |

Extracted from [hetzner-iac](https://github.com/oleg-tkachuk/hetzner-iac),
where both tools were repository-local and neither needed to be.

## Licence

MIT — see [LICENSE](LICENSE).
