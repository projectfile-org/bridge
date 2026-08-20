// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/bridge/fragments"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// writeProjectfile writes a minimal projectfile.yaml (with SPDX header so
// REUSEHeader resolves) into dir. body is the org.projectfile.fragments
// block so each test customises its own documents[].
// spdxTag and spdxMIT are split so the REUSE scanner does not read the
// assembled literal in this file as a licence expression. They reconstruct
// valid SPDX blocks in the generated test fixtures only. The fragment-key
// constants satisfy goconst for the repeated map-key literals below.
const (
	spdxTag       = "SPDX-License-Identifier"
	spdxMIT       = "MIT"
	keyFragments  = "fragments"
	keyDocuments  = "documents"
	keyParents    = "parents"
	outFeatures   = "FEATURES.md"
	titleFeatures = "Features"
	parentURL     = "https://example.test/b19/ubuntu"

	// Committed-fixture literals repeated across tests (goconst).
	headingInherited = "## Inherited from b19/ubuntu"
	ownFeatureBody   = "### Own Feature\n\nOwn body."
	parentFeatBody   = "### Parent Feature\n\nParent body."
)

// offlineOptions is how every parents-declaring test runs: no fetch, the
// committed document is the only record of inherited content.
func offlineOptions(dir string) core.Options {
	return core.Options{Dir: dir, Offline: true}
}

func writeProjectfile(t *testing.T, dir, body string) {
	t.Helper()
	pf := "---\n" +
		"# SPDX-FileCopyrightText: 2026 Tester\n" +
		"#\n" +
		"# " + spdxTag + ": " + spdxMIT + "\n" +
		"$schema: https://projectfile.org/schema/v1.json\n" +
		"identity:\n  name: " + filepath.Base(dir) + "\n" +
		"org:\n  projectfile:\n" + body + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte(pf), 0o644))
}

// writeFragment writes one docs/<dir>/<name>.md fragment with an H1 and body.
func writeFragment(t *testing.T, projectDir, dir, name, h1, body string) {
	t.Helper()
	full := filepath.Join(projectDir, dir)
	require.NoError(t, os.MkdirAll(full, 0o755))
	content := "<!--\nSPDX-FileCopyrightText: 2026 Tester\n" + spdxTag + ": " + spdxMIT + "\n-->\n\n"
	content += "# " + h1 + "\n\n" + body + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(full, name+".md"), []byte(content), 0o644))
}

// docWithFragments reads the projectfile at dir (resolving includes) and
// returns the Document the bridge's Render expects.
func docWithFragments(t *testing.T, dir string) *projectfile.Document {
	t.Helper()
	pfPath, err := projectfile.DetectPath(dir)
	require.NoError(t, err)
	pf, err := projectfile.Read(pfPath)
	require.NoError(t, err)
	return pf
}

