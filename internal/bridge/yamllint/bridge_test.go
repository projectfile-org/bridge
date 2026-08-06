// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package yamllint

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	testForgejo  = ".forgejo/"
	testGithub   = ".github/"
	testVendor   = "vendor"
	testNodeMods = "node_modules"
	testInclude  = "include"
)

// GetIgnoresExtension must parse the yamllint sub-table off the extension,
// including the bare-list shorthand.
func TestGetIgnoresExtensionYamllint(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{
				extKeyYamllint: []any{testForgejo, testGithub},
			},
		},
	}
	ext, err := pfmodel.GetIgnoresExtension(doc)
	assert.NoError(t, err)
	assert.Equal(t, []string{testForgejo, testGithub}, ext.Yamllint.Include)
	assert.Nil(t, ext.Yamllint.Exclude)
}

// A present include list renders the full config: doc-start, REUSE header,
// banner with Marker, fleet rules, and an ignore block carrying the entries.
func TestAssembleIncludeApplied(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	body := string(assemble(pf, []string{testGithub, testForgejo, testVendor}))
	// Document-start first.
	assert.True(t, strings.HasPrefix(body, "---\n"))
	// Banner + marker present.
	assert.Contains(t, body, core.Marker)
	assert.Contains(t, body, "org.projectfile.ignores.yamllint")
	// Fleet rule block mirrored.
	assert.Contains(t, body, "extends: default")
	assert.Contains(t, body, "max-spaces-inside: 1")
	assert.Contains(t, body, "max: 120")
	// Ignore block carries all three entries, sorted (forgejo, github, vendor).
	assert.Contains(t, body, "ignore: |\n")
	assert.Contains(t, body, "  "+testForgejo)
	assert.Contains(t, body, "  "+testGithub)
	assert.Contains(t, body, "  "+testVendor)
}

// Duplicate and out-of-order includes collapse to sorted, de-duplicated output.
func TestAssembleDedupSort(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	body := string(assemble(pf, []string{
		testGithub, testForgejo, testGithub, testForgejo, testVendor,
	}))
	// Each pattern appears exactly once.
	assert.Equal(t, 1, strings.Count(body, "  "+testForgejo))
	assert.Equal(t, 1, strings.Count(body, "  "+testGithub))
	assert.Equal(t, 1, strings.Count(body, "  "+testVendor))
	// Sorted order: .forgejo/, .github/, vendor.
	forgejo := strings.Index(body, "  "+testForgejo)
	github := strings.Index(body, "  "+testGithub)
	vendor := strings.Index(body, "  "+testVendor)
	assert.Less(t, forgejo, github, "expected .forgejo/ before .github/")
	assert.Less(t, github, vendor, "expected .github/ before vendor")
}

// Render with a populated include list writes one file named .yamllint.
func TestRenderProducesFile(t *testing.T) {
	b := Bridge{filename: filenameYamllint}
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{
				extKeyYamllint: map[string]any{
					testInclude: []any{testForgejo},
				},
			},
		},
	}
	out, err := b.Render(pf, core.Options{})
	assert.NoError(t, err)
	assert.Len(t, out.Files, 1)
	assert.Contains(t, out.Files, filenameYamllint)
}

// Render emits nothing when the yamllint sub-namespace is absent.
func TestRenderEmptyNoFile(t *testing.T) {
	b := Bridge{filename: filenameYamllint}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	out, err := b.Render(pf, core.Options{})
	assert.NoError(t, err)
	assert.Empty(t, out.Files)
}

// Render emits nothing when the include list is empty.
func TestRenderEmptyIncludeNoFile(t *testing.T) {
	b := Bridge{filename: filenameYamllint}
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{
				extKeyYamllint: map[string]any{
					testInclude: []any{},
				},
			},
		},
	}
	out, err := b.Render(pf, core.Options{})
	assert.NoError(t, err)
	assert.Empty(t, out.Files)
}

// A `generate` opt-out list that omits yamllint suppresses generation even
// when includes are present. Unset means generate (default).
func TestRenderGenerateOptOut(t *testing.T) {
	b := Bridge{filename: filenameYamllint}
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{
				"generate": []any{"git", "docker"},
				extKeyYamllint: map[string]any{
					testInclude: []any{testForgejo},
				},
			},
		},
	}
	out, err := b.Render(pf, core.Options{})
	assert.NoError(t, err)
	assert.Empty(t, out.Files, "yamllint not in generate list → no file")
}

// A `generate` list that INCLUDES yamllint still produces the file.
func TestRenderGenerateOptIn(t *testing.T) {
	b := Bridge{filename: filenameYamllint}
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{
				"generate": []any{extKeyYamllint, "git"},
				extKeyYamllint: map[string]any{
					testInclude: []any{testForgejo},
				},
			},
		},
	}
	out, err := b.Render(pf, core.Options{})
	assert.NoError(t, err)
	assert.Len(t, out.Files, 1)
}
