package stack_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oleg-tkachuk/pulumi-kit/stack"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCLI puts a pulumi on PATH that answers `stack ls` with listing and logs
// every other call, and returns a project directory and the log.
//
// Not parallel wherever it is used: it sets PATH for the process.
func fakeCLI(t *testing.T, listing string) (project, log string) {
	t.Helper()

	bin := t.TempDir()
	listingFile := filepath.Join(bin, "listing.json")
	log = filepath.Join(bin, "calls")

	require.NoError(t, os.WriteFile(listingFile, []byte(listing), 0o600))

	script := "#!/bin/sh\n" +
		"case \"$*\" in *' ls '*) cat '" + listingFile + "' ;; *) printf '%s\\n' \"$*\" >> '" + log + "' ;; esac\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "pulumi"), []byte(script), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	project = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: services\n"), 0o600))

	return project, log
}

func calls(t *testing.T, log string) []string {
	t.Helper()

	raw, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return nil
	}

	require.NoError(t, err)

	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func TestEnsure_CreatesWhatIsAbsentAndSelectsWhatIsThere(t *testing.T) {
	project, log := fakeCLI(t, `[{"name":"dev"}]`)

	state, err := stack.Ensure(context.Background(), project, "dev")
	require.NoError(t, err)
	assert.Equal(t, stack.StateExisting, state)

	state, err = stack.Ensure(context.Background(), project, "prod")
	require.NoError(t, err)
	assert.Equal(t, stack.StateCreated, state)

	assert.Equal(t, []string{"--non-interactive stack select dev", "--non-interactive stack init prod"}, calls(t, log))
}

func TestExists_AndNames(t *testing.T) {
	project, _ := fakeCLI(t, `[{"name":"dev"},{"name":"prod"}]`)

	present, err := stack.Exists(context.Background(), project, "dev")
	require.NoError(t, err)
	assert.True(t, present)

	present, err = stack.Exists(context.Background(), project, "qa")
	require.NoError(t, err)
	assert.False(t, present)

	names, err := stack.Names(context.Background(), project)
	require.NoError(t, err)
	assert.Equal(t, []string{"dev", "prod"}, names)
}

// TestPublic_RefusesBeforeTheCLIRuns holds every exported entry point to the
// command's checks: a Go caller gets no weaker contract than a shell one.
func TestPublic_RefusesBeforeTheCLIRuns(t *testing.T) {
	project, log := fakeCLI(t, `[]`)
	notAProject := t.TempDir()

	for name, tc := range map[string]struct {
		dir, stack, fails string
	}{
		"no stack":                          {project, "", "is not a stack name"},
		"a traversing stack":                {project, "../other", "is not a stack name"},
		"a directory that is not a project": {notAProject, "dev", "not a Pulumi project"},
	} {
		_, err := stack.Ensure(context.Background(), tc.dir, tc.stack)
		require.ErrorContains(t, err, tc.fails, name)

		_, err = stack.Exists(context.Background(), tc.dir, tc.stack)
		require.ErrorContains(t, err, tc.fails, name)

		_, err = stack.Reference(context.Background(), tc.dir, tc.stack)
		require.ErrorContains(t, err, tc.fails, name)
	}

	_, err := stack.Names(context.Background(), notAProject)
	require.ErrorContains(t, err, "not a Pulumi project")

	assert.Empty(t, calls(t, log), "nothing may reach the CLI")
}

func TestEnsure_ReportsAnAbsentCLIFirst(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := stack.Ensure(context.Background(), t.TempDir(), "../bad")
	require.ErrorIs(t, err, stack.ErrNotInstalled)
	assert.NotContains(t, err.Error(), "is not a stack name")
}
