//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/pulumi"
	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/stack"
	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/target"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The stack in testdata/dev.json, exported from a run of a Go program that
// declared two acme:platform:Network components with one Subnet under each —
// the shape that once made a group selector take the wrong children.
const (
	stackName    = "dev"
	groupPackage = "acme"

	netA    = "urn:pulumi:dev::probe::acme:platform:Network::net-a"
	netASub = "urn:pulumi:dev::probe::acme:platform:Network$acme:platform:Subnet::net-a-sub"
)

// project is a Pulumi project in a temporary directory, with the fixture stack
// imported into a backend that is also a temporary directory.
//
// Imported rather than produced by `pulumi up`: running a program would need a
// language SDK and the network, and what is under test is what the CLI prints
// about a state, not how a program arrives at one. The state itself came from
// a real run, so its parents and URNs are the CLI's, not a guess at them.
//
// Not parallel anywhere in this file: t.Setenv points the CLI at the backend
// for the whole process.
func project(t *testing.T) string {
	t.Helper()

	require.NoError(t, pulumi.Require(), "these tests drive the real CLI, so it has to be installed")

	t.Setenv("PULUMI_BACKEND_URL", "file://"+t.TempDir())
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "integration")
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")

	dir := filepath.Join(t.TempDir(), "probe")
	require.NoError(t, os.Mkdir(dir, 0o750))

	manifest, err := os.ReadFile(filepath.Join("testdata", "project", pulumi.ProjectFile))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, pulumi.ProjectFile), manifest, 0o600))

	state, err := filepath.Abs(filepath.Join("testdata", "dev.json"))
	require.NoError(t, err)

	ctx := context.Background()

	_, err = pulumi.Run(ctx, dir, "stack", "init", stackName)
	require.NoError(t, err)

	_, err = pulumi.Run(ctx, dir, "--stack", stackName, "stack", "import", "--file", state)
	require.NoError(t, err)

	return dir
}

func TestTarget_ListsWhatTheStackHolds(t *testing.T) {
	dir := project(t)

	resources, err := target.Resources(context.Background(), dir, stackName)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"Network:net-a",
		"Network:net-b",
		"Stack:probe-dev",
		"Subnet:net-a-sub",
		"Subnet:net-b-sub",
	}, target.Names(resources))
}

// TestTarget_ResolvesOneOfTwoSameTypedGroups is the case ErrNoParents guards:
// a group is walked by `parent`, so a listing that dropped the field — or
// renamed it — would resolve net-a to its own node and leave its Subnet behind.
func TestTarget_ResolvesOneOfTwoSameTypedGroups(t *testing.T) {
	dir := project(t)

	resources, err := target.Resources(context.Background(), dir, stackName)
	require.NoError(t, err)

	urns, err := target.MatchAll(resources, "group:Network:net-a", groupPackage)
	require.NoError(t, err)
	assert.Equal(t, []string{netA, netASub}, urns)

	_, err = target.MatchAll(resources, "group:Network", groupPackage)
	require.Error(t, err, "two Network components: the bare type is ambiguous")
}

func TestStack_ExistsEnsureAndNames(t *testing.T) {
	dir := project(t)
	ctx := context.Background()

	present, err := stack.Exists(ctx, dir, stackName)
	require.NoError(t, err)
	assert.True(t, present)

	present, err = stack.Exists(ctx, dir, "staging")
	require.NoError(t, err)
	assert.False(t, present)

	state, err := stack.Ensure(ctx, dir, "staging")
	require.NoError(t, err)
	assert.Equal(t, stack.StateCreated, state)

	state, err = stack.Ensure(ctx, dir, "staging")
	require.NoError(t, err)
	assert.Equal(t, stack.StateExisting, state, "the second ensure must select, not create")

	names, err := stack.Names(ctx, dir)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{stackName, "staging"}, names)
}

// TestStack_ReferenceOnASelfManagedBackend pins what a file backend answers.
//
// It once printed a bare stack name under -Q, and `ref` refused it. The
// project-scoped layout, the default for new backends, names the organization
// `organization` — and a StackReference resolves that name. The bare form is
// now the legacy layout's alone, which the CLI refuses to open by default.
func TestStack_ReferenceOnASelfManagedBackend(t *testing.T) {
	dir := project(t)

	reference, err := stack.Reference(context.Background(), dir, stackName)
	require.NoError(t, err)
	assert.Equal(t, "organization/probe/dev", reference)
}

// TestPulumi_StillSucceedsWhenATargetMatchesNothing is a canary on the reason
// cmd/target exists, rather than a test of anything in this module.
//
// When a Renovate bump of the CLI turns it red, Pulumi has started refusing
// these on its own: docs/design.md and cmd/target/README.md say otherwise and
// need revisiting, and so might the tool.
func TestPulumi_StillSucceedsWhenATargetMatchesNothing(t *testing.T) {
	dir := project(t)
	ctx := context.Background()

	for _, flag := range []string{"--target", "--exclude"} {
		_, err := pulumi.Run(ctx, dir, "--stack", stackName, "preview", flag, "**::Network::does-not-exist")
		require.NoError(t, err, "%s with a pattern that matches nothing is still silent success", flag)
	}

	// The exact form is what Pulumi does catch, and has not always: the
	// refusal says what the stack holds, and Pulumi's does not.
	_, err := pulumi.Run(ctx, dir, "--stack", stackName, "preview",
		"--target", "urn:pulumi:dev::probe::acme:platform:Network::does-not-exist")
	require.Error(t, err, "an exact URN that is not in the stack is refused by the CLI itself")
}
