package stack_test

import (
	"testing"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/stack"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listing is what `pulumi stack ls --json` prints, trimmed to the field this
// package reads.
const listing = `[{"name":"dev"},{"name":"prod"}]`

// qualified is the same under -Q, where Pulumi Cloud names all three parts.
const qualified = `[{"name":"acme/services/dev"},{"name":"acme/services/prod"}]`

func TestNamed(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]bool{"dev": true, "prod": true, "staging": false, "": false} {
		got, err := stack.Named([]byte(listing), name)
		require.NoError(t, err, name)
		assert.Equal(t, want, got, name)
	}
}

func TestNamed_UnparseableIsAnErrorRatherThanAbsent(t *testing.T) {
	t.Parallel()

	// The distinction the jq pipeline could not make. "Absent" here would send
	// Ensure on to create a stack that already exists.
	_, err := stack.Named([]byte("error: no credentials\n"), "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no usable json")
}

func TestParseRows_EmptyListIsNotAnError(t *testing.T) {
	t.Parallel()

	rows, err := stack.ParseRows([]byte(`[]`))
	require.NoError(t, err)
	assert.Empty(t, rows, "a project with no stacks is a state, not a failure")
}

func TestQualifiedName(t *testing.T) {
	t.Parallel()

	got, err := stack.QualifiedName([]byte(qualified), "dev")
	require.NoError(t, err)
	assert.Equal(t, "acme/services/dev", got,
		"matched on the last segment, because -Q qualifies every row while the caller "+
			"knows only the stack's own name")
}

func TestQualifiedName_ASelfManagedBackendSaysSo(t *testing.T) {
	t.Parallel()

	// One segment is a name pulumi.NewStackReference cannot resolve, and the
	// error has to say that rather than "not found".
	_, err := stack.QualifiedName([]byte(listing), "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<org>/<project>/<stack>")
}

func TestQualifiedName_NotFoundCarriesTheList(t *testing.T) {
	t.Parallel()

	_, err := stack.QualifiedName([]byte(qualified), "staging")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acme/services/dev")
}

func TestQualifiedName_NoStacksAtAll(t *testing.T) {
	t.Parallel()

	// A different sentence from the one above: there is no list to offer.
	_, err := stack.QualifiedName([]byte(`[]`), "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the project has none")
}

func TestAction(t *testing.T) {
	t.Parallel()

	// Reporting "created" for a stack that was only selected is a wrong answer
	// in the one place an operator looks to see whether a stack is new.
	verb, state := stack.Action(true)
	assert.Equal(t, "select", verb)
	assert.Equal(t, stack.StateExisting, state)

	verb, state = stack.Action(false)
	assert.Equal(t, "init", verb)
	assert.Equal(t, stack.StateCreated, state)
}
