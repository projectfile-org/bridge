// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package ociregistries_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive/ociregistries"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Fixture literals shared across the table, declared once (goconst).
const (
	keyHost     = "host"
	keyRef      = "ref"
	hostKiota   = "kiota.ch"
	hostGHCR    = "ghcr.io"
	ownerFleet  = "damian-buho"
	slugKiota   = "kiota"
	slugGHCR    = "ghcr"
	keyRegistry = "registry"
	keyOwner    = "owner"
	// defaultRefTail is what the default template composes after the host: the
	// basename already carries the project namespace, so no owner segment.
	defaultRefTail = "/${image.basename}:${image.tag}"
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

// refOf reads one slug's composed reference out of the result.
func refOf(t *testing.T, refs map[string]any, slug string) string {
	t.Helper()
	entry, ok := refs[slug].(map[string]any)
	require.True(t, ok, "slug %q missing from %v", slug, refs)
	ref, ok := entry[keyRef].(string)
	require.True(t, ok, "slug %q carries no ref", slug)
	return ref
}

// The path the whole fleet rides: no registries namespace at all, only the
// readme.registry scalar the container fragment has always set. The composed
// reference must be the one those projects already publish, or ~130 READMEs
// change on their next regeneration.
func TestLegacyScalarReproducesTodaysReference(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.ReadmeExtensionNS: map[string]any{keyRegistry: hostKiota},
	})

	refs := ociregistries.Refs(doc)

	require.Len(t, refs, 1, "the legacy scalar names exactly one registry")
	assert.Equal(t, hostKiota+defaultRefTail, refOf(t, refs, slugKiota))
}

// A project with no container build has no registry. Inventing a host would put
// a pull line in a README for an image nothing ever pushed.
func TestNoRegistryAndNoScalarComposesNothing(t *testing.T) {
	assert.Empty(t, ociregistries.Refs(docWith("org.b19", "ubuntu", nil)))
}

// An entry that declares only a host is complete: the default template omits the
// owner segment, because the basename already carries the project's namespace.
// Defaulting it to ${image.namespace} would publish kiota.ch/b19/b19/ubuntu.
func TestBareHostOmitsTheOwnerSegment(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.RegistriesExtensionNS: map[string]any{
			slugKiota: map[string]any{keyHost: hostKiota},
		},
	})

	assert.Equal(t, hostKiota+defaultRefTail,
		refOf(t, ociregistries.Refs(doc), slugKiota))
}

// A registry that forces one account on the whole fleet declares an owner, and
// the default template grows the segment.
func TestDeclaredOwnerAddsTheSegment(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.RegistriesExtensionNS: map[string]any{
			slugGHCR: map[string]any{keyHost: hostGHCR, keyOwner: ownerFleet},
		},
	})

	assert.Equal(t, hostGHCR+"/"+ownerFleet+defaultRefTail,
		refOf(t, ociregistries.Refs(doc), slugGHCR))
}

// Only the two entry-scoped variables are substituted here. Everything else —
// document addresses and matrix placeholders alike — must survive verbatim for
// the layers that already resolve them, or this package becomes a second
// template engine.
func TestOnlyEntryScopedVariablesAreSubstituted(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.RegistriesExtensionNS: map[string]any{
			"ecr": map[string]any{
				"host":   "public.ecr.aws",
				keyOwner: ownerFleet,
				keyRef:   "${registry.host}/${registry.owner}/${image.flatname}:${image.tag}-{B19_UBUNTU_SERIES}",
			},
		},
	})

	assert.Equal(t, "public.ecr.aws/damian-buho/${image.flatname}:${image.tag}-{B19_UBUNTU_SERIES}",
		refOf(t, ociregistries.Refs(doc), "ecr"))
}

