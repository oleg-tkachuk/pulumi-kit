package pulumi_test

import (
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
