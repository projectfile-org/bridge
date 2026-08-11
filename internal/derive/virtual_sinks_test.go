// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package derive_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/fieldpath"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/sink"
	"projectfile.org/projectfile/bridge/internal/derive"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Fixture literals shared across the table, declared once (goconst).
const (
	fixtureNS   = "org.b19"
	fixtureName = "ubuntu"
	keyHost     = "host"
	keyPriority = "priority"
	// addrPrimary / addrFallback are the two selectors the shared container
	// fragment writes. They are the contract this file exists to protect.
	addrPrimary  = "org.projectfile.sinks{role=primary}.ref"
	addrFallback = "org.projectfile.sinks{role=fallback}.ref"
)

// resolve runs one address against the document, the way the interpolator does.
func resolve(t *testing.T, doc *projectfile.Document, addr string) []any {
	t.Helper()
	path, err := fieldpath.Parse(addr)
	require.NoError(t, err)
	result, err := fieldpath.Resolve(doc, path)
	if err != nil {
		return nil
	}
	return result.Values
}

// The composed refs have to be REACHABLE by the selector a shared fragment
// writes. Composing them correctly and then parking them where the address
// grammar cannot see them renders no pull line at all — which is how this
// first shipped.
func TestVirtualSinksAreAddressableBySelector(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: fixtureNS, Name: fixtureName},
	}
	projectfile.SetExtension(doc, sink.ExtensionNS, map[string]any{
		"ghcr":  map[string]any{keyHost: "ghcr.io", "owner": "damian-buho", keyPriority: 90},
		"kiota": map[string]any{keyHost: "kiota.ch", "role": "fallback", keyPriority: 10},
	})

	derive.AddVirtual(doc)

	primary := resolve(t, doc, addrPrimary)
	require.Len(t, primary, 1, "the unroled entry must be reachable as primary")
	assert.Equal(t, "ghcr.io/damian-buho/b19/ubuntu:latest", primary[0])

	fallback := resolve(t, doc, addrFallback)
	require.Len(t, fallback, 1)
	assert.Equal(t, "kiota.ch/b19/ubuntu:latest", fallback[0])
}

// The fleet's path: no sinks namespace, only the legacy scalar. The synthesized
// entry must be reachable by the same selector, or ~130 READMEs lose their pull
// line.
func TestVirtualLegacySinkIsAddressable(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: fixtureNS, Name: fixtureName},
	}
	projectfile.SetExtension(doc, pfmodel.ReadmeExtensionNS, map[string]any{
		"registry": "kiota.ch",
	})

	derive.AddVirtual(doc)

	primary := resolve(t, doc, addrPrimary)
	require.Len(t, primary, 1)
	assert.Equal(t, "kiota.ch/b19/ubuntu:latest", primary[0])
}

// Fan-out order is the document's, not the map's spelling: `ecr` precedes `ghcr`
// alphabetically, and priority has to overrule that or the README recommends the
// wrong destination first.
func TestVirtualSinksFanOutInPriorityOrder(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: fixtureNS, Name: fixtureName},
	}
	projectfile.SetExtension(doc, sink.ExtensionNS, map[string]any{
		"ecr":  map[string]any{keyHost: "public.ecr.aws", keyPriority: 80},
		"ghcr": map[string]any{keyHost: "ghcr.io", keyPriority: 90},
	})

	derive.AddVirtual(doc)

	got := resolve(t, doc, addrPrimary)
	require.Len(t, got, 2)
	assert.Equal(t, "ghcr.io/b19/ubuntu:latest", got[0], "priority 90 leads")
	assert.Equal(t, "public.ecr.aws/b19/ubuntu:latest", got[1])
}

// Two sinks on ONE host under two accounts is the architectural falsifier at the
// derive layer: nothing here may collapse them, or the entry name secretly means
// a registry rather than a label.
func TestVirtualSinksKeepTwoAccountsOnOneHost(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: fixtureNS, Name: fixtureName},
	}
	projectfile.SetExtension(doc, sink.ExtensionNS, map[string]any{
		"hub-main": map[string]any{
			"ref": "docker.io/damianbuho/${image.flatname}:${image.tag}", keyPriority: 70,
		},
		"hub-oss": map[string]any{
			"ref": "docker.io/buho-oss/${image.flatname}:${image.tag}", keyPriority: 65,
		},
	})

	derive.AddVirtual(doc)

	got := resolve(t, doc, addrPrimary)
	require.Len(t, got, 2)
	assert.Equal(t, "docker.io/damianbuho/b19-ubuntu:latest", got[0])
	assert.Equal(t, "docker.io/buho-oss/b19-ubuntu:latest", got[1])
}