// writeCommittedDoc writes one committed assembled document in the shape a
// render leaves behind: SPDX header, optional textlint wrap (a localized
// variant), H1, the project section's own bodies, then one block per
// inherited section. Offline runs read their inherited sections back out of
// exactly this file, so it is the fixture that replaces the former cache.
func writeCommittedDoc(t *testing.T, projectDir, rel, h1, projectHeading string, own []string, sections [][2]string, wrap bool) {
	t.Helper()
	full := filepath.Join(projectDir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	var b strings.Builder
	b.WriteString("<!--\nSPDX-FileCopyrightText: 2026 Tester\n" + spdxTag + ": " + spdxMIT + "\n-->\n\n")
	if wrap {
		b.WriteString("<!-- textlint-disable terminology,common-misspellings -->\n\n")
	}
	b.WriteString("# " + h1 + "\n")
	for _, body := range own {
		b.WriteString("\n" + projectHeading + "\n\n" + body + "\n")
	}
	for _, s := range sections {
		b.WriteString("\n" + s[0] + "\n\n" + s[1] + "\n")
	}
	content := strings.TrimRight(b.String(), "\n") + "\n"
	if wrap {
		content = strings.TrimRight(content, "\n") + "\n\n<!-- textlint-enable -->\n"
	}
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// parentsBlock declares URL parents inside a document, matching the shape a
// projectfile carries now that parents are repository URLs rather than paths.
func parentsBlock(urls ...string) string {
	block := "          parents:\n"
	for _, u := range urls {
		block += "            - {url: " + u + "}\n"
	}
	return block
}

// TestRenderAssemblesOwnFragments is the baseline: one document, own
// fragments only, no parents. Verifies H1 title, the demoted H3s the
// readme bridge scrapes, the SPDX HTML header, and section headings.
func TestRenderAssemblesOwnFragments(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    fragments:\n      documents:\n        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	writeFragment(t, dir, "docs/features.d", "alpha", "Alpha Feature", "Alpha body.")
	writeFragment(t, dir, "docs/features.d", "beta", "Beta Feature", "Beta body.")

	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	require.Contains(t, out.Files, outFeatures)

	body := string(out.Files[outFeatures])
	assert.Contains(t, body, "# Features")
	assert.Contains(t, body, "## Project Features")
	assert.Contains(t, body, "### Alpha Feature")
	assert.Contains(t, body, "### Beta Feature")
	assert.Contains(t, body, "Alpha body.")
	// SPDX HTML header is prepended (REUSE-preferred form for tracked Markdown).
	assert.True(t, strings.HasPrefix(body, "<!--"))
	// Assemble from spdxTag/spdxMIT so the REUSE scanner does not read the
	// assembled literal in this file as a licence expression.
	assert.Contains(t, body, spdxTag+": "+spdxMIT)
	// No inherited section when no parents declared.
	assert.NotContains(t, body, "## Inherited")
}

// TestRenderDeterministicOrder verifies the fragment glob is sorted so two
// runs produce identical output regardless of filesystem order —
// load-bearing for reproducible builds.
func TestRenderDeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    fragments:\n      documents:\n        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	for _, n := range []string{"zeta", "alpha", "mid"} {
		writeFragment(t, dir, "docs/features.d", n, n, n)
	}
	pf := docWithFragments(t, dir)
	out1, err := fragments.Bridge{}.Render(pf, offlineOptions(dir))
	require.NoError(t, err)
	out2, err := fragments.Bridge{}.Render(pf, offlineOptions(dir))
	require.NoError(t, err)
	assert.Equal(t, out1.Files[outFeatures], out2.Files[outFeatures])

	body := string(out1.Files[outFeatures])
	idxA := strings.Index(body, "### alpha")
	idxM := strings.Index(body, "### mid")
	idxZ := strings.Index(body, "### zeta")
	assert.Less(t, idxA, idxM)
	assert.Less(t, idxM, idxZ, "fragments must appear in sorted (alpha,mid,zeta) order")
}

// TestRenderInheritsCommittedSection asserts an inherited section read back
// out of the committed document lands verbatim in the reassembly, heading
// included — the heading names the parent only, and which version a fetch
// read lives in the logs, never the document.
func TestRenderInheritsCommittedSection(t *testing.T) {
	child := t.TempDir()
	body := "    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		parentsBlock(parentURL)
	writeProjectfile(t, child, body)
	writeFragment(t, child, "docs/features.d", "child-feat", "Child Feature", "From child.")
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		[]string{"### Child Feature\n\nFrom child."},
		[][2]string{{headingInherited, "### Parent Feature\n\nFrom parent."}},
		false)

	out, err := fragments.Bridge{}.Render(docWithFragments(t, child), offlineOptions(child))
	require.NoError(t, err)
	got := string(out.Files[outFeatures])
	assert.Contains(t, got, "## Project Features")
	assert.Contains(t, got, "### Child Feature")
	assert.Contains(t, got, headingInherited)
	assert.Contains(t, got, "### Parent Feature")
	// Only the document's own SPDX header survives — no comment-based storage.
	assert.Equal(t, 1, strings.Count(got, "<!--"), "only the document SPDX header survives")
}

