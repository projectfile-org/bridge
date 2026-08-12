// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package ocisinks_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive/ocisinks"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Fixture literals shared across the table, declared once (goconst).
const (
	keyRef      = "ref"
	keyRole     = "role"
	hostKiota   = "kiota.ch"
	nameKiota   = "kiota"
	nameGHCR    = "ghcr"
	keyRegistry = "registry"
	// b19Ref is what the fixture parts compose to. The path already carries the
	// project's own org, so a destination adds only what IT forces.
	b19Ref = "/b19/ubuntu:latest"
	// refGHCR is the entry the fleet declares: a host, a forced account, then the
	// project's own path. Every segment is literal text in the template.
	refGHCR = "ghcr.io/damian-buho/${path}:${tag}"
)

// parts is the image vocabulary the shared container fragment declares for every
// project on this plane. It is ordinary document data, and it is the ONLY input
// a sink template composes from.
func parts() map[string]any {
	return map[string]any{
		"org":  "b19",
		"name": "${identity.name}",
		"path": "${org}/${name}",
		"tag":  "latest",
	}
}

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

// docWithParts is docWith plus the standard image parts.
func docWithParts(exts map[string]any) *projectfile.Document {
	if exts == nil {
		exts = map[string]any{}
	}
	exts[pfmodel.ImageExtensionNS] = parts()
	return docWith("org.b19", "ubuntu", exts)
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
	doc := docWithParts(map[string]any{
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

// A project that declares NO image parts composes nothing: every template names
// parts, none of them resolve, and each entry drops. That drop rule is why the
// container fragment is safe to default-wire across every project — a project
// with no image simply renders no pull line.
func TestNoDeclaredPartsComposesNothing(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.SinksExtensionNS: map[string]any{
			nameGHCR: map[string]any{keyRef: refGHCR},
		},
	})

	assert.Empty(t, ocisinks.Refs(doc))
}

// An entry with no `ref` addresses nothing. There is no default template to fall
// back on any more: the destination's grammar is the entry's own text, so an
// entry that states none states no destination.
func TestEntryWithNoTemplateIsSkipped(t *testing.T) {
	doc := docWithParts(map[string]any{
		pfmodel.SinksExtensionNS: map[string]any{
			nameGHCR:  map[string]any{keyRef: refGHCR},
			"broken":  map[string]any{"owner": "nobody"},
			"alsobad": map[string]any{},
		},
	})

	refs := ocisinks.Refs(doc)

	assert.Len(t, refs, 1)
	assert.Contains(t, refs, nameGHCR)
}

// The composed reference is COMPLETE — no `${…}` survives it — but a matrix
// placeholder does, because it carries no `$` and belongs to the layer that
// substitutes per cell. That is what lets one composed ref fan out into one pull
// line per published series.
func TestMatrixPlaceholderSurvivesComposition(t *testing.T) {
	image := parts()
	image["series"] = "{B19_UBUNTU_SERIES}"
	image["flatpath"] = "${org}-${name}-${series}"
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.ImageExtensionNS: image,
		pfmodel.SinksExtensionNS: map[string]any{
			"ecr": map[string]any{keyRef: "public.ecr.aws/damian-buho/${flatpath}:${tag}"},
		},
	})

	assert.Equal(t, "public.ecr.aws/damian-buho/b19-ubuntu-{B19_UBUNTU_SERIES}:latest",
		refOf(t, ocisinks.Refs(doc), "ecr"))
}

// A half-resolved ref is REFUSED, not written. A reference that silently lost a
// segment is a push to the wrong repository, and a README advertising it would
// send readers there too.
func TestHalfResolvedRefIsDropped(t *testing.T) {
	doc := docWithParts(map[string]any{
		pfmodel.SinksExtensionNS: map[string]any{
			nameGHCR: map[string]any{keyRef: "ghcr.io/${account}/${path}:${tag}"},
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
	doc := docWithParts(map[string]any{
		pfmodel.SinksExtensionNS: map[string]any{
			nameKiota: map[string]any{
				keyRef: "kiota.ch/${path}:${tag}", keyRole: "fallback", "priority": 10,
			},
			nameGHCR: map[string]any{keyRef: refGHCR, "priority": 90},
		},
	})

	refs := ocisinks.Refs(doc)

	kiota, ok := refs[nameKiota].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "fallback", kiota[keyRole])
	assert.Equal(t, 10, kiota["priority"])

	ghcr, ok := refs[nameGHCR].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, pfmodel.SinkRolePrimary, ghcr[keyRole], "an unroled entry defaults to primary")
	assert.Equal(t, 90, ghcr["priority"])
}

// The legacy scalar synthesizes a PRIMARY sink. It is the only one the project
// has, so introducing it as a fallback would tell a reader to prefer a
// destination that does not exist.
func TestLegacyScalarIsPrimary(t *testing.T) {
	doc := docWithParts(map[string]any{
		pfmodel.ReadmeExtensionNS: map[string]any{keyRegistry: hostKiota},
	})

	kiota, ok := ocisinks.Refs(doc)[nameKiota].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, pfmodel.SinkRolePrimary, kiota[keyRole])
}

// The composition replaces the template rather than sitting beside it. Two
// spellings of one reference would drift the moment a sink moved.
func TestTemplateIsReplacedNotKept(t *testing.T) {
	doc := docWithParts(map[string]any{
		pfmodel.SinksExtensionNS: map[string]any{
			nameGHCR: map[string]any{keyRef: "ghcr.io/${path}"},
		},
	})

	assert.Equal(t, "ghcr.io/b19/ubuntu", refOf(t, ocisinks.Refs(doc), nameGHCR))
}

// Declared sinks win over the legacy scalar outright. Merging the two would
// render a pull line for a destination the project stopped using.
func TestDeclaredSinksSupersedeTheLegacyScalar(t *testing.T) {
	doc := docWithParts(map[string]any{
		pfmodel.ReadmeExtensionNS: map[string]any{keyRegistry: hostKiota},
		pfmodel.SinksExtensionNS: map[string]any{
			nameGHCR: map[string]any{keyRef: refGHCR},
		},
	})

	refs := ocisinks.Refs(doc)

	assert.Len(t, refs, 1)
	assert.Contains(t, refs, nameGHCR)
}

// Re-running must land the same subtree. The pass runs once per read today, but
// nothing stops a caller reading twice, and a composition that re-composed its
// own output would double every host label.
func TestRefsIsIdempotent(t *testing.T) {
	doc := docWithParts(map[string]any{
		pfmodel.SinksExtensionNS: map[string]any{
			nameGHCR: map[string]any{keyRef: refGHCR},
		},
	})

	first := ocisinks.Refs(doc)
	projectfile.SetExtension(doc, pfmodel.SinksExtensionNS, first)

	assert.Equal(t, first, ocisinks.Refs(doc))
}

// The phase's exit criterion, pinned: an unheard-of path grammar and two invented
// parts reach a working destination by EDITING YAML ONLY. If this test ever needs
// a Go change to keep passing, the domain has leaked back into the code.
func TestInventedPartsComposeWithNoCodeChange(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.ImageExtensionNS: map[string]any{
			"org": "b19", "name": "${identity.name}", "tag": "latest",
			"arch": "arm64", "vibe": "unhinged",
			"path": "${arch}/${vibe}~${org}--${name}",
		},
		pfmodel.SinksExtensionNS: map[string]any{
			"weird": map[string]any{keyRef: "registry.example.test/${path}#${tag}"},
		},
	})

	assert.Equal(t, "registry.example.test/arm64/unhinged~b19--ubuntu#latest",
		refOf(t, ocisinks.Refs(doc), "weird"))
}
