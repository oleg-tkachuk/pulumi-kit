// Package stack answers and settles questions about Pulumi stacks.
//
// It replaces one shell idiom:
//
//	pulumi stack ls --json | jq -e --arg s "$STACK" 'any(.[]; .name == $s)'
//
// Ensure does the whole decision rather than half of it. The obvious
// `pulumi stack init || pulumi stack select` is worse than it looks: it sends
// init's stderr to /dev/null to keep the "already exists" case quiet, so ANY
// init failure — a rejected stack tag, a bad token, no network — surfaces only
// as select complaining the stack does not exist.
//
// Every function that parses is separated from the one that calls the CLI, so
// the parsing can be tested without a Pulumi backend.
package stack

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/pulumi"
)

// Row is the part of `pulumi stack ls --json` this package reads.
type Row struct {
	Name string `json:"name"`
}

// NoStacks is what Names reports for a project with none, so a caller
// formatting a list never prints an empty string.
const NoStacks = "none"

// The one word Ensure returns, so a caller can put it in a column of its own
// table rather than parse a sentence out of it.
const (
	StateCreated  = "created"
	StateExisting = "existing"
)

// qualifiedSegments is how many parts a fully qualified stack name has:
// <org>/<project>/<stack>. Pulumi Cloud prints all three under -Q; a
// self-managed backend has no organization and prints one, which is a name
// pulumi.NewStackReference cannot resolve.
const qualifiedSegments = 3

// listing asks the backend what stacks the project has.
var listing = []string{"stack", "ls", "--json"}

// qualifiedListing is the same, with every name fully qualified.
var qualifiedListing = []string{"stack", "ls", "-Q", "--json"}

// ParseRows is the one place that turns the CLI's json into rows, so every
// caller reports the same thing when it is not json at all.
func ParseRows(raw []byte) ([]Row, error) {
	var rows []Row
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("pulumi stack ls returned no usable json: %w", err)
	}

	return rows, nil
}

// Named answers the question the jq pipeline answered.
//
// An unparseable list is an error rather than "absent": "absent" would make
// Ensure try to create a stack that already exists.
func Named(raw []byte, name string) (bool, error) {
	rows, err := ParseRows(raw)
	if err != nil {
		return false, err
	}

	for _, row := range rows {
		if row.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// Exists says whether the project in dir has a stack of that name.
func Exists(ctx context.Context, dir, name string) (bool, error) {
	out, err := pulumi.Run(ctx, dir, listing...)
	if err != nil {
		return false, err
	}

	return Named(out, name)
}

// Names lists the stacks a project has, for an error that tells the operator
// what to pick instead.
func Names(ctx context.Context, dir string) ([]string, error) {
	out, err := pulumi.Run(ctx, dir, listing...)
	if err != nil {
		return nil, err
	}

	rows, err := ParseRows(out)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return []string{NoStacks}, nil
	}

	available := make([]string, 0, len(rows))
	for _, row := range rows {
		available = append(available, row.Name)
	}

	return available, nil
}

// QualifiedName picks one stack out of a fully qualified listing.
//
// Reading the backend's answer rather than assembling one: the organization
// from `pulumi whoami` is a guess as soon as an account has two. Matched on
// the last segment, because -Q qualifies every row while the caller knows
// only the stack's own name.
func QualifiedName(raw []byte, name string) (string, error) {
	rows, err := ParseRows(raw)
	if err != nil {
		return "", err
	}

	available := make([]string, 0, len(rows))

	for _, row := range rows {
		segments := strings.Split(row.Name, "/")

		available = append(available, row.Name)

		if segments[len(segments)-1] != name {
			continue
		}

		if len(segments) != qualifiedSegments {
			return "", fmt.Errorf("the backend names this stack %q, not <org>/<project>/<stack>: "+
				"a self-managed backend has no organization to reference, so pass the reference explicitly",
				row.Name)
		}

		return row.Name, nil
	}

	if len(available) == 0 {
		return "", fmt.Errorf("no stack named %q, and the project has none", name)
	}

	return "", fmt.Errorf("no stack named %q. Stacks that do exist: %s", name, strings.Join(available, ", "))
}

// Reference returns the <org>/<project>/<stack> a StackReference needs.
//
// `pulumi stack ls`, not `pulumi --stack <name> stack --show-name`: the latter
// falls back to the SELECTED stack when the name is empty, so a caller with an
// unset variable gets a confident answer about the wrong stack.
func Reference(ctx context.Context, dir, name string) (string, error) {
	out, err := pulumi.Run(ctx, dir, qualifiedListing...)
	if err != nil {
		return "", err
	}

	reference, err := QualifiedName(out, name)
	if err != nil {
		return "", fmt.Errorf("%s: %w", dir, err)
	}

	return reference, nil
}

// Action pairs the pulumi subcommand with the word that describes it.
//
// Separated from Ensure so the pairing can be tested without a pulumi binary:
// reporting "created" for a stack that was only selected is a wrong answer in
// the one place an operator looks to see whether a stack is new.
func Action(present bool) (verb, state string) {
	if present {
		return "select", StateExisting
	}

	return "init", StateCreated
}

// Ensure selects the stack, creating it first if it is not there, and returns
// which of those two it did.
func Ensure(ctx context.Context, dir, name string) (string, error) {
	present, err := Exists(ctx, dir, name)
	if err != nil {
		return "", err
	}

	verb, state := Action(present)

	if _, err := pulumi.Run(ctx, dir, "stack", verb, name); err != nil {
		return "", err
	}

	return state, nil
}
