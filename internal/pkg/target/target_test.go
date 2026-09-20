package target_test

import (
	"testing"

	"github.com/oleg-tkachuk/pulumi-kit/internal/pkg/target"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stack is a small state, shaped the way `pulumi stack --show-urns --output
// json` shapes one: a component group with two children, a grandchild under one
// of them, and a provider resource whose type LEAF collides with the group's.
//
// Parents are spelled out because that is what a group is resolved by. They
// were absent from this fixture while the group matcher worked on URN
// substrings, which is how it passed while being wrong.
const (
	groupURN     = "urn:pulumi:dev::services::acme:platform:Ingress::ingress"
	releaseURN   = "urn:pulumi:dev::services::acme:platform:Ingress$kubernetes:helm.sh/v3:Release::traefik"
	balancerURN  = "urn:pulumi:dev::services::acme:platform:Ingress$hcloud:index/loadBalancer:LoadBalancer::ingress-lb"
	targetURN    = "urn:pulumi:dev::services::acme:platform:Ingress$hcloud:index/loadBalancer:LoadBalancer$hcloud:index/loadBalancerTarget:LoadBalancerTarget::ingress-targets"
	outsideURN   = "urn:pulumi:dev::services::kubernetes:networking.k8s.io/v1:Ingress::argocd"
	otherHelmURN = "urn:pulumi:dev::services::kubernetes:helm.sh/v3:Release::argocd"
	rootURN      = "urn:pulumi:dev::services::pulumi:pulumi:Stack::services-dev"
)

var stack = []target.Resource{
	{URN: rootURN, Type: "pulumi:pulumi:Stack", Name: "services-dev"},
	{URN: groupURN, Type: "acme:platform:Ingress", Name: "ingress", Parent: rootURN},
	{URN: releaseURN, Type: "kubernetes:helm.sh/v3:Release", Name: "traefik", Parent: groupURN},
	{URN: balancerURN, Type: "hcloud:index/loadBalancer:LoadBalancer", Name: "ingress-lb", Parent: groupURN},
	// Two hops down, which the substring form happened to catch and a
	// single-level walk would not.
	{URN: targetURN, Type: "hcloud:index/loadBalancerTarget:LoadBalancerTarget",
		Name: "ingress-targets", Parent: balancerURN},
	{URN: outsideURN, Type: "kubernetes:networking.k8s.io/v1:Ingress", Name: "argocd", Parent: rootURN},
	{URN: otherHelmURN, Type: "kubernetes:helm.sh/v3:Release", Name: "argocd", Parent: rootURN},
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
	assert.Equal(t, []string{releaseURN}, urns)
}

func TestMatch_QualifiedByTypeLeaf(t *testing.T) {
	t.Parallel()

	// The disambiguating form the error below advertises, so the advice and
	// the parser cannot drift apart.
	urns, err := target.Match(stack, "Release:argocd", groupPackage)
	require.NoError(t, err)
	assert.Equal(t, []string{otherHelmURN}, urns)
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
	assert.Equal(t, []string{groupURN, releaseURN, balancerURN, targetURN}, urns,
		"a group is its own node plus everything under it")
}

func TestMatch_GroupDoesNotTakeAProviderResourceWithTheSameLeaf(t *testing.T) {
	t.Parallel()

	// The property the group package exists for. `kubernetes:…:Ingress` has
	// the same type leaf as the component, and matching on the leaf alone
	// would take every Ingress object in the cluster.
	urns, err := target.Match(stack, target.GroupPrefix+"Ingress", groupPackage)
	require.NoError(t, err)
	assert.NotContains(t, urns, outsideURN)
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
		"one": {selectors: "traefik", want: []string{releaseURN}},
		"two": {
			selectors: "traefik,Release:argocd",
			want:      []string{releaseURN, otherHelmURN},
		},
		"whitespace is trimmed": {
			selectors: " traefik , Release:argocd ",
			want:      []string{releaseURN, otherHelmURN},
		},
		// Legitimate, and Pulumi must not be handed the same --target twice.
		"overlapping selectors are deduplicated": {
			selectors: target.GroupPrefix + "Ingress,traefik",
			want:      []string{groupURN, releaseURN, balancerURN, targetURN},
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

// twinURNs is two components of the SAME type, each with its own child. This
// is the state the old group matcher was confidently wrong about.
const (
	twinRootURN = "urn:pulumi:dev::p::pulumi:pulumi:Stack::p-dev"
	netAURN     = "urn:pulumi:dev::p::acme:cluster:Network::net-a"
	childAURN   = "urn:pulumi:dev::p::acme:cluster:Network$hcloud:index/network:Network::child-a"
	netBURN     = "urn:pulumi:dev::p::acme:cluster:Network::net-b"
	childBURN   = "urn:pulumi:dev::p::acme:cluster:Network$hcloud:index/network:Network::child-b"
)

var twins = []target.Resource{
	{URN: twinRootURN, Type: "pulumi:pulumi:Stack", Name: "p-dev"},
	{URN: netAURN, Type: "acme:cluster:Network", Name: "net-a", Parent: twinRootURN},
	{URN: childAURN, Type: "hcloud:index/network:Network", Name: "child-a", Parent: netAURN},
	{URN: netBURN, Type: "acme:cluster:Network", Name: "net-b", Parent: twinRootURN},
	{URN: childBURN, Type: "hcloud:index/network:Network", Name: "child-b", Parent: netBURN},
}

// TestMatch_GroupTakesTheWholeSubtree is the level the old matcher reached by
// accident: a grandchild is in the group, and a walk one level deep would drop
// it.
func TestMatch_GroupTakesTheWholeSubtree(t *testing.T) {
	t.Parallel()

	urns, err := target.Match(stack, target.GroupPrefix+"Ingress", groupPackage)
	require.NoError(t, err)
	assert.Contains(t, urns, targetURN, "a resource two hops under the group is in it")
}

// TestMatch_GroupRefusesTwoComponentsOfOneType is the bug, from the outside.
//
// Measured before the fix: `group:Network` returned net-a, net-a's child AND
// net-b's child, leaving net-b itself out. A targeted destroy would have
// removed a resource under a component nobody named and orphaned the one it
// belonged to.
func TestMatch_GroupRefusesTwoComponentsOfOneType(t *testing.T) {
	t.Parallel()

	_, err := target.Match(twins, target.GroupPrefix+"Network", groupPackage)
	require.Error(t, err, "taking the first of two is a selection that is confidently wrong")
	assert.Contains(t, err.Error(), "group:Network:net-a")
	assert.Contains(t, err.Error(), "group:Network:net-b")
}

func TestMatch_GroupQualifiedByName(t *testing.T) {
	t.Parallel()

	// The disambiguating form the error above advertises, so the advice and
	// the parser cannot drift apart.
	urns, err := target.Match(twins, target.GroupPrefix+"Network:net-a", groupPackage)
	require.NoError(t, err)
	assert.Equal(t, []string{netAURN, childAURN}, urns)
	assert.NotContains(t, urns, childBURN, "the other component's child is not in this group")
	assert.NotContains(t, urns, netBURN)
}

func TestMatch_GroupQualifiedByANameThatDoesNotExist(t *testing.T) {
	t.Parallel()

	_, err := target.Match(twins, target.GroupPrefix+"Network:net-c", groupPackage)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matches nothing")
}

// TestMatch_GroupRefusesAListingWithNoParents is the guard on the field the
// fix depends on.
//
// Without it a listing that carried no parent would resolve every group to its
// own node and quietly leave the children behind — a selection narrower than
// asked for, which for an apply is a partial update reporting success.
func TestMatch_GroupRefusesAListingWithNoParents(t *testing.T) {
	t.Parallel()

	orphaned := make([]target.Resource, 0, len(twins))
	for _, resource := range twins {
		resource.Parent = ""
		orphaned = append(orphaned, resource)
	}

	_, err := target.Match(orphaned, target.GroupPrefix+"Network:net-a", groupPackage)
	require.ErrorIs(t, err, target.ErrNoParents)
}

func TestNames_AreSortedAndWithoutRepeats(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{
		"Ingress:argocd",
		"Ingress:ingress",
		"LoadBalancer:ingress-lb",
		"LoadBalancerTarget:ingress-targets",
		"Release:argocd",
		"Release:traefik",
		"Stack:services-dev",
	}, target.Names(stack))
}

// TestNames_AreWhatTheRefusalCarries is the property -list exists for.
//
// Its whole value is that an operator no longer has to mistype a selector on
// purpose to see what a stack holds. If the two lists were assembled
// separately they would drift, and the drift would be invisible — each would
// look right on its own, and the one an operator reads under pressure would
// be the stale one.
func TestNames_AreWhatTheRefusalCarries(t *testing.T) {
	t.Parallel()

	_, err := target.Match(stack, "no-such-resource", groupPackage)
	require.Error(t, err)

	for _, name := range target.Names(stack) {
		assert.Contains(t, err.Error(), name,
			"the refusal must carry the same list -list prints")
	}
}

func TestNames_OnAnEmptyState(t *testing.T) {
	t.Parallel()

	// Not reachable through the command — ResourcesIn refuses an empty stack
	// first — but a caller of the package should not get a nil surprise.
	assert.Empty(t, target.Names(nil))
}