// A template may name ${registry.owner} on a registry that forces no account.
// Falling back to the project's own image namespace keeps that template working
// instead of collapsing it to a double slash.
func TestOwnerFallsBackToTheImageNamespace(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.RegistriesExtensionNS: map[string]any{
			slugKiota: map[string]any{
				keyHost: hostKiota,
				keyRef:  "${registry.host}/${registry.owner}/${image.name}:${image.tag}",
			},
		},
	})

	assert.Equal(t, "kiota.ch/${image.namespace}/${image.name}:${image.tag}",
		refOf(t, ociregistries.Refs(doc), slugKiota))
}

// A registry with no authority addresses nothing. Composing from it would print
// a pull line beginning with a slash.
func TestHostlessEntryIsSkipped(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.RegistriesExtensionNS: map[string]any{
			slugGHCR:  map[string]any{keyHost: hostGHCR},
			"broken":  map[string]any{keyOwner: "nobody"},
			"alsobad": map[string]any{},
		},
	})

	refs := ociregistries.Refs(doc)

	assert.Len(t, refs, 1)
	assert.Contains(t, refs, slugGHCR)
}

// role and priority are what make an entry reachable and ordered:
// {role=fallback} selects the install group carrying the fallback prose, and
// priority orders the fan-out. An entry that declares NO role still gets one —
// a bare {} projection admits no trailing field, so an unroled entry would be
// addressable by nothing and its pull line would never render.
func TestRoleAndPriorityMakeEntriesAddressable(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.RegistriesExtensionNS: map[string]any{
			slugKiota: map[string]any{keyHost: hostKiota, "role": "fallback", "priority": 10},
			slugGHCR:  map[string]any{keyHost: hostGHCR, "priority": 90},
		},
	})

	refs := ociregistries.Refs(doc)

	kiota, ok := refs[slugKiota].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, pfmodel.RegistryRoleFallback, kiota["role"])
	assert.Equal(t, 10, kiota["priority"])

	ghcr, ok := refs[slugGHCR].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, pfmodel.RegistryRolePrimary, ghcr["role"], "an unroled entry defaults to primary")
	assert.Equal(t, 90, ghcr["priority"])
}

// The legacy scalar synthesizes a PRIMARY registry. It is the only one the
// project has, so introducing it as a fallback would tell a reader to prefer a
// registry that does not exist.
func TestLegacyScalarIsPrimary(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.ReadmeExtensionNS: map[string]any{keyRegistry: hostKiota},
	})

	kiota, ok := ociregistries.Refs(doc)["kiota"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, pfmodel.RegistryRolePrimary, kiota["role"])
}

// The composition replaces the template rather than sitting beside it. Two
// spellings of one reference would drift the moment a registry moved.
func TestTemplateIsReplacedNotKept(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.RegistriesExtensionNS: map[string]any{
			slugGHCR: map[string]any{
				keyHost: hostGHCR,
				keyRef:  "${registry.host}/${image.flatname}",
			},
		},
	})

	assert.Equal(t, "ghcr.io/${image.flatname}", refOf(t, ociregistries.Refs(doc), slugGHCR))
}

// A declared registries namespace wins over the legacy scalar outright. Merging
// the two would render a pull line for a registry the project stopped using.
func TestDeclaredRegistriesSupersedeTheLegacyScalar(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.ReadmeExtensionNS:     map[string]any{keyRegistry: hostKiota},
		pfmodel.RegistriesExtensionNS: map[string]any{slugGHCR: map[string]any{keyHost: hostGHCR}},
	})

	refs := ociregistries.Refs(doc)

	assert.Len(t, refs, 1)
	assert.Contains(t, refs, slugGHCR)
}

// Re-running must land the same subtree. The pass runs once per read today, but
// nothing stops a caller reading twice, and a composition that re-composed its
// own output would double every host label.
func TestRefsIsIdempotent(t *testing.T) {
	doc := docWith("org.b19", "ubuntu", map[string]any{
		pfmodel.RegistriesExtensionNS: map[string]any{
			slugGHCR: map[string]any{keyHost: hostGHCR, keyOwner: ownerFleet},
		},
	})

	first := ociregistries.Refs(doc)
	projectfile.SetExtension(doc, pfmodel.RegistriesExtensionNS, first)

	assert.Equal(t, first, ociregistries.Refs(doc))
}
