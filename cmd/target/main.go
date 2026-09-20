// Command target resolves a resource name into the URNs `pulumi --target`
// takes.
//
// It exists because of one measured thing: a `--target` that matches nothing
// SUCCEEDS.
//
//	pulumi preview --target '**::Release::does-not-exist'
//	Resources:
//	    + 1 to create
//	    24 unchanged
//
// Exit code zero. So a task that passes a mistyped name reports success and
// applies nothing. Pulumi will not catch that, so this does: a selector either
// resolves to URNs that exist in the stack's state, or this refuses and prints
// what the stack actually holds.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// GroupPrefix selects a whole group rather than one resource.
//
// A group is a component resource, and its TYPE appears in the URN of every
// resource under it:
//
//	urn:…::acme:platform:Ingress$kubernetes:helm.sh/v3:Release::traefik
//
// so selecting one reads what the state already says rather than inferring
// anything. `group:Ingress` takes that resource, its children, and the group's
// own node.
const GroupPrefix = "group:"

// Resource is one entry of `pulumi stack --show-urns --output json`.
//
// Three fields is all that listing gives, and all this needs. `stack export`
// would give the whole checkpoint including every resource's inputs and any
// encrypted secret — more than the question asks for, and not something to
// write to a pipe.
type Resource struct {
	URN  string `json:"urn"`
	Type string `json:"type"`
	Name string `json:"name"`
}

// listing is the shape of that command's output.
type listing struct {
	Resources []Resource `json:"resources"`
}

// ErrNoSelector is returned when the caller passed none. Distinct from "the
// selector matched nothing": an empty selector is a bug in the caller, and a
// selector that matches nothing is a typo by an operator.
var ErrNoSelector = errors.New("no selector given")

// ErrNoGroupPackage is returned when a group selector is used without saying
// which package a group's type token begins with.
//
// Required rather than guessed, so that a group selector cannot mean a
// provider's resource: a Kubernetes Ingress is
// `kubernetes:networking.k8s.io/v1:Ingress` and a component named Ingress is
// `<pkg>:platform:Ingress` — the same type LEAF. Matching on the leaf alone
// would take every Ingress object in the cluster, and targeting more than was
// asked for is the one failure a tool about targeting must not have.
var ErrNoGroupPackage = errors.New("a group selector needs -group-package")

