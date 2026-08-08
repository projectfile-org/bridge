// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// The Phase 1 escape hatch lets a template reach any projectfile field via
// .Doc.* and the pf/ls FuncMaps, so a custom extension block needs no Go edit.
// These tests pin that surface — and the nil-safe boundaries that make it safe
// to ship to user-supplied local templates.

// customExtNS is a sample reverse-DNS namespace for the pf-helper fixture. Its
// own constant so goconst sees one canonical home.
const customExtNS = "org.acme.todo"

// renderLocal renders the project through a project-local template at
// readme.md/<block>.tmpl and returns the produced body. This matches the
// real user-facing override path.
func renderLocal(t *testing.T, dir string, pf *projectfile.Document) string {
	t.Helper()
	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)
	return string(out.Files[filenameReadme])
}

// writeLocalTemplate writes a project-local override for a single block. The
// caller owns pf.Extensions so tests can layer custom namespaces on top of the
// readme.blocks config the helper writes here.
func writeLocalTemplate(t *testing.T, dir, block, body string) {
	t.Helper()
	writeFile(t, dir, ".projectfile/templates/readme.md/"+block+".tmpl", body)
}

// blocksOnly narrows the document's blocks list to the given sequence. Used by
// escape-hatch tests so the assertions stay focused on the one body under test.
func blocksOnly(pf *projectfile.Document, blocks ...string) {
	pf.Extensions = map[string]any{
		readmeNS: map[string]any{
			keyBlocks: toAnySlice(blocks),
		},
	}
}

// withExtension merges one custom namespace into pf.Extensions while keeping
// any readme.blocks config that's already there. Used to layer custom extension
// data on top of the blocks config without clobbering it.
func withExtension(pf *projectfile.Document, ns string, value any) {
	if pf.Extensions == nil {
		pf.Extensions = map[string]any{}
	}
	if _, ok := pf.Extensions[readmeNS].(map[string]any); !ok {
		pf.Extensions[readmeNS] = map[string]any{}
	}
	pf.Extensions[ns] = value
}

func toAnySlice(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

// TestTemplateReachesDocIdentity verifies a local template reading .Doc.Identity
// surfaces the raw identity fields without a dedicated view-model field.
func TestTemplateReachesDocIdentity(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	pf.Identity.Version = "1.2.3"
	blocksOnly(pf, blockBasics, blockLicense)
	writeLocalTemplate(t, dir, "basics", `version={{.Doc.Identity.Version}}`)
	body := renderLocal(t, dir, pf)
	assert.Contains(t, body, "version=1.2.3")
}

// TestTemplatePFResolvesCustomExtension verifies the pf FuncMap surfaces a
// value stored under an arbitrary [org.<ns>] extension namespace.
func TestTemplatePFResolvesCustomExtension(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	blocksOnly(pf, blockBasics, blockLicense)
	writeLocalTemplate(t, dir, "basics", `{{pf "org.acme.todo"}}`)
	withExtension(pf, customExtNS, "ship it")
	body := renderLocal(t, dir, pf)
	assert.Contains(t, body, "ship it")
}

// TestTemplatePFWalksNestedMap verifies the dotted walker descends nested
// tables, the form most TOML/YAML custom extensions take.
func TestTemplatePFWalksNestedMap(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	blocksOnly(pf, blockBasics, blockLicense)
	writeLocalTemplate(t, dir, "basics", `status={{pf "org.acme.status.phase"}}`)
	withExtension(pf, "org.acme.status", map[string]any{"phase": "beta"})
	body := renderLocal(t, dir, pf)
	assert.Contains(t, body, "status=beta")
}

// TestTemplatePFMissingPathRendersEmpty verifies a missing path never panics
// nor errors — it just renders empty so a template can probe safely.
func TestTemplatePFMissingPathRendersEmpty(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	blocksOnly(pf, blockBasics, blockLicense)
	writeLocalTemplate(t, dir, "basics", `[{{pf "org.does.not.exist"}}]`)
	body := renderLocal(t, dir, pf)
	assert.Contains(t, body, "[]")
	assert.NotContains(t, body, "<no value>")
}

// TestTemplateLSNilRendersEmpty verifies the ls FuncMap is nil-safe: a nil
// LocalizedString renders empty rather than panicking.
func TestTemplateLSNilRendersEmpty(t *testing.T) {
	tpl, err := template.New("t").
		Funcs(template.FuncMap{
			"ls": func(ls *projectfile.LocalizedString) string {
				return projectfile.ExtractLocalizedStringForLang(ls, "")
			},
		}).
		Parse(`[{{ls nil}}]`)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, tpl.Execute(&buf, nil))
	assert.Equal(t, "[]", buf.String())
}

// TestTemplateLSResolvesActiveLang verifies the ls FuncMap picks the right
// variant for the active render language — the reason it can't be a field.
func TestTemplateLSResolvesActiveLang(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	pf.Identity.Summary = &projectfile.LocalizedString{
		Langs: map[string]string{"en": summaryEN, "es": "Hola"},
	}
	pf.Extensions = map[string]any{
		pfmodel.I18NExtensionNS: map[string]any{keyLanguages: []any{"es"}},
		readmeNS: map[string]any{
			keyBlocks: []any{blockLanguages, blockBasics, blockLicense},
		},
	}
	// Override basics with a template that calls ls directly on the raw Doc
	// summary — exercises the same FuncMap the engine registers in production.
	writeFile(t, dir, ".projectfile/templates/readme.md/basics.tmpl", `[{{ls .Doc.Identity.Summary}}]`)
	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files["README.md"]), "[Hello]")
	assert.Contains(t, string(out.Files["docs/es/README.md"]), "[Hola]")
}

// TestPFLookupNilDoc verifies the helper never panics on a nil Doc — the
// invariant the FuncMap relies on so any template can probe freely.
func TestPFLookupNilDoc(t *testing.T) {
	assert.Nil(t, pfLookup(nil, "org.acme.todo"))
	assert.Nil(t, pfLookup(&projectfile.Document{}, "org.acme.todo"))
	assert.Nil(t, pfLookup(&projectfile.Document{
		Extensions: map[string]any{"org.acme": map[string]any{}},
	}, "org.acme.missing"))
}

// TestPFLookupLiteralDottedKey verifies the literal-dotted encoding (YAML/JSON
// and TOML quoted) is matched — the encoding LookupExtension also honours.
func TestPFLookupLiteralDottedKey(t *testing.T) {
	pf := &projectfile.Document{
		Extensions: map[string]any{
			"org.acme.todo": "literal",
		},
	}
	assert.Equal(t, "literal", pfLookup(pf, "org.acme.todo"))
}
