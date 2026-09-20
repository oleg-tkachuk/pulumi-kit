# stack

Answers and settles questions about Pulumi stacks.

```bash
stack exists <project-dir> <stack>   # exit zero if it is there, else the list of stacks that are
stack ensure <project-dir> <stack>   # select it, creating it first if needed; prints created|existing
stack ref    <project-dir> <stack>   # the <org>/<project>/<stack> a StackReference needs
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