// StackName is what may be handed to the Pulumi CLI as --stack.
//
// Pulumi's own stack names allow letters, digits, hyphens, underscores and
// periods. A leading period is refused, which is what keeps `..` out.
var StackName = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*$`)

// StackNameExtra names the rest for the error, so the message and the pattern
// cannot drift apart in a reader's head.
const StackNameExtra = "hyphens, underscores and periods, and not a leading period"

// ProjectFile is what makes a directory a Pulumi project.
const ProjectFile = "Pulumi.yaml"

// options are the inputs, so that adding one does not change an argument
// position every caller has hard-coded.
type options struct {
	dir          string
	stack        string
	groupPackage string
}

func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	var opts options

	flags := flag.NewFlagSet("target", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.StringVar(&opts.dir, "dir", ".", "the Pulumi project directory")
	flags.StringVar(&opts.stack, "stack", "", "the stack to read (required)")
	flags.StringVar(&opts.groupPackage, "group-package", "",
		"the package a component's type token begins with, for "+GroupPrefix+" selectors")
	flags.Usage = func() {
		fmt.Fprint(errOut, "usage: target [flags] <selector[,selector…]>\n"+
			"  selector: a resource name, Type:name, or "+GroupPrefix+"Type\n"+
			"            several, comma-separated, resolve to every one of their URNs\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() != 1 {
		flags.Usage()

		return errors.New("exactly one selector argument")
	}

	selector := flags.Arg(0)

	if strings.TrimSpace(selector) == "" {
		return ErrNoSelector
	}

	if !StackName.MatchString(opts.stack) {
		return fmt.Errorf("%q is not a stack name: letters, digits, %s", opts.stack, StackNameExtra)
	}

	directory, err := projectDirectory(opts.dir)
	if err != nil {
		return err
	}

	resources, err := resourcesOf(ctx, directory, opts.stack)
	if err != nil {
		return err
	}

	urns, err := MatchAll(resources, selector, opts.groupPackage)
	if err != nil {
		return err
	}

	for _, urn := range urns {
		fmt.Fprintln(out, urn)
	}

	return nil
}

// projectDirectory resolves the directory Pulumi is pointed at.
//
// Pulumi.yaml is checked here because `pulumi --cwd` on a directory without
// one fails with a message about the current project rather than the argument.
func projectDirectory(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", dir, err)
	}

	if _, err := os.Stat(filepath.Join(absolute, ProjectFile)); err != nil {
		return "", fmt.Errorf("%s is not a Pulumi project: %w", absolute, err)
	}

	return absolute, nil
}

// SelectorSeparator joins several selectors in one argument.
//
// A comma because it cannot appear in a Pulumi resource name, a type token or
// a URN, so splitting on it can never cut a selector in half.
const SelectorSeparator = ","

// MatchAll resolves every selector in a comma-separated list.
//
// One pulumi invocation takes many `--target` flags, so two names are one
// apply rather than two — which matters beyond typing: two applies are two
// chances for the second to run against state the first one changed.
//
// Every selector has to resolve. A list where one name is a typo refuses as a
// whole rather than applying the rest, because the operator asked for both and
// a partial apply that reports success is the failure this program prevents.
func MatchAll(resources []Resource, selectors, groupPackage string) ([]string, error) {
	seen := map[string]struct{}{}

	var (
		urns  []string
		taken = map[string]struct{}{}
	)

	for _, selector := range strings.Split(selectors, SelectorSeparator) {
		selector = strings.TrimSpace(selector)

		if selector == "" {
			return nil, fmt.Errorf("%q has an empty selector: %w", selectors, ErrNoSelector)
		}

		if _, duplicate := seen[selector]; duplicate {
			return nil, fmt.Errorf("%q names %q twice", selectors, selector)
		}

		seen[selector] = struct{}{}

		matched, err := Match(resources, selector, groupPackage)
		if err != nil {
			return nil, err
		}

		// Overlapping selectors are legitimate — `group:Ingress,traefik` names
		// the group and one of its members — and a repeated --target is not
		// something to hand to Pulumi twice.
		for _, urn := range matched {
			if _, already := taken[urn]; already {
				continue
			}

			taken[urn] = struct{}{}
			urns = append(urns, urn)
		}
	}

	return urns, nil
}

// Match turns a selector into URNs, or explains why it cannot.
//
// Three outcomes, and the middle one is the reason this is a function rather
// than a grep:
//
//   - one or more matches: their URNs, in state order.
//   - nothing: an error naming what the stack does hold. An operator who
//     mistyped needs the list, not a shrug — and Pulumi's own answer to this
//     case is silent success.
//   - one name, two types: an error naming both, and how to disambiguate.
//     Silently picking one would target a resource the operator did not mean.
func Match(resources []Resource, selector, groupPackage string) ([]string, error) {
	if group, found := strings.CutPrefix(selector, GroupPrefix); found {
		if groupPackage == "" {
			return nil, fmt.Errorf("%q: %w", selector, ErrNoGroupPackage)
		}

		return matchGroup(resources, group, groupPackage)
	}

	wantType, wantName := "", selector
	if before, after, qualified := strings.Cut(selector, ":"); qualified {
		wantType, wantName = before, after
	}

	var (
		urns  []string
		types []string
	)

	for _, resource := range resources {
		if resource.Name != wantName {
			continue
		}

		leaf := TypeLeaf(resource.Type)

		if wantType != "" && leaf != wantType {
			continue
		}

		urns = append(urns, resource.URN)
		types = append(types, leaf)
	}

	switch {
	case len(urns) == 0:
		return nil, noMatch(resources, selector)
	case len(urns) > 1 && wantType == "":
		// Every qualified form, not a guess at the intended one. Suggesting
		// the alphabetically first type would be arbitrary advice with the
		// authority of a tool behind it.
		qualified := make([]string, 0, len(types))
		for _, leaf := range types {
			qualified = append(qualified, leaf+":"+wantName)
		}

		slices.Sort(qualified)

		return nil, fmt.Errorf("%q names %d resources: say which one, as %s",
			selector, len(urns), strings.Join(slices.Compact(qualified), " or "))
	}

	return urns, nil
}

// nested is the separator Pulumi puts between a parent's type and a child's.
const nested = "$"

// matchGroup takes a component resource and everything under it.
//
// Both halves matter. The children are what an apply is usually about; the
// node itself is what a destroy has to remove as well, or the group's own
// entry is orphaned in the state.
//
// The node is found first, and its OWN type token is then what children are
// matched by. That removes the guesswork: a child's URN contains its parent's
// full type followed by `$`, so there is nothing to infer and no leaf to
// collide on.
func matchGroup(resources []Resource, group, groupPackage string) ([]string, error) {
	node, found := groupNode(resources, group, groupPackage)
	if !found {
		return nil, noMatch(resources, GroupPrefix+group)
	}

	urns := []string{node.URN}

	for _, resource := range resources {
		if strings.Contains(resource.URN, node.Type+nested) {
			urns = append(urns, resource.URN)
		}
	}

	return urns, nil
}

// groupNode is the component resource a group selector names.
func groupNode(resources []Resource, group, groupPackage string) (Resource, bool) {
	for _, resource := range resources {
		if strings.HasPrefix(resource.Type, groupPackage+":") && TypeLeaf(resource.Type) == group {
			return resource, true
		}
	}

	return Resource{}, false
}

// TypeLeaf is the last segment of a Pulumi type token: Release from
// kubernetes:helm.sh/v3:Release, Ingress from acme:platform:Ingress.
//
// The leaf rather than the whole token, because the whole token is what a
// selector should not have to carry: `Release:traefik` is something a person
// can type and `kubernetes:helm.sh/v3:Release:traefik` is not.
func TypeLeaf(token string) string {
	if index := strings.LastIndex(token, ":"); index >= 0 {
		return token[index+1:]
	}

	return token
}

// noMatch is the error an operator reads, so it carries the list.
//
// Qualified as Type:name rather than bare names, for two reasons. It says what
// each thing IS, and it is itself an example of the syntax that disambiguates
// a repeated name. URNs would be exact and unreadable; what a mistyped
// selector needs is the spelling it was reaching for.
//
// Nothing is filtered out. A list that hides the stack's own node and its
// providers would be tidier and would also be a judgement about what an
// operator is allowed to look for.
func noMatch(resources []Resource, selector string) error {
	qualified := make([]string, 0, len(resources))
	for _, resource := range resources {
		qualified = append(qualified, TypeLeaf(resource.Type)+":"+resource.Name)
	}

	slices.Sort(qualified)

	return fmt.Errorf(
		"%q matches nothing in this stack, and a --target that matches nothing "+
			"is an apply that reports success and does nothing.\nThis stack holds: %s",
		selector, strings.Join(slices.Compact(qualified), ", "))
}

// StackCommand is the listing this parses, and it is a contract rather than a
// convenience: `pulumi stack --show-urns` without --output prints a tree laid
// out for a terminal, and its columns move between releases.
var StackCommand = []string{"stack", "--show-urns", "--output", "json"}

// resourcesOf asks the stack what it holds.
func resourcesOf(ctx context.Context, directory, stack string) ([]Resource, error) {
	argv := append([]string{"--non-interactive", "--cwd", directory, "--stack", stack}, StackCommand...)

	// #nosec G204,G702 -- the binary is a literal; the stack name is checked
	// against StackName and the directory against ProjectFile, and
	// CommandContext takes an argument vector, so there is no shell. A caller
	// who can pass these can already run pulumi itself.
	out, err := exec.CommandContext(ctx, "pulumi", argv...).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return nil, fmt.Errorf("read the stack's resources: %w: %s",
				err, strings.TrimSpace(string(exit.Stderr)))
		}

		return nil, fmt.Errorf("read the stack's resources: %w", err)
	}

	return resourcesIn(out)
}

// resourcesIn parses that output, separated so a test can hold the parser to
// the shape the CLI produces without running it.
func resourcesIn(raw []byte) ([]Resource, error) {
	var parsed listing

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse the stack listing: %w", err)
	}

	if len(parsed.Resources) == 0 {
		return nil, errors.New("the stack holds no resources: apply it before targeting part of it")
	}

	return parsed.Resources, nil
}