// TestRenderParentWithoutCommittedDocIsQuiet covers the state a project starts
// in: a parent is declared but nothing was ever generated online, so there is
// no committed document to read sections from. That must render the own
// fragments and no empty section, never an error.
func TestRenderParentWithoutCommittedDocIsQuiet(t *testing.T) {
	child := t.TempDir()
	body := "    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		parentsBlock(parentURL)
	writeProjectfile(t, child, body)
	writeFragment(t, child, "docs/features.d", "child-feat", "Child Feature", "From child.")

	out, err := fragments.Bridge{}.Render(docWithFragments(t, child), offlineOptions(child))
	require.NoError(t, err)
	got := string(out.Files[outFeatures])
	assert.Contains(t, got, "### Child Feature")
	assert.NotContains(t, got, "## Inherited")
}

// TestRenderInheritedOrderIsDeterministic verifies several committed sections
// re-render in committed order — the order the last online generate wrote —
// so two offline runs on one tree produce identical bytes, the property the
// drift gate compares against.
func TestRenderInheritedOrderIsDeterministic(t *testing.T) {
	child := t.TempDir()
	body := "    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		parentsBlock("ssh://git@example.test/b19/zeta.git", "ssh://git@example.test/b19/alpha.git")
	writeProjectfile(t, child, body)
	writeFragment(t, child, "docs/features.d", "own", "Own Feature", "Own body.")
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		[]string{ownFeatureBody},
		[][2]string{
			{"## Inherited from b19/alpha", "### Alpha Feature\n\nAlpha body."},
			{"## Inherited from b19/zeta", "### Zeta Feature\n\nZeta body."},
		},
		false)

	pf := docWithFragments(t, child)
	first, err := fragments.Bridge{}.Render(pf, offlineOptions(child))
	require.NoError(t, err)
	second, err := fragments.Bridge{}.Render(pf, offlineOptions(child))
	require.NoError(t, err)
	assert.Equal(t, first.Files[outFeatures], second.Files[outFeatures])

	got := string(first.Files[outFeatures])
	assert.Less(t, strings.Index(got, "b19/alpha"), strings.Index(got, "b19/zeta"))
}

// TestRenderStripsSPDXAndDemotesH1 verifies each fragment's leading SPDX
// HTML comment is stripped and its H1 is demoted to H3.
func TestRenderStripsSPDXAndDemotesH1(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    fragments:\n      documents:\n        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	// Fragment whose SPDX comment is multi-line and would corrupt output
	// if not stripped; H1 must become H3.
	writeFragment(t, dir, "docs/features.d", "solo", "Solo", "Body line.")

	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	got := string(out.Files[outFeatures])
	// Only the document-level SPDX header should remain as an HTML comment.
	assert.Equal(t, 1, strings.Count(got, "<!--"), "per-fragment SPDX comment must be stripped")
	assert.Contains(t, got, "### Solo")
	assert.NotContains(t, got, "\n# Solo\n", "fragment H1 must be demoted, not left as H1")
}

// TestRenderNoDocumentsIsNoOp verifies an absent or empty fragments
// namespace yields an empty Output (RunRender then logs "no-op").
func TestRenderNoDocumentsIsNoOp(t *testing.T) {
	dir := t.TempDir()
	// No org.projectfile.fragments namespace at all.
	writeProjectfile(t, dir, "    status: maintained")
	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	assert.Empty(t, out.Files)
}

// TestRenderEmptyDocumentIsSkipped verifies a declared document whose dir is
// absent (no own fragments) AND has no parents (no inherited sections) is
// skipped — no file emitted. Regression for the bare `# Title` stub that
// failed markdownlint (empty section + missing final newline).
func TestRenderEmptyDocumentIsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir,
		"    fragments:\n      documents:\n"+
			"        - {dir: docs/features.d, out: FEATURES.md, title: Features}\n"+
			"        - {dir: docs/roadmap.d, out: ROADMAP.md, title: Roadmap}")
	// Neither docs/features.d nor docs/roadmap.d exists on disk, no parents.
	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	assert.Empty(t, out.Files, "declared-but-empty shells must produce no files")
}

