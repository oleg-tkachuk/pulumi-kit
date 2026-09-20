package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/pulumi"
	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/target"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_HelpIsNotAFailure(t *testing.T) {
	t.Parallel()

	// The README tells people to run -h, so it has to exit zero. flag returns
	// ErrHelp for it, which main distinguishes from a real error.
	var out, errOut bytes.Buffer

	err := run(context.Background(), []string{"-h"}, &out, &errOut)
	require.ErrorIs(t, err, flag.ErrHelp)
	assert.Contains(t, errOut.String(), "usage: target")
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
		"no stack": {args: []string{"traefik"}, fails: "is not a stack name"},
		"a traversing stack": {
			args: []string{"-stack", "../other", "traefik"}, fails: "is not a stack name",
		},
		"no selector":    {args: []string{"-stack", "dev"}, fails: "exactly one selector"},
		"two selectors":  {args: []string{"-stack", "dev", "a", "b"}, fails: "exactly one selector"},
		"blank selector": {args: []string{"-stack", "dev", "   "}, fails: target.ErrNoSelector.Error()},
		"a directory that is not a project": {
			args:  []string{"-stack", "dev", "-dir", t.TempDir(), "traefik"},
			fails: "not a Pulumi project",
		},
		"an unknown flag": {
			args: []string{"-nope", "-stack", "dev", "traefik"}, fails: "not defined",
		},
	} {
		var out, errOut bytes.Buffer

		err := run(context.Background(), tc.args, &out, &errOut)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.fails, name)
		assert.Empty(t, out.String(), "%s printed a URN", name)
	}
}

// TestRun_TheAbsentCLIIsReportedBeforeTheArguments pins the order.
//
// Not parallel: it sets PATH for the process. A missing CLI makes the whole
// command impossible, so reporting a bad stack name instead would send the
// operator to their own command line for a problem on the machine.
func TestRun_TheAbsentCLIIsReportedBeforeTheArguments(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	var out, errOut bytes.Buffer

	err := run(context.Background(), []string{"-stack", "../not-a-stack-name", "traefik"}, &out, &errOut)
	require.ErrorIs(t, err, pulumi.ErrNotInstalled)
	assert.NotContains(t, err.Error(), "is not a stack name")
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
