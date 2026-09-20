package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/pulumi"
	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/stack"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_AnswersAHelpRequest(t *testing.T) {
	t.Parallel()

	// The defect this fixed: the usage arrived under an `error:` prefix with
	// exit 1, while cmd/target exited 0 for the same request.
	for flag := range HelpFlags {
		var out bytes.Buffer

		err := run(context.Background(), []string{flag}, &out)
		require.NoError(t, err, flag)
		assert.Contains(t, out.String(), Usage, flag)
	}
}

func TestRun_RefusesBeforeItReachesPulumi(t *testing.T) {
	// Not parallel: onPath sets PATH for the process.
	onPath(t)

	// Every one of these has to fail without a backend, or the checks are not
	// where they are claimed to be.
	for name, tc := range map[string]struct {
		args  []string
		fails string
	}{
		"no arguments":    {args: nil, fails: Usage},
		"too few":         {args: []string{"ensure", "."}, fails: Usage},
		"too many":        {args: []string{"ensure", ".", "dev", "extra"}, fails: Usage},
		"unknown command": {args: []string{"delete", ".", "dev"}, fails: "unknown command"},
		// `list` moved to this repository's own tooling when the generic half
		// came here; naming it must fail rather than be quietly ignored.
		"list is not here": {args: []string{"list", ".", "dev"}, fails: "unknown command"},
		// A help flag among arguments is a caller that built its list wrong,
		// and printing the usage with exit 0 would let that pass as success.
		"a help flag among arguments": {
			args: []string{"-h", ".", "dev"}, fails: "unknown command",
		},
		// Validated before the name reaches an exec, and before the command
		// is even dispatched.
		"a traversing stack name": {
			args: []string{"ensure", ".", "../other"}, fails: "is not a stack name",
		},
	} {
		var out bytes.Buffer

		err := run(context.Background(), tc.args, &out)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.fails, name)
		assert.Empty(t, out.String(), "%s wrote to stdout", name)
	}
}

// Not parallel: it sets PATH for the process.
func TestRun_RefusesWhenTheCLIIsAbsent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	var out bytes.Buffer

	err := run(context.Background(), []string{CommandEnsure, ".", "dev"}, &out)
	require.ErrorIs(t, err, pulumi.ErrNotInstalled)
	assert.Empty(t, out.String())
}

func TestRun_HelpWorksWithoutTheCLI(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	// Asking what a command takes must not require the thing it drives.
	var out bytes.Buffer

	require.NoError(t, run(context.Background(), []string{"-h"}, &out))
	assert.Contains(t, out.String(), Usage)
}

func TestExitCode_SeparatesAbsenceFromFailure(t *testing.T) {
	t.Parallel()

	// Both were ExitFailed, and a caller cannot tell them apart from one
	// status: `if stack exists "$dir" "$name"; then … else <create it> fi`
	// created a stack because the network was down.
	assert.Equal(t, ExitAbsent, exitCode(fmt.Errorf("looked in %s: %w", ".", stack.ErrNotFound)),
		"absence has its own status, however deeply it is wrapped")

	assert.Equal(t, ExitFailed, exitCode(errors.New("pulumi stack ls: dial tcp: connection refused")),
		"a backend that did not answer is not an answer")

	assert.Equal(t, ExitFailed, exitCode(errors.New(Usage)))
}

// onPath puts a file named after the CLI on PATH, so a test of argument
// handling reaches the argument handling.
//
// Without it these cases asserted whatever the host happened to have. CI on
// ubuntu-24.04 passed because that image ships the Pulumi CLI; ubuntu-26.04
// does not, and eleven assertions failed on a message about PATH. The checks
// were right and the test was reading the environment.
func onPath(t *testing.T) {
	t.Helper()

	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "pulumi"), []byte("#!/bin/sh\n"), 0o700))
	t.Setenv("PATH", dir)
}