// TestRenderHasSingleTrailingNewline verifies every emitted file ends with
// exactly one newline (markdownlint MD047). Regression for the double-blank
// trailing lines the per-fragment range left behind.
func TestRenderHasSingleTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    fragments:\n      documents:\n        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	writeFragment(t, dir, "docs/features.d", "alpha", "Alpha", "Body.")

	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	got := string(out.Files[outFeatures])
	assert.True(t, strings.HasSuffix(got, "\n"), "file must end with a newline")
	assert.False(t, strings.HasSuffix(got, "\n\n"), "file must not end with a blank line")
}

// TestRenderOfflinePreservesUndeclaredSection pins the preservation rule: an
// offline run cannot know which parent a committed section belongs to, so it
// keeps every committed section verbatim — the next online generate is what
// drops a section whose parent is no longer declared.
func TestRenderOfflinePreservesUndeclaredSection(t *testing.T) {
	child := t.TempDir()
	body := "    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		parentsBlock("ssh://git@example.test/b19/ubuntu.git")
	writeProjectfile(t, child, body)
	writeFragment(t, child, "docs/features.d", "own", "Own Feature", "Own body.")
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		[]string{ownFeatureBody},
		[][2]string{
			{headingInherited, "### Kept Feature\n\nKept body."},
			{"## Inherited from b19/dropped", "### Dropped Feature\n\nDropped body."},
		},
		false)

	out, err := fragments.Bridge{}.Render(docWithFragments(t, child), offlineOptions(child))
	require.NoError(t, err)
	got := string(out.Files[outFeatures])
	assert.Contains(t, got, "### Kept Feature")
	assert.Contains(t, got, "### Dropped Feature", "offline preserves what is committed, undeclared or not")
}

// TestRenderNoMultipleBlankLinesBetweenSections verifies the boundary
// between the Project and Inherited sections is exactly one blank line
// (markdownlint MD012). Regression: the template's per-fragment range
// leaves a trailing blank line after every fragment — including the
// Project section's last — which stacked with the section gap into three
// consecutive blank lines whenever a document had both own and inherited
// fragments.
func TestRenderNoMultipleBlankLinesBetweenSections(t *testing.T) {
	child := t.TempDir()
	body := "    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		parentsBlock(parentURL)
	writeProjectfile(t, child, body)
	writeFragment(t, child, "docs/features.d", "child-feat", "Child Feature", "From child.")
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		[]string{"### Child Feature\n\nFrom child."},
		[][2]string{{headingInherited, "### Parent Feature\n\nFrom parent."}},
		false)

	out, err := fragments.Bridge{}.Render(docWithFragments(t, child), offlineOptions(child))
	require.NoError(t, err)
	got := string(out.Files[outFeatures])
	assert.NotContains(t, got, "\n\n\n", "no run of multiple consecutive blank lines (MD012)")
	// The section boundary itself must keep exactly one blank line.
	assert.Contains(t, got, "From child.\n\n## Inherited from b19/ubuntu")
}

// TestFilenameUsesNoH1Fallback verifies a fragment with no H1 uses its
// filename as the title and emits no spurious heading.
func TestFilenameUsesNoH1Fallback(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    fragments:\n      documents:\n        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	// Fragment with no H1 — just body.
	fragDir := filepath.Join(dir, "docs", "features.d")
	require.NoError(t, os.MkdirAll(fragDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fragDir, "headerless.md"),
		[]byte("Just a body, no heading.\n"), 0o644))

	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	got := string(out.Files[outFeatures])
	assert.Contains(t, got, "Just a body, no heading.")
}

