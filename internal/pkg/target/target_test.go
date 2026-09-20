package target_test

import (
	"testing"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/target"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stack is a small state: a component group, two children under it, and a
// provider resource whose type LEAF collides with the group's.
var stack = []target.Resource{
	{
		URN:  "urn:pulumi:dev::services::acme:platform:Ingress::ingress",
		Type: "acme:platform:Ingress", Name: "ingress",
	},
	{
		URN: "urn:pulumi:dev::services::acme:platform:Ingress$kubernetes:helm.sh/v3:" +
			"Release::traefik",
		Type: "kubernetes:helm.sh/v3:Release", Name: "traefik",
	},
	{
		URN: "urn:pulumi:dev::services::acme:platform:Ingress$hcloud:index/" +
			"loadBalancer:LoadBalancer::ingress-lb",
		Type: "hcloud:index/loadBalancer:LoadBalancer", Name: "ingress-lb",
	},
	{
		URN:  "urn:pulumi:dev::services::kubernetes:networking.k8s.io/v1:Ingress::argocd",
		Type: "kubernetes:networking.k8s.io/v1:Ingress", Name: "argocd",
	},
	{
		URN:  "urn:pulumi:dev::services::kubernetes:helm.sh/v3:Release::argocd",
		Type: "kubernetes:helm.sh/v3:Release", Name: "argocd",
	},
}

const groupPackage = "acme"

func TestTypeLeaf(t *testing.T) {
	t.Parallel()

	for token, want := range map[string]string{
		"kubernetes:helm.sh/v3:Release": "Release",
		"acme:platform:Ingress":         "Ingress",
		"Release":                       "Release",
		"":                              "",
	} {
		assert.Equal(t, want, target.TypeLeaf(token), token)
	}
}

func TestMatch_OneResourceByName(t *testing.T) {
	t.Parallel()

	urns, err := target.Match(stack, "traefik", groupPackage)
	require.NoError(t, err)
	assert.Equal(t, []string{stack[1].URN}, urns)
}

func TestMatch_QualifiedByTypeLeaf(t *testing.T) {
	t.Parallel()

	// The disambiguating form the error below advertises, so the advice and
	// the parser cannot drift apart.
	urns, err := target.Match(stack, "Release:argocd", groupPackage)
	require.NoError(t, err)
	assert.Equal(t, []string{stack[4].URN}, urns)
}

func TestMatch_AmbiguousNameNamesEveryQualifiedForm(t *testing.T) {
	t.Parallel()

	// Picking one silently would target a resource the operator did not mean.
	_, err := target.Match(stack, "argocd", groupPackage)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Ingress:argocd")
	assert.Contains(t, err.Error(), "Release:argocd")
}

func TestMatch_NothingCarriesWhatTheStackHolds(t *testing.T) {
	t.Parallel()

	// The whole reason this package exists: Pulumi's own answer here is silent
	// success, so the error has to be the one an operator reads.
	_, err := target.Match(stack, "traefk", groupPackage)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matches nothing")
	assert.Contains(t, err.Error(), "Release:traefik")
}

func TestMatch_GroupTakesTheNodeAndItsChildren(t *testing.T) {
	t.Parallel()

	urns, err := target.Match(stack, target.GroupPrefix+"Ingress", groupPackage)
	require.NoError(t, err)
	assert.Equal(t, []string{stack[0].URN, stack[1].URN, stack[2].URN}, urns,
		"a group is its own node plus everything under it")
}

func TestMatch_GroupDoesNotTakeAProviderResourceWithTheSameLeaf(t *testing.T) {
	t.Parallel()

	// The property the group package exists for. `kubernetes:…:Ingress` has
	// the same type leaf as the component, and matching on the leaf alone
	// would take every Ingress object in the cluster.
	urns, err := target.Match(stack, target.GroupPrefix+"Ingress", groupPackage)
	require.NoError(t, err)
	assert.NotContains(t, urns, stack[3].URN)
}

func TestMatch_GroupWithoutAPackageRefuses(t *testing.T) {
	t.Parallel()

	// Refused rather than defaulted: a guessed package is how a group selector
	// comes to mean a provider's resource.
	_, err := target.Match(stack, target.GroupPrefix+"Ingress", "")
	require.ErrorIs(t, err, target.ErrNoGroupPackage)
}

func TestMatch_GroupThatDoesNotExist(t *testing.T) {
	t.Parallel()

	_, err := target.Match(stack, target.GroupPrefix+"Database", groupPackage)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matches nothing")
}

func TestMatchAll(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		selectors string
		want      []string
		fails     string
	}{
		"one": {selectors: "traefik", want: []string{stack[1].URN}},
		"two": {
			selectors: "traefik,Release:argocd",
			want:      []string{stack[1].URN, stack[4].URN},
		},
		"whitespace is trimmed": {
			selectors: " traefik , Release:argocd ",
			want:      []string{stack[1].URN, stack[4].URN},
		},
		// Legitimate, and Pulumi must not be handed the same --target twice.
		"overlapping selectors are deduplicated": {
			selectors: target.GroupPrefix + "Ingress,traefik",
			want:      []string{stack[0].URN, stack[1].URN, stack[2].URN},
		},
		"the same selector twice is a mistake": {
			selectors: "traefik,traefik", fails: "twice",
		},
		"an empty element": {selectors: "traefik,", fails: "empty selector"},
		// The list refuses as a whole: the operator asked for both, and a
		// partial apply that reports success is what this prevents.
		"one typo refuses the list": {selectors: "traefik,traefk", fails: "matches nothing"},
	} {
		got, err := target.MatchAll(stack, tc.selectors, groupPackage)

		if tc.fails != "" {
			require.Error(t, err, name)
			assert.Contains(t, err.Error(), tc.fails, name)

			continue
		}

		require.NoError(t, err, name)
		assert.Equal(t, tc.want, got, name)
	}
}

func TestResourcesIn(t *testing.T) {
	t.Parallel()

	parsed, err := target.ResourcesIn([]byte(`{"resources":[
	  {"urn":"urn:pulumi:dev::p::kubernetes:helm.sh/v3:Release::traefik",
	   "type":"kubernetes:helm.sh/v3:Release","name":"traefik"}]}`))
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, "traefik", parsed[0].Name)
}

func TestResourcesIn_RefusesAnEmptyStack(t *testing.T) {
	t.Parallel()

	// An empty stack is not a selector problem, and saying "matches nothing"
	// here would send the operator looking for a typo.
	_, err := target.ResourcesIn([]byte(`{"resources":[]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "holds no resources")
}

func TestResourcesIn_RefusesWhatIsNotJSON(t *testing.T) {
	t.Parallel()

	// What `pulumi stack --show-urns` prints without --output: a tree for a
	// terminal, whose columns move between releases.
	_, err := target.ResourcesIn([]byte("Current stack resources (25):\n    TYPE  NAME\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse the stack listing")
}
