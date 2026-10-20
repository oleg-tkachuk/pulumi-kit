package target_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/oleg-tkachuk/pulumi-kit/target"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCLI puts a pulumi on PATH that prints listing, as
// `pulumi stack --show-urns --output json` does, and returns a directory that
// is a project.
//
// Not parallel wherever it is used: it sets PATH for the process.
func fakeCLI(t *testing.T, listing []target.Resource) string {
	t.Helper()

	raw, err := json.Marshal(map[string][]target.Resource{"resources": listing})
	require.NoError(t, err)

	bin := t.TempDir()
	listingFile := filepath.Join(bin, "listing.json")
	require.NoError(t, os.WriteFile(listingFile, raw, 0o600))

	script := "#!/bin/sh\ncat '" + listingFile + "'\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "pulumi"), []byte(script), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: services\n"), 0o600))

	return project
}

func TestResolve_ReadsTheStackAndMatches(t *testing.T) {
	dir := fakeCLI(t, stack)

	urns, err := target.Resolve(context.Background(), target.Stack{Dir: dir, Name: "dev"},
		"group:Ingress,argocd", "acme")
	require.ErrorContains(t, err, `"argocd" names 2 resources`,
		"every selector has to resolve, as MatchAll requires")
	assert.Empty(t, urns)

	urns, err = target.Resolve(context.Background(), target.Stack{Dir: dir, Name: "dev"},
		"group:Ingress,Release:argocd", "acme")
	require.NoError(t, err)
	assert.Equal(t, []string{groupURN, releaseURN, balancerURN, targetURN, otherHelmURN}, urns)
}

func TestResolve_RefusesANameTheStackDoesNotHold(t *testing.T) {
	dir := fakeCLI(t, stack)

	_, err := target.Resolve(context.Background(), target.Stack{Dir: dir, Name: "dev"}, "traefk", "")
	require.ErrorContains(t, err, "matches nothing")
	assert.ErrorContains(t, err, "Release:traefik", "the refusal lists what the stack holds")
}

func TestList_IsNames(t *testing.T) {
	dir := fakeCLI(t, stack)

	names, err := target.List(context.Background(), target.Stack{Dir: dir, Name: "dev"})
	require.NoError(t, err)
	assert.Equal(t, target.Names(stack), names)
}

// TestResolve_RefusesBeforeTheCLIRuns holds the library to the command's
// checks: a Go caller gets no weaker contract than a shell one.
func TestResolve_RefusesBeforeTheCLIRuns(t *testing.T) {
	project := fakeCLI(t, stack)

	for name, tc := range map[string]struct {
		stack target.Stack
		fails string
	}{
		"no stack":                          {target.Stack{Dir: project}, "is not a stack name"},
		"a traversing stack":                {target.Stack{Dir: project, Name: "../other"}, "is not a stack name"},
		"a directory that is not a project": {target.Stack{Dir: t.TempDir(), Name: "dev"}, "not a Pulumi project"},
	} {
		_, err := target.Resolve(context.Background(), tc.stack, "traefik", "")
		require.ErrorContains(t, err, tc.fails, name)

		_, err = target.List(context.Background(), tc.stack)
		require.ErrorContains(t, err, tc.fails, name)
	}
}

func TestResolve_ReportsAnAbsentCLIFirst(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := target.Resolve(context.Background(), target.Stack{Dir: t.TempDir(), Name: "../bad"}, "traefik", "")
	require.ErrorIs(t, err, target.ErrNotInstalled)
	assert.NotContains(t, err.Error(), "is not a stack name")
}