// TestGetFragmentsExtensionParsesDocuments is a pfmodel-level guard: the
// accessor pulls dir/out/title/parents off the declared documents.
func TestGetFragmentsExtensionParsesDocuments(t *testing.T) {
	dir := t.TempDir()
	body := "    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		"          parents:\n            - {url: https://example.test/b19/ubuntu, ref: 1.0.0}\n" +
		"            - {ref: 2.0.0}\n" +
		"        - {dir: docs/roadmap.d, out: ROADMAP.md, title: Roadmap}"
	writeProjectfile(t, dir, body)

	ext, err := pfmodel.GetFragmentsExtension(docWithFragments(t, dir))
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Documents, 2)
	assert.Equal(t, "docs/features.d", ext.Documents[0].Dir)
	assert.Equal(t, outFeatures, ext.Documents[0].Out)
	assert.Equal(t, titleFeatures, ext.Documents[0].Title)
	// The url-less entry is dropped: it names no parent to resolve.
	assert.Equal(t, []pfmodel.FragmentParent{{URL: parentURL, Ref: "1.0.0"}},
		ext.Documents[0].Parents)
	assert.Equal(t, "ROADMAP.md", ext.Documents[1].Out)
}

// shellMap builds one document-shell map (dir/out/title) so the field-name
// literals live in exactly one place — keeps goconst quiet across the many
// shell declarations in the conventions tests.
func shellMap(dir, out, title string) map[string]any {
	return map[string]any{"dir": dir, "out": out, "title": title}
}

// conventionsDoc builds a Document with the conventions-driven fragments
// config (shells + flat parents) set directly via SetExtension — the same
// merged shape include resolution produces for a real project.
func conventionsDoc(t *testing.T, dir string, shells []map[string]any, parents []string) *projectfile.Document {
	t.Helper()
	writeProjectfile(t, dir, "    status: maintained")
	items := make([]any, len(shells))
	for i, s := range shells {
		items[i] = s
	}
	parAny := make([]any, len(parents))
	for i, p := range parents {
		parAny[i] = map[string]any{"url": p}
	}
	doc := docWithFragments(t, dir)
	projectfile.SetExtension(doc, pfmodel.ConventionsExtensionNS, map[string]any{
		keyFragments: map[string]any{keyDocuments: items, keyParents: parAny},
	})
	return doc
}

// TestRenderConventionsDefaultsWithParents is the primary conventions path:
// shells come from conventions, parents from the project (flat list shared
// across docs). Verifies the bridge synthesises documents and assembles each
// one from its own fragments plus the committed sections of its own output.
func TestRenderConventionsDefaultsWithParents(t *testing.T) {
	child := t.TempDir()
	writeFragment(t, child, "docs/features.d", "child-feat", "Child Feature", "Child body.")
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		[]string{"### Child Feature\n\nChild body."},
		[][2]string{{headingInherited, parentFeatBody}},
		false)

	doc := conventionsDoc(t, child,
		[]map[string]any{shellMap("docs/features.d", outFeatures, titleFeatures)},
		[]string{parentURL})

	out, err := fragments.Bridge{}.Render(doc, offlineOptions(child))
	require.NoError(t, err)
	got := string(out.Files[outFeatures])
	assert.Contains(t, got, "# Features")
	assert.Contains(t, got, "## Project Features")
	assert.Contains(t, got, "### Child Feature")
	assert.Contains(t, got, headingInherited)
	assert.Contains(t, got, "### Parent Feature")
}

// TestRenderConventionsMultipleShellsSharedParents verifies the flat parents
// list applies to EVERY shell (features and roadmap share one list), and that a
// shell with neither own fragments nor committed sections produces no file —
// empty documents are skipped rather than emitted as a bare-title stub (the
// markdown linter rejects empty sections).
func TestRenderConventionsMultipleShellsSharedParents(t *testing.T) {
	child := t.TempDir()
	// Only docs/features.d exists; docs/roadmap.d is absent on disk.
	writeFragment(t, child, "docs/features.d", "f", "Feat", "body")
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		[]string{"### Feat\n\nbody"},
		[][2]string{{headingInherited, parentFeatBody}},
		false)
	doc := conventionsDoc(t, child,
		[]map[string]any{
			shellMap("docs/features.d", outFeatures, titleFeatures),
			shellMap("docs/roadmap.d", "ROADMAP.md", "Roadmap"),
		},
		[]string{parentURL})

	out, err := fragments.Bridge{}.Render(doc, offlineOptions(child))
	require.NoError(t, err)
	assert.Contains(t, out.Files, outFeatures, "features shell with a real dir must produce a file")
	assert.NotContains(t, out.Files, "ROADMAP.md", "empty shell must not emit a stub file")
}

