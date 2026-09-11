// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package gitattributes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const attrShell = "shell"

func testDoc() *projectfile.Document {
	return &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
}

// An absent namespace still yields the universal vocabulary: without `*.sh`
// carrying the attribute a `:(attr:shell)` pathspec would match nothing, and a
// predicate that matches nothing is the failure mode this file exists to end.
func TestAssembleNilExtensionKeepsBuiltins(t *testing.T) {
	body := string(assemble(testDoc(), nil))
	assert.Contains(t, body, core.Marker)
	assert.Contains(t, body, "*.sh")
	assert.Contains(t, body, "*.bash")
	assert.Contains(t, body, "# >>> builtin")
}

// Excludes MUST be emitted after includes: git resolves attributes
// last-match-wins, so the reverse order would silently drop every negation.
func TestAssembleExcludesFollowIncludes(t *testing.T) {
	ext := &pfmodel.AttributesExtension{
		Rules: map[string]*pfmodel.AttributeRule{
			attrShell: {
				Include: []string{"**/command.d/*"},
				Exclude: []string{"*.md"},
			},
		},
	}
	body := string(assemble(testDoc(), ext))

	inc := strings.Index(body, "# >>> shell-include")
	exc := strings.Index(body, "# >>> shell-exclude")
	require.Positive(t, inc, "include block missing")
	require.Positive(t, exc, "exclude block missing")
	assert.Less(t, inc, exc, "exclude block must come after include block")

	assert.Contains(t, body, "**/command.d/*  shell")
	assert.Contains(t, body, "*.md  -shell")
}

// The file feeds a drift gate, so identical input must produce identical bytes
// regardless of map iteration order or how the include lists were merged.
func TestAssembleIsDeterministic(t *testing.T) {
	build := func() string {
		return string(assemble(testDoc(), &pfmodel.AttributesExtension{
			Rules: map[string]*pfmodel.AttributeRule{
				attrShell:            {Include: []string{"b", "a", "b"}},
				"binary":             {Include: []string{"*.png"}},
				"linguist-generated": {Include: []string{"*.pot"}},
			},
		}))
	}
	first := build()
	for range 8 {
		assert.Equal(t, first, build())
	}
	// Duplicates collapse and order is sorted, not insertion order.
	assert.Contains(t, first, "a  shell\nb  shell\n")
}

// Raw extra lines pass through verbatim — they are the escape hatch for
// attributes the structured form cannot express (`*.bin diff=hex`).
func TestAssembleExtraPassesThrough(t *testing.T) {
	body := string(assemble(testDoc(), &pfmodel.AttributesExtension{
		Extra: []string{"*.bin diff=hex"},
	}))
	assert.Contains(t, body, "# >>> user-extra")
	assert.Contains(t, body, "*.bin diff=hex")
}

// Render is the dispatcher's entry point; it must survive a document with no extension namespace at all, which is every project before rollout.
func TestRenderWithoutNamespace(t *testing.T) {
	out, err := Bridge{}.Render(testDoc(), core.Options{})
	require.NoError(t, err)
	require.Len(t, out.Files, 1)
	assert.Contains(t, string(out.Files[filename]), "*.sh")
}

// The builtin block is alphabetically sorted like every user block, regardless of declaration order.
func TestAssembleBuiltinBlockSorted(t *testing.T) {
	body := string(assemble(testDoc(), nil))
	assert.Less(t, strings.Index(body, "*.bash"), strings.Index(body, "*.sh"),
		"expected *.bash before *.sh")
}
