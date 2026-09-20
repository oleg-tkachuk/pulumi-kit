package pulumi_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/pulumi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStackName(t *testing.T) {
	t.Parallel()

	for name, valid := range map[string]bool{
		"dev":        true,
		"prod-2":     true,
		"a_b.c":      true,
		"_leading":   true,
		"":           false,
		"../other":   false,
		".hidden":    false,
		"has space":  false,
		"semi;colon": false,
	} {
		err := pulumi.ValidateStackName(name)

		if valid {
			assert.NoError(t, err, name)

			continue
		}

		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "is not a stack name", name)
	}
}

func TestProjectDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := pulumi.ProjectDirectory(dir)
	require.Error(t, err, "a directory with no %s is not a project", pulumi.ProjectFile)
	assert.Contains(t, err.Error(), "not a Pulumi project")

	require.NoError(t, os.WriteFile(filepath.Join(dir, pulumi.ProjectFile), []byte("name: p\n"), 0o600))

	resolved, err := pulumi.ProjectDirectory(dir)
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(resolved), "pulumi --cwd needs an absolute path")
}

// executable writes a file named after the CLI into its own directory and
// returns that directory, for a PATH that has one.
func executable(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, pulumi.Binary), []byte("#!/bin/sh\n"), 0o700))

	return dir
}

// Not parallel, and none of the three below are: they set PATH for the
// process, which every other test in this package reads.
func TestRequire_SaysWhatIsMissingAndWhereToGetIt(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := pulumi.Require()
	require.ErrorIs(t, err, pulumi.ErrNotInstalled)
	assert.Contains(t, err.Error(), pulumi.InstallURL,
		"the operator reading this is the one who has to act on it")
}

func TestRequire_PassesWhenItIsThere(t *testing.T) {
	t.Setenv("PATH", executable(t))

	assert.NoError(t, pulumi.Require())
}

// TestRun_RefusesRatherThanWrappingTheExecError is the backstop.
//
// Each command calls Require first, so this fires only for a caller that
// reaches Run directly — which would otherwise get `exec: "pulumi":
// executable file not found in $PATH` wrapped in whatever it was doing.
func TestRun_RefusesWhenTheCLIIsAbsent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := pulumi.Run(context.Background(), t.TempDir(), "stack", "ls")
	require.ErrorIs(t, err, pulumi.ErrNotInstalled)
	assert.NotContains(t, err.Error(), "executable file not found",
		"the exec error is what this replaces")
}