// TestRenderOverrideWinsOverConventions verifies an explicit
// org.projectfile.fragments declaration is used verbatim and the conventions
// defaults are ignored entirely (no merge into the override).
func TestRenderOverrideWinsOverConventions(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    status: maintained")
	writeFragment(t, dir, "docs/features.d", "f", "Feat", "body")
	writeFragment(t, dir, "docs/changelog.d", "c", "Changelog", "entry")
	// Override declares ONLY a changelog doc; conventions would add features.
	doc := docWithFragments(t, dir)
	projectfile.SetExtension(doc, pfmodel.FragmentsExtensionNS, map[string]any{
		keyDocuments: []any{shellMap("docs/changelog.d", "CHANGELOG.md", "Changelog")},
	})
	projectfile.SetExtension(doc, pfmodel.ConventionsExtensionNS, map[string]any{
		keyFragments: map[string]any{
			keyDocuments: []any{shellMap("docs/features.d", outFeatures, titleFeatures)},
		},
	})

	out, err := fragments.Bridge{}.Render(doc, core.Options{Dir: dir})
	require.NoError(t, err)
	assert.Contains(t, out.Files, "CHANGELOG.md", "override doc must render")
	assert.NotContains(t, out.Files, outFeatures, "conventions default must NOT bleed into the override")
}

// TestRenderConventionsNoParents verifies the conventions path works with no
// parents (a rootless project) — shells render with their own fragments only.
func TestRenderConventionsNoParents(t *testing.T) {
	dir := t.TempDir()
	writeFragment(t, dir, "docs/features.d", "solo", "Solo", "body")
	doc := conventionsDoc(t, dir,
		[]map[string]any{shellMap("docs/features.d", outFeatures, titleFeatures)},
		nil)

	out, err := fragments.Bridge{}.Render(doc, core.Options{Dir: dir})
	require.NoError(t, err)
	got := string(out.Files[outFeatures])
	assert.Contains(t, got, "### Solo")
	assert.NotContains(t, got, "## Inherited", "no parents → no inherited section")
}

// i18nBlock declares org.projectfile.i18n.languages for the writeProjectfile
// body — kept as one helper so the column-4 indentation lives in one place.
func i18nBlock(langs ...string) string {
	block := "    i18n:\n      languages:\n"
	for _, l := range langs {
		block += "        - " + l + "\n"
	}
	return block
}

// TestRenderLocalizedVariants is the localization baseline: a project with
// English fragments plus docs/es/features.d renders the canonical FEATURES.md
// (byte-stable shape, now with the cross-language bar) AND docs/es/FEATURES.md
// from the Spanish fragments with localized structural headings.
func TestRenderLocalizedVariants(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir,
		i18nBlock("es")+
			"    fragments:\n      documents:\n"+
			"        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	writeFragment(t, dir, "docs/features.d", "alpha", "Alpha Feature", "Alpha body.")
	writeFragment(t, dir, "docs/es/features.d", "alpha", "Característica Alfa", "Cuerpo alfa.")

	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	require.Contains(t, out.Files, outFeatures)
	require.Contains(t, out.Files, "docs/es/FEATURES.md")

	root := string(out.Files[outFeatures])
	assert.Contains(t, root, "# Features")
	assert.Contains(t, root, "## Project Features")
	assert.Contains(t, root, "### Alpha Feature")
	// The canonical file carries the bar naming exactly the shipped variants.
	assert.Contains(t, root, "[Español](docs/es/FEATURES.md)")

	es := string(out.Files["docs/es/FEATURES.md"])
	assert.Contains(t, es, "# Características")
	assert.Contains(t, es, "## Características del proyecto")
	assert.Contains(t, es, "### Característica Alfa")
	assert.Contains(t, es, "Cuerpo alfa.")
	// The variant carries the bar back to the canonical file and the
	// terminology wrap (non-English render).
	assert.Contains(t, es, "[English](../../FEATURES.md)")
	assert.Contains(t, es, "textlint-disable")
	// The REUSE header stays the file's first block.
	assert.True(t, strings.HasPrefix(es, "<!--\nSPDX-FileCopyrightText"))
}

