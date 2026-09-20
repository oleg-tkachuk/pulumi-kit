# stack

Answers and settles questions about Pulumi stacks.

```bash
stack exists <project-dir> <stack>   # status says whether it is there; see below
stack ensure <project-dir> <stack>   # select it, creating it first if needed; prints created|existing
stack ref    <project-dir> <stack>   # the <org>/<project>/<stack> a StackReference needs
```

## `exists` has three answers, not two

| Status | Means |
|--------|-------|
| `0` | the stack is there |
| `2` | the project has no stack of that name; the ones it does have are on stderr |
| `1` | the question could not be answered — no credentials, no network, not a project |

Two of those were one status, and a caller cannot tell them apart from one:

```bash
if stack exists "$dir" "$name"; then :; else create_it; fi   # creates on a network failure
```

Test for absence explicitly instead:

```bash
stack exists "$dir" "$name" && exit 0
[ $? -eq 2 ] || exit 1   # a real failure, not an absence
create_it
```

## Why it exists

It replaces one shell idiom, which appeared three times in the repository it
came from and was the reason `jq` was a prerequisite at all:

```bash
pulumi stack ls --json | jq -e --arg s "$STACK" 'any(.[]; .name == $s)'
```

`ensure` makes the whole decision rather than half of it. The obvious
`pulumi stack init || pulumi stack select` is worse than it looks: it discards
init's stderr to keep the "already exists" case quiet, so **any** init failure
— a rejected stack tag, a bad token, no network — surfaces only as select
complaining the stack does not exist. That cost a real diagnosis once: init was
refusing a description over 256 characters, and the operator read
`no stack named 'dev' found`.

`ref` reads `pulumi stack ls -Q`, not `pulumi --stack <name> stack --show-name`:
the latter falls back to the **selected** stack when the name is empty, so a
caller with an unset variable gets a confident answer about the wrong stack. A
self-managed backend has no organization to qualify with, and `ref` says so
rather than returning a name `pulumi.NewStackReference` cannot resolve.

`ensure` prints one word so a caller can put it in a column of its own table
rather than parse a sentence out of it.
