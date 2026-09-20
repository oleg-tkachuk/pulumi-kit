package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listing is what `pulumi stack ls --json` prints, trimmed to the field this
// reads.
const listing = `[{"name":"dev"},{"name":"prod"}]`

// qualified is the same under -Q, where Pulumi Cloud names all three parts.
const qualified = `[{"name":"acme/services/dev"},{"name":"acme/services/prod"}]`

func TestStackNamed(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]bool{"dev": true, "prod": true, "staging": false, "": false} {
		got, err := stackNamed([]byte(listing), name)
		require.NoError(t, err, name)
		assert.Equal(t, want, got, name)
	}
}

func TestStackNamed_UnparseableIsAnErrorRatherThanAbsent(t *testing.T) {
	t.Parallel()

	// The distinction the jq pipeline could not make. "Absent" here would send
	// ensure on to create a stack that already exists.
	_, err := stackNamed([]byte("error: no credentials\n"), "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no usable json")
}

func TestParseRows_EmptyListIsNotAnError(t *testing.T) {
	t.Parallel()

	rows, err := parseRows([]byte(`[]`))
	require.NoError(t, err)
	assert.Empty(t, rows, "a project with no stacks is a state, not a failure")
}

func TestQualifiedName(t *testing.T) {
	t.Parallel()

	got, err := qualifiedName([]byte(qualified), "dev")
	require.NoError(t, err)
	assert.Equal(t, "acme/services/dev", got,
		"matched on the last segment, because -Q qualifies every row while the caller "+
			"knows only the stack's own name")
}

func TestQualifiedName_ASelfManagedBackendSaysSo(t *testing.T) {
	t.Parallel()

	// One segment is a name pulumi.NewStackReference cannot resolve, and the
	// error has to say that rather than "not found".
	_, err := qualifiedName([]byte(listing), "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<org>/<project>/<stack>")
}

func TestQualifiedName_NotFoundCarriesTheList(t *testing.T) {
	t.Parallel()

	_, err := qualifiedName([]byte(qualified), "staging")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acme/services/dev")
}

func TestQualifiedName_NoStacksAtAll(t *testing.T) {
	t.Parallel()

	// A different sentence from the one above: there is no list to offer.
	_, err := qualifiedName([]byte(`[]`), "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the project has none")
}

func TestStackAction(t *testing.T) {
	t.Parallel()

	// Reporting "created" for a stack that was only selected is a wrong answer
	// in the one place an operator looks to see whether a stack is new.
	verb, state := stackAction(true)
	assert.Equal(t, "select", verb)
	assert.Equal(t, StateExisting, state)

	verb, state = stackAction(false)
	assert.Equal(t, "init", verb)
	assert.Equal(t, StateCreated, state)
}

func TestRun_RefusesBeforeItReachesPulumi(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		args  []string
		fails string
	}{
		"no arguments":     {args: nil, fails: Usage},
		"too few":          {args: []string{"ensure", "."}, fails: Usage},
		"too many":         {args: []string{"ensure", ".", "dev", "extra"}, fails: Usage},
		"unknown command":  {args: []string{"delete", ".", "dev"}, fails: "unknown command"},
		"list is gone now": {args: []string{"list", ".", "dev"}, fails: "unknown command"},
	} {
		var out bytes.Buffer

		err := run(context.Background(), tc.args, &out)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.fails, name)
		assert.Empty(t, out.String(), "%s wrote to stdout", name)
	}
}