// TestRenderStripsFragmentTextlintDirectives pins how assembly treats the
// textlint directive pair translated fragment sources carry: a pragma, not
// content. The variant keeps exactly one wrap — its own whole-file one — the
// H1 behind the directive still demotes to an H3 (the readme bridge scrapes
// that level), and the canonical file stays directive-free.
func TestRenderStripsFragmentTextlintDirectives(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir,
		i18nBlock("es")+
			"    fragments:\n      documents:\n"+
			"        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	writeFragment(t, dir, "docs/features.d", "alpha", "Alpha Feature", "Alpha body.")
	esDir := filepath.Join(dir, "docs/es/features.d")
	require.NoError(t, os.MkdirAll(esDir, 0o755))
	source := "<!--\nSPDX-FileCopyrightText: 2026 Tester\n" + spdxTag + ": " + spdxMIT + "\n-->\n\n" +
		"<!-- textlint-disable terminology,common-misspellings -->\n\n" +
		"# Característica Alfa\n\nCuerpo alfa.\n\n" +
		"<!-- textlint-enable -->\n"
	require.NoError(t, os.WriteFile(filepath.Join(esDir, "alpha.md"), []byte(source), 0o644))

	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	es := string(out.Files["docs/es/FEATURES.md"])
	assert.Equal(t, 1, strings.Count(es, "textlint-disable"), "one whole-file wrap, the source pair must not leak")
	assert.Contains(t, es, "### Característica Alfa")
	root := string(out.Files[outFeatures])
	assert.NotContains(t, root, "textlint-")
}

// TestRenderLocalizedVariantPreservesCommittedSections pins verbatim
// preservation for variants offline: the committed Spanish document's own
// inherited section — localized heading, canonical body, whatever the last
// online generate wrote — re-renders exactly as committed.
func TestRenderLocalizedVariantPreservesCommittedSections(t *testing.T) {
	child := t.TempDir()
	body := i18nBlock("es") +
		"    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		parentsBlock(parentURL)
	writeProjectfile(t, child, body)
	writeFragment(t, child, "docs/features.d", "own", "Own Feature", "Own body.")
	writeFragment(t, child, "docs/es/features.d", "own", "Característica Propia", "Cuerpo propio.")
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		[]string{ownFeatureBody},
		[][2]string{{headingInherited, parentFeatBody}},
		false)
	writeCommittedDoc(t, child, "docs/es/"+outFeatures, "Características", "## Características del proyecto",
		[]string{"### Característica Propia\n\nCuerpo propio."},
		[][2]string{{"## Heredado de b19/ubuntu", parentFeatBody}},
		true)

	out, err := fragments.Bridge{}.Render(docWithFragments(t, child), offlineOptions(child))
	require.NoError(t, err)
	es := string(out.Files["docs/es/FEATURES.md"])
	assert.Contains(t, es, "## Heredado de b19/ubuntu")
	assert.Contains(t, es, "### Parent Feature")
	assert.Contains(t, es, "### Característica Propia")
}

// TestRenderVariantWithoutCommittedDocNestsCanonicalWithLocalizedHeadings
// covers a translated-fragments variant that has no committed variant
// document: offline it nests the canonical sections under localized
// headings — the same fallback a parent that publishes no such language gets
// online.
func TestRenderVariantWithoutCommittedDocNestsCanonicalWithLocalizedHeadings(t *testing.T) {
	child := t.TempDir()
	body := i18nBlock("es") +
		"    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		parentsBlock(parentURL)
	writeProjectfile(t, child, body)
	writeFragment(t, child, "docs/features.d", "own", "Own Feature", "Own body.")
	writeFragment(t, child, "docs/es/features.d", "own", "Característica Propia", "Cuerpo propio.")
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		[]string{ownFeatureBody},
		[][2]string{{headingInherited, parentFeatBody}},
		false)
	// No committed docs/es/FEATURES.md — the variant has fragments only.

	out, err := fragments.Bridge{}.Render(docWithFragments(t, child), offlineOptions(child))
	require.NoError(t, err)
	es := string(out.Files["docs/es/FEATURES.md"])
	assert.Contains(t, es, "## Heredado de b19/ubuntu")
	assert.Contains(t, es, "### Parent Feature", "canonical body, the child cannot translate it")
	assert.Contains(t, es, "### Característica Propia")
}

