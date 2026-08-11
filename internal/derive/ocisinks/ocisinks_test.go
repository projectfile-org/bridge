// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package ocisinks_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/sink"
	"projectfile.org/projectfile/bridge/internal/derive/ocisinks"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Fixture literals shared across the table, declared once (goconst).
const (
	keyHost     = "host"
	keyRef      = "ref"
	hostKiota   = "kiota.ch"
	hostGHCR    = "ghcr.io"
	ownerFleet  = "damian-buho"
	nameKiota   = "kiota"
	nameGHCR    = "ghcr"
	keyRegistry = "registry"
	keyOwner    = "owner"
	// b19Ref is what the default template composes for the fixture identity:
	// the basename already carries the project namespace, so no owner segment.
	b19Ref = "/b19/ubuntu:latest"
)

// docWith builds a document carrying an identity plus the given extension
// namespaces, which is every input the composition reads.
func docWith(namespace, name string, exts map[string]any) *projectfile.Document {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: namespace, Name: name},
	}
	for ns, v := range exts {
		projectfile.SetExtension(doc, ns, v)
	}
	return doc
}

// refOf reads one sink's composed reference out of the result.
func refOf(t *testing.T, refs map[string]any, name string) string {
	t.Helper()
	entry, ok := refs[name].(map[string]any)
	require.True(t, ok, "sink %q missing from %v", name, refs)
	ref, ok := entry[keyRef].(string)
	require.True(t, ok, "sink %q carries no ref", name)
	return ref
}

// The path the whole fleet rides: no sinks namespace at all, only the
// readme.registry scalar the container fragment has always set. The composed
// reference must be the one those projects already publish, or ~130 READMEs
// change on their next regeneration.
func TestLegacyScalarReproducesTodaysReference(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.ReadmeExtensionNS: map[string]any{keyRegistry: hostKiota},
	})

	refs := ocisinks.Refs(doc)

	require.Len(t, refs, 1, "the legacy scalar names exactly one sink")
	assert.Equal(t, hostKiota+b19Ref, refOf(t, refs, nameKiota))
}

// A project with no container build has no destination. Inventing a host would
// put a pull line in a README for an image nothing ever pushed.
func TestNoSinkAndNoScalarComposesNothing(t *testing.T) {
	assert.Empty(t, ocisinks.Refs(docWith("org.b19", "ubuntu", nil)))
}

// A project with no identity name resolves no basename, so there are no
// coordinates to compose against and the whole reference drops. That drop rule
// is why the container fragment is safe to default-wire across every project.
func TestNoImageBasenameComposesNothing(t *testing.T) {
	doc := docWith("", "", map[string]any{
		sink.ExtensionNS: map[string]any{nameGHCR: map[string]any{keyHost: hostGHCR}},
	})

	assert.Empty(t, ocisinks.Refs(doc))
}

// An entry that declares only a host is complete: the default template omits the
// owner segment, because the basename already carries the project's namespace.
// Defaulting it to ${image.namespace} would publish kiota.ch/b19/b19/ubuntu.
func TestBareHostOmitsTheOwnerSegment(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		sink.ExtensionNS: map[string]any{
			nameKiota: map[string]any{keyHost: hostKiota},
		},
	})

	assert.Equal(t, hostKiota+b19Ref, refOf(t, ocisinks.Refs(doc), nameKiota))
}

// A registry that forces one account on the whole fleet declares an owner, and
// the default template grows the segment.
func TestDeclaredOwnerAddsTheSegment(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		sink.ExtensionNS: map[string]any{
			nameGHCR: map[string]any{keyHost: hostGHCR, keyOwner: ownerFleet},
		},
	})

	assert.Equal(t, hostGHCR+"/"+ownerFleet+b19Ref, refOf(t, ocisinks.Refs(doc), nameGHCR))
}

// The composed reference is COMPLETE — no `${…}` survives it — but a matrix
// placeholder does, because it carries no `$` and belongs to the layer that
// substitutes per cell. That is what lets one composed ref fan out into one pull
// line per published series.
func TestMatrixPlaceholderSurvivesComposition(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		"org.projectfile.ci": map[string]any{"image": "b19/ubuntu/{B19_UBUNTU_SERIES}"},
		sink.ExtensionNS: map[string]any{
			"ecr": map[string]any{
				keyHost:  "public.ecr.aws",
				keyOwner: ownerFleet,
				keyRef:   "${sink.host}/${sink.owner}/${image.flatname}:${image.tag}",
			},
		},
	})

	assert.Equal(t, "public.ecr.aws/damian-buho/b19-ubuntu-{B19_UBUNTU_SERIES}:latest",
		refOf(t, ocisinks.Refs(doc), "ecr"))
}

