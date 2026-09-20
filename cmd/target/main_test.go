package main

import (
	"bytes"
	"context"
	"flag"
	"testing"

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
	t.Parallel()

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