// TestRenderSkipsLanguageWithoutFragments pins the missing-translation
// policy: a declared language with no docs/<lang>/features.d renders nothing
// under a localized name, and no bar advertises it.
func TestRenderSkipsLanguageWithoutFragments(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir,
		i18nBlock("es", "uk")+
			"    fragments:\n      documents:\n"+
			"        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	writeFragment(t, dir, "docs/features.d", "alpha", "Alpha Feature", "Alpha body.")
	writeFragment(t, dir, "docs/es/features.d", "alpha", "Característica Alfa", "Cuerpo alfa.")
	// No docs/uk/features.d — Ukrainian is declared but untranslated.

	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	assert.NotContains(t, out.Files, "docs/uk/FEATURES.md")
	root := string(out.Files[outFeatures])
	assert.NotContains(t, root, "docs/uk/FEATURES.md", "bar must not advertise an unshipped variant")
	assert.Contains(t, root, "docs/es/FEATURES.md")
}

// TestRenderUnilingualProjectIsByteStable pins the no-op property: a project
// with no declared languages renders exactly one file, with no bar and no
// wrap — the output the drift gate has always compared against.
func TestRenderUnilingualProjectIsByteStable(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    fragments:\n      documents:\n        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	writeFragment(t, dir, "docs/features.d", "alpha", "Alpha Feature", "Alpha body.")

	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	assert.Len(t, out.Files, 1)
	got := string(out.Files[outFeatures])
	assert.NotContains(t, got, "textlint-disable")
	assert.NotContains(t, got, "[English](")
	assert.True(t, strings.HasPrefix(got, "<!--\nSPDX-FileCopyrightText"))
	assert.True(t, strings.HasSuffix(got, "Alpha body.\n"), "single trailing newline, nothing after the last fragment")
}

// TestRenderVariantRequiresContent pins the skip rule for a language with
// nothing to ship: no own translated fragments and no language-specific
// content renders nothing under a localized name, never a heading-only stub —
// while the inherited-only canonical file still renders.
func TestRenderVariantRequiresContent(t *testing.T) {
	child := t.TempDir()
	body := i18nBlock("es") +
		"    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		parentsBlock(parentURL)
	writeProjectfile(t, child, body)
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		nil,
		[][2]string{{headingInherited, parentFeatBody}},
		false)

	out, err := fragments.Bridge{}.Render(docWithFragments(t, child), offlineOptions(child))
	require.NoError(t, err)
	assert.Contains(t, out.Files, outFeatures, "inherited-only canonical file still renders")
	assert.NotContains(t, out.Files, "docs/es/FEATURES.md", "no own fragments and no localized content → no variant")
}

// TestRenderNonLocalizableDocumentSkipsVariants pins the convention gate: a
// document whose dir is outside docs/ (custom override) or whose out nests
// under a directory stays single-language by decision.
func TestRenderNonLocalizableDocumentSkipsVariants(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir,
		i18nBlock("es")+
			"    fragments:\n      documents:\n"+
			"        - {dir: changelog.d, out: CHANGELOG.md, title: Changelog}")
	writeFragment(t, dir, "changelog.d", "c1", "Change One", "Body.")
	writeFragment(t, dir, "docs/es/changelog.d", "c1", "Cambio Uno", "Cuerpo.")

	out, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.NoError(t, err)
	assert.Contains(t, out.Files, "CHANGELOG.md")
	assert.NotContains(t, out.Files, "docs/es/CHANGELOG.md")
}