// A template may name ${sink.owner} on a destination that forces no account.
// Falling back to the project's own image namespace keeps that template working
// instead of collapsing it to a double slash.
func TestOwnerFallsBackToTheImageNamespace(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		sink.ExtensionNS: map[string]any{
			nameKiota: map[string]any{
				keyHost: hostKiota,
				keyRef:  "${sink.host}/${sink.owner}/${image.name}:${image.tag}",
			},
		},
	})

	assert.Equal(t, "kiota.ch/b19/ubuntu:latest", refOf(t, ocisinks.Refs(doc), nameKiota))
}

// A sink with no authority addresses nothing. Composing from it would print a
// pull line beginning with a slash.
func TestUnaddressableEntryIsSkipped(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		sink.ExtensionNS: map[string]any{
			nameGHCR:  map[string]any{keyHost: hostGHCR},
			"broken":  map[string]any{keyOwner: "nobody"},
			"alsobad": map[string]any{},
		},
	})

	refs := ocisinks.Refs(doc)

	assert.Len(t, refs, 1)
	assert.Contains(t, refs, nameGHCR)
}

// A half-resolved ref is REFUSED, not written. A reference that silently lost a
// segment is a push to the wrong repository, and a README advertising it would
// send readers there too.
func TestHalfResolvedRefIsDropped(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		sink.ExtensionNS: map[string]any{
			nameGHCR: map[string]any{
				keyHost: hostGHCR,
				keyRef:  "${sink.host}/${sink.account}/${image.basename}:${image.tag}",
			},
		},
	})

	assert.Empty(t, ocisinks.Refs(doc))
}

// role and priority are what make an entry reachable and ordered:
// {role=fallback} selects the install group carrying the fallback prose, and
// priority orders the fan-out. An entry that declares NO role still gets one —
// a bare {} projection admits no trailing field, so an unroled entry would be
// addressable by nothing and its pull line would never render.
func TestRoleAndPriorityMakeEntriesAddressable(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		sink.ExtensionNS: map[string]any{
			nameKiota: map[string]any{keyHost: hostKiota, "role": "fallback", "priority": 10},
			nameGHCR:  map[string]any{keyHost: hostGHCR, "priority": 90},
		},
	})

	refs := ocisinks.Refs(doc)

	kiota, ok := refs[nameKiota].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, sink.RoleFallback, kiota["role"])
	assert.Equal(t, 10, kiota["priority"])

	ghcr, ok := refs[nameGHCR].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, sink.RolePrimary, ghcr["role"], "an unroled entry defaults to primary")
	assert.Equal(t, 90, ghcr["priority"])
}

// The legacy scalar synthesizes a PRIMARY sink. It is the only one the project
// has, so introducing it as a fallback would tell a reader to prefer a
// destination that does not exist.
func TestLegacyScalarIsPrimary(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.ReadmeExtensionNS: map[string]any{keyRegistry: hostKiota},
	})

	kiota, ok := ocisinks.Refs(doc)[nameKiota].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, sink.RolePrimary, kiota["role"])
}

// The composition replaces the template rather than sitting beside it. Two
// spellings of one reference would drift the moment a sink moved.
func TestTemplateIsReplacedNotKept(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		sink.ExtensionNS: map[string]any{
			nameGHCR: map[string]any{
				keyHost: hostGHCR,
				keyRef:  "${sink.host}/${image.flatname}",
			},
		},
	})

	assert.Equal(t, "ghcr.io/b19-ubuntu", refOf(t, ocisinks.Refs(doc), nameGHCR))
}

// Declared sinks win over the legacy scalar outright. Merging the two would
// render a pull line for a destination the project stopped using.
func TestDeclaredSinksSupersedeTheLegacyScalar(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.ReadmeExtensionNS: map[string]any{keyRegistry: hostKiota},
		sink.ExtensionNS:          map[string]any{nameGHCR: map[string]any{keyHost: hostGHCR}},
	})

	refs := ocisinks.Refs(doc)

	assert.Len(t, refs, 1)
	assert.Contains(t, refs, nameGHCR)
}

// Re-running must land the same subtree. The pass runs once per read today, but
// nothing stops a caller reading twice, and a composition that re-composed its
// own output would double every host label.
func TestRefsIsIdempotent(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		sink.ExtensionNS: map[string]any{
			nameGHCR: map[string]any{keyHost: hostGHCR, keyOwner: ownerFleet},
		},
	})

	first := ocisinks.Refs(doc)
	projectfile.SetExtension(doc, sink.ExtensionNS, first)

	assert.Equal(t, first, ocisinks.Refs(doc))
}
