// Package ci holds gates over the files no Go compiler reads.
package ci

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workflow is the file the pattern below is read out of, rather than a copy of
// it: a second spelling here would pass while CI used a different one.
const workflow = ".github/workflows/ci.yaml"

// inertAssignment finds the shell assignment the changed-paths job filters by.
var inertAssignment = regexp.MustCompile(`inert='([^']+)'`)

// TestInertPaths_ClassifyTheTreeCorrectly holds the changed-paths filter to a
// table of real paths.
//
// The pattern decides whether the test suite runs at all, and both ways of
// being wrong are quiet. Too broad, and a change to code is called
// documentation and every check reports skipped — green, and nothing ran. Too
// narrow, and documentation runs the whole build, which is only wasteful.
//
// The first is why this exists.
func TestInertPaths_ClassifyTheTreeCorrectly(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", workflow))
	require.NoError(t, err)

	found := inertAssignment.FindStringSubmatch(string(raw))
	require.NotNil(t, found, "%s declares no inert pattern, so nothing gates the checks", workflow)

	// grep's ERE and Go's RE2 agree on this pattern. One that stops being
	// portable fails here rather than in a shell nobody reads.
	inert, err := regexp.Compile(found[1])
	require.NoError(t, err, "the configured pattern must be a valid regular expression")

	for path, isInert := range map[string]bool{
		// Documentation, which is the whole point.
		"README.md":               true,
		"docs/ci.md":              true,
		"docs/README.md":          true,
		"cmd/stack/README.md":     true,
		".github/CONTRIBUTING.md": true,
		"LICENSE":                 true,
		".gitignore":              true,
		"assets/diagram.svg":      true,

		// Code, and the files the checks read that are not code. Each of
		// these has been a surprise somewhere: a workflow change that skipped
		// the workflow audits, a linter config change that skipped the
		// linter, a lockfile change that skipped the release tooling.
		"cmd/stack/main.go":             false,
		"internal/pkg/target/target.go": false,
		"internal/pkg/pulumi/pulumi.go": false,
		"go.mod":                        false,
		"go.sum":                        false,
		".golangci.yaml":                false,
		".github/workflows/ci.yaml":     false,
		".github/workflows/release.yml": false,
		".github/zizmor.yml":            false,
		".github/dependabot.yml":        false,
		"package.json":                  false,
		"package-lock.json":             false,
		"release.config.cjs":            false,
		// A markdown file is inert; a Go file that merely mentions one is not.
		"internal/ci/workflow_test.go": false,
	} {
		assert.Equal(t, isInert, inert.MatchString(path), path)
	}
}

// dispatchCondition is the `if:` of the job that starts a release, read as one
// folded scalar ending at the next key.
var dispatchCondition = regexp.MustCompile(`(?s)dispatch-release:.*?if: >-\n(.*?)\n    steps:`)

// TestDispatchRelease_RunsAfterSkippedChecksButNeverAfterFailedOnes pins a
// condition with two independent ways of being wrong.
//
// Without always(), a documentation-only push skips the five checks and this
// job is skipped with them — so no release is ever dispatched for a push that
// only edits prose, and the version that a `fix:` in the same push would have
// cut never appears. Nothing reports that.
//
// Without the failure guard, always() is worse than the bug it fixes: a
// release dispatched after a check failed.
func TestDispatchRelease_RunsAfterSkippedChecksButNeverAfterFailedOnes(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", workflow))
	require.NoError(t, err)

	found := dispatchCondition.FindStringSubmatch(string(raw))
	require.NotNil(t, found, "%s has no dispatch-release condition to read", workflow)

	condition := found[1]

	assert.Contains(t, condition, "always()",
		"without always() a documentation-only push dispatches no release at all")

	for _, guard := range []string{"'failure'", "'cancelled'"} {
		assert.Contains(t, condition, guard,
			"always() without a %s guard dispatches a release after a check did not pass", guard)
	}
}

// renovateConfig is the part of .github/renovate.json this file reads.
type renovateConfig struct {
	CustomManagers []struct {
		ManagerFilePatterns []string `json:"managerFilePatterns"`
		MatchStrings        []string `json:"matchStrings"`
	} `json:"customManagers"`
}

// TestRenovateMatchesEveryPinnedTool is the gate on a bot that cannot report
// its own failure.
//
// Renovate does not error on a pin it cannot match — it opens no pull request,
// for ever, and the repository looks maintained because every other bot
// request keeps arriving. Both directions are asserted: a pin the pattern
// misses is a tool nobody upgrades, and an annotation with no pin under it is
// a pattern that has drifted from the file it was written for.
func TestRenovateMatchesEveryPinnedTool(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")

	raw, err := os.ReadFile(filepath.Join(root, ".github", "renovate.json"))
	require.NoError(t, err)

	var config renovateConfig
	require.NoError(t, json.Unmarshal(raw, &config))

	var pattern *regexp.Regexp

	for _, manager := range config.CustomManagers {
		for _, watched := range manager.ManagerFilePatterns {
			if !strings.Contains(watched, "workflows") {
				continue
			}

			require.Len(t, manager.MatchStrings, 1, "one pattern for the workflow pins")

			// Go accepts JavaScript's (?<name>…) syntax, so Renovate's own
			// pattern compiles here unchanged. One that stops being portable
			// fails this test rather than passing silently.
			pattern, err = regexp.Compile(manager.MatchStrings[0])
			require.NoError(t, err, "the configured pattern must be a valid regular expression")
		}
	}

	require.NotNil(t, pattern,
		"no custom manager watches the workflows — the pinned tools are upgraded by nobody")

	// The datasources these pins resolve from: Go modules for two, PyPI for
	// zizmor. A fourth would be a change here as well.
	allowed := map[string]bool{"go": true, "pypi": true}

	annotation := regexp.MustCompile(`# renovate: datasource=(\S+) depName=(\S+)`)

	workflows, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.y*ml"))
	require.NoError(t, err)
	require.NotEmpty(t, workflows)

	var seen int

	for _, path := range workflows {
		text, readErr := os.ReadFile(path)
		require.NoError(t, readErr, path)

		annotations := annotation.FindAllStringSubmatch(string(text), -1)
		matched := pattern.FindAllStringSubmatch(string(text), -1)

		assert.Len(t, matched, len(annotations),
			"%s carries %d annotation(s) and the pattern matches %d pin(s)",
			filepath.Base(path), len(annotations), len(matched))

		for _, found := range annotations {
			seen++

			assert.True(t, allowed[found[1]],
				"%s: datasource %q is not one these pins resolve from",
				filepath.Base(path), found[1])
		}
	}

	assert.Equal(t, 3, seen, "three tool pins are annotated; found %d", seen)
}
