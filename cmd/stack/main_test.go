package main

import (
	"bytes"
	"context"
	"testing"

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
	t.Parallel()

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
	} {
		var out bytes.Buffer

		err := run(context.Background(), tc.args, &out)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.fails, name)
		assert.Empty(t, out.String(), "%s wrote to stdout", name)
	}
}
