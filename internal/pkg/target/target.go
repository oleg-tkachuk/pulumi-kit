// Package target resolves a resource name into the URNs `pulumi --target`
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
// Exit code zero. So a caller that passes a mistyped name reports success and
// applies nothing. Pulumi will not catch that, so this does: a selector either
// resolves to URNs that exist in the stack's state, or it refuses and says
// what the stack actually holds.
//
// Matching is separated from the call that reads the state, so every decision
// in here can be tested against a state written by hand.
package target

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/pulumi"
)

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

// SelectorSeparator joins several selectors in one argument.
//
// A comma because it cannot appear in a Pulumi resource name, a type token or
// a URN, so splitting on it can never cut a selector in half.
const SelectorSeparator = ","

// Resource is one entry of `pulumi stack --show-urns --output json`.
//
// Four fields is all that listing gives, and all this needs. `stack export`
// would give the whole checkpoint including every resource's inputs and any
// encrypted secret — more than the question asks for, and not something to
// write to a pipe.
//
// Parent is what makes a group exact. A URN's type path carries the types of a
// resource's ancestors and not their names, so it cannot say which instance of
// a twice-declared component a child belongs to; the parent URN can, because it
// ends in that instance's name.
type Resource struct {
	URN    string `json:"urn"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Parent string `json:"parent"`
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
var ErrNoGroupPackage = errors.New("a group selector needs a group package")

// ErrNoParents is returned when the state carries no parent information at all.
//
// Descendants are found by following `parent`, so a listing without it would
// resolve every group to its own node and quietly leave the children behind —
// a selection narrower than asked for, which for an apply is a partial update
// reporting success. Refused instead: the field has been in
// `pulumi stack --show-urns --output json` for a long time, and this only
// fires if that changes.
var ErrNoParents = errors.New("the stack listing carries no parent information, so a group cannot be resolved")

// StackCommand is the listing this parses, and it is a contract rather than a
// convenience: `pulumi stack --show-urns` without --output prints a tree laid
// out for a terminal, and its columns move between releases.
var StackCommand = []string{"stack", "--show-urns", "--output", "json"}

// MatchAll resolves every selector in a comma-separated list.
//
// One pulumi invocation takes many `--target` flags, so two names are one
// apply rather than two — which matters beyond typing: two applies are two
// chances for the second to run against state the first one changed.
//
// Every selector has to resolve. A list where one name is a typo refuses as a
// whole rather than applying the rest, because the operator asked for both and
// a partial apply that reports success is the failure this prevents.
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

// matchGroup takes a component resource and everything under it.
//
// Both halves matter. The children are what an apply is usually about; the
// node itself is what a destroy has to remove as well, or the group's own
// entry is orphaned in the state.
//
// Descendants are found by following `parent` from the node's URN, not by
// looking for the node's type in other URNs. The substring form was wrong and
// measured wrong: a URN's type path carries ancestors' TYPES and not their
// names, so with two components of one type `group:Network` took the first
// node, its child, and the OTHER node's child — while leaving the other node
// itself out. A targeted destroy would have removed a resource under a
// component nobody named and orphaned the one it belonged to.
func matchGroup(resources []Resource, group, groupPackage string) ([]string, error) {
	node, err := groupNode(resources, group, groupPackage)
	if err != nil {
		return nil, err
	}

	children := map[string][]Resource{}

	var parents int

	for _, resource := range resources {
		if resource.Parent == "" {
			continue
		}

		parents++

		children[resource.Parent] = append(children[resource.Parent], resource)
	}

	// The stack's own root has no parent, so some resources legitimately carry
	// none. None of them carrying one is the case this cannot work in.
	if parents == 0 {
		return nil, fmt.Errorf("%q: %w", GroupPrefix+group, ErrNoParents)
	}

	// Breadth-first, because a group is a tree rather than one level: a
	// LoadBalancer under a Network under a Cluster is two hops down, and an
	// apply that skipped it would be a partial update reporting success.
	urns := []string{node.URN}

	for queue := []string{node.URN}; len(queue) > 0; {
		urn := queue[0]
		queue = queue[1:]

		for _, child := range children[urn] {
			urns = append(urns, child.URN)
			queue = append(queue, child.URN)
		}
	}

	return urns, nil
}

// GroupSeparator disambiguates two components of the same type:
// `group:Network:net-a`.
const GroupSeparator = ":"

// groupNode is the component resource a group selector names.
//
// Ambiguity is an error rather than a choice, the same as it is for a plain
// name. Taking the first of two same-typed components is how the bug above
// looked from the outside: a selection that was confidently wrong.
func groupNode(resources []Resource, group, groupPackage string) (Resource, error) {
	wantType, wantName := group, ""
	if before, after, qualified := strings.Cut(group, GroupSeparator); qualified {
		wantType, wantName = before, after
	}

	var found []Resource

	for _, resource := range resources {
		if !strings.HasPrefix(resource.Type, groupPackage+":") {
			continue
		}

		if TypeLeaf(resource.Type) != wantType {
			continue
		}

		if wantName != "" && resource.Name != wantName {
			continue
		}

		found = append(found, resource)
	}

	switch len(found) {
	case 0:
		return Resource{}, noMatch(resources, GroupPrefix+group)
	case 1:
		return found[0], nil
	}

	qualified := make([]string, 0, len(found))
	for _, resource := range found {
		qualified = append(qualified, GroupPrefix+wantType+GroupSeparator+resource.Name)
	}

	slices.Sort(qualified)

	return Resource{}, fmt.Errorf("%q names %d components: say which one, as %s",
		GroupPrefix+group, len(found), strings.Join(qualified, " or "))
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

// Names is what the stack holds, as the selectors that would each match one
// thing: `Release:traefik`, `LoadBalancer:ingress`.
//
// One function because two callers must agree exactly. It is what a mistyped
// selector is answered with, and what -list prints — and the whole value of
// -list is that an operator no longer has to provoke that error to see the
// list. Two copies would drift, and the drift would be invisible: both would
// look right in isolation.
//
// Nothing is filtered out. A list that hid the stack's own node and its
// providers would be tidier and would also be a judgement about what an
// operator is allowed to look for.
func Names(resources []Resource) []string {
	qualified := make([]string, 0, len(resources))
	for _, resource := range resources {
		qualified = append(qualified, TypeLeaf(resource.Type)+":"+resource.Name)
	}

	slices.Sort(qualified)

	return slices.Compact(qualified)
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
	return fmt.Errorf(
		"%q matches nothing in this stack, and a --target that matches nothing "+
			"is an apply that reports success and does nothing.\nThis stack holds: %s",
		selector, strings.Join(Names(resources), ", "))
}

// Resources asks the stack what it holds.
func Resources(ctx context.Context, dir, stack string) ([]Resource, error) {
	out, err := pulumi.Run(ctx, dir, append([]string{"--stack", stack}, StackCommand...)...)
	if err != nil {
		return nil, fmt.Errorf("read the stack's resources: %w", err)
	}

	return ResourcesIn(out)
}

// ResourcesIn parses that output, separated so a test can hold the parser to
// the shape the CLI produces without running it.
func ResourcesIn(raw []byte) ([]Resource, error) {
	var parsed listing

	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse the stack listing: %w", err)
	}

	if len(parsed.Resources) == 0 {
		return nil, errors.New("the stack holds no resources: apply it before targeting part of it")
	}

	return parsed.Resources, nil
}
